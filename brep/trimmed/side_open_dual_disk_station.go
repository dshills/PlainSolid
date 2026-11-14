package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// SideOpenDualDiskStation is the exact closed material section perpendicular
// to the cutter axes. Outer is counterclockwise in the original local XY frame.
// CutterOwner is -1 for a stock edge or the original input cutter index.
// Empty is true outside the stock. At a blind floor the full box section
// remains material; Floor marks that geometric event.
type SideOpenDualDiskStation struct {
	Outer       []ExactQuadraticUV
	CutterOwner []int
	Empty       bool
	Floor       bool
}

// SectionAtStation intersects the original closed material with the plane at
// exact local Z. It returns the source circle arcs through the notch and the
// full rectangular loop at and above a blind floor. Tangencies to a stock end
// cap retain its actual closed material. Use the original cut's frame to map
// local section coordinates into world space.
func (c SideOpenDualDiskCut) SectionAtStation(ctx context.Context, localZ *big.Rat) (SideOpenDualDiskStation, error) {
	var out SideOpenDualDiskStation
	s := c.source
	if ctx == nil || s == nil || localZ == nil {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if localZ.Sign() < 0 || localZ.Cmp(ratInset(s.height)) > 0 {
		return SideOpenDualDiskStation{Empty: true}, nil
	}
	cmp := localZ.Cmp(ratInset(s.depth))
	if s.depth == s.height || cmp < 0 {
		out.Outer = append([]ExactQuadraticUV(nil), s.outer...)
		out.CutterOwner = append([]int(nil), s.owners...)
		return out, nil
	}
	out.Floor = cmp == 0
	a, b := ratInset(s.min.X), ratInset(s.max.X)
	d, f := ratInset(s.min.Y), ratInset(s.max.Y)
	corners := [4][2]*big.Rat{{a, d}, {b, d}, {b, f}, {a, f}}
	for i, start := range corners {
		line, e := quadraticLine(start, corners[(i+1)%4])
		if e != nil {
			return SideOpenDualDiskStation{}, e
		}
		out.Outer = append(out.Outer, line)
		out.CutterOwner = append(out.CutterOwner, -1)
	}
	return out, nil
}
