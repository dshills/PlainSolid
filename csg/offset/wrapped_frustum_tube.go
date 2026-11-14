package offset

import (
	"context"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// FrustumTubeInsets specifies independent inward material setbacks. OuterWall
// shrinks the external support; InnerWall expands the bore. Caps move axially.
type FrustumTubeInsets struct{ OuterWall, InnerWall, LowerCap, UpperCap float64 }

// FrustumTubeInsetHistory retains original native faces and the two endpoint
// clearance witnesses. Strict endpoint radial gaps prove separation throughout
// the retained interval since both circular wall radii are affine in height.
type FrustumTubeInsetHistory struct {
	SourceFaces                                      [][]int
	OuterFaces, InnerFaces                           [3]int
	Heights, OuterRadii, InnerRadii, RadialClearance [2]*big.Rat
}

// InsetWrappedCircularFrustumTube moves both walls and caps of a revalidated
// native coaxial circular through-bore. It preserves a strict annular section
// throughout; tangency, collapse, eccentric axes, blind bores and cavities fail
// atomically. Unequal support distances are not a uniform Euclidean erosion.
// Normal scales and native geometry must be exactly representable. Work is a
// fixed sequence of source validation, two support inset calls, an inner draft
// and native bore construction; each called proof uses its supplied limits.
func InsetWrappedCircularFrustumTube(ctx context.Context, source trimmed.Solid, in FrustumTubeInsets, o geom.SurfacePatchOptions) (trimmed.Solid, FrustumTubeInsetHistory, error) {
	fail := func(e error) (trimmed.Solid, FrustumTubeInsetHistory, error) {
		return trimmed.Solid{}, FrustumTubeInsetHistory{}, e
	}
	if ctx == nil {
		return fail(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	for _, x := range []float64{in.OuterWall, in.InnerWall, in.LowerCap, in.UpperCap} {
		r := new(big.Rat).SetFloat64(x)
		if r == nil || r.Sign() < 0 {
			return fail(geom.ErrInvalid)
		}
	}
	p, e := source.ValidateCanonicalWrappedFrustumBore(ctx, o)
	if e != nil {
		return fail(e)
	}
	if !p.Complete || p.Kind != "tube" {
		return fail(geom.ErrConvergence)
	}
	outer, op, e := trimmed.NewWrappedFrustum(ctx, p.Outer, source.Tolerance(), o)
	if e != nil {
		return fail(e)
	}
	inner, ip, e := trimmed.NewWrappedFrustum(ctx, p.Inner, source.Tolerance(), o)
	if e != nil {
		return fail(e)
	}
	if op.CenterX.Cmp(ip.CenterX) != 0 || op.CenterY.Cmp(ip.CenterY) != 0 || op.LowerZ.Cmp(ip.LowerZ) != 0 || op.UpperZ.Cmp(ip.UpperZ) != 0 {
		return fail(geom.ErrConvergence)
	}
	outer, oh, e := InsetWrappedCircularFrustumSupports(ctx, outer, FrustumSupportInsets{in.OuterWall, in.LowerCap, in.UpperCap}, o)
	if e != nil {
		return fail(e)
	}
	inner, ih, e := InsetWrappedCircularFrustumSupports(ctx, inner, FrustumSupportInsets{0, in.LowerCap, in.UpperCap}, o)
	if e != nil {
		return fail(e)
	}
	move := new(big.Rat).Mul(new(big.Rat).SetFloat64(in.InnerWall), ih.NormalLength)
	innerRadii, clearance := [2]*big.Rat{}, [2]*big.Rat{}
	for i := range innerRadii {
		innerRadii[i] = new(big.Rat).Add(ih.Radii[i], move)
		clearance[i] = new(big.Rat).Sub(oh.Radii[i], innerRadii[i])
		if clearance[i].Sign() <= 0 {
			return fail(geom.ErrDomain)
		}
	}
	ow := outer.Faces()[oh.Lateral].Surface.(*geom.WrappedSurfacePatch)
	iw := inner.Faces()[ih.Lateral].Surface.(*geom.WrappedSurfacePatch)
	iw, _, e = geom.DraftWrappedFrustumExact(ctx, iw, move, move, o)
	if e != nil {
		return fail(e)
	}
	out, h, e := trimmed.NewWrappedCircularFrustumBore(ctx, ow, iw, source.Tolerance(), o)
	if e != nil {
		return fail(e)
	}
	r := FrustumTubeInsetHistory{SourceFaces: make([][]int, len(out.Faces())), OuterFaces: h.OuterFaces, InnerFaces: h.InnerFaces, Heights: oh.Heights, OuterRadii: oh.Radii, InnerRadii: innerRadii, RadialClearance: clearance}
	for i := 0; i < 3; i++ {
		for _, pair := range [][2]int{{h.OuterFaces[i], p.OuterFaces[i]}, {h.InnerFaces[i], p.InnerFaces[i]}} {
			if len(r.SourceFaces[pair[0]]) != 0 && r.SourceFaces[pair[0]][0] != pair[1] {
				return fail(geom.ErrConvergence)
			}
			r.SourceFaces[pair[0]] = []int{pair[1]}
		}
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	return out, r, nil
}
