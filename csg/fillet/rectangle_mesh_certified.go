package fillet

import (
	"context"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

type RectangleMeshOptions struct {
	Model        Options
	Deflection   float64
	Profile      trimmed.AffineQuadraticTrimMeshOptions
	MaxTriangles int
	Collision    collision.Options
}

// CertifiedRectangleMesh retains the authored selected-corner Solid/history.
// Its distance target is the ideal rectangular quarter-circle fillet profile,
// not exact sewing of Model's independently rounded analytic charts.
// ProfileSources maps each exact rational profile piece to Model's face ID.
// All cap and side samples share one index at each height. The two-sided bound
// includes display rounding, and the full display must be closed and embedded.
type CertifiedRectangleMesh struct {
	Model                                             SelectiveResult
	Mesh                                              mesh.Mesh
	CapProof                                          trimmed.CertifiedAffineQuadraticTrimMesh
	ProfileSources                                    []int
	OriginalFace                                      []int
	IdealBoundarySquaredDistance, SignedDisplayVolume *big.Rat
	Collision                                         collision.Report
}

// PrismRectangleCornersCertified is an opt-in selected unequal-radius vertical
// fillet mesher for a CCW XY rectangle. Corner IDs start at min and proceed
// through (max.X,min.Y),max,(min.X,max.Y). Ideal tangent points must be exactly
// representable; other inputs reject instead of silently changing dimensions.
// Positive rational quarter arcs use weights (1,1,2): w0*w2=2*w1² proves the
// circular conic without an approximate sqrt(2) weight. Exact source and chord
// loop validation plus filled-cap homotopy establish the cap certificate.
func PrismRectangleCornersCertified(ctx context.Context, min, max geom.Vec2, zMin, zMax float64, selection []CornerRadius, o RectangleMeshOptions) (out CertifiedRectangleMesh, err error) {
	defer func() {
		if err != nil {
			out = CertifiedRectangleMesh{}
		}
	}()
	if ctx == nil || !finite(min.X) || !finite(min.Y) || !finite(max.X) || !finite(max.Y) || min.X >= max.X || min.Y >= max.Y || !finite(o.Deflection) || o.Deflection <= 0 {
		return out, ErrInvalid
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 100000
	}
	if o.MaxTriangles < 1 || o.MaxTriangles > 1000000 {
		return out, ErrInvalid
	}
	polygon := []geom.Vec2{min, {X: max.X, Y: min.Y}, max, {X: min.X, Y: max.Y}}
	model, e := PrismCorners(ctx, polygon, zMin, zMax, selection, o.Model)
	if e != nil {
		return out, e
	}
	radii := [4]float64{}
	for _, s := range selection {
		radii[s.Corner] = s.Radius
	}
	directions := [4]geom.Vec2{{X: 1}, {Y: 1}, {X: -1}, {Y: -1}}
	var incoming, outgoing [4]geom.Vec2
	for i, p := range polygon {
		for end := 0; end < 2; end++ {
			d := directions[(i+3)%4]
			sign := -1.0
			if end == 1 {
				d = directions[i]
				sign = 1
			}
			x, e := geom.RepresentedExactScalar(qa(num(p.X), qm(num(sign*d.X), num(radii[i]))))
			if e != nil {
				return out, e
			}
			y, e := geom.RepresentedExactScalar(qa(num(p.Y), qm(num(sign*d.Y), num(radii[i]))))
			if e != nil {
				return out, e
			}
			if end == 0 {
				incoming[i] = geom.Vec2{X: x, Y: y}
			} else {
				outgoing[i] = geom.Vec2{X: x, Y: y}
			}
		}
	}
	var curves []trimmed.SplineUV
	var sources []int
	add := func(p []geom.Vec2, weights []float64, face int) error {
		poles := make([]geom.Vec3, len(p))
		for i, q := range p {
			poles[i] = geom.Vec3{X: q.X, Y: q.Y}
		}
		degree := len(p) - 1
		knots := []float64{0, 0, 1, 1}
		if degree == 2 {
			knots = []float64{0, 0, 0, 1, 1, 1}
		}
		c, e := geom.NewBSpline(degree, poles, knots, weights)
		if e != nil {
			return e
		}
		curves = append(curves, trimmed.SplineUV{Curve: c, Range: trimmed.UVInterval{Max: 1}})
		sources = append(sources, face)
		return nil
	}
	for i := 0; i < 4; i++ {
		j := (i + 1) % 4
		if e := add([]geom.Vec2{outgoing[i], incoming[j]}, nil, model.StraightFaces[i]); e != nil {
			return out, e
		}
		if radii[j] > 0 {
			if e := add([]geom.Vec2{incoming[j], polygon[j], outgoing[j]}, []float64{1, 1, 2}, model.Corners[j].CylinderFace); e != nil {
				return out, e
			}
		}
	}
	surface, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{X: min.X, Y: min.Y}, {X: min.X, Y: max.Y}, {X: max.X, Y: min.Y}, {X: max.X, Y: max.Y}}, []float64{min.X, min.X, max.X, max.X}, []float64{min.Y, min.Y, max.Y, max.Y}, nil)
	if e != nil {
		return out, e
	}
	patch, e := geom.NewSurfacePatch(surface, geom.SurfacePatchOptions{})
	if e != nil {
		return out, e
	}
	profile := o.Profile
	profile.Region.Tessellation.Deflection = o.Deflection / 4
	if profile.Region.Tessellation.Deflection <= 0 {
		return out, geom.ErrConvergence
	}
	if profile.MaxTriangles == 0 || profile.MaxTriangles > o.MaxTriangles {
		profile.MaxTriangles = o.MaxTriangles
	}
	cap, e := trimmed.TessellateAffineQuadraticTrimRegionCertified(ctx, patch, curves, nil, profile)
	if e != nil {
		return out, e
	}
	if len(cap.BoundaryLoops) != 1 || len(cap.Region.Loops) != 1 {
		return out, geom.ErrConvergence
	}
	result := CertifiedRectangleMesh{Model: model, CapProof: cap, ProfileSources: sources, IdealBoundarySquaredDistance: new(big.Rat).Set(cap.FillSquaredDistance)}
	count := len(cap.Mesh.Vertices)
	for _, z := range []float64{zMin, zMax} {
		for _, p := range cap.Mesh.Vertices {
			result.Mesh.Vertices = append(result.Mesh.Vertices, geom.Vec3{X: p.X, Y: p.Y, Z: z})
		}
	}
	appendTriangle := func(t [3]int, face int) error {
		if len(result.Mesh.Triangles) >= o.MaxTriangles {
			return ErrLimit
		}
		result.Mesh.Triangles = append(result.Mesh.Triangles, t)
		result.OriginalFace = append(result.OriginalFace, face)
		return nil
	}
	for _, t := range cap.Mesh.Triangles {
		if e := appendTriangle([3]int{t[0], t[2], t[1]}, model.Caps[0]); e != nil {
			return out, e
		}
		if e := appendTriangle([3]int{t[0] + count, t[1] + count, t[2] + count}, model.Caps[1]); e != nil {
			return out, e
		}
	}
	key := func(p [2]*big.Rat) string { return p[0].RatString() + "," + p[1].RatString() }
	ids := map[string]int{}
	for i, p := range cap.ExactUV {
		ids[key(p)] = i
	}
	boundary := cap.Region.Loops[0].Boundary
	for i, segment := range boundary.Segments {
		a, ok := ids[key(boundary.ExactUV[segment.Vertices[0]])]
		if !ok {
			return out, geom.ErrConvergence
		}
		b, ok := ids[key(boundary.ExactUV[segment.Vertices[1]])]
		if !ok || segment.Source < 0 || segment.Source >= len(sources) {
			return out, geom.ErrConvergence
		}
		face := sources[segment.Source]
		if e := appendTriangle([3]int{a, b, b + count}, face); e != nil {
			return out, e
		}
		if e := appendTriangle([3]int{a, b + count, a + count}, face); e != nil {
			return out, e
		}
		if boundary.SquaredErrors[i].Cmp(result.IdealBoundarySquaredDistance) > 0 {
			result.IdealBoundarySquaredDistance.Set(boundary.SquaredErrors[i])
		}
	}
	tolerance := qm(num(o.Deflection), num(o.Deflection))
	if result.IdealBoundarySquaredDistance.Cmp(tolerance) > 0 {
		return out, geom.ErrConvergence
	}
	if e := result.Mesh.ValidateSolid(); e != nil {
		return out, e
	}
	volume := new(big.Rat)
	point := func(p geom.Vec3) [3]*big.Rat { return [3]*big.Rat{num(p.X), num(p.Y), num(p.Z)} }
	for _, t := range result.Mesh.Triangles {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		a, b, c := point(result.Mesh.Vertices[t[0]]), point(result.Mesh.Vertices[t[1]]), point(result.Mesh.Vertices[t[2]])
		for i := 0; i < 3; i++ {
			j, k := (i+1)%3, (i+2)%3
			volume.Add(volume, qm(a[i], qs(qm(b[j], c[k]), qm(b[k], c[j]))))
		}
	}
	volume.Quo(volume, big.NewRat(6, 1))
	if volume.Sign() <= 0 {
		return out, geom.ErrDegenerate
	}
	result.SignedDisplayVolume = volume
	result.Collision, e = collision.SelfIntersectionsBVHContext(ctx, result.Mesh, o.Collision)
	if e != nil {
		return out, e
	}
	if !result.Collision.Complete || len(result.Collision.Contacts) != 0 {
		return out, geom.ErrConvergence
	}
	return result, nil
}
