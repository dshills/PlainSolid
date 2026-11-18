//go:build research

package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"plainsolid/mesh"
	"sort"
)

// MixedRoofCollarOptions bounds a proof-only dyadic subdivision; it does not
// move or subdivide shared display edges. Width is a physical XY distance to a
// represented boundary chord. SourceTrimSquaredDistance adds that chord's
// independent original curve error and the original graph Lipschitz bound.
type MixedRoofCollarOptions struct {
	Roof                                        MixedRoofInteriorOptions
	Width                                       float64
	MaxDepth, MaxCells, MaxPairs, MaxCandidates int
}

// MixedRoofClearanceCell covers part of a displayed triangle's exact original
// UV domain. Positive MaterialClearance proves the indicated material constraint.
// A zero MaterialClearance instead means NativeEdge/Chord identify an independently certified ORIGINAL trim occurrence
// whose projected display chord is near the whole cell. This is an explicit
// collar, not a claim that the chord or cell lies inside the exact source trim.
// VerticalClearance retains the intersection/inside-other Z margin only; it is
// zero for complementary witnesses even when MaterialClearance is positive.
type MixedRoofClearanceCell struct {
	// MaterialClearance is a positive signed witness; Constraint identifies its units.
	MaterialClearance                               *big.Rat
	Constraint                                      MixedRoofMaterialConstraint
	Triangle                                        int
	UV                                              [3][2]*big.Rat
	VerticalClearance                               *big.Rat
	NativeEdge, Chord                               int
	CurveKind                                       MixedBoundaryCurveKind
	ChordSquaredDistance, SourceTrimSquaredDistance *big.Rat
}

// CertifiedMixedRoofFragment proves an entire original roof fragment's image
// approximation, including explicitly certified trim collars. No shell closure
// or nonintersection with other face interiors is asserted. The source's exact
// algebraic trim incidence remains in the input network; it is never replaced
// by signed containment of rounded chords.
type CertifiedMixedRoofFragment struct {
	base        *CertifiedMixedRoofInterior
	cells       []MixedRoofClearanceCell
	collarPairs int
}

func (r *CertifiedMixedRoofFragment) Mesh() mesh.Mesh {
	if r == nil {
		return mesh.Mesh{}
	}
	return r.base.Mesh()
}
func (r *CertifiedMixedRoofFragment) ExactUV() [][2]*big.Rat {
	if r == nil {
		return nil
	}
	return r.base.ExactUV()
}
func (r *CertifiedMixedRoofFragment) OriginalFace() (int, int, int) {
	if r == nil {
		return -1, -1, -1
	}
	return r.base.OriginalFace()
}
func (r *CertifiedMixedRoofFragment) SourceImageSquaredDistance() *big.Rat {
	if r == nil {
		return nil
	}
	return new(big.Rat).Set(r.base.imageBound)
}
func (r *CertifiedMixedRoofFragment) ClearanceCells() []MixedRoofClearanceCell {
	if r == nil {
		return nil
	}
	out := make([]MixedRoofClearanceCell, len(r.cells))
	for i, c := range r.cells {
		out[i] = c
		out[i].MaterialClearance = new(big.Rat).Set(c.MaterialClearance)
		out[i].VerticalClearance = new(big.Rat).Set(c.VerticalClearance)
		for j := 0; j < 3; j++ {
			for k := 0; k < 2; k++ {
				out[i].UV[j][k] = new(big.Rat).Set(c.UV[j][k])
			}
		}
		if c.ChordSquaredDistance != nil {
			out[i].ChordSquaredDistance = new(big.Rat).Set(c.ChordSquaredDistance)
			out[i].SourceTrimSquaredDistance = new(big.Rat).Set(c.SourceTrimSquaredDistance)
		}
	}
	return out
}
func (r *CertifiedMixedRoofFragment) Work() (operations, triangleChecks, contactPairs, collarPairs int) {
	if r == nil {
		return
	}
	operations, triangleChecks, contactPairs = r.base.Work()
	return operations, triangleChecks, contactPairs, r.collarPairs
}
func (r *CertifiedMixedRoofFragment) MarshalJSON() ([]byte, error) {
	return nil, geom.ErrUnrepresentable
}

// TessellateRoofFragmentCertified covers one retained intersection roof with
// exhaustive strict-material or original-trim collar cells. Unproved cells
// reject the entire fragment; the existing strict-interior API is unchanged.
func (n *CertifiedMixedAlgebraicShellBoundary) TessellateRoofFragmentCertified(ctx context.Context, face, oppositeRoof int, o MixedRoofCollarOptions) (*CertifiedMixedRoofFragment, error) {
	return n.tessellateRoofFragment(ctx, face, oppositeRoof, o, false, false)
}
func (n *CertifiedMixedAlgebraicShellBoundary) tessellateRoofFragment(ctx context.Context, face, oppositeRoof int, o MixedRoofCollarOptions, difference, union bool) (*CertifiedMixedRoofFragment, error) {
	if !finite(o.Width) || o.Width <= 0 {
		return nil, geom.ErrInvalid
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 10
	}
	if o.MaxCells == 0 {
		o.MaxCells = 32768
	}
	if o.MaxPairs == 0 {
		o.MaxPairs = 131072
	}
	if o.MaxCandidates == 0 {
		o.MaxCandidates = 8
	}
	if o.MaxDepth < 1 || o.MaxDepth > 24 || o.MaxCells < 1 || o.MaxCells > 1000000 || o.MaxPairs < 1 || o.MaxPairs > 8000000 || o.MaxCandidates < 1 || o.MaxCandidates > 128 {
		return nil, geom.ErrInvalid
	}
	work := &mixedRoofCollarWork{difference: difference, union: union, options: o, width2: new(big.Rat).Mul(ratInset(o.Width), ratInset(o.Width))}
	base, e := n.tessellateMixedRoof(ctx, face, oppositeRoof, o.Roof, work)
	if e != nil {
		return nil, e
	}
	return &CertifiedMixedRoofFragment{base: base, cells: work.cells, collarPairs: work.pairs}, nil
}

type mixedRoofCollarWork struct {
	difference     bool
	union          bool
	outside        bool
	options        MixedRoofCollarOptions
	width2         *big.Rat
	cells          []MixedRoofClearanceCell
	visited, pairs int
}

func mixedRoofTriangleClearance(host, other *mixedRoofBox, xy [3][2]*big.Rat) (*big.Rat, error) {
	for _, box := range []*mixedRoofBox{host, other} {
		d := box.domain
		for _, p := range xy {
			if p[0].Cmp(ratInset(d.UMin)) <= 0 || p[0].Cmp(ratInset(d.UMax)) >= 0 || p[1].Cmp(ratInset(d.VMin)) <= 0 || p[1].Cmp(ratInset(d.VMax)) >= 0 {
				return new(big.Rat), nil
			}
		}
	}
	a, e := mixedRoofTrianglePower(host.roof, xy)
	if e != nil {
		return nil, e
	}
	b, e := mixedRoofTrianglePower(other.roof, xy)
	if e != nil {
		return nil, e
	}
	height, _ := mixedRoofPolynomialRange(a)
	height.Sub(height, host.floor)
	otherFloor, _ := mixedRoofPolynomialRange(a)
	otherFloor.Sub(otherFloor, other.floor)
	for i := 0; i <= 5; i++ {
		for j := 0; j <= 5-i; j++ {
			b[i][j].Sub(b[i][j], a[i][j])
		}
	}
	gap, _ := mixedRoofPolynomialRange(b)
	if height.Sign() <= 0 || otherFloor.Sign() <= 0 || gap.Sign() <= 0 {
		return new(big.Rat), nil
	}
	for _, x := range []*big.Rat{height, otherFloor} {
		if x.Cmp(gap) < 0 {
			gap.Set(x)
		}
	}
	return gap, nil
}
func roofPointSegmentDistance(p, a, b [2]*big.Rat) *big.Rat {
	dx, dy := new(big.Rat).Sub(b[0], a[0]), new(big.Rat).Sub(b[1], a[1])
	x, y := new(big.Rat).Sub(p[0], a[0]), new(big.Rat).Sub(p[1], a[1])
	den := new(big.Rat).Add(new(big.Rat).Mul(dx, dx), new(big.Rat).Mul(dy, dy))
	t := new(big.Rat)
	if den.Sign() > 0 {
		t.Quo(new(big.Rat).Add(new(big.Rat).Mul(x, dx), new(big.Rat).Mul(y, dy)), den)
		if t.Sign() < 0 {
			t.SetInt64(0)
		}
		if t.Cmp(big.NewRat(1, 1)) > 0 {
			t.SetInt64(1)
		}
	}
	x.Sub(x, new(big.Rat).Mul(t, dx))
	y.Sub(y, new(big.Rat).Mul(t, dy))
	return new(big.Rat).Add(new(big.Rat).Mul(x, x), new(big.Rat).Mul(y, y))
}
func roofUVArea(q [3][2]*big.Rat) *big.Rat {
	return new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(new(big.Rat).Sub(q[1][0], q[0][0]), new(big.Rat).Sub(q[2][1], q[0][1])), new(big.Rat).Mul(new(big.Rat).Sub(q[1][1], q[0][1]), new(big.Rat).Sub(q[2][0], q[0][0]))), big.NewRat(2, 1))
}
func (w *mixedRoofCollarWork) certifyTriangle(ctx context.Context, triangle int, root [3][2]*big.Rat, host, other *mixedRoofBox, n *CertifiedMixedAlgebraicShellBoundary, ring []int, face int) error {
	type chord struct {
		edge, segment  int
		a, b           [2]*big.Rat
		ax, ay, bx, by float64
	}
	var chords []chord
	f := n.source.boundary.faces[face]
	uses := append([]AlgebraicShellBooleanUse(nil), f.Outer...)
	for _, hole := range f.Holes {
		uses = append(uses, hole...)
	}
	for _, use := range uses {
		path := n.network.Edges[use.Edge]
		for j := 1; j < len(path); j++ {
			a, b := n.network.Vertices[path[j-1]], n.network.Vertices[path[j]]
			chords = append(chords, chord{use.Edge, j - 1, [2]*big.Rat{ratInset(a.X), ratInset(a.Y)}, [2]*big.Rat{ratInset(b.X), ratInset(b.Y)}, a.X, a.Y, b.X, b.Y})
		}
	}
	lip, _, ok := quadraticPatchLipschitz(host.roof)
	if !ok {
		return geom.ErrConvergence
	}
	sum := new(big.Rat)
	var visit func([3][2]*big.Rat, int) error
	visit = func(q [3][2]*big.Rat, depth int) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		w.visited++
		if w.visited > w.options.MaxCells {
			return geom.ErrConvergence
		}
		margin, kind, e := mixedRoofMaterialClearanceMode(host, other, q, w.outside, w.union)
		if e != nil {
			return e
		}
		vertical := new(big.Rat)
		if kind == MixedRoofInsideOther {
			vertical.Set(margin)
		}
		cell := MixedRoofClearanceCell{Triangle: triangle, UV: q, VerticalClearance: vertical, MaterialClearance: margin, Constraint: kind, NativeEdge: -1, Chord: -1}
		if margin.Sign() > 0 {
			w.cells = append(w.cells, cell)
			sum.Add(sum, roofUVArea(q))
			return nil
		}
		// Floating distance only orders proposals. Every acceptance below is rational.
		cx, cy := 0., 0.
		for _, p := range q {
			x, _ := p[0].Float64()
			y, _ := p[1].Float64()
			cx += x / 3
			cy += y / 3
		}
		type candidate struct {
			id       int
			distance float64
		}
		order := make([]candidate, len(chords))
		for i, c := range chords {
			dx, dy := c.bx-c.ax, c.by-c.ay
			den := dx*dx + dy*dy
			t := 0.
			if den > 0 {
				t = math.Max(0, math.Min(1, ((cx-c.ax)*dx+(cy-c.ay)*dy)/den))
			}
			x, y := cx-c.ax-t*dx, cy-c.ay-t*dy
			order[i] = candidate{i, x*x + y*y}
		}
		sort.SliceStable(order, func(i, j int) bool { return order[i].distance < order[j].distance })
		for k, candidate := range order {
			if k >= w.options.MaxCandidates {
				break
			}
			w.pairs++
			if w.pairs > w.options.MaxPairs {
				return geom.ErrConvergence
			}
			c := chords[candidate.id]
			maximum := new(big.Rat)
			for _, p := range q {
				d := roofPointSegmentDistance(p, c.a, c.b)
				if d.Cmp(maximum) > 0 {
					maximum.Set(d)
				}
			}
			if maximum.Cmp(w.width2) > 0 {
				continue
			}
			cert, e := n.EdgeCertificate(c.edge)
			if e != nil {
				return e
			}
			delta := ratInset(cert.ErrorUpper)
			delta.Mul(delta, delta)
			cell.NativeEdge, cell.Chord, cell.CurveKind = c.edge, c.segment, cert.Kind
			cell.ChordSquaredDistance = maximum
			cell.SourceTrimSquaredDistance = new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(lip, new(big.Rat).Add(maximum, delta)))
			w.cells = append(w.cells, cell)
			sum.Add(sum, roofUVArea(q))
			return nil
		}
		if depth == w.options.MaxDepth {
			return geom.ErrConvergence
		}
		var mid [3][2]*big.Rat
		for j := 0; j < 3; j++ {
			for k := 0; k < 2; k++ {
				mid[j][k] = new(big.Rat).Quo(new(big.Rat).Add(q[j][k], q[(j+1)%3][k]), big.NewRat(2, 1))
			}
		}
		for _, child := range [][3][2]*big.Rat{{q[0], mid[0], mid[2]}, {mid[0], q[1], mid[1]}, {mid[2], mid[1], q[2]}, {mid[0], mid[1], mid[2]}} {
			if e := visit(child, depth+1); e != nil {
				return e
			}
		}
		return nil
	}
	if e := visit(root, 0); e != nil {
		return e
	}
	if sum.Cmp(roofUVArea(root)) != 0 {
		return geom.ErrInvalid
	}
	return nil
}
