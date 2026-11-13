package fillet

import (
	"context"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// CertifiedBoredPocketMesh bounds both directions to the ideal circular
// construction, not exact sewing of rounded native analytic charts. Caps retain
// independent fill proofs; every shared rim has one represented vertex index.
type CertifiedBoredPocketMesh struct {
	Model                        BoredRectanglePocket
	Mesh                         mesh.Mesh
	OriginalFace                 []int
	Caps                         [2]trimmed.CertifiedAffineQuadraticTrimMesh
	IdealBoundarySquaredDistance *big.Rat
	Collision                    collision.Report
}

func certifiedCircleProfile(center geom.Vec2, r float64) ([]trimmed.SplineUV, error) {
	var points [4]geom.Vec3
	for i, d := range [4]geom.Vec2{{X: 1}, {Y: -1}, {X: -1}, {Y: 1}} {
		x, e := geom.RepresentedExactScalar(qa(num(center.X), qm(num(r), num(d.X))))
		if e != nil {
			return nil, e
		}
		y, e := geom.RepresentedExactScalar(qa(num(center.Y), qm(num(r), num(d.Y))))
		if e != nil {
			return nil, e
		}
		points[i] = geom.Vec3{X: x, Y: y}
	}
	var out []trimmed.SplineUV
	for i, a := range points {
		b := points[(i+1)%4]
		corner := geom.Vec3{X: a.X, Y: b.Y}
		if i%2 == 1 {
			corner = geom.Vec3{X: b.X, Y: a.Y}
		}
		c, e := geom.NewBSpline(2, []geom.Vec3{a, corner, b}, []float64{0, 0, 0, 1, 1, 1}, []float64{1, 1, 2})
		if e != nil {
			return nil, e
		}
		out = append(out, trimmed.SplineUV{Curve: c, Range: trimmed.UVInterval{Max: 1}})
	}
	return out, nil
}

// PrismRectangleCornersBoredPocketCertified is an opt-in bounded companion to
// the native constructor. Its ideal circle arcs are exact rational conics.
// Pocket floors use the same convex chord ring as their walls: radial segments
// from the exact center extend the boundary correspondence across the disk,
// without increasing its error. Positive quadrant conics and their chords cover
// that disk and convex polygon respectively. Cap homotopy and
// vertical boundary correspondence bound all faces; exact contact testing is
// mandatory. Failure is atomic, including unsynchronized adaptive boundaries.
func PrismRectangleCornersBoredPocketCertified(ctx context.Context, min, max geom.Vec2, zMin, zMax, boreRadius float64, selection []CornerRadius, pocket RectanglePocket, o RectangleMeshOptions) (out CertifiedBoredPocketMesh, err error) {
	defer func() {
		if err != nil {
			out = CertifiedBoredPocketMesh{}
		}
	}()
	if !finite(o.Deflection) || o.Deflection <= 0 {
		return out, ErrInvalid
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 100000
	}
	if o.MaxTriangles < 1 || o.MaxTriangles > 1000000 {
		return out, ErrInvalid
	}
	model, e := PrismRectangleCornersBoredPocket(ctx, min, max, zMin, zMax, boreRadius, selection, pocket, o.Model)
	if e != nil {
		return out, e
	}
	outer, sources, patch, e := rectangleCertifiedProfile(min, max, selection, model.Bore.Stock)
	if e != nil {
		return out, e
	}
	bore, e := certifiedCircleProfile(model.Bore.Center, boreRadius)
	if e != nil {
		return out, e
	}
	blind, e := certifiedCircleProfile(pocket.Center, pocket.Radius)
	if e != nil {
		return out, e
	}
	out.Model = model
	out.IdealBoundarySquaredDistance = new(big.Rat)
	profile := o.Profile
	profile.Region.Tessellation.Deflection = o.Deflection / 4
	if profile.MaxTriangles == 0 || profile.MaxTriangles > o.MaxTriangles {
		profile.MaxTriangles = o.MaxTriangles
	}
	ids := map[string]int{}
	key := func(p [2]*big.Rat, z float64) string {
		return p[0].RatString() + "," + p[1].RatString() + "," + num(z).RatString()
	}
	vertex := func(p [2]*big.Rat, z float64) int {
		k := key(p, z)
		if i, ok := ids[k]; ok {
			return i
		}
		x, _ := p[0].Float64()
		y, _ := p[1].Float64()
		i := len(out.Mesh.Vertices)
		ids[k] = i
		out.Mesh.Vertices = append(out.Mesh.Vertices, geom.Vec3{X: x, Y: y, Z: z})
		return i
	}
	triangle := func(a, b, c, face int) error {
		if len(out.Mesh.Triangles) >= o.MaxTriangles {
			return ErrLimit
		}
		out.Mesh.Triangles = append(out.Mesh.Triangles, [3]int{a, b, c})
		out.OriginalFace = append(out.OriginalFace, face)
		return nil
	}
	for cap, z := range []float64{zMin, zMax} {
		holes := [][]trimmed.SplineUV{bore}
		if cap == pocket.EntryCap {
			holes = append(holes, blind)
		}
		proof, e := trimmed.TessellateAffineQuadraticTrimRegionCertified(ctx, patch, outer, holes, profile)
		if e != nil {
			return out, e
		}
		out.Caps[cap] = proof
		if proof.FillSquaredDistance.Cmp(out.IdealBoundarySquaredDistance) > 0 {
			out.IdealBoundarySquaredDistance.Set(proof.FillSquaredDistance)
		}
		for _, t := range proof.Mesh.Triangles {
			a, b, c := vertex(proof.ExactUV[t[0]], z), vertex(proof.ExactUV[t[1]], z), vertex(proof.ExactUV[t[2]], z)
			if cap == 0 {
				b, c = c, b
			}
			if e := triangle(a, b, c, model.Bore.Stock.Caps[cap]); e != nil {
				return out, e
			}
		}
	}
	// Full-height boundaries must have identical exact segment sets on both caps.
	for loop := 0; loop < 2; loop++ {
		a, b := out.Caps[0].Region.Loops[loop].Boundary, out.Caps[1].Region.Loops[loop].Boundary
		if len(a.Segments) != len(b.Segments) {
			return out, geom.ErrConvergence
		}
		for i, s := range a.Segments {
			t := b.Segments[i]
			for j := 0; j < 2; j++ {
				if key(a.ExactUV[s.Vertices[j]], 0) != key(b.ExactUV[t.Vertices[j]], 0) {
					return out, geom.ErrConvergence
				}
			}
		}
	}
	walls := func(bound trimmed.CertifiedQuadraticTrimLoop, z0, z1 float64, faceIDs []int, mapping []int, reverse bool) error {
		for i, s := range bound.Segments {
			a, b := bound.ExactUV[s.Vertices[0]], bound.ExactUV[s.Vertices[1]]
			if reverse {
				a, b = b, a
			}
			face := s.Source
			if mapping != nil {
				face = mapping[face]
			}
			v0, v1, v2, v3 := vertex(a, z0), vertex(b, z0), vertex(b, z1), vertex(a, z1)
			if e := triangle(v0, v1, v2, faceIDs[face]); e != nil {
				return e
			}
			if e := triangle(v0, v2, v3, faceIDs[face]); e != nil {
				return e
			}
			if bound.SquaredErrors[i].Cmp(out.IdealBoundarySquaredDistance) > 0 {
				out.IdealBoundarySquaredDistance.Set(bound.SquaredErrors[i])
			}
		}
		return nil
	}
	if e := walls(out.Caps[0].Region.Loops[0].Boundary, zMin, zMax, sources, nil, false); e != nil {
		return out, e
	}
	if e := walls(out.Caps[0].Region.Loops[1].Boundary, zMin, zMax, model.Bore.BoreFaces[:], out.Caps[0].Region.OriginalSource[1], true); e != nil {
		return out, e
	}
	entry := zMin
	sign := 1.0
	if pocket.EntryCap == 1 {
		entry = zMax
		sign = -1
	}
	floor, e := geom.RepresentedExactScalar(qa(num(entry), qm(num(sign), num(pocket.Depth))))
	if e != nil {
		return out, e
	}
	ring := out.Caps[pocket.EntryCap].Region.Loops[2].Boundary
	lo, hi := entry, floor
	if lo > hi {
		lo, hi = hi, lo
	}
	if e := walls(ring, lo, hi, model.WallFaces[:], out.Caps[pocket.EntryCap].Region.OriginalSource[2], true); e != nil {
		return out, e
	}
	center := vertex([2]*big.Rat{num(pocket.Center.X), num(pocket.Center.Y)}, floor)
	for _, s := range ring.Segments {
		a, b := vertex(ring.ExactUV[s.Vertices[0]], floor), vertex(ring.ExactUV[s.Vertices[1]], floor)
		if pocket.EntryCap == 0 {
			a, b = b, a
		}
		if e := triangle(center, a, b, model.FloorFace); e != nil {
			return out, e
		}
	}
	if out.IdealBoundarySquaredDistance.Cmp(qm(num(o.Deflection), num(o.Deflection))) > 0 {
		return out, geom.ErrConvergence
	}
	if e := out.Mesh.ValidateSolid(); e != nil {
		return out, e
	}
	if out.Mesh.SignedVolume() <= 0 {
		return out, geom.ErrDegenerate
	}
	out.Collision, e = collision.SelfIntersectionsBVHContext(ctx, out.Mesh, o.Collision)
	if e != nil {
		return out, e
	}
	if !out.Collision.Complete || len(out.Collision.Contacts) > 0 {
		return out, geom.ErrConvergence
	}
	return out, nil
}
