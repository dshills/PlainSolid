package fillet

import (
	"context"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// ChamferWrappedFrustumRims replaces selected sharp circular cap rims of a
// canonical circular frustum with exact conical bands. Radial setback is
// measured on the original cap; axial runout ends on the ORIGINAL sloped wall.
// The central original wall equation, circular basis and cap planes are kept.
// Each selected band's removed radial width varies affinely from the authored
// setback to zero at its shoulder, proving the chamfer stays within the stock.
// Setbacks must be below their own cap radii and summed runouts below height.
// Apex, touching/overlapping runouts and nonrepresentable native output fail
// atomically. Source/rim ancestry and per-call budgets match the cylinder API,
// which remains unchanged. No normal-length square root is required.
func ChamferWrappedFrustumRims(ctx context.Context, source trimmed.Solid, lower, upper CircularRimChamfer, o geom.SurfacePatchOptions) (WrappedCylinderChamfer, error) {
	fail := func(e error) (WrappedCylinderChamfer, error) { return WrappedCylinderChamfer{}, e }
	if ctx == nil {
		return fail(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	w, p, e := source.ValidateCanonicalWrappedFrustum(ctx, o)
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
		if radial[i].Cmp([2]*big.Rat{p.LowerRadius, p.UpperRadius}[i]) >= 0 {
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
	slope := new(big.Rat).Quo(new(big.Rat).Sub(p.UpperRadius, p.LowerRadius), new(big.Rat).Sub(p.UpperZ, p.LowerZ))
	radiusAt := func(z *big.Rat) *big.Rat {
		return new(big.Rat).Add(p.LowerRadius, new(big.Rat).Mul(slope, new(big.Rat).Sub(z, p.LowerZ)))
	}
	base, _, e := geom.DraftWrappedFrustumExact(ctx, w, new(big.Rat), new(big.Rat).Sub(p.LowerRadius, p.UpperRadius), o)
	if e != nil {
		return fail(e)
	}
	z := []*big.Rat{p.LowerZ}
	r := []*big.Rat{new(big.Rat).Sub(p.LowerRadius, radial[0])}
	if axial[0].Sign() > 0 {
		z = append(z, new(big.Rat).Add(p.LowerZ, axial[0]))
		r = append(r, radiusAt(z[len(z)-1]))
	}
	if axial[1].Sign() > 0 {
		z = append(z, new(big.Rat).Sub(p.UpperZ, axial[1]))
		r = append(r, radiusAt(z[len(z)-1]))
	}
	z = append(z, p.UpperZ)
	r = append(r, new(big.Rat).Sub(p.UpperRadius, radial[1]))
	bands := make([]*geom.WrappedSurfacePatch, len(z)-1)
	for i := range bands {
		band, e := geom.ReheightWrappedCylinder(ctx, base, z[i], z[i+1], o)
		if e != nil {
			return fail(e)
		}
		bands[i], _, e = geom.DraftWrappedFrustumExact(ctx, band, new(big.Rat).Sub(r[i], p.LowerRadius), new(big.Rat).Sub(r[i+1], p.LowerRadius), o)
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
