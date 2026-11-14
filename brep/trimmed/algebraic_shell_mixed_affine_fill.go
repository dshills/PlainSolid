package trimmed

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// MixedAffineFragmentFillOptions bounds additional work after boundary creation.
// Zero values select 1024 ring vertices, 8192 triangles, 8M exact chart
// operations, 1M predicates and 2M combined triangle/segment and triangle
// broadphase pairs. Coefficient bits are
// capped at 4096 before the fixed-size exact polygon/contact arithmetic.
type MixedAffineFragmentFillOptions struct {
	MaxVertices, MaxTriangles, MaxOperations, MaxPredicates, MaxPairs int
}

// MixedAffineFragment is an original-chart transport of an unchanged network
// ring. UV is the exact affine projection inverse, one-to-one with
// BoundaryVertices. PlaneResidual is the signed displacement along ResidualAxis
// from that original affine chart to the unchanged represented vertex. AreaUV is
// the exact area of the represented polygon, not of its curved source boundary.
type MixedAffineFragment struct {
	Face, Operand, Original int
	BoundaryVertices        []int
	UV                      [][2]*big.Rat
	PlaneResidual           []*big.Rat
	ResidualAxis            int
	ErrorUpper              *big.Rat
	AreaUV                  *big.Rat
	Triangles               [][3]int
	projectionAxes          [2]int
	projectionReversed      bool
}

// CertifiedMixedAffineFragmentFill is a partial open display surface. It does
// not fill nonlinear roofs, assert a closed mesh, or bound distance between
// unfilled face interiors and its triangles. All network chords are checked
// against every filled triangle, and all filled triangles against each other.
//
// Each selected source is an affine chart with a hole-free original fragment.
// Its pointwise boundary chord proofs, followed by coordinate-plane projection,
// give a planar straight-line homotopy of size boundary error plus maximum
// nodal projection displacement. Triangles are affine graphs over that plane
// with displacement bounded by their nodal values. Outside that homotopy the
// two oriented Jordan loops have equal winding. Their filled sets therefore differ only inside the homotopy,
// proving both directed filled-set distances <= Deflection (boundary error plus
// twice the maximum exact nodal projection displacement, rounded upward).
// No source boundary is replaced by an unproved polygon for material classification.
type CertifiedMixedAffineFragmentFill struct {
	source                        *CertifiedMixedAlgebraicShellBoundary
	geometry                      mesh.Mesh
	fragments                     []MixedAffineFragment
	unfilled                      []int
	operations, predicates, pairs int
	deflection                    float64
}

func (r *CertifiedMixedAffineFragmentFill) Mesh() mesh.Mesh {
	if r == nil {
		return mesh.Mesh{}
	}
	return mesh.Mesh{Vertices: append([]geom.Vec3(nil), r.geometry.Vertices...), Triangles: append([][3]int(nil), r.geometry.Triangles...)}
}
func (r *CertifiedMixedAffineFragmentFill) Fragments() []MixedAffineFragment {
	if r == nil {
		return nil
	}
	out := make([]MixedAffineFragment, len(r.fragments))
	for i, f := range r.fragments {
		out[i] = f
		out[i].BoundaryVertices = append([]int(nil), f.BoundaryVertices...)
		out[i].Triangles = append([][3]int(nil), f.Triangles...)
		out[i].AreaUV = new(big.Rat).Set(f.AreaUV)
		out[i].ErrorUpper = new(big.Rat).Set(f.ErrorUpper)
		out[i].PlaneResidual = make([]*big.Rat, len(f.PlaneResidual))
		for j, x := range f.PlaneResidual {
			out[i].PlaneResidual[j] = new(big.Rat).Set(x)
		}
		out[i].UV = make([][2]*big.Rat, len(f.UV))
		for j, p := range f.UV {
			out[i].UV[j] = [2]*big.Rat{new(big.Rat).Set(p[0]), new(big.Rat).Set(p[1])}
		}
	}
	return out
}
func (r *CertifiedMixedAffineFragmentFill) UnfilledFaces() []int {
	if r == nil {
		return nil
	}
	return append([]int(nil), r.unfilled...)
}
func (r *CertifiedMixedAffineFragmentFill) Deflection() float64 {
	if r == nil {
		return 0
	}
	return r.deflection
}
func (r *CertifiedMixedAffineFragmentFill) Work() (operations, predicates, pairs int) {
	if r == nil {
		return
	}
	return r.operations, r.predicates, r.pairs
}
func (r *CertifiedMixedAffineFragmentFill) MarshalJSON() ([]byte, error) {
	return nil, geom.ErrUnrepresentable
}

// FillAffineFragmentsCertified fills only explicitly selected original affine
// fragments. All selected faces must qualify; unsupported charts, holes,
// projected vertices outside the original rectangle and exhausted budgets return no partial result.
func (n *CertifiedMixedAlgebraicShellBoundary) FillAffineFragmentsCertified(ctx context.Context, faces []int, o MixedAffineFragmentFillOptions) (*CertifiedMixedAffineFragmentFill, error) {
	return n.fillMixedPlanarFragments(ctx, faces, o, nil)
}

func (n *CertifiedMixedAlgebraicShellBoundary) fillMixedPlanarFragments(ctx context.Context, faces []int, o MixedAffineFragmentFillOptions, planes map[int]MixedPlanarFragmentProposal) (*CertifiedMixedAffineFragmentFill, error) {
	if ctx == nil || n == nil || n.source == nil || !n.embedded || len(faces) == 0 {
		return nil, geom.ErrInvalid
	}
	if o.MaxVertices == 0 {
		o.MaxVertices = 1024
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 8192
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxPredicates == 0 {
		o.MaxPredicates = 1000000
	}
	if o.MaxPairs == 0 {
		o.MaxPairs = 2000000
	}
	if o.MaxVertices < 3 || o.MaxVertices > 4096 || o.MaxTriangles < 1 || o.MaxTriangles > 32768 || o.MaxOperations < 1 || o.MaxOperations > 128000000 || o.MaxPredicates < 1 || o.MaxPredicates > 100000000 || o.MaxPairs < 1 || o.MaxPairs > 16000000 {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	r := &CertifiedMixedAffineFragmentFill{source: n, deflection: n.deflection, geometry: mesh.Mesh{Vertices: n.Vertices()}}
	selected := map[int]bool{}
	originals := n.source.boundary.faces
	for _, id := range faces {
		if id < 0 || id >= len(originals) || selected[id] {
			return nil, geom.ErrInvalid
		}
		selected[id] = true
	}
	// Native face order makes output independent of caller selection order.
	for id, f := range originals {
		if !selected[id] {
			r.unfilled = append(r.unfilled, id)
			continue
		}
		if len(f.Holes) != 0 {
			return nil, geom.ErrUnrepresentable
		}
		remain := o.MaxOperations - r.operations
		if remain <= 0 {
			return nil, geom.ErrConvergence
		}
		original, e := n.source.OriginalFace(id)
		if e != nil {
			return nil, e
		}
		patch, e := exactFaceSupportPatch(original, geom.SurfacePatchOptions{MaxOperations: remain, MaxCoefficientBits: 4096})
		if e != nil {
			return nil, e
		}
		r.operations += patch.ExactOperations()
		remain = o.MaxOperations - r.operations
		if remain <= 0 {
			return nil, geom.ErrConvergence
		}
		var chart *geom.AffineSurfaceChart
		var physical *mixedPhysicalPlane
		if planes == nil {
			chart, e = geom.NewAffineSurfaceChart(ctx, patch, geom.AffineSurfaceChartOptions{MaxOperations: remain, MaxCoefficientBits: 4096})
			if e != nil {
				return nil, fmt.Errorf("affine fragment %d: %w", id, e)
			}
			r.operations += chart.ExactOperations()
		} else {
			physical, e = proveMixedPhysicalPlane(ctx, patch, planes[id], remain)
			if e != nil {
				return nil, fmt.Errorf("planar fragment %d: %w", id, e)
			}
			r.operations += physical.operations
		}
		ring := []int{}
		for _, u := range f.Outer {
			path := n.network.Edges[u.Edge]
			if len(path) < 2 {
				return nil, geom.ErrInvalid
			}
			for j := range path {
				k := j
				if u.Reversed {
					k = len(path) - 1 - j
				}
				v := path[k]
				if j == 0 && len(ring) > 0 {
					if ring[len(ring)-1] != v {
						return nil, geom.ErrInvalid
					}
					continue
				}
				ring = append(ring, v)
				if len(ring) > o.MaxVertices+1 {
					return nil, geom.ErrConvergence
				}
			}
		}
		if len(ring) < 4 || ring[0] != ring[len(ring)-1] {
			return nil, geom.ErrInvalid
		}
		ring = ring[:len(ring)-1]
		on := map[int]bool{}
		for _, v := range ring {
			on[v] = true
		}
		p := partitionFillProof{ctx: ctx, limit: o.MaxPredicates, work: r.predicates, points: make([][2]*big.Rat, len(r.geometry.Vertices))}
		residual := make([]*big.Rat, len(p.points))
		maxDisplacement := new(big.Rat)
		unused := 0
		if chart != nil {
			domain := chart.Domain()
			unused = 3 - chart.ProjectionAxes()[0] - chart.ProjectionAxes()[1]
			for j, v := range r.geometry.Vertices {
				remain = o.MaxOperations - r.operations
				if remain <= 64 {
					return nil, geom.ErrConvergence
				}
				xyz := [3]*big.Rat{ratInset(v.X), ratInset(v.Y), ratInset(v.Z)}
				inv, e := chart.InvertExactContext(ctx, xyz, geom.AffineChartInverseOptions{MaxOperations: remain - 64, MaxCoefficientBits: 4096})
				if e != nil {
					return nil, e
				}
				r.operations += inv.ExactOperations + 64
				if on[j] && (inv.U.Cmp(ratInset(domain.UMin)) < 0 || inv.U.Cmp(ratInset(domain.UMax)) > 0 || inv.V.Cmp(ratInset(domain.VMin)) < 0 || inv.V.Cmp(ratInset(domain.VMax)) > 0) {
					return nil, fmt.Errorf("fragment %d projected point %d outside chart: %w", id, j, geom.ErrUnrepresentable)
				}
				p.points[j] = [2]*big.Rat{inv.U, inv.V}
				plane, e := chart.EvalExact(inv.U, inv.V)
				if e != nil {
					return nil, e
				}
				residual[j] = new(big.Rat).Sub(xyz[unused], plane[unused])
				if on[j] {
					d := new(big.Rat).Abs(residual[j])
					if d.Cmp(maxDisplacement) > 0 {
						maxDisplacement.Set(d)
					}
				}
			}
		} else {
			unused = 3 - physical.axes[0] - physical.axes[1]
			for j, v := range r.geometry.Vertices {
				if e := ctx.Err(); e != nil {
					return nil, e
				}
				r.operations += 64
				if r.operations > o.MaxOperations {
					return nil, geom.ErrConvergence
				}
				xyz := [3]*big.Rat{ratInset(v.X), ratInset(v.Y), ratInset(v.Z)}
				p.points[j] = [2]*big.Rat{xyz[physical.axes[0]], xyz[physical.axes[1]]}
				value := new(big.Rat).Neg(physical.offset)
				for k, x := range xyz {
					value.Add(value, new(big.Rat).Mul(physical.normal[k], x))
				}
				residual[j] = value.Quo(value, physical.normal[unused])
				if residual[j].Num().BitLen() > 4096 || residual[j].Denom().BitLen() > 4096 {
					return nil, geom.ErrConvergence
				}
				if on[j] {
					d := new(big.Rat).Abs(residual[j])
					if d.Cmp(maxDisplacement) > 0 {
						maxDisplacement.Set(d)
					}
				}
			}
			if physical.orientation < 0 {
				for a, b := 0, len(ring)-1; a < b; a, b = a+1, b-1 {
					ring[a], ring[b] = ring[b], ring[a]
				}
			}
		}
		bound := new(big.Rat).Add(ratInset(n.deflection), new(big.Rat).Mul(big.NewRat(2, 1), maxDisplacement))
		upper, exact := bound.Float64()
		if math.IsInf(upper, 0) {
			return nil, geom.ErrUnrepresentable
		}
		if !exact && ratInset(upper).Cmp(bound) < 0 {
			upper = math.Nextafter(upper, math.Inf(1))
		}
		if math.IsInf(upper, 0) {
			return nil, geom.ErrUnrepresentable
		}
		if upper > r.deflection {
			r.deflection = upper
		}
		if e := p.simple(ring); e != nil {
			return nil, e
		}
		area := p.area(ring)
		if area.Sign() <= 0 {
			return nil, geom.ErrInvalid
		}
		tris, e := p.triangulate(ring)
		if e != nil {
			return nil, e
		}
		if len(r.geometry.Triangles)+len(tris) > o.MaxTriangles {
			return nil, geom.ErrConvergence
		}
		chain := map[[2]int]int{}
		sum := new(big.Rat)
		for _, t := range tris {
			a := p.area(t[:])
			if a.Sign() <= 0 {
				return nil, geom.ErrInvalid
			}
			sum.Add(sum, a)
			for j := 0; j < 3; j++ {
				partitionChainAdd(chain, t[j], t[(j+1)%3], 1)
			}
		}
		for j, a := range ring {
			partitionChainAdd(chain, a, ring[(j+1)%len(ring)], -1)
		}
		if len(chain) != 0 || sum.Cmp(area) != 0 {
			return nil, geom.ErrInvalid
		}
		for _, t := range tris {
			for _, edge := range n.network.Edges {
				for j := 1; j < len(edge); j++ {
					if e := ctx.Err(); e != nil {
						return nil, e
					}
					r.pairs++
					if r.pairs > o.MaxPairs {
						return nil, geom.ErrConvergence
					}
					if e := mixedAffineSegmentTriangle(&p, residual, t, edge[j-1], edge[j]); e != nil {
						return nil, fmt.Errorf("fragment %d boundary contact: %w", id, e)
					}
				}
			}
		}
		r.predicates = p.work
		fragment := MixedAffineFragment{Face: id, Operand: f.Operand, Original: f.Original, BoundaryVertices: ring, AreaUV: area, UV: make([][2]*big.Rat, len(ring)), PlaneResidual: make([]*big.Rat, len(ring)), ResidualAxis: unused, ErrorUpper: bound}
		if physical != nil {
			fragment.projectionAxes = physical.axes
			fragment.projectionReversed = physical.orientation < 0
		}
		for j, v := range ring {
			fragment.UV[j] = p.points[v]
			fragment.PlaneResidual[j] = residual[v]
		}
		for _, t := range tris {
			if f.Reversed != fragment.projectionReversed {
				t[1], t[2] = t[2], t[1]
			}
			fragment.Triangles = append(fragment.Triangles, t)
			r.geometry.Triangles = append(r.geometry.Triangles, t)
		}
		r.fragments = append(r.fragments, fragment)
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if r.pairs >= o.MaxPairs {
		return nil, geom.ErrConvergence
	}
	report, e := collision.SelfIntersections(r.geometry, collision.Options{MaxPairs: o.MaxPairs - r.pairs, MaxContacts: 1})
	r.pairs += report.BroadPhasePairs
	if e != nil {
		return nil, e
	}
	if !report.Complete || len(report.Contacts) != 0 {
		return nil, geom.ErrConvergence
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	return r, nil
}

// Exact closed segment/triangle intersection, admitting ONLY indexed incidence.
func mixedAffineSegmentTriangle(p *partitionFillProof, s []*big.Rat, t [3]int, a, b int) error {
	p.work++
	if p.work > p.limit {
		return geom.ErrConvergence
	}
	orient := func(i, j, k int) *big.Rat {
		x, y, z := p.points[i], p.points[j], p.points[k]
		return new(big.Rat).Sub(new(big.Rat).Mul(new(big.Rat).Sub(y[0], x[0]), new(big.Rat).Sub(z[1], x[1])), new(big.Rat).Mul(new(big.Rat).Sub(y[1], x[1]), new(big.Rat).Sub(z[0], x[0])))
	}
	denominator := orient(t[0], t[1], t[2])
	relative := func(k int) *big.Rat {
		v := new(big.Rat).Mul(s[k], denominator)
		for j := 0; j < 3; j++ {
			v.Sub(v, new(big.Rat).Mul(s[t[j]], orient(t[(j+1)%3], t[(j+2)%3], k)))
		}
		return v
	}
	sa, sb := relative(a), relative(b)
	if sa.Sign()*sb.Sign() > 0 {
		return nil
	}
	sharedA, sharedB := false, false
	for _, v := range t {
		sharedA = sharedA || v == a
		sharedB = sharedB || v == b
	}
	lo, hi := new(big.Rat), big.NewRat(1, 1)
	if sa.Sign() != 0 || sb.Sign() != 0 {
		x := new(big.Rat).Quo(sa, new(big.Rat).Sub(sa, sb))
		if x.Sign() < 0 || x.Cmp(hi) > 0 {
			return nil
		}
		lo.Set(x)
		hi.Set(x)
	}
	for j := 0; j < 3; j++ {
		x, y := orient(t[j], t[(j+1)%3], a), orient(t[j], t[(j+1)%3], b)
		d := new(big.Rat).Sub(y, x)
		if d.Sign() == 0 {
			if x.Sign() < 0 {
				return nil
			}
			continue
		}
		cut := new(big.Rat).Quo(new(big.Rat).Neg(x), d)
		if d.Sign() > 0 {
			if cut.Cmp(lo) > 0 {
				lo.Set(cut)
			}
		} else {
			if cut.Cmp(hi) < 0 {
				hi.Set(cut)
			}
		}
		if lo.Cmp(hi) > 0 {
			return nil
		}
	}
	if sharedA && sharedB {
		return nil
	}
	if lo.Cmp(hi) == 0 && (sharedA && lo.Sign() == 0 || sharedB && lo.Cmp(big.NewRat(1, 1)) == 0) {
		return nil
	}
	return geom.ErrInvalid
}
