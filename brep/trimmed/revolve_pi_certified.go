package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// Machin's identity pi=16*atan(1/5)-4*atan(1/239), with alternating-series
// remainder bounds, yields an exact rational interval. Outward dyadic rounding
// keeps subsequent Taylor coefficient denominators bounded. No libm result is
// used in the proof. The fixed enclosure has width at most a few 2^-128 units;
// tighter requested display errors conservatively remain unresolved.
func certifiedSweepPi(ctx context.Context, o CertifiedRevolvePolygonOptions, work *int) (*big.Rat, *big.Rat, error) {
	atan := func(q int64) (*big.Rat, *big.Rat, error) {
		sum := new(big.Rat)
		power := big.NewInt(q)
		square := big.NewInt(q * q)
		for k := 0; k < 64; k++ {
			if e := ctx.Err(); e != nil {
				return nil, nil, e
			}
			if *work >= o.MaxTrigTerms {
				return nil, nil, geom.ErrConvergence
			}
			*work++
			denom := new(big.Int).Mul(new(big.Int).Set(power), big.NewInt(int64(2*k+1)))
			term := new(big.Rat).SetFrac(big.NewInt(1), denom)
			if k%2 == 0 {
				sum.Add(sum, term)
			} else {
				sum.Sub(sum, term)
			}
			if sum.Num().BitLen() > o.MaxBits || sum.Denom().BitLen() > o.MaxBits {
				return nil, nil, geom.ErrConvergence
			}
			power.Mul(power, square)
		}
		remainder := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Mul(power, big.NewInt(129)))
		return sum, remainder, nil
	}
	a, ae, e := atan(5)
	if e != nil {
		return nil, nil, e
	}
	b, be, e := atan(239)
	if e != nil {
		return nil, nil, e
	}
	center := new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(16, 1), a), new(big.Rat).Mul(big.NewRat(4, 1), b))
	radius := new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(16, 1), ae), new(big.Rat).Mul(big.NewRat(4, 1), be))
	scale := new(big.Int).Lsh(big.NewInt(1), 128)
	low := new(big.Rat).Mul(new(big.Rat).Sub(center, radius), new(big.Rat).SetInt(scale))
	high := new(big.Rat).Mul(new(big.Rat).Add(center, radius), new(big.Rat).SetInt(scale))
	lo := new(big.Int).Quo(low.Num(), low.Denom())
	hi, rem := new(big.Int), new(big.Int)
	hi.QuoRem(high.Num(), high.Denom(), rem)
	if rem.Sign() != 0 {
		hi.Add(hi, big.NewInt(1))
	}
	lower, upper := new(big.Rat).SetFrac(lo, new(big.Int).Set(scale)), new(big.Rat).SetFrac(hi, new(big.Int).Set(scale))
	if lower.Cmp(big.NewRat(3, 1)) <= 0 || upper.Cmp(big.NewRat(7, 2)) >= 0 {
		return nil, nil, geom.ErrConvergence
	}
	return new(big.Rat).Quo(new(big.Rat).Add(lower, upper), big.NewRat(2, 1)), new(big.Rat).Quo(new(big.Rat).Sub(upper, lower), big.NewRat(2, 1)), nil
}
