package fillet

import (
	"context"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// CertifiedBeveledPocketMesh bounds both directions to the ideal rounded stock,
// conical bevel, bore and blind pocket. It does not assert exact sewing of the
// independently rounded native analytic charts. All shared rims use one index.
type CertifiedBeveledPocketMesh struct {
	Model                        BoredBeveledRectanglePocket
	Mesh                         mesh.Mesh
	OriginalFace                 []int
	Caps                         [2]trimmed.CertifiedAffineQuadraticTrimMesh
	IdealBoundarySquaredDistance *big.Rat
	Collision                    collision.Report
}

// PrismRectangleTopBeveledBoredPocketCertified uses sixteen fixed rational
// parameter intervals per outer quarter arc before cap certification. Further
// adaptive partitions must match across bevel endpoints; otherwise it rejects.
// A ruled-band interpolation bound covers both directed surface distances;
// exact global mesh incidence and contact checks are mandatory. Failure is atomic.
func PrismRectangleTopBeveledBoredPocketCertified(ctx context.Context, min, max geom.Vec2, zMin, zMax, radius, inset, rise, boreRadius float64, pocket RectanglePocket, o RectangleMeshOptions) (out CertifiedBeveledPocketMesh, err error) {
	defer func() {
		if err != nil {
			out = CertifiedBeveledPocketMesh{}
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
	model, e := PrismRectangleTopBeveledBoredPocket(ctx, min, max, zMin, zMax, radius, inset, rise, boreRadius, pocket, o.Model)
	if e != nil {
		return out, e
	}
	selection := []CornerRadius{{Corner: 0, Radius: radius}, {Corner: 1, Radius: radius}, {Corner: 2, Radius: radius}, {Corner: 3, Radius: radius}}
	outer, sources, patch, e := rectangleCertifiedProfile(min, max, selection, model.Stock.Stock.Lower)
	if e != nil {
		return out, e
	}
	topMin := geom.Vec2{}
	topMax := geom.Vec2{}
	topMin.X, e = geom.RepresentedExactScalar(qa(num(min.X), num(inset)))
	if e != nil {
		return out, e
	}
	topMin.Y, e = geom.RepresentedExactScalar(qa(num(min.Y), num(inset)))
	if e != nil {
		return out, e
	}
	topMax.X, e = geom.RepresentedExactScalar(qs(num(max.X), num(inset)))
	if e != nil {
		return out, e
	}
	topMax.Y, e = geom.RepresentedExactScalar(qs(num(max.Y), num(inset)))
	if e != nil {
		return out, e
	}
	topRadius, e := geom.RepresentedExactScalar(qs(num(radius), num(inset)))
	if e != nil {
		return out, e
	}
	topSelection := []CornerRadius{{Corner: 0, Radius: topRadius}, {Corner: 1, Radius: topRadius}, {Corner: 2, Radius: topRadius}, {Corner: 3, Radius: topRadius}}
	topOuter, _, topPatch, e := rectangleCertifiedProfile(topMin, topMax, topSelection, model.Stock.Stock.Lower)
	if e != nil {
		return out, e
	}
	// Fixed dyadic source cuts precede certification, not polygon chord splits.
	// Remaining adaptive partitions must match exactly or the operation rejects.
	partition := func(in []trimmed.SplineUV) ([]trimmed.SplineUV, []int) {
		var out []trimmed.SplineUV
		var owners []int
		for i, c := range in {
			n := 1
			if i%2 == 1 {
				n = 16
			}
			for j := 0; j < n; j++ {
				q := c
				q.Range = trimmed.UVInterval{Min: float64(j) / float64(n), Max: float64(j+1) / float64(n)}
				out = append(out, q)
				owners = append(owners, i)
			}
		}
		return out, owners
	}
	outer, sourceMap := partition(outer)
	topOuter, _ = partition(topOuter)
	bore, e := certifiedCircleProfile(model.Stock.Center, boreRadius)
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
		capPatch, capOuter := patch, outer
		if cap == 1 {
			capPatch, capOuter = topPatch, topOuter
		}
		proof, e := trimmed.TessellateAffineQuadraticTrimRegionCertified(ctx, capPatch, capOuter, holes, profile)
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
			if e := triangle(a, b, c, model.Stock.Stock.Lower.Caps[cap]); e != nil {
				return out, e
			}
		}
	}
	// Full-height boundaries must have identical exact segment sets on both caps.
	for loop := 1; loop < 2; loop++ {
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
	if e := walls(out.Caps[0].Region.Loops[0].Boundary, zMin, model.Stock.Stock.ShoulderZ, sources, sourceMap, false); e != nil {
		return out, e
	}
	lowerBoundary, topBoundary := out.Caps[0].Region.Loops[0].Boundary, out.Caps[1].Region.Loops[0].Boundary
	if len(lowerBoundary.Segments) != len(topBoundary.Segments) {
		return out, geom.ErrConvergence
	}
	for i, s := range lowerBoundary.Segments {
		t := topBoundary.Segments[i]
		if s.Source != t.Source || s.Start != t.Start || s.End != t.End {
			return out, geom.ErrConvergence
		}
		a, b := lowerBoundary.ExactUV[s.Vertices[0]], lowerBoundary.ExactUV[s.Vertices[1]]
		d, c := topBoundary.ExactUV[t.Vertices[0]], topBoundary.ExactUV[t.Vertices[1]]
		v0, v1, v2, v3 := vertex(a, model.Stock.Stock.ShoulderZ), vertex(b, model.Stock.Stock.ShoulderZ), vertex(c, zMax), vertex(d, zMax)
		// Profile sequence is straight0,corner1,...; native bevel is corner0,straight0,...
		source := sourceMap[s.Source]
		band := 0
		if source%2 == 0 {
			band = source + 1
		} else {
			band = (source + 1) % 8
		}
		face := model.Stock.Stock.BevelFaces[band]
		if e := triangle(v0, v1, v2, face); e != nil {
			return out, e
		}
		if e := triangle(v0, v2, v3, face); e != nil {
			return out, e
		}
		// Source versus displayed bilinear strip is bounded by the larger endpoint
		// chord bound. Its two-triangle interpolant differs by at most |twist|/4.
		twist := new(big.Rat)
		for k := 0; k < 3; k++ {
			coords := func(id int) float64 { p := out.Mesh.Vertices[id]; return [3]float64{p.X, p.Y, p.Z}[k] }
			x := qa(qs(num(coords(v0)), num(coords(v1))), qs(num(coords(v2)), num(coords(v3))))
			twist.Add(twist, qm(x, x))
		}
		bound := new(big.Rat).Set(lowerBoundary.SquaredErrors[i])
		if topBoundary.SquaredErrors[i].Cmp(bound) > 0 {
			bound.Set(topBoundary.SquaredErrors[i])
		}
		bound = qm(num(2), qa(bound, new(big.Rat).Quo(twist, num(16))))
		if bound.Cmp(out.IdealBoundarySquaredDistance) > 0 {
			out.IdealBoundarySquaredDistance.Set(bound)
		}
	}
	if e := walls(out.Caps[0].Region.Loops[1].Boundary, zMin, zMax, model.Stock.BoreFaces[:], out.Caps[0].Region.OriginalSource[1], true); e != nil {
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
