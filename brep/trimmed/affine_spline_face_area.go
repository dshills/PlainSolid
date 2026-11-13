package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// AffineSplineFaceArea encloses the exact trimmed area in physical model units.
// GramSquared is the exact squared affine area scale in original UV units.
// AreaVector follows the face orientation; ModelArea is unsigned even when
// Reversed. No chord polygon or sampled quadrature replaces the source trims.
type AffineSplineFaceArea struct {
	UVArea, ModelArea  RationalAreaInterval
	GramSquared        *big.Rat
	AreaVector         [3]RationalAreaInterval
	Work, Subdivisions int
}

// AreaBounds bounds the actual rational trim Green integral and exact affine
// Gram scale to a model-area width <=maxWidth. sqrtBits (default128,max4096)
// bounds the dyadic square-root enclosure; insufficient precision fails closed.
// The retained trim certificate supplies its own fixed work/subdivision budget.
func (q *AffineSplineFaceQuery) AreaBounds(ctx context.Context, maxWidth *big.Rat, sqrtBits int) (AffineSplineFaceArea, error) {
	var zero AffineSplineFaceArea
	if ctx == nil || q == nil || q.loops == nil || maxWidth == nil || maxWidth.Sign() <= 0 {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if sqrtBits == 0 {
		sqrtBits = 128
	}
	if sqrtBits < 1 || sqrtBits > 4096 {
		return zero, geom.ErrInvalid
	}
	check := func(v *big.Rat) bool {
		return v.Num().BitLen() <= q.trim.MaxBits && v.Denom().BitLen() <= q.trim.MaxBits
	}
	if !check(maxWidth) {
		return zero, geom.ErrConvergence
	}
	d := q.chart.Domain()
	uvMax := new(big.Rat).Mul(new(big.Rat).Sub(ratInset(d.UMax), ratInset(d.UMin)), new(big.Rat).Sub(ratInset(d.VMax), ratInset(d.VMin)))
	var normal [3]*big.Rat
	gram := new(big.Rat)
	for i := 0; i < 3; i++ {
		normal[i] = new(big.Rat).Quo(q.plane[i], uvMax)
		gram.Add(gram, new(big.Rat).Mul(normal[i], normal[i]))
		if !check(normal[i]) || !check(gram) {
			return zero, geom.ErrConvergence
		}
	}
	if gram.Sign() <= 0 {
		return zero, geom.ErrDegenerate
	}
	if gram.Num().BitLen()+2*sqrtBits > q.trim.MaxBits {
		return zero, geom.ErrConvergence
	}
	denominator := new(big.Int).Lsh(big.NewInt(1), uint(sqrtBits))
	scaled := new(big.Int).Lsh(new(big.Int).Set(gram.Num()), uint(2*sqrtBits))
	scaled.Quo(scaled, gram.Denom())
	floor := new(big.Int).Sqrt(scaled)
	low := new(big.Rat).SetFrac(floor, denominator)
	high := new(big.Rat).Set(low)
	if new(big.Rat).Mul(low, low).Cmp(gram) != 0 {
		high.SetFrac(new(big.Int).Add(floor, big.NewInt(1)), denominator)
	}
	half := new(big.Rat).Quo(maxWidth, big.NewRat(2, 1))
	scaleError := new(big.Rat).Mul(uvMax, new(big.Rat).Sub(high, low))
	if scaleError.Cmp(half) > 0 {
		return zero, geom.ErrConvergence
	}
	tolerance := new(big.Rat).Quo(half, high)
	if !check(tolerance) {
		return zero, geom.ErrConvergence
	}
	area, e := q.loops.BoundSignedArea(ctx, tolerance)
	if e != nil {
		return zero, e
	}
	if area.Lower.Sign() < 0 {
		area.Lower.SetInt64(0)
	}
	if area.Upper.Cmp(uvMax) > 0 {
		area.Upper.Set(uvMax)
	}
	if area.Upper.Sign() <= 0 || area.Lower.Cmp(area.Upper) > 0 {
		return zero, geom.ErrConvergence
	}
	out := AffineSplineFaceArea{UVArea: RationalAreaInterval{area.Lower, area.Upper}, GramSquared: gram, Work: area.Work, Subdivisions: area.Subdivisions}
	out.ModelArea = RationalAreaInterval{new(big.Rat).Mul(area.Lower, low), new(big.Rat).Mul(area.Upper, high)}
	if !check(out.ModelArea.Lower) || !check(out.ModelArea.Upper) || new(big.Rat).Sub(out.ModelArea.Upper, out.ModelArea.Lower).Cmp(maxWidth) > 0 {
		return zero, geom.ErrConvergence
	}
	for i, n := range normal {
		a := new(big.Rat).Mul(area.Lower, n)
		b := new(big.Rat).Mul(area.Upper, n)
		if a.Cmp(b) > 0 {
			a, b = b, a
		}
		if !check(a) || !check(b) {
			return zero, geom.ErrConvergence
		}
		out.AreaVector[i] = RationalAreaInterval{a, b}
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}
