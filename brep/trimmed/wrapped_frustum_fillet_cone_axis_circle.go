package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// BoundConeAxisNearestCircleRational proves a full circular subset of nearest
// points on the original retained cone. The meridian-line foot has radius
// R/(1+s²) and height z-sR/(1+s²), where R is the support radius at query z.
// Positive radius and strict original tangent-height limits prove retention;
// native bore clearance proves the whole circle remains. All other original
// faces must separate. A nil Circle keeps the conservative all-face baseline.
// Uses one support query and one exact rational projection; no isolated
// azimuth is selected and other equally nearest faces may exist.
func (q FilletedFrustumMaterial) BoundConeAxisNearestCircleRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumDistanceOptions) (out FilletedFrustumAxisCircleDistance, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumAxisCircleDistance{}
		}
	}()
	base, e := q.BoundBoundaryDistanceRational(ctx, p, o)
	if e != nil {
		return out, e
	}
	out.Baseline = base
	if p[0].Cmp(q.center[0]) != 0 || p[1].Cmp(q.center[1]) != 0 {
		return out, nil
	}
	R := fdAdd(q.r[0], fdMul(q.slope, fdSub(p[2], q.z[0])))
	den := fdAdd(big.NewRat(1, 1), fdSquare(q.slope))
	radius := new(big.Rat).Quo(R, den)
	height := fdSub(p[2], fdMul(q.slope, radius))
	if radius.Sign() <= 0 || height.Cmp(q.fillets.Circles[0].Tangency[1]) <= 0 || height.Cmp(q.fillets.Circles[1].Tangency[1]) >= 0 {
		return out, nil
	}
	d := new(big.Rat).Quo(fdSquare(R), den)
	if !base.Exact || base.Lower.Cmp(d) != 0 {
		return out, nil
	}
	for _, f := range base.PerFace {
		if f.Face != q.fillets.Lateral && f.SquaredLower.Cmp(d) < 0 {
			return out, nil
		}
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	out.Circle = &FilletedFrustumNearestCircle{Center: [3]*big.Rat{new(big.Rat).Set(q.center[0]), new(big.Rat).Set(q.center[1]), height}, U: [3]*big.Rat{big.NewRat(1, 1), new(big.Rat), new(big.Rat)}, V: [3]*big.Rat{new(big.Rat), big.NewRat(1, 1), new(big.Rat)}, Radius: radius, SquaredDistance: d, Face: q.fillets.Lateral}
	return out, nil
}

// BoundConeAxisNearestCircleRational places the entire source circle through
// the exact proper rigid occurrence, preserving original ownership/distance.
// Source coordinates remain explicitly in Original; pose failures are atomic.
func (q FilletedFrustumOccurrence) BoundConeAxisNearestCircleRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumDistanceOptions) (out FilletedFrustumOccurrenceAxisCircleDistance, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumOccurrenceAxisCircleDistance{}
		}
	}()
	if q.source.recipe == nil {
		return out, geom.ErrInvalid
	}
	local, e := q.pose.TransformPoint(ctx, p, true, 4096)
	if e != nil {
		return out, e
	}
	original, e := q.source.BoundConeAxisNearestCircleRational(ctx, local, o)
	if e != nil {
		return out, e
	}
	out.Original = original
	if original.Circle == nil {
		return out, nil
	}
	c := original.Circle
	center, e := q.pose.TransformPoint(ctx, c.Center, false, 4096)
	if e != nil {
		return out, e
	}
	u, e := q.pose.TransformVector(ctx, c.U, false, 4096)
	if e != nil {
		return out, e
	}
	v, e := q.pose.TransformVector(ctx, c.V, false, 4096)
	if e != nil {
		return out, e
	}
	out.WorldCircle = &FilletedFrustumNearestCircle{Center: center, U: u, V: v, Radius: new(big.Rat).Set(c.Radius), SquaredDistance: new(big.Rat).Set(c.SquaredDistance), Face: c.Face}
	return out, nil
}
