package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/collision"
	"plainsolid/geom"
)

// SideOpenDualDiskMeshOptions requests a two-sided source/display distance and
// total inward normal turn in radians. Every display facet is additionally
// proved within AngularTurn/2 of its source normals. Both tolerances are
// required; AngularTurn must be <=1. MaxRimIntervals defaults256 (maximum256),
// independently of MaxTriangles (default100000). The schedule is dyadic, so a
// non-power-of-two interval limit uses its largest smaller power of two.
// The interval limit also bounds each FINAL shared circular rim partition.
// Circle reconstruction and each triangle residual use the existing bounded
// exact geometry proof; total scheduled intervals are less than twice the
// accepted dyadic count per wall. Face and collision proof budgets retain the
// closed mixed mesher defaults except the explicit Topology defaults below and
// a2048-vertex/2000000-operation exact cap triangulation allowance. Proof failures
// are atomic. Boundary sampling reserves Deflection/4 before Lipschitz
// transport; all actual quarter/total distance checks remain mandatory.
// Exact trim-domain containment may use up to1024 homogeneous
// subdivisions per face, depth16, without changing any original chart.
// Only exact signed-coordinate circular extrusions and affine remaining faces
// are accepted. This is not a generic mesher or a source embedding certificate.
type SideOpenDualDiskMeshOptions struct {
	// Topology bounds exact source/chord polygon validation independently.
	// Zero MaxEdges/MaxWork use2048/2000000; remaining defaults unchanged.
	Topology                      trimvalidate.Options
	Deflection, AngularTurn       float64
	MaxRimIntervals, MaxTriangles int
	Collision                     collision.Options
}

type CertifiedSideOpenDualDiskMesh struct {
	CertifiedStoredMixedClosedMesh
	RimIntervals int
	// Exact upper bounds for tan² of total source normal turn and facet deviation.
	SquaredTurnTangent, SquaredFacetTangent *big.Rat
}

type dualDiskWallProof struct {
	patch  *geom.SurfacePatch
	center [3]*big.Rat
	axis   int
}

func ddDot(a, b [3]*big.Rat) *big.Rat {
	r := new(big.Rat)
	for i := range a {
		r.Add(r, new(big.Rat).Mul(a[i], b[i]))
	}
	return r
}
func ddSub(a, b [3]*big.Rat) (r [3]*big.Rat) {
	for i := range r {
		r[i] = new(big.Rat).Sub(a[i], b[i])
	}
	return
}
func ddCross(a, b [3]*big.Rat) (r [3]*big.Rat) {
	for i := range r {
		j, k := (i+1)%3, (i+2)%3
		r[i] = new(big.Rat).Sub(new(big.Rat).Mul(a[j], b[k]), new(big.Rat).Mul(a[k], b[j]))
	}
	return
}

// Positive dot selects an angle below pi/2, where angle <= tan(angle).
// Together with the independently proved monotone half-plane arc this bounds
// the actual continuous angular turn, without evaluating rounded acos/cos.
func ddTanSquared(a, b [3]*big.Rat) (*big.Rat, bool) {
	d := ddDot(a, b)
	if d.Sign() <= 0 {
		return nil, false
	}
	d.Mul(d, d)
	return new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(ddDot(a, a), ddDot(b, b)), d), d), true
}
func ddSqrt(q *big.Rat) (*big.Rat, bool) {
	if q.Sign() < 0 {
		return nil, false
	}
	n, d := new(big.Int).Sqrt(q.Num()), new(big.Int).Sqrt(q.Denom())
	if new(big.Int).Mul(n, n).Cmp(q.Num()) != 0 || new(big.Int).Mul(d, d).Cmp(q.Denom()) != 0 {
		return nil, false
	}
	return new(big.Rat).SetFrac(n, d), true
}
func (w dualDiskWallProof) radial(ctx context.Context, u *big.Rat) ([3]*big.Rat, error) {
	j, e := w.patch.EvaluateRational(ctx, u, new(big.Rat), geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		return [3]*big.Rat{}, e
	}
	r := ddSub(j.Point, w.center)
	r[w.axis] = new(big.Rat)
	return r, nil
}

// Reconstructing exact homogeneous controls independently proves the whole
// circle equation, nonzero angular derivative, and open-halfplane arc bound.
// Metadata is never accepted as evidence of circularity.
func proveDualDiskWall(ctx context.Context, f Face) (dualDiskWallProof, error) {
	var out dualDiskWallProof
	p, e := exactFaceSupportPatch(f, geom.SurfacePatchOptions{})
	if e != nil {
		return out, e
	}
	recipe, e := p.ExportExactBezierRecipe()
	if e != nil {
		return out, e
	}
	if recipe.DegreeU != 2 || recipe.DegreeV != 1 || len(recipe.Controls) != 6 || recipe.Domain != (geom.ParameterRectangle{UMax: 1, VMax: 1}) || len(f.Holes) != 0 || len(f.Outer) != 4 {
		return out, geom.ErrDomain
	}
	poles := [6][3]*big.Rat{}
	weights := [6]*big.Rat{}
	for i, row := range recipe.Controls {
		var h [4]*big.Rat
		for k, text := range row {
			if len(text) > 8192 {
				return out, geom.ErrConvergence
			}
			q, ok := new(big.Rat).SetString(text)
			if !ok {
				return out, geom.ErrInvalid
			}
			if q.Num().BitLen() > 4096 || q.Denom().BitLen() > 4096 {
				return out, geom.ErrConvergence
			}
			h[k] = q
		}
		if h[3].Sign() <= 0 {
			return out, geom.ErrInvalid
		}
		weights[i] = h[3]
		for k := range poles[i] {
			poles[i][k] = new(big.Rat).Quo(h[k], h[3])
		}
	}
	delta := ddSub(poles[1], poles[0])
	axis := -1
	for i, v := range delta {
		if v.Sign() != 0 {
			if axis != -1 {
				return out, geom.ErrDomain
			}
			axis = i
		}
	}
	if axis < 0 {
		return out, geom.ErrDomain
	}
	for i := 0; i < 3; i++ {
		if weights[2*i].Cmp(weights[2*i+1]) != 0 {
			return out, geom.ErrDomain
		}
		q := ddSub(poles[2*i+1], poles[2*i])
		for k := range q {
			if q[k].Cmp(delta[k]) != 0 {
				return out, geom.ErrDomain
			}
		}
	}
	// Require the actual four normalized affine pcurves, not only their endpoints.
	corners := [4]geom.Vec2{{}, {X: 1}, {X: 1, Y: 1}, {Y: 1}}
	for i, u := range f.Outer {
		if u.UVStart != corners[i] || u.UVEnd != corners[(i+1)%4] {
			return out, geom.ErrDomain
		}
		c, e := mixedStoredUseCurve(u)
		if e != nil {
			return out, e
		}
		h, e := mixedExactControls(c)
		if e != nil {
			return out, e
		}
		for j := range h {
			t := big.NewRat(int64(j), int64(len(h)-1))
			for k := 0; k < 2; k++ {
				a, b := ratInset([]float64{u.UVStart.X, u.UVStart.Y}[k]), ratInset([]float64{u.UVEnd.X, u.UVEnd.Y}[k])
				want := new(big.Rat).Add(a, new(big.Rat).Mul(t, new(big.Rat).Sub(b, a)))
				if new(big.Rat).Quo(h[j][k], h[j][2]).Cmp(want) != 0 || h[j][2].Cmp(h[0][2]) != 0 {
					return out, geom.ErrDomain
				}
			}
		}
	}
	xaxis := (axis + 1) % 3
	yaxis := (axis + 2) % 3
	sign := delta[axis].Sign()
	xyz := func(a int, v float64) geom.Vec3 {
		q := [3]float64{}
		q[a] = v
		return geom.Vec3{X: q[0], Y: q[1], Z: q[2]}
	}
	frame, e := geom.NewFrame(geom.Vec3{}, xyz(xaxis, 1), xyz(axis, float64(sign)))
	if e != nil {
		return out, e
	}
	var samples [3][2]*big.Rat
	for i, t := range []*big.Rat{new(big.Rat), big.NewRat(1, 2), big.NewRat(1, 1)} {
		j, e := p.EvaluateRational(ctx, t, new(big.Rat), geom.SurfacePatchEvaluationOptions{})
		if e != nil {
			return out, e
		}
		samples[i] = [2]*big.Rat{j.Point[xaxis], new(big.Rat).Mul(j.Point[yaxis], big.NewRat(int64(sign), 1))}
	}
	var rows [2][3]*big.Rat
	for i := 0; i < 2; i++ {
		rows[i][2] = new(big.Rat)
		for k := 0; k < 2; k++ {
			rows[i][k] = new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Sub(samples[i+1][k], samples[0][k]))
			rows[i][2].Add(rows[i][2], new(big.Rat).Sub(new(big.Rat).Mul(samples[i+1][k], samples[i+1][k]), new(big.Rat).Mul(samples[0][k], samples[0][k])))
		}
	}
	det := new(big.Rat).Sub(new(big.Rat).Mul(rows[0][0], rows[1][1]), new(big.Rat).Mul(rows[0][1], rows[1][0]))
	if det.Sign() == 0 {
		return out, geom.ErrDomain
	}
	center := [2]*big.Rat{new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(rows[0][2], rows[1][1]), new(big.Rat).Mul(rows[0][1], rows[1][2])), det), new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(rows[0][0], rows[1][2]), new(big.Rat).Mul(rows[0][2], rows[1][0])), det)}
	r2 := new(big.Rat)
	for k := range center {
		v := new(big.Rat).Sub(samples[0][k], center[k])
		r2.Add(r2, new(big.Rat).Mul(v, v))
	}
	radius, ok := ddSqrt(r2)
	if !ok {
		return out, geom.ErrDomain
	}
	var arc [3][3]*big.Rat
	for i := range arc {
		w := weights[2*i]
		arc[i] = [3]*big.Rat{new(big.Rat).Mul(w, poles[2*i][xaxis]), new(big.Rat).Mul(new(big.Rat).Mul(w, poles[2*i][yaxis]), big.NewRat(int64(sign), 1)), w}
	}
	z0, z1 := new(big.Rat).Mul(poles[0][axis], big.NewRat(int64(sign), 1)), new(big.Rat).Mul(poles[1][axis], big.NewRat(int64(sign), 1))
	rebuilt, e := geom.NewExactRationalCylindricalSpanPatch(ctx, frame, center, radius, z0, z1, arc)
	if e != nil {
		return out, e
	}
	rebuiltRecipe, e := rebuilt.ExportExactBezierRecipe()
	if e != nil {
		return out, e
	}
	if rebuiltRecipe.Domain != recipe.Domain || len(rebuiltRecipe.Controls) != len(recipe.Controls) {
		return out, geom.ErrDomain
	}
	for i, h := range recipe.Controls {
		if h != rebuiltRecipe.Controls[i] {
			return out, geom.ErrDomain
		}
	}
	out = dualDiskWallProof{patch: p, axis: axis, center: [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}}
	out.center[xaxis] = center[0]
	out.center[yaxis] = new(big.Rat).Mul(center[1], big.NewRat(int64(sign), 1))
	return out, nil
}

func (s SideOpenDualDiskCut) TessellateCertified(ctx context.Context, o SideOpenDualDiskMeshOptions) (CertifiedSideOpenDualDiskMesh, error) {
	var empty CertifiedSideOpenDualDiskMesh
	if ctx == nil || !finite(o.Deflection) || o.Deflection <= 0 || !finite(o.AngularTurn) || o.AngularTurn <= 0 || o.AngularTurn > 1 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if o.MaxRimIntervals == 0 {
		o.MaxRimIntervals = 256
	}
	if o.MaxRimIntervals < 1 || o.MaxRimIntervals > 256 {
		return empty, geom.ErrInvalid
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 100000
	}
	if o.MaxTriangles < 1 || o.MaxTriangles > collision.MaxTriangles || o.Collision.MaxPairs < 0 || o.Collision.MaxContacts < 0 {
		return empty, geom.ErrInvalid
	}
	if o.MaxTriangles < len(s.Solid.faces) {
		return empty, geom.ErrConvergence
	}
	if o.Topology.MaxEdges == 0 {
		o.Topology.MaxEdges = 2048
	}
	if o.Topology.MaxWork == 0 {
		o.Topology.MaxWork = 2000000
	}
	if _, e := o.Topology.WithDefaults(); e != nil {
		return empty, e
	}
	walls := map[int]dualDiskWallProof{}
	rim := map[int]bool{}
	for _, ids := range s.WallFaces {
		if len(ids) != 1 || ids[0] < 0 || ids[0] >= len(s.Solid.faces) {
			return empty, geom.ErrDomain
		}
		fi := ids[0]
		if _, ok := walls[fi]; ok {
			return empty, geom.ErrDomain
		}
		w, e := proveDualDiskWall(ctx, s.Solid.faces[fi])
		if e != nil {
			return empty, fmt.Errorf("circular wall %d: %w", fi, e)
		}
		walls[fi] = w
		for _, u := range s.Solid.faces[fi].Outer {
			if u.UVStart.Y == u.UVEnd.Y {
				rim[u.Edge] = true
			}
		}
	}
	normals := map[int][3]*big.Rat{}
	for fi, f := range s.Solid.faces {
		if _, ok := walls[fi]; ok {
			continue
		}
		p, e := exactFaceSupportPatch(f, geom.SurfacePatchOptions{})
		if e != nil {
			return empty, e
		}
		chart, e := geom.NewAffineSurfaceChart(ctx, p, geom.AffineSurfaceChartOptions{})
		if e != nil {
			return empty, e
		}
		a, e := chart.EvalExact(new(big.Rat), new(big.Rat))
		if e != nil {
			return empty, e
		}
		b, e := chart.EvalExact(big.NewRat(1, 1), new(big.Rat))
		if e != nil {
			return empty, e
		}
		c, e := chart.EvalExact(new(big.Rat), big.NewRat(1, 1))
		if e != nil {
			return empty, e
		}
		n := ddCross(ddSub(b, a), ddSub(c, a))
		if f.Reversed {
			for k := range n {
				n[k].Neg(n[k])
			}
		}
		normals[fi] = n
	}
	angle2 := new(big.Rat).Mul(ratInset(o.AngularTurn), ratInset(o.AngularTurn))
	facet2 := new(big.Rat).Quo(angle2, big.NewRat(4, 1))
	linear2 := new(big.Rat).Quo(new(big.Rat).Mul(ratInset(o.Deflection), ratInset(o.Deflection)), big.NewRat(4, 1))
	n := 1
	for ; n <= o.MaxRimIntervals; n *= 2 {
		accepted := true
		for _, w := range walls {
			for k := 0; k < n; k++ {
				if e := ctx.Err(); e != nil {
					return empty, e
				}
				lo, hi := big.NewRat(int64(k), int64(n)), big.NewRat(int64(k+1), int64(n))
				a, e := w.radial(ctx, lo)
				if e != nil {
					return empty, e
				}
				b, e := w.radial(ctx, hi)
				if e != nil {
					return empty, e
				}
				bound, ok := ddTanSquared(a, b)
				if !ok || bound.Cmp(angle2) > 0 {
					accepted = false
					break
				}
				uv := [4][2]*big.Rat{{lo, new(big.Rat)}, {hi, new(big.Rat)}, {hi, big.NewRat(1, 1)}, {lo, big.NewRat(1, 1)}}
				var points [4]geom.Vec3
				for i := range uv {
					j, e := w.patch.EvaluateRational(ctx, uv[i][0], uv[i][1], geom.SurfacePatchEvaluationOptions{})
					if e != nil {
						return empty, e
					}
					x, _ := j.Point[0].Float64()
					y, _ := j.Point[1].Float64()
					z, _ := j.Point[2].Float64()
					points[i] = geom.Vec3{X: x, Y: y, Z: z}
				}
				for _, t := range [][3]int{{0, 1, 2}, {0, 2, 3}} {
					v, ok := rationalBiquadraticExactCorrespondenceBound(w.patch, [3][2]*big.Rat{uv[t[0]], uv[t[1]], uv[t[2]]}, [3]geom.Vec3{points[t[0]], points[t[1]], points[t[2]]})
					if !ok || v.Cmp(linear2) > 0 {
						accepted = false
						break
					}
				}
				if !accepted {
					break
				}
			}
			if !accepted {
				break
			}
		}
		if accepted {
			break
		}
	}
	if n > o.MaxRimIntervals {
		return empty, fmt.Errorf("circular rim schedule: %w", geom.ErrConvergence)
	}
	options := make([]SolidMixedFaceMeshOptions, len(s.Solid.faces))
	for fi, f := range s.Solid.faces {
		loops := append([][]Use{f.Outer}, f.Holes...)
		cuts := make([][][]*big.Rat, len(loops))
		for li, loop := range loops {
			cuts[li] = make([][]*big.Rat, len(loop))
			for ui, u := range loop {
				count := 1
				if rim[u.Edge] {
					count = n
				}
				for k := 0; k <= count; k++ {
					cuts[li][ui] = append(cuts[li][ui], big.NewRat(int64(k), int64(count)))
				}
			}
		}
		options[fi] = SolidMixedFaceMeshOptions{Face: MixedExactMultiSpanMeshOptions{Face: MixedExactCurvedMeshOptions{BoundaryModelBudget: o.Deflection / 4, RectangularGrid: true, Cuts: cuts, Boundary: MixedExactAffineMeshOptions{DomainSubdivisions: 1024, DomainDepth: 16, Topology: o.Topology, MaxBoundaryVertices: 2048, MaxTriangulationWork: 2000000}, Interior: CurvedQuadraticTrimMeshOptions{Deflection: o.Deflection, MaxTriangles: o.MaxTriangles}}}}
	}
	display, e := s.Solid.TessellateMixedClosedCertified(ctx, StoredMixedClosedMeshOptions{Faces: options, MaxTriangles: o.MaxTriangles, Collision: o.Collision})
	if e != nil {
		return empty, fmt.Errorf("closed display at %d rim intervals: %w", n, e)
	}
	for edge := range rim {
		if len(display.Network.EdgeParameters[edge])-1 > o.MaxRimIntervals {
			return empty, fmt.Errorf("final shared rim interval limit: %w", geom.ErrConvergence)
		}
	}
	result := CertifiedSideOpenDualDiskMesh{CertifiedStoredMixedClosedMesh: display, RimIntervals: n, SquaredTurnTangent: new(big.Rat), SquaredFacetTangent: new(big.Rat)}
	// The exact dot minimum on a short monotone circular arc with both endpoint
	// dots positive is at an endpoint. Thus endpoint checks bound ALL source
	// normals, including any axial component introduced by display rounding.
	for fi, f := range display.Network.Faces {
		for _, t := range f.Mesh.Triangles {
			if e := ctx.Err(); e != nil {
				return empty, e
			}
			a, b, c := exactBoundPoint(f.Mesh.Vertices[t[0]]), exactBoundPoint(f.Mesh.Vertices[t[1]]), exactBoundPoint(f.Mesh.Vertices[t[2]])
			normal := ddCross(ddSub(b, a), ddSub(c, a))
			check := func(target [3]*big.Rat) bool {
				v, ok := ddTanSquared(normal, target)
				if !ok || v.Cmp(facet2) > 0 {
					return false
				}
				if v.Cmp(result.SquaredFacetTangent) > 0 {
					result.SquaredFacetTangent.Set(v)
				}
				return true
			}
			if w, ok := walls[fi]; ok {
				lo, hi := f.ExactUV[t[0]][0], f.ExactUV[t[0]][0]
				for _, id := range t {
					u := f.ExactUV[id][0]
					if u.Cmp(lo) < 0 {
						lo = u
					}
					if u.Cmp(hi) > 0 {
						hi = u
					}
				}
				a, e := w.radial(ctx, lo)
				if e != nil {
					return empty, e
				}
				b, e := w.radial(ctx, hi)
				if e != nil {
					return empty, e
				}
				v, ok := ddTanSquared(a, b)
				if !ok || v.Cmp(angle2) > 0 {
					return empty, geom.ErrConvergence
				}
				if v.Cmp(result.SquaredTurnTangent) > 0 {
					result.SquaredTurnTangent.Set(v)
				}
				for k := range a {
					a[k].Neg(a[k])
					b[k].Neg(b[k])
				}
				if !check(a) || !check(b) {
					return empty, geom.ErrConvergence
				}
			} else if !check(normals[fi]) {
				return empty, geom.ErrConvergence
			}
		}
	}
	return result, nil
}
