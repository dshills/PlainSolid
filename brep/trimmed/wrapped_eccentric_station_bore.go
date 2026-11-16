package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// CutWrappedEccentricCircularStationLofts subtracts one complete circular
// station loft from another with shared end planes and parallel, possibly
// offset axes. Strict nesting is proved at every station of either loft;
// the radial gap is affine between consecutive stations. Both axes, wall
// charts and original annular cap trims are retained.
func CutWrappedEccentricCircularStationLofts(ctx context.Context, outer, inner Solid, o geom.SurfacePatchOptions) (Solid, WrappedStationBooleanHistory, error) {
	a, _, err := outer.validateWrappedCircularStationLoft(ctx, o)
	if err != nil {
		return Solid{}, WrappedStationBooleanHistory{}, err
	}
	b, _, err := inner.validateWrappedCircularStationLoft(ctx, o)
	if err != nil {
		return Solid{}, WrappedStationBooleanHistory{}, err
	}
	return cutProvedEccentricCircularStationLofts(ctx, outer, inner, a, b, o)
}

func cutProvedEccentricCircularStationLofts(ctx context.Context, outer, inner Solid, a, b WrappedStationLoftProof, o geom.SurfacePatchOptions) (Solid, WrappedStationBooleanHistory, error) {
	if a.Heights[0].Cmp(b.Heights[0]) != 0 || a.Heights[len(a.Heights)-1].Cmp(b.Heights[len(b.Heights)-1]) != 0 {
		return Solid{}, WrappedStationBooleanHistory{}, geom.ErrDomain
	}
	dx := new(big.Rat).Sub(a.Center[0], b.Center[0])
	dy := new(big.Rat).Sub(a.Center[1], b.Center[1])
	distance2 := new(big.Rat).Add(dx.Mul(dx, dx), dy.Mul(dy, dy))
	for _, stations := range [][]*big.Rat{a.Heights, b.Heights} {
		for _, z := range stations {
			if err := ctx.Err(); err != nil {
				return Solid{}, WrappedStationBooleanHistory{}, err
			}
			// Both authenticated station lofts cover this common height.
			r, _ := wrappedStationRadiusAt(a, z)
			q, _ := wrappedStationRadiusAt(b, z)
			gap := new(big.Rat).Sub(r, q)
			if gap.Sign() <= 0 || gap.Mul(gap, gap).Cmp(distance2) <= 0 {
				return Solid{}, WrappedStationBooleanHistory{}, geom.ErrDomain
			}
		}
	}
	return joinWrappedStationLoftBore(ctx, outer, inner, a, b, o)
}

// ValidateWrappedEccentricCircularStationBore recovers both original axes
// and the original wall and cap topology of a complete circular station
// bore. It re-proves strict eccentric nesting after native replay.
func (s Solid) ValidateWrappedEccentricCircularStationBore(ctx context.Context, o geom.SurfacePatchOptions) (WrappedStationBoreProof, error) {
	return s.validateWrappedStationBore(ctx, o, true, false)
}
