package parametric

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
)

// AlgebraicSteinmetzCap retains exact root provenance alongside its bounded
// rounded native chart assembly. OrdinateInterval and all bounds are owned;
// OrdinateRoot is immutable. The native JSON codec preserves chart geometry,
// not this additional ideal-source proof record.
type AlgebraicSteinmetzCap struct {
	SteinmetzPositiveCap
	OrdinateRoot     *geom.ExactAlgebraicRoot
	OrdinateInterval geom.ExactParameterInterval
}

// PerpendicularCylinderAlgebraicCap supports either nonzero cap sign without
// requiring the seam ordinate sqrt(radius²-cap²) to fit binary64. Exact root
// isolation and Taylor bounds prove the split-angle discrepancy and enclose
// ideal volume; native sewing retains the explicit tolerance. Existing strict
// positive/negative cap APIs retain their lossless-ordinate gate.
func PerpendicularCylinderAlgebraicCap(ctx context.Context, center geom.Vec3, radius, cap, xOtherHalfLength, zHalfLength, tolerance float64, o geom.ExactCurvePlaneEventOptions) (AlgebraicSteinmetzCap, error) {
	fail := func(e error) (AlgebraicSteinmetzCap, error) { return AlgebraicSteinmetzCap{}, e }
	if !finite(cap) || cap == 0 || math.Abs(cap) >= radius {
		return fail(geom.ErrInvalid)
	}
	if _, e := PerpendicularCylinderCommon(ctx, center, radius, xOtherHalfLength, zHalfLength, tolerance); e != nil {
		return fail(e)
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	sq := func(x *big.Rat) *big.Rat { return new(big.Rat).Mul(x, x) }
	delta := new(big.Rat).Sub(sq(q(radius)), sq(q(cap)))
	root, e := geom.ImportExactRoot(ctx, geom.ExactRootRecipe{Version: 1, Coefficients: []string{new(big.Rat).Neg(delta).RatString(), "0", "1"}, Lower: "0", Upper: q(radius).RatString()}, geom.ExactRootCodecOptions{ExactCurvePlaneEventOptions: o})
	if e != nil {
		return fail(e)
	}
	enc, e := root.Enclose(ctx, new(big.Rat).Quo(q(tolerance), big.NewRat(64, 1)), o)
	if e != nil {
		return fail(e)
	}
	if !enc.Converged {
		return fail(geom.ErrConvergence)
	}
	lo, hi := enc.Interval.Lower, enc.Interval.Upper
	rounded, _ := new(big.Rat).Quo(new(big.Rat).Add(lo, hi), big.NewRat(2, 1)).Float64()
	if !finite(rounded) || rounded <= 0 {
		return fail(geom.ErrConvergence)
	}
	proof, e := buildPositiveSteinmetzCap(ctx, center, radius, math.Abs(cap), tolerance, rounded, lo, hi)
	if e != nil {
		return fail(e)
	}
	if cap < 0 {
		negative, e := buildNegativeSteinmetzCap(ctx, center, radius, cap, tolerance, proof)
		if e != nil {
			return fail(e)
		}
		proof = SteinmetzPositiveCap(negative)
	}
	return AlgebraicSteinmetzCap{SteinmetzPositiveCap: proof, OrdinateRoot: root, OrdinateInterval: geom.ExactParameterInterval{Lower: new(big.Rat).Set(lo), Upper: new(big.Rat).Set(hi)}}, nil
}
