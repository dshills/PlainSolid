package fillet

import (
	"context"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// CircularRimChamfer specifies exact represented radial cap setback and axial
// wall runout. Both zero leaves an end sharp; otherwise both must be positive.
type CircularRimChamfer struct{ Radial, Axial float64 }

type WrappedCylinderChamfer struct {
	Solid                 trimmed.Solid
	SourceFaces           [][]int
	Lateral               int
	Caps, Chamfers        [2]int // Chamfers is -1 for an unselected end
	SourceRims            [2][]int
	CapRims, ShoulderRims [2][4]int
}

// ChamferWrappedCylinderRims replaces selected sharp circular end rims by exact
// conical bands. The original circular basis/weights and cap planes are retained;
// the central wall has the unchanged cylinder equation. Every positive radial
// setback is strictly below radius, and summed axial runouts strictly below
// height. These inequalities prove positive radii and a nonempty central band,
// excluding apex collapse and intersecting or merely touching chamfers.
//
// SourceFaces maps each retained cap/wall to its original face and each new
// conical band to its original cap plus wall. SourceRims identify removed or
// retained original rim edges; CapRims/ShoulderRims are new topology, not claims
// that trimmed original edges survived. No original topology IDs are promised.
// Source validation, each of at most three exact band edits and native joining
// use the supplied per-call proof limits. Failures return no partial result.
func ChamferWrappedCylinderRims(ctx context.Context, source trimmed.Solid, lower, upper CircularRimChamfer, o geom.SurfacePatchOptions) (WrappedCylinderChamfer, error) {
	fail := func(e error) (WrappedCylinderChamfer, error) { return WrappedCylinderChamfer{}, e }
	if ctx == nil {
		return fail(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	w, p, e := source.ValidateCanonicalWrappedCylinder(ctx, o)
	if e != nil {
		return fail(e)
	}
	if !p.Circular {
		return fail(geom.ErrConvergence)
	}
	selections := [2]CircularRimChamfer{lower, upper}
	radial, axial := [2]*big.Rat{}, [2]*big.Rat{}
	for i, c := range selections {
		radial[i], axial[i] = new(big.Rat).SetFloat64(c.Radial), new(big.Rat).SetFloat64(c.Axial)
		if radial[i] == nil || axial[i] == nil || radial[i].Sign() < 0 || axial[i].Sign() < 0 || (radial[i].Sign() == 0) != (axial[i].Sign() == 0) {
			return fail(geom.ErrInvalid)
		}
		if radial[i].Cmp(p.Radius) >= 0 {
			return fail(geom.ErrDomain)
		}
	}
	if axial[0].Sign() == 0 && axial[1].Sign() == 0 {
		return fail(geom.ErrInvalid)
	}
	if new(big.Rat).Add(axial[0], axial[1]).Cmp(new(big.Rat).Sub(p.UpperZ, p.LowerZ)) >= 0 {
		return fail(geom.ErrDomain)
	}
	oldWall, oldCaps := -1, [2]int{-1, -1}
	sf := source.Faces()
	for i, f := range sf {
		switch chart := f.Surface.(type) {
		case *geom.WrappedSurfacePatch:
			if chart == w {
				oldWall = i
			}
		case geom.PlaneSurface:
			z := new(big.Rat).SetFloat64(chart.Frame().Origin().Z)
			if z.Cmp(p.LowerZ) == 0 {
				oldCaps[0] = i
			}
			if z.Cmp(p.UpperZ) == 0 {
				oldCaps[1] = i
			}
		}
	}
	if oldWall < 0 || oldCaps[0] < 0 || oldCaps[1] < 0 {
		return fail(geom.ErrConvergence)
	}
	result := WrappedCylinderChamfer{Chamfers: [2]int{-1, -1}}
	for end, cap := range oldCaps {
		for _, a := range sf[cap].Outer {
			for _, b := range sf[oldWall].Outer {
				if a.Edge == b.Edge {
					result.SourceRims[end] = append(result.SourceRims[end], a.Edge)
				}
			}
		}
		if len(result.SourceRims[end]) != 4 {
			return fail(geom.ErrConvergence)
		}
	}
	z := []*big.Rat{p.LowerZ}
	r := []*big.Rat{new(big.Rat).Sub(p.Radius, radial[0])}
	if axial[0].Sign() > 0 {
		z = append(z, new(big.Rat).Add(p.LowerZ, axial[0]))
		r = append(r, p.Radius)
	}
	if axial[1].Sign() > 0 {
		z = append(z, new(big.Rat).Sub(p.UpperZ, axial[1]))
		r = append(r, p.Radius)
	}
	z = append(z, p.UpperZ)
	r = append(r, new(big.Rat).Sub(p.Radius, radial[1]))
	bands := make([]*geom.WrappedSurfacePatch, len(z)-1)
	for i := range bands {
		band, e := geom.ReheightWrappedCylinder(ctx, w, z[i], z[i+1], o)
		if e != nil {
			return fail(e)
		}
		bands[i], _, e = geom.DraftWrappedFrustumExact(ctx, band, new(big.Rat).Sub(r[i], p.Radius), new(big.Rat).Sub(r[i+1], p.Radius), o)
		if e != nil {
			return fail(e)
		}
	}
	out, h, e := trimmed.NewWrappedCircularStationLoft(ctx, bands, source.Tolerance(), o)
	if e != nil {
		return fail(e)
	}
	result.Solid = out
	result.Caps = h.CapFaces
	result.SourceFaces = make([][]int, len(out.Faces()))
	middle := 0
	if axial[0].Sign() > 0 {
		middle = 1
	}
	result.Lateral = h.WallFaces[middle]
	result.SourceFaces[result.Lateral] = []int{oldWall}
	for end, cap := range result.Caps {
		result.SourceFaces[cap] = []int{oldCaps[end]}
		station := 0
		if end == 1 {
			station = len(z) - 1
		}
		result.CapRims[end] = h.StationRims[station]
		result.ShoulderRims[end] = result.CapRims[end]
		if axial[end].Sign() > 0 {
			band := 0
			shoulder := 1
			if end == 1 {
				band = len(bands) - 1
				shoulder = len(z) - 2
			}
			result.Chamfers[end] = h.WallFaces[band]
			result.SourceFaces[result.Chamfers[end]] = []int{oldCaps[end], oldWall}
			result.ShoulderRims[end] = h.StationRims[shoulder]
		}
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	return result, nil
}
