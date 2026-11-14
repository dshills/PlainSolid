package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// MixedProjectedRegionOptions bounds the selected directed loop arrangement.
// Defaults: 2048 network vertices, 8192 triangles, 8M source operations and
// 2M exact predicates. At most 32 loops are admitted; no loop is simplified.
type MixedProjectedRegionOptions struct{ MaxVertices, MaxTriangles, MaxOperations, MaxPredicates int }

// CertifiedMixedProjectedRegion is a triangulation of a shared display boundary
// in a proved injective model-axis projection of its ORIGINAL support. It is
// not a 3D surface fill. Coordinates are exact represented model coordinates,
// never a guessed original UV inverse. Loop zero is CCW; all holes are CW.
// OriginalUses retain original native directions before projection reversal.
// The original projected region and display polygon differ only within the
// certified boundary homotopy. ProjectedFillSquaredDistance bounds both
// directed planar filled-set distances, including the holes.
type CertifiedMixedProjectedRegion struct {
	network                *CertifiedMixedAlgebraicShellBoundary
	face                   int
	axes                   [2]int
	orientation            int
	reversed               bool
	points                 [][2]*big.Rat
	loops                  [][]int
	uses                   [][]AlgebraicShellBooleanUse
	triangles              [][3]int
	area, bound            *big.Rat
	operations, predicates int
}

func (r *CertifiedMixedProjectedRegion) Face() int {
	if r == nil {
		return -1
	}
	return r.face
}
func (r *CertifiedMixedProjectedRegion) Projection() ([2]int, int, bool) {
	if r == nil {
		return [2]int{}, 0, false
	}
	return r.axes, r.orientation, r.reversed
}
func (r *CertifiedMixedProjectedRegion) Coordinates() [][2]*big.Rat {
	if r == nil {
		return nil
	}
	out := make([][2]*big.Rat, len(r.points))
	for i, p := range r.points {
		out[i] = [2]*big.Rat{new(big.Rat).Set(p[0]), new(big.Rat).Set(p[1])}
	}
	return out
}
func (r *CertifiedMixedProjectedRegion) Loops() [][]int {
	if r == nil {
		return nil
	}
	out := make([][]int, len(r.loops))
	for i, p := range r.loops {
		out[i] = append([]int(nil), p...)
	}
	return out
}
func (r *CertifiedMixedProjectedRegion) OriginalUses() [][]AlgebraicShellBooleanUse {
	if r == nil {
		return nil
	}
	out := make([][]AlgebraicShellBooleanUse, len(r.uses))
	for i, p := range r.uses {
		out[i] = append([]AlgebraicShellBooleanUse(nil), p...)
	}
	return out
}
func (r *CertifiedMixedProjectedRegion) Triangles() [][3]int {
	if r == nil {
		return nil
	}
	return append([][3]int(nil), r.triangles...)
}
func (r *CertifiedMixedProjectedRegion) Area() *big.Rat {
	if r == nil {
		return nil
	}
	return new(big.Rat).Set(r.area)
}
func (r *CertifiedMixedProjectedRegion) ProjectedFillSquaredDistance() *big.Rat {
	if r == nil {
		return nil
	}
	return new(big.Rat).Set(r.bound)
}
func (r *CertifiedMixedProjectedRegion) Work() (int, int) {
	if r == nil {
		return 0, 0
	}
	return r.operations, r.predicates
}
func (r *CertifiedMixedProjectedRegion) MarshalJSON() ([]byte, error) {
	return nil, geom.ErrUnrepresentable
}

// ProjectFaceRegionCertified re-proves original support projection injectivity,
// every directed owner path, complete loop nesting and exact polygon coverage.
// Failure, cancellation and exhaustion publish no partial certificate.
func (n *CertifiedMixedAlgebraicShellBoundary) ProjectFaceRegionCertified(ctx context.Context, face int, axes [2]int, o MixedProjectedRegionOptions) (*CertifiedMixedProjectedRegion, error) {
	if ctx == nil || n == nil || n.source == nil || !n.embedded || face < 0 || face >= len(n.source.boundary.faces) || axes[0] < 0 || axes[0] > 2 || axes[1] < 0 || axes[1] > 2 || axes[0] == axes[1] {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if o.MaxVertices == 0 {
		o.MaxVertices = 2048
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 8192
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxPredicates == 0 {
		o.MaxPredicates = 2000000
	}
	if o.MaxVertices < 3 || o.MaxVertices > 4096 || o.MaxTriangles < 1 || o.MaxTriangles > 32768 || o.MaxOperations < 1 || o.MaxOperations > 128000000 || o.MaxPredicates < 1 || o.MaxPredicates > 100000000 {
		return nil, geom.ErrInvalid
	}
	if len(n.network.Vertices) > o.MaxVertices {
		return nil, geom.ErrConvergence
	}
	f := n.source.boundary.faces[face]
	if len(f.Holes) > 31 {
		return nil, geom.ErrConvergence
	}
	original, e := n.source.OriginalFace(face)
	if e != nil {
		return nil, e
	}
	patch, e := exactFaceSupportPatch(original, geom.SurfacePatchOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: 4096})
	if e != nil {
		return nil, e
	}
	ops := patch.ExactOperations()
	if ops >= o.MaxOperations {
		return nil, geom.ErrConvergence
	}
	proof, e := geom.CertifySurfaceInjectivity(patch, geom.SurfaceInjectivityOptions{ProjectionAxes: axes, MaxOperations: o.MaxOperations - ops, MaxCoefficientBits: 4096})
	if e != nil {
		return nil, e
	}
	ops += proof.ExactOperations
	if !proof.ProvenInjective || proof.ProjectionOrientation == 0 {
		return nil, geom.ErrConvergence
	}
	r := &CertifiedMixedProjectedRegion{network: n, face: face, axes: axes, orientation: proof.ProjectionOrientation, reversed: f.Reversed, operations: ops, bound: new(big.Rat).Mul(ratInset(n.deflection), ratInset(n.deflection))}
	r.points = make([][2]*big.Rat, len(n.network.Vertices))
	for i, v := range n.network.Vertices {
		xyz := [3]float64{v.X, v.Y, v.Z}
		r.points[i] = [2]*big.Rat{ratInset(xyz[axes[0]]), ratInset(xyz[axes[1]])}
	}
	uses := append([][]AlgebraicShellBooleanUse{f.Outer}, f.Holes...)
	r.loops, e = mixedDirectedNetworkLoops(n.network.Edges, uses, o.MaxVertices)
	if e != nil {
		return nil, e
	}
	for i, loop := range r.loops {
		r.uses = append(r.uses, append([]AlgebraicShellBooleanUse(nil), uses[i]...))
		if r.orientation < 0 {
			for a, b := 0, len(loop)-1; a < b; a, b = a+1, b-1 {
				loop[a], loop[b] = loop[b], loop[a]
			}
		}
	}
	p := partitionFillProof{ctx: ctx, points: r.points, limit: o.MaxPredicates}
	r.triangles, r.area, e = mixedTriangulateDirectedRegion(&p, r.loops, o.MaxTriangles)
	if e != nil {
		return nil, e
	}
	r.predicates = p.work
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	return r, nil
}

func mixedDirectedNetworkLoops(edges [][]int, uses [][]AlgebraicShellBooleanUse, maximum int) ([][]int, error) {
	var loops [][]int
	seen := map[int]bool{}
	total := 0
	for _, wire := range uses {
		var ring []int
		for _, u := range wire {
			if u.Edge < 0 || u.Edge >= len(edges) || seen[u.Edge] || len(edges[u.Edge]) < 2 {
				return nil, geom.ErrInvalid
			}
			seen[u.Edge] = true
			path := edges[u.Edge]
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
				if len(ring) > maximum+1 {
					return nil, geom.ErrConvergence
				}
			}
		}
		if len(ring) < 4 || ring[0] != ring[len(ring)-1] {
			return nil, geom.ErrInvalid
		}
		ring = ring[:len(ring)-1]
		total += len(ring)
		if total > maximum {
			return nil, geom.ErrConvergence
		}
		loops = append(loops, ring)
	}
	return loops, nil
}

// mixedTriangulateDirectedRegion supplies the independent prerequisites of the
// exact bridge/ear routine, then verifies all directed incidences and Euler.
func mixedTriangulateDirectedRegion(p *partitionFillProof, loops [][]int, maximum int) ([][3]int, *big.Rat, error) {
	if len(loops) == 0 || len(loops) > 32 || maximum < 1 {
		return nil, nil, geom.ErrInvalid
	}
	area := new(big.Rat)
	seen := map[int]bool{}
	for li, ring := range loops {
		for _, id := range ring {
			if id < 0 || id >= len(p.points) || seen[id] || p.points[id][0] == nil || p.points[id][1] == nil {
				return nil, nil, geom.ErrInvalid
			}
			seen[id] = true
		}
		if e := p.simple(ring); e != nil {
			return nil, nil, e
		}
		for i, b := range ring {
			a, c := ring[(i+len(ring)-1)%len(ring)], ring[(i+1)%len(ring)]
			turn, e := p.orient(a, b, c)
			if e != nil {
				return nil, nil, e
			}
			if turn == 0 && !p.on(a, c, b) {
				return nil, nil, geom.ErrInvalid
			}
		}
		a := p.area(ring)
		if li == 0 && a.Sign() <= 0 || li > 0 && a.Sign() >= 0 {
			return nil, nil, geom.ErrInvalid
		}
		area.Add(area, a)
	}
	// No cross-loop touch is allowed, even at differently indexed equal points.
	for i, a := range loops {
		for j := i + 1; j < len(loops); j++ {
			for k, x := range a {
				y := a[(k+1)%len(a)]
				for l, u := range loops[j] {
					v := loops[j][(l+1)%len(loops[j])]
					s, e := p.orient(x, y, u)
					if e != nil {
						return nil, nil, e
					}
					t, e := p.orient(x, y, v)
					if e != nil {
						return nil, nil, e
					}
					h, e := p.orient(u, v, x)
					if e != nil {
						return nil, nil, e
					}
					g, e := p.orient(u, v, y)
					if e != nil {
						return nil, nil, e
					}
					if s*t < 0 && h*g < 0 || s == 0 && p.on(x, y, u) || t == 0 && p.on(x, y, v) || h == 0 && p.on(u, v, x) || g == 0 && p.on(u, v, y) {
						return nil, nil, geom.ErrInvalid
					}
				}
			}
		}
	}
	inside := func(ring []int, id int) (bool, error) {
		w := 0
		q := p.points[id]
		for i, a := range ring {
			b := ring[(i+1)%len(ring)]
			s, e := p.orient(a, b, id)
			if e != nil {
				return false, e
			}
			if p.points[a][1].Cmp(q[1]) <= 0 && p.points[b][1].Cmp(q[1]) > 0 && s > 0 {
				w++
			}
			if p.points[a][1].Cmp(q[1]) > 0 && p.points[b][1].Cmp(q[1]) <= 0 && s < 0 {
				w--
			}
		}
		return w != 0, nil
	}
	for i := 1; i < len(loops); i++ {
		ok, e := inside(loops[0], loops[i][0])
		if e != nil {
			return nil, nil, e
		}
		if !ok {
			return nil, nil, geom.ErrInvalid
		}
		for j := 1; j < len(loops); j++ {
			if i == j {
				continue
			}
			ok, e := inside(loops[j], loops[i][0])
			if e != nil {
				return nil, nil, e
			}
			if ok {
				return nil, nil, geom.ErrInvalid
			}
		}
	}
	if area.Sign() <= 0 {
		return nil, nil, geom.ErrInvalid
	}
	if len(seen)+2*(len(loops)-1)-2 > maximum {
		return nil, nil, geom.ErrConvergence
	}
	var tris [][3]int
	var e error
	if len(loops) == 1 {
		tris, e = p.triangulate(loops[0])
	} else {
		if p.work >= p.limit {
			return nil, nil, geom.ErrConvergence
		}
		var work int
		tris, work, e = exactHoledFootprintTriangulation(p.ctx, p.points, loops, p.limit-p.work)
		p.work += work
	}
	if e != nil {
		return nil, nil, e
	}
	if len(tris) > maximum {
		return nil, nil, geom.ErrConvergence
	}
	expected := map[[2]int]int{}
	for _, ring := range loops {
		for i, a := range ring {
			partitionChainAdd(expected, a, ring[(i+1)%len(ring)], 1)
		}
	}
	actual := map[[2]int]int{}
	incidence := map[[2]int]int{}
	sum := new(big.Rat)
	for _, tri := range tris {
		if e := p.ctx.Err(); e != nil {
			return nil, nil, e
		}
		a := p.area(tri[:])
		if a.Sign() <= 0 {
			return nil, nil, geom.ErrInvalid
		}
		sum.Add(sum, a)
		for j, x := range tri {
			y := tri[(j+1)%3]
			partitionChainAdd(actual, x, y, 1)
			if x > y {
				x, y = y, x
			}
			incidence[[2]int{x, y}]++
		}
	}
	for edge, n := range incidence {
		want := expected[edge]
		if actual[edge] != want || want == 0 && n != 2 || want != 0 && n != 1 {
			return nil, nil, geom.ErrInvalid
		}
	}
	for edge, want := range expected {
		if actual[edge] != want || incidence[edge] != 1 {
			return nil, nil, geom.ErrInvalid
		}
	}
	if sum.Cmp(area) != 0 || len(seen)-len(incidence)+len(tris) != 2-len(loops) {
		return nil, nil, geom.ErrInvalid
	}
	return tris, area, nil
}
