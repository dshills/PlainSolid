package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// CertifiedCircularHoleExtrusion certifies the ideal circular-hole construction
// specified by the input sketch and signed heights. Solid retains the existing
// analytic constructor and original face IDs. The certificate does not assert
// exact sewing of that constructor's independently rounded analytic charts.
// IdealBoundarySquaredDistance is two-sided and includes all display rounding.
// UVFillSquaredDistance is the planar closed-fill homotopy bound, including
// holes. Display topology and absence of contacts are independently checked.
type CertifiedCircularHoleExtrusion struct {
	Solid                                                                    Solid
	Mesh                                                                     mesh.Mesh
	OriginalFace                                                             []int
	RimVertices                                                              [][2][]int // Outer first, then through holes, then blind holes.
	IdealBoundarySquaredDistance, UVFillSquaredDistance, SignedDisplayVolume *big.Rat
	Collision                                                                collision.Report
	Segments, TrigTerms, Predicates                                          int
}

// ExtrudePolygonWithHolesCertified supports signed extrusion in exact
// right-handed signed-permutation frames, with any finite represented origin.
// Source containment/disjointness is proved by the analytic constructor. Exact
// displayed loop validation and triangulation prove each polygonal cap/floor.
// The circle-to-chord straight homotopy transfers closed-fill distance through
// all holes; cylindrical sides use the same angular correspondence. All rims
// are assigned once and shared by caps/walls/floors. MaxSegments limits circle
// samples; MaxTrigTerms and MaxPredicates are cumulative construction budgets.
func ExtrudePolygonWithHolesCertified(ctx context.Context, frame geom.Frame, outer []geom.Vec2, through []ExtrusionCircle, blind []ExtrusionBlindHole, height float64, o CertifiedRevolvePolygonOptions) (out CertifiedCircularHoleExtrusion, err error) {
	defer func() {
		if err != nil {
			out = CertifiedCircularHoleExtrusion{}
		}
	}()
	if ctx == nil || !finite(o.Deflection) || o.Deflection <= 0 {
		return out, geom.ErrInvalid
	}
	axes := [3]geom.Vec3{frame.XAxis(), frame.YAxis(), frame.ZAxis()}
	for _, a := range axes {
		count := 0
		for _, v := range []float64{a.X, a.Y, a.Z} {
			if v != 0 {
				if v != 1 && v != -1 {
					return out, geom.ErrConvergence
				}
				count++
			}
		}
		if count != 1 {
			return out, geom.ErrConvergence
		}
	}
	if axes[0].Cross(axes[1]) != axes[2] {
		return out, geom.ErrConvergence
	}
	if o.MaxSegments == 0 {
		o.MaxSegments = 512
	}
	if o.MaxVertices == 0 {
		o.MaxVertices = 65536
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 262144
	}
	if o.MaxPredicates == 0 {
		o.MaxPredicates = 1000000
	}
	if o.MaxTrigTerms == 0 {
		o.MaxTrigTerms = 262144
	}
	if o.MaxBits == 0 {
		o.MaxBits = 16384
	}
	if o.MaxSegments < 4 || o.MaxSegments > 4096 || o.MaxVertices < 3 || o.MaxVertices > 1000000 || o.MaxTriangles < 1 || o.MaxTriangles > 2000000 || o.MaxPredicates < 1 || o.MaxPredicates > 100000000 || o.MaxTrigTerms < 1 || o.MaxTrigTerms > 2000000 || o.MaxBits < 64 || o.MaxBits > 65536 {
		return out, geom.ErrInvalid
	}
	source, e := ExtrudePolygonWithHoles(ctx, frame, outer, through, blind, height)
	if e != nil {
		return out, e
	}
	var p []geom.Vec2
	for _, use := range source.faces[0].Outer {
		p = append(p, use.UVStart)
	}
	holes := append([]ExtrusionCircle(nil), through...)
	for _, h := range blind {
		holes = append(holes, h.ExtrusionCircle)
	}
	tolerance := ratInset(o.Deflection)
	tolerance.Mul(tolerance, tolerance)
	n := 4
	for ; n <= o.MaxSegments; n *= 2 {
		ok := true
		for _, h := range holes {
			b := new(big.Rat).Mul(ratInset(h.Radius), big.NewRat(49, int64(8*n*n)))
			if new(big.Rat).Mul(big.NewRat(8, 1), new(big.Rat).Mul(b, b)).Cmp(tolerance) > 0 {
				ok = false
			}
		}
		if ok {
			break
		}
	}
	if n > o.MaxSegments || 2*(len(p)+len(holes)*n) > o.MaxVertices {
		return out, geom.ErrConvergence
	}
	work := 0
	var piMid, piError *big.Rat
	var sine, cosine, errors []*big.Rat
	if len(holes) > 0 {
		piMid, piError, e = certifiedSweepPi(ctx, o, &work)
		if e != nil {
			return out, e
		}
		for i := 0; i < n; i++ {
			fraction := big.NewRat(int64(i), int64(n))
			if fraction.Cmp(big.NewRat(1, 2)) > 0 {
				fraction.Sub(fraction, big.NewRat(1, 1))
			}
			factor := new(big.Rat).Mul(big.NewRat(-2, 1), fraction)
			sn, cs, r, e := revolutionTrigTaylor(ctx, new(big.Rat).Mul(piMid, factor), o, &work)
			if e != nil {
				return out, e
			}
			r.Add(r, new(big.Rat).Mul(new(big.Rat).Abs(factor), piError))
			sine = append(sine, sn)
			cosine = append(cosine, cs)
			errors = append(errors, r)
		}
	}
	var points []geom.Vec2
	var exact [][2]*big.Rat
	var loops [][]int
	circleBound := new(big.Rat)
	appendPoint := func(v geom.Vec2) int {
		id := len(points)
		points = append(points, v)
		exact = append(exact, [2]*big.Rat{ratInset(v.X), ratInset(v.Y)})
		return id
	}
	var ring []int
	for _, v := range p {
		ring = append(ring, appendPoint(v))
	}
	loops = append(loops, ring)
	for _, h := range holes {
		var ring []int
		round := new(big.Rat)
		for i := 0; i < n; i++ {
			centers := [2]*big.Rat{new(big.Rat).Add(ratInset(h.Center.X), new(big.Rat).Mul(ratInset(h.Radius), cosine[i])), new(big.Rat).Add(ratInset(h.Center.Y), new(big.Rat).Mul(ratInset(h.Radius), sine[i]))}
			var xy [2]float64
			squared := new(big.Rat)
			for axis, c := range centers {
				xy[axis], _ = c.Float64()
				if !finite(xy[axis]) {
					return out, geom.ErrConvergence
				}
				r := new(big.Rat).Add(new(big.Rat).Abs(new(big.Rat).Sub(c, ratInset(xy[axis]))), new(big.Rat).Mul(ratInset(h.Radius), errors[i]))
				squared.Add(squared, new(big.Rat).Mul(r, r))
			}
			if squared.Cmp(round) > 0 {
				round.Set(squared)
			}
			ring = append(ring, appendPoint(geom.Vec2{X: xy[0], Y: xy[1]}))
		}
		loops = append(loops, ring)
		b := new(big.Rat).Mul(ratInset(h.Radius), big.NewRat(49, int64(8*n*n)))
		bound := new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Add(new(big.Rat).Mul(b, b), round))
		if bound.Cmp(circleBound) > 0 {
			circleBound.Set(bound)
		}
	}
	var polygonLoops []trimvalidate.Loop
	for _, ring := range loops {
		var l trimvalidate.Loop
		for i, id := range ring {
			l = append(l, trimvalidate.Segment{Start: points[id], End: points[ring[(i+1)%len(ring)]]})
		}
		polygonLoops = append(polygonLoops, l)
	}
	validation, e := trimvalidate.ValidateLoops(ctx, polygonLoops[0], polygonLoops[1:], trimvalidate.Options{MaxEdges: len(points), MaxWork: o.MaxPredicates, MaxBits: o.MaxBits})
	if e != nil || validation.Status != trimvalidate.Certified {
		if e == nil {
			e = geom.ErrConvergence
		}
		return out, e
	}
	result := CertifiedCircularHoleExtrusion{Solid: source, Segments: n, TrigTerms: work, UVFillSquaredDistance: circleBound, IdealBoundarySquaredDistance: new(big.Rat)}
	// The exact planar polygon topology is independently proved above. Positive
	// ear triangles with exact area and boundary cancellation prove filled caps.
	predicates := validation.Work
	triangulate := func(selected [][]int) ([][3]int, error) {
		remaining := o.MaxPredicates - predicates
		if remaining < 1 {
			return nil, geom.ErrConvergence
		}
		t, w, e := exactHoledFootprintTriangulation(ctx, exact, selected, remaining)
		predicates += w
		return t, e
	}
	lo, hi := math.Min(0, height), math.Max(0, height)
	mapping := [2][]int{make([]int, len(points)), make([]int, len(points))}
	round := new(big.Rat)
	origin := exactBoundPoint(frame.Origin())
	matrix := [3][3]*big.Rat{exactBoundPoint(axes[0]), exactBoundPoint(axes[1]), exactBoundPoint(axes[2])}
	for loop, ring := range loops {
		z0, z1 := lo, hi
		if loop > len(through) {
			depth := math.Copysign(blind[loop-len(through)-1].Depth, height)
			z0, z1 = math.Min(0, depth), math.Max(0, depth)
		}
		var rims [2][]int
		for end, z := range []float64{z0, z1} {
			for _, id := range ring {
				local := [3]*big.Rat{exact[id][0], exact[id][1], ratInset(z)}
				var xyz [3]float64
				squared := new(big.Rat)
				for axis := 0; axis < 3; axis++ {
					value := new(big.Rat).Set(origin[axis])
					for k := 0; k < 3; k++ {
						value.Add(value, new(big.Rat).Mul(matrix[k][axis], local[k]))
					}
					xyz[axis], _ = value.Float64()
					if !finite(xyz[axis]) {
						return out, geom.ErrConvergence
					}
					d := new(big.Rat).Sub(value, ratInset(xyz[axis]))
					squared.Add(squared, new(big.Rat).Mul(d, d))
				}
				if squared.Cmp(round) > 0 {
					round.Set(squared)
				}
				mapping[end][id] = len(result.Mesh.Vertices)
				rims[end] = append(rims[end], len(result.Mesh.Vertices))
				result.Mesh.Vertices = append(result.Mesh.Vertices, geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]})
			}
		}
		result.RimVertices = append(result.RimVertices, rims)
	}
	result.IdealBoundarySquaredDistance.Mul(big.NewRat(2, 1), new(big.Rat).Add(circleBound, round))
	if result.IdealBoundarySquaredDistance.Cmp(tolerance) > 0 {
		return out, geom.ErrConvergence
	}
	appendTri := func(t [3]int, face int) error {
		if len(result.Mesh.Triangles) >= o.MaxTriangles {
			return geom.ErrConvergence
		}
		result.Mesh.Triangles = append(result.Mesh.Triangles, t)
		result.OriginalFace = append(result.OriginalFace, face)
		return nil
	}
	for cap := 0; cap < 2; cap++ {
		selected := [][]int{loops[0]}
		for i := range holes {
			if i < len(through) || (height > 0 && cap == 0) || (height < 0 && cap == 1) {
				selected = append(selected, loops[i+1])
			}
		}
		tris, e := triangulate(selected)
		if e != nil {
			return out, e
		}
		for _, t := range tris {
			q := [3]int{mapping[cap][t[0]], mapping[cap][t[1]], mapping[cap][t[2]]}
			if cap == 0 {
				q[1], q[2] = q[2], q[1]
			}
			if e := appendTri(q, cap); e != nil {
				return out, e
			}
		}
	}
	for loop, ring := range loops {
		for i, a := range ring {
			b := ring[(i+1)%len(ring)]
			face := 2 + i
			if loop > 0 {
				face = 2 + len(p) + 4*(loop-1) + i/(n/4)
			}
			quad := [4]int{mapping[0][a], mapping[0][b], mapping[1][b], mapping[1][a]}
			for _, t := range [][3]int{{0, 1, 2}, {0, 2, 3}} {
				if e := appendTri([3]int{quad[t[0]], quad[t[1]], quad[t[2]]}, face); e != nil {
					return out, e
				}
			}
		}
	}
	for i := range blind {
		loop := loops[len(through)+i+1]
		positive := append([]int(nil), loop...)
		for j, k := 0, len(positive)-1; j < k; j, k = j+1, k-1 {
			positive[j], positive[k] = positive[k], positive[j]
		}
		tris, e := triangulate([][]int{positive})
		if e != nil {
			return out, e
		}
		floor := 1
		if height < 0 {
			floor = 0
		}
		for _, t := range tris {
			q := [3]int{mapping[floor][t[0]], mapping[floor][t[1]], mapping[floor][t[2]]}
			if height > 0 {
				q[1], q[2] = q[2], q[1]
			}
			if e := appendTri(q, 2+len(p)+4*len(holes)+i); e != nil {
				return out, e
			}
		}
	}
	result.Predicates = predicates
	result.SignedDisplayVolume, result.Collision, e = checkClosedPeriodicMesh(ctx, result.Mesh, o.Collision)
	if e != nil {
		return out, e
	}
	return result, nil
}
