package offset

import (
	"context"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// ResizeWrappedCircularFrustumTube trims or extends a validated parallel-axis
// circular through-bore to new exact represented axial cap heights. Both wall
// equations, centers and circumferential parameterizations remain unchanged;
// the annular caps move and keep their original face ancestry. Unlike support
// setbacks, extensions need not be contained in the original body.
//
// Each radius is evaluated on its original affine function of model Z, even
// outside the old interval. Strict positive endpoint radii and positive radial
// differences whose squares exceed the squared axis separation prove a regular
// nonintersecting annular interval. Apex contact, wall contact, blind/cavity
// inputs and nonrepresentable native vertices fail atomically. Each validation,
// exact support edit and construction receives its own supplied work budget;
// there are two walls and no adaptive or unbounded loop.
func ResizeWrappedCircularFrustumTube(ctx context.Context, source trimmed.Solid, lower, upper float64, o geom.SurfacePatchOptions) (trimmed.Solid, FrustumTubeInsetHistory, error) {
	fail := func(e error) (trimmed.Solid, FrustumTubeInsetHistory, error) {
		return trimmed.Solid{}, FrustumTubeInsetHistory{}, e
	}
	if ctx == nil {
		return fail(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	heights := [2]*big.Rat{new(big.Rat).SetFloat64(lower), new(big.Rat).SetFloat64(upper)}
	if heights[0] == nil || heights[1] == nil {
		return fail(geom.ErrInvalid)
	}
	if heights[0].Cmp(heights[1]) >= 0 {
		return fail(geom.ErrDomain)
	}
	p, e := source.ValidateCanonicalWrappedFrustumBore(ctx, o)
	if e != nil {
		return fail(e)
	}
	if !p.Complete || p.Kind != "tube" {
		return fail(geom.ErrConvergence)
	}
	supports := [2]*geom.WrappedSurfacePatch{p.Outer, p.Inner}
	proofs := [2]geom.WrappedFrustumEmbedding{}
	radii := [2][2]*big.Rat{}
	for side, w := range supports {
		proofs[side], e = geom.CertifyWrappedFrustumEmbedding(ctx, w, o)
		if e != nil {
			return fail(e)
		}
		q := proofs[side]
		if !q.Circular {
			return fail(geom.ErrConvergence)
		}
		slope := new(big.Rat).Quo(new(big.Rat).Sub(q.UpperRadius, q.LowerRadius), new(big.Rat).Sub(q.UpperZ, q.LowerZ))
		for i, z := range heights {
			radii[side][i] = new(big.Rat).Add(q.LowerRadius, new(big.Rat).Mul(slope, new(big.Rat).Sub(z, q.LowerZ)))
			if radii[side][i].Sign() <= 0 {
				return fail(geom.ErrDomain)
			}
		}
	}
	if proofs[0].LowerZ.Cmp(proofs[1].LowerZ) != 0 || proofs[0].UpperZ.Cmp(proofs[1].UpperZ) != 0 {
		return fail(geom.ErrConvergence)
	}
	dx, dy := new(big.Rat).Sub(proofs[0].CenterX, proofs[1].CenterX), new(big.Rat).Sub(proofs[0].CenterY, proofs[1].CenterY)
	axisSquared := new(big.Rat).Add(new(big.Rat).Mul(dx, dx), new(big.Rat).Mul(dy, dy))
	clearance := [2]*big.Rat{}
	for i := range clearance {
		clearance[i] = new(big.Rat).Sub(radii[0][i], radii[1][i])
		if clearance[i].Sign() <= 0 || new(big.Rat).Mul(clearance[i], clearance[i]).Cmp(axisSquared) <= 0 {
			return fail(geom.ErrDomain)
		}
	}
	for side, w := range supports {
		q := proofs[side]
		cylinder, _, e := geom.DraftWrappedFrustumExact(ctx, w, new(big.Rat), new(big.Rat).Sub(q.LowerRadius, q.UpperRadius), o)
		if e != nil {
			return fail(e)
		}
		cylinder, e = geom.ReheightWrappedCylinder(ctx, cylinder, heights[0], heights[1], o)
		if e != nil {
			return fail(e)
		}
		supports[side], _, e = geom.DraftWrappedFrustumExact(ctx, cylinder, new(big.Rat).Sub(radii[side][0], q.LowerRadius), new(big.Rat).Sub(radii[side][1], q.LowerRadius), o)
		if e != nil {
			return fail(e)
		}
	}
	out, h, e := trimmed.NewWrappedEccentricCircularFrustumBore(ctx, supports[0], supports[1], source.Tolerance(), o)
	if e != nil {
		return fail(e)
	}
	history := FrustumTubeInsetHistory{SourceFaces: make([][]int, len(out.Faces())), OuterFaces: h.OuterFaces, InnerFaces: h.InnerFaces, Heights: heights, OuterRadii: radii[0], InnerRadii: radii[1], RadialClearance: clearance}
	for i := 0; i < 3; i++ {
		for _, pair := range [][2]int{{h.OuterFaces[i], p.OuterFaces[i]}, {h.InnerFaces[i], p.InnerFaces[i]}} {
			if len(history.SourceFaces[pair[0]]) != 0 && history.SourceFaces[pair[0]][0] != pair[1] {
				return fail(geom.ErrConvergence)
			}
			history.SourceFaces[pair[0]] = []int{pair[1]}
		}
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	return out, history, nil
}
