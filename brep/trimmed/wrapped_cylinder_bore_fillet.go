package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// WrappedCylinderBoreFilletHistory identifies the original circular bore and
// the rebuilt cap, wall and blend faces. SourceBoreRims lists the original
// cap-hole coedges in loop order. BlendRims indexes lower/upper, cap/wall,
// then the new canonical quadrants. A sharp end has no blend and retains
// its rim in the wall row.
type WrappedCylinderBoreFilletHistory struct {
	SourceOuter, SourceBore int
	SourceCaps              [2]int
	SourceBoreRims          [2][4]int
	Outer, Bore             int
	Caps, Blends            [2]int
	BlendRims               [2][2][4]int
}

// FilletWrappedCylinderBoreRims rounds the inside circular rims of a canonical
// through-bore between parallel circular cylinders, including eccentric axes.
// Use zero to leave an end sharp. Each rounded opening must stay strictly
// inside the original outer cylinder and the two radii must leave a positive
// cylindrical bore span. The blends are exact quarter-toroids. The original
// outer support, both cap planes and the bore's circumferential chart survive.
func FilletWrappedCylinderBoreRims(ctx context.Context, source Solid, lower, upper float64, o geom.SurfacePatchOptions) (Solid, WrappedCylinderBoreFilletHistory, error) {
	fail := func(err error) (Solid, WrappedCylinderBoreFilletHistory, error) {
		return Solid{}, WrappedCylinderBoreFilletHistory{}, err
	}
	if ctx == nil || !finite(lower) || !finite(upper) || lower < 0 || upper < 0 || lower == 0 && upper == 0 {
		return fail(geom.ErrInvalid)
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if len(source.faces) != 4 {
		return fail(geom.ErrConvergence)
	}
	p, err := source.ValidateWrappedEccentricCircularStationBore(ctx, o)
	if err != nil {
		return fail(err)
	}
	if len(p.OuterWalls) != 1 || len(p.InnerWalls) != 1 ||
		p.Outer.Radii[0].Cmp(p.Outer.Radii[1]) != 0 || p.Inner.Radii[0].Cmp(p.Inner.Radii[1]) != 0 {
		return fail(geom.ErrConvergence)
	}
	d := [2]*big.Rat{ratInset(lower), ratInset(upper)}
	z := p.Inner.Heights
	if new(big.Rat).Add(d[0], d[1]).Cmp(new(big.Rat).Sub(z[1], z[0])) >= 0 {
		return fail(geom.ErrDomain)
	}
	// Each cap radius is the maximum over its adjacent quarter-circle.
	// The larger radius therefore bounds the bore over its whole height.
	radius := p.Inner.Radii[0]
	largest := d[0]
	if d[1].Cmp(largest) > 0 {
		largest = d[1]
	}
	maxOpening := new(big.Rat).Add(radius, largest)
	envelope := p.Inner
	envelope.Radii = []*big.Rat{maxOpening, maxOpening}
	if err := proveEccentricCircularStationNesting(ctx, p.Outer, envelope); err != nil {
		return fail(err)
	}

	h := WrappedCylinderBoreFilletHistory{
		SourceOuter: p.OuterWalls[0], SourceBore: p.InnerWalls[0], SourceCaps: p.Caps,
		Blends: [2]int{-1, -1},
	}
	for end, cap := range p.Caps {
		for i, use := range source.faces[cap].Holes[0] {
			h.SourceBoreRims[end][i] = use.Edge
		}
	}
	outer, _, err := NewWrappedCylinder(ctx, source.faces[h.SourceOuter].Surface.(*geom.WrappedSurfacePatch), source.tolerance, o)
	if err != nil {
		return fail(err)
	}
	sharp, _, err := NewWrappedCylinder(ctx, source.faces[h.SourceBore].Surface.(*geom.WrappedSurfacePatch), source.tolerance, o)
	if err != nil {
		return fail(err)
	}
	bore, bh, err := filletWrappedCylinderRims(ctx, sharp, lower, upper, true, o)
	if err != nil {
		return fail(err)
	}
	boreFaces := [3]int{bh.Blends[0], bh.Lateral, bh.Blends[1]}
	inner := WrappedLoftHistory{CapFaces: bh.Caps}
	innerZ := []*big.Rat{z[0]}
	for i, face := range boreFaces {
		if face < 0 {
			continue
		}
		inner.WallFaces = append(inner.WallFaces, face)
		switch i {
		case 0:
			innerZ = append(innerZ, new(big.Rat).Add(z[0], d[0]))
		case 1:
			innerZ = append(innerZ, new(big.Rat).Sub(z[1], d[1]))
		case 2:
			innerZ = append(innerZ, z[1])
		}
	}
	stock := WrappedLoftHistory{WallFaces: []int{0}, CapFaces: [2]int{1, 2}}
	out, joined, err := joinProvedWrappedStationLoftBore(ctx, outer, bore, stock, inner, z, innerZ, o)
	if err != nil {
		return fail(err)
	}
	h.Outer, h.Caps = joined.Bands[0].ResultWall, joined.ResultCaps
	for _, b := range joined.InnerBands {
		switch b.SourceWalls[1] {
		case bh.Lateral:
			h.Bore = b.ResultWall
		case bh.Blends[0]:
			h.Blends[0] = b.ResultWall
		case bh.Blends[1]:
			h.Blends[1] = b.ResultWall
		}
	}
	h.BlendRims = bh.BlendRims
	for end := range h.BlendRims {
		for side := range h.BlendRims[end] {
			for i, id := range h.BlendRims[end][side] {
				if id >= 0 {
					h.BlendRims[end][side][i] = id + len(outer.edges)
				}
			}
		}
	}
	return out, h, nil
}
