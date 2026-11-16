package step

import (
	"context"
	"errors"
	"math/big"

	"plainsolid/geom"
)

var errAmbiguousConicalFrameRatio = errors.New("step: ambiguous bounded occurrence direction ratio")

// Candidate source rays have primitive integer components bounded by limit.
// If the unit vectors differ by chord d, the original dominant component
// has magnitude at least 1/sqrt(3), and the candidate's magnitude exceeds
// 1/4 since d < 1/4. It keeps its sign. Dividing by that component changes
// each relative direction ratio by at most 8d.
func reconstructMixedConicalFrameAxis(ctx context.Context, raw [3]*big.Rat, chord *big.Rat, limit int) ([3]int64, [3]*big.Rat, *big.Rat, error) {
	var primitive [3]int64
	var exact [3]*big.Rat
	axis := 0
	abs := func(r *big.Rat) *big.Rat { return new(big.Rat).Abs(r) }
	for i := 1; i < 3; i++ {
		if abs(raw[i]).Cmp(abs(raw[axis])) > 0 {
			axis = i
		}
	}
	if raw[axis].Sign() == 0 {
		return primitive, exact, nil, geom.ErrUnrepresentable
	}
	halfWidth := new(big.Rat).Mul(chord, big.NewRat(8, 1))
	var ratios [3]*big.Rat
	scale := big.NewInt(1)
	for i, q := range raw {
		if i == axis {
			ratios[i] = big.NewRat(1, 1)
			continue
		}
		center := new(big.Rat).Quo(q, raw[axis])
		ratio, err := uniqueMixedConicalFrameRatio(ctx, new(big.Rat).Sub(center, halfWidth), new(big.Rat).Add(center, halfWidth), limit)
		if err != nil {
			return primitive, exact, nil, err
		}
		ratios[i] = ratio
		denom := ratio.Denom()
		gcd := new(big.Int).GCD(nil, nil, scale, denom)
		scale.Quo(scale, gcd).Mul(scale, denom)
	}
	if raw[axis].Sign() < 0 {
		scale.Neg(scale)
	}
	factor := new(big.Rat).SetInt(scale)
	for i, q := range ratios {
		exact[i] = new(big.Rat).Mul(q, factor)
		if !exact[i].IsInt() || new(big.Int).Abs(exact[i].Num()).Cmp(big.NewInt(int64(limit))) > 0 {
			return primitive, [3]*big.Rat{}, nil, geom.ErrUnrepresentable
		}
		primitive[i] = exact[i].Num().Int64()
	}
	unit, err := mappedRationalUnit(exact)
	if err != nil {
		return primitive, [3]*big.Rat{}, nil, err
	}
	upper, err := mixedConicalFrameAxisChordUpper(raw, unit, chord)
	return primitive, unit, upper, err
}

// Distinct fractions in the closed interval reject the inference. A ratio's
// reduced denominator cannot exceed the original candidate's dominant
// integer component, so this enumerates every possible reduced ratio.
func uniqueMixedConicalFrameRatio(ctx context.Context, low, high *big.Rat, limit int) (*big.Rat, error) {
	var found *big.Rat
	for den := 1; den <= limit; den++ {
		if den%64 == 1 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		d := big.NewRat(int64(den), 1)
		l, h := new(big.Rat).Mul(low, d), new(big.Rat).Mul(high, d)
		lo := new(big.Int).Div(new(big.Int).Neg(l.Num()), l.Denom())
		lo.Neg(lo) // ceil(x) = -floor(-x)
		hi := new(big.Int).Div(h.Num(), h.Denom())
		switch lo.Cmp(hi) {
		case 1:
			continue
		case -1:
			return nil, errAmbiguousConicalFrameRatio
		}
		q := new(big.Rat).SetFrac(lo, d.Num())
		if found != nil && q.Cmp(found) != 0 {
			return nil, errAmbiguousConicalFrameRatio
		}
		found = q
	}
	if found == nil {
		return nil, geom.ErrUnrepresentable
	}
	return found, nil
}

// With a positive dot product, the normalized chord is at most d exactly
// when dot^2 >= norm^2 (1-d^2/2)^2. Bisection retains a rational upper
// enclosure while avoiding cancellation near a unit cosine.
func mixedConicalFrameAxisChordUpper(raw, unit [3]*big.Rat, budget *big.Rat) (*big.Rat, error) {
	dot := mappedRationalDot(raw, unit)
	if dot.Sign() <= 0 {
		return nil, geom.ErrUnrepresentable
	}
	normalizedCosineSquared := new(big.Rat).Quo(new(big.Rat).Mul(dot, dot), mappedRationalDot(raw, raw))
	one, half := big.NewRat(1, 1), big.NewRat(1, 2)
	allowed := func(chord *big.Rat) bool {
		z := new(big.Rat).Mul(chord, chord)
		z.Sub(one, z.Mul(z, half))
		return normalizedCosineSquared.Cmp(z.Mul(z, z)) >= 0
	}
	if !allowed(budget) {
		return nil, geom.ErrConvergence
	}
	if normalizedCosineSquared.Cmp(one) == 0 {
		return new(big.Rat), nil
	}
	lo, hi := new(big.Rat), new(big.Rat).Set(budget)
	for range 64 {
		m := new(big.Rat).Add(lo, hi)
		m.Mul(m, half)
		if allowed(m) {
			hi = m
		} else {
			lo = m
		}
	}
	return hi, nil
}
