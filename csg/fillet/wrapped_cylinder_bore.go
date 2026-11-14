package fillet

import (
	"context"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// WrappedRoundedCylinderBore retains every original vertex, edge and face ID.
// Only the cap hole lists change; BoreFace and BoreRims identify appended
// topology. SourceFaces is nil for the newly cut cylindrical wall. Original
// blends keep their exact support objects and face identities.
type WrappedRoundedCylinderBore struct {
	Solid                    trimmed.Solid
	SourceFaces              [][]int
	BoreFace                 int
	FloorFace                int      // -1 for a through-bore; generated support has no original face ancestor
	FloorZ                   *big.Rat // nil for a through-bore; owned exact floor height
	BoreRims                 [2][]int
	SourceCaps, SourceBlends [2]int
}

// BoreWrappedRoundedCylinder removes an axial circular through-bore from a
// validated canonical rim-filleted cylinder, possibly eccentric. The bore disk
// must lie strictly within both intact flat cap disks: if g=R-fillet-radius,
// require g>0 and g²>centerOffset² at each end. Every intervening outer section
// has at least these cap radii, proving the cut cannot reach an original blend.
//
// Shared original topology is never welded or rebuilt. New topology comes from
// an independently proved tube donor; exact cap chart equality, complete trim
// validation and model-edge agreement gate the graft. Native coordinates must
// be representable. Source/construction proofs each use supplied patch limits;
// final trim and edge proofs use their bounded defaults. Failure is atomic.
func BoreWrappedRoundedCylinder(ctx context.Context, source trimmed.Solid, center geom.Vec2, radius float64, o geom.SurfacePatchOptions) (WrappedRoundedCylinderBore, error) {
	return boreWrappedRoundedCylinder(ctx, source, center, radius, 0, false, false, o)
}

func boreWrappedRoundedCylinder(ctx context.Context, source trimmed.Solid, center geom.Vec2, radius, depth float64, fromUpper, blind bool, o geom.SurfacePatchOptions) (WrappedRoundedCylinderBore, error) {
	fail := func(e error) (WrappedRoundedCylinderBore, error) { return WrappedRoundedCylinderBore{}, e }
	if ctx == nil {
		return fail(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	r, cx, cy := new(big.Rat).SetFloat64(radius), new(big.Rat).SetFloat64(center.X), new(big.Rat).SetFloat64(center.Y)
	if r == nil || r.Sign() <= 0 || cx == nil || cy == nil {
		return fail(geom.ErrInvalid)
	}
	p, e := source.ValidateWrappedCylinderRimFillet(ctx, o)
	if e != nil {
		return fail(e)
	}
	if !p.Complete {
		return fail(geom.ErrConvergence)
	}
	dx, dy := new(big.Rat).Sub(cx, p.Center[0]), new(big.Rat).Sub(cy, p.Center[1])
	offsetSquared := new(big.Rat).Add(new(big.Rat).Mul(dx, dx), new(big.Rat).Mul(dy, dy))
	for _, blend := range p.Fillets {
		gap := new(big.Rat).Sub(new(big.Rat).Sub(p.Radius, blend), r)
		if gap.Sign() <= 0 || new(big.Rat).Mul(gap, gap).Cmp(offsetSquared) <= 0 {
			return fail(geom.ErrDomain)
		}
	}
	values := []*big.Rat{p.Center[0], p.Center[1], p.LowerZ, p.Radius, new(big.Rat).Sub(p.UpperZ, p.LowerZ)}
	f := make([]float64, len(values))
	for i, v := range values {
		f[i], e = geom.RepresentedExactScalar(v)
		if e != nil {
			return fail(e)
		}
	}
	outer, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{X: f[0], Y: f[1], Z: f[2]}, f[3], f[4], o)
	if e != nil {
		return fail(e)
	}
	innerZ, innerHeight := f[2], f[4]
	open := [2]bool{true, true}
	var floorZ *big.Rat
	floorEnd := 0
	if blind {
		d := new(big.Rat).SetFloat64(depth)
		if d == nil || d.Sign() <= 0 {
			return fail(geom.ErrInvalid)
		}
		floorZ = new(big.Rat).Add(p.LowerZ, d)
		open = [2]bool{true, false}
		floorEnd = 1
		if fromUpper {
			floorZ.Sub(p.UpperZ, d)
			open = [2]bool{false, true}
			floorEnd = 0
		}
		if floorZ.Cmp(new(big.Rat).Add(p.LowerZ, p.Fillets[0])) <= 0 || floorZ.Cmp(new(big.Rat).Sub(p.UpperZ, p.Fillets[1])) >= 0 {
			return fail(geom.ErrDomain)
		}
		innerHeight = depth
		if fromUpper {
			innerZ, e = geom.RepresentedExactScalar(floorZ)
			if e != nil {
				return fail(e)
			}
		}
	}
	inner, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{X: center.X, Y: center.Y, Z: innerZ}, radius, innerHeight, o)
	if e != nil {
		return fail(e)
	}
	donor, h, e := trimmed.NewWrappedEccentricCircularFrustumBore(ctx, outer, inner, source.Tolerance(), o)
	if e != nil {
		return fail(e)
	}
	vertices, edges, faces := source.Vertices(), source.Edges(), source.Faces()
	dv, de, df := donor.Vertices(), donor.Edges(), donor.Faces()
	wall := df[h.InnerFaces[0]]
	for end, cap := range p.Caps {
		if !open[end] {
			continue
		}
		a, ok := faces[cap].Surface.(geom.PlaneSurface)
		if !ok {
			return fail(geom.ErrConvergence)
		}
		b, ok := df[h.OuterFaces[end+1]].Surface.(geom.PlaneSurface)
		if !ok || a.Frame() != b.Frame() || faces[cap].Reversed != df[h.OuterFaces[end+1]].Reversed || len(df[h.OuterFaces[end+1]].Holes) != 1 {
			return fail(geom.ErrConvergence)
		}
	}
	usedE, usedV := make([]bool, len(de)), make([]bool, len(dv))
	for _, u := range wall.Outer {
		usedE[u.Edge] = true
		usedV[de[u.Edge].Start] = true
		usedV[de[u.Edge].End] = true
	}
	vm, em := make([]int, len(dv)), make([]int, len(de))
	for i := range em {
		em[i] = -1
	}
	for i, yes := range usedV {
		if yes {
			vm[i] = len(vertices)
			vertices = append(vertices, dv[i])
		}
	}
	for i, yes := range usedE {
		if yes {
			x := de[i]
			x.Start = vm[x.Start]
			x.End = vm[x.End]
			em[i] = len(edges)
			edges = append(edges, x)
		}
	}
	remap := func(loop []trimmed.Use) ([]trimmed.Use, error) {
		out := append([]trimmed.Use(nil), loop...)
		for i := range out {
			if out[i].Edge < 0 || out[i].Edge >= len(em) || em[out[i].Edge] < 0 {
				return nil, geom.ErrConvergence
			}
			out[i].Edge = em[out[i].Edge]
		}
		return out, nil
	}
	result := WrappedRoundedCylinderBore{FloorFace: -1, FloorZ: floorZ, SourceCaps: p.Caps, SourceBlends: p.Blends, BoreFace: len(faces), SourceFaces: make([][]int, len(faces)+1)}
	for i := range faces {
		result.SourceFaces[i] = []int{i}
	}
	for end, cap := range p.Caps {
		if !open[end] {
			continue
		}
		hole, e := remap(df[h.OuterFaces[end+1]].Holes[0])
		if e != nil {
			return fail(e)
		}
		faces[cap].Holes = append(faces[cap].Holes, hole)
		for _, u := range hole {
			result.BoreRims[end] = append(result.BoreRims[end], u.Edge)
		}
		proof, e := trimmed.ValidateFaceTrimLoops(ctx, faces[cap], trimvalidate.Options{})
		if e != nil || proof.Status != trimvalidate.Certified {
			if e == nil {
				e = geom.ErrConvergence
			}
			return fail(e)
		}
	}
	wall.Outer, e = remap(wall.Outer)
	if e != nil {
		return fail(e)
	}
	faces = append(faces, wall)
	if blind {
		floor := df[h.InnerFaces[floorEnd+1]]
		floor.Outer, e = remap(floor.Outer)
		if e != nil {
			return fail(e)
		}
		result.FloorFace = len(faces)
		for _, u := range floor.Outer {
			result.BoreRims[floorEnd] = append(result.BoreRims[floorEnd], u.Edge)
		}
		faces = append(faces, floor)
		result.SourceFaces = append(result.SourceFaces, nil)
	}
	result.Solid, e = trimmed.NewSolidContext(ctx, vertices, edges, faces, source.Tolerance())
	if e != nil {
		return fail(e)
	}
	agreement, e := result.Solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !agreement.Complete {
		if e == nil {
			e = geom.ErrConvergence
		}
		return fail(e)
	}
	return result, nil
}
