package geom

import (
	"context"
	"math/big"
)

// RadialGraphVolumeOptions bounds exact polynomial integration and the
// alternating-series enclosure of pi. PrecisionBits (default64, range16..256)
// requests an absolute total volume width at most 2^-PrecisionBits.
type RadialGraphVolumeOptions struct {
	PrecisionBits, MaxOperations, MaxCoefficientBits int
}

type RadialGraphBandVolume struct {
	// Band0 is the disk through radius0. Band i>0 is between radii i-1 and i.
	Band          int
	HeightSign    int // sign(ZA-ZB), retaining original ordered supports
	PiCoefficient *big.Rat
	Volume        ExactParameterInterval
}

type RadialGraphVolume struct {
	Bands           []RadialGraphBandVolume
	PiCoefficient   *big.Rat
	Volume          ExactParameterInterval
	ExactOperations int
}

// BoundVolumeBands encloses the volume BETWEEN the original graphs over the
// selected complete finite circular bands. It is not a solid/cap classification:
// callers must separately prove that these uncut bands are their material.
// The whole height difference must have a positive CONSTANT denominator in the
// already-proved representation. A numerator identity alone is insufficient.
// Band indices must be strictly increasing; the exterior of the last circle is
// excluded. Adjacent bands may be measured but no manifold claim is made.
func (r *SplineGraphRadialContours) BoundVolumeBands(ctx context.Context, bands []int, o RadialGraphVolumeOptions) (RadialGraphVolume, error) {
	zero := RadialGraphVolume{}
	if ctx == nil || r == nil || r.center[0] == nil || len(bands) == 0 || len(bands) > len(r.radii) {
		return zero, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if r.heightScale == nil {
		return zero, ErrConvergence
	}
	if o.PrecisionBits == 0 {
		o.PrecisionBits = 64
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 4096
	}
	if o.PrecisionBits < 16 || o.PrecisionBits > 256 || o.MaxOperations < 1 || o.MaxOperations > 128000000 || o.MaxCoefficientBits < 64 || o.MaxCoefficientBits > 16384 {
		return zero, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	for _, q := range append([]*big.Rat{r.heightScale}, r.radii...) {
		if e := b.check(q); e != nil {
			return zero, e
		}
	}
	poly, square, e := radialGraphHeightPolynomial(r, b)
	if e != nil {
		return zero, e
	}
	// In polar coordinates q=radius^2, dA=pi*dq. Integrate every
	// coefficient exactly, including cancellation of large signed terms.
	primitive := make(rationalPolynomial, len(poly)+1)
	primitive[0] = new(big.Rat)
	for i, c := range poly {
		primitive[i+1] = new(big.Rat).Quo(c, big.NewRat(int64(i+1), 1))
		if e := b.check(primitive[i+1]); e != nil {
			return zero, e
		}
	}
	eval := func(q *big.Rat) (*big.Rat, error) {
		value := new(big.Rat)
		for i := len(primitive) - 1; i >= 0; i-- {
			value.Mul(value, q)
			if e := b.check(value); e != nil {
				return nil, e
			}
			value.Add(value, primitive[i])
			if e := b.check(value); e != nil {
				return nil, e
			}
		}
		return value, nil
	}
	out := RadialGraphVolume{PiCoefficient: new(big.Rat)}
	signs := r.BandSigns()
	previous := -1
	for _, band := range bands {
		if e := ctx.Err(); e != nil {
			return zero, e
		}
		if band < 0 || band >= len(square) || band <= previous {
			return zero, ErrInvalid
		}
		previous = band
		lower := new(big.Rat)
		if band > 0 {
			lower = square[band-1]
		}
		lo, e := eval(lower)
		if e != nil {
			return zero, e
		}
		hi, e := eval(square[band])
		if e != nil {
			return zero, e
		}
		coefficient := new(big.Rat).Sub(hi, lo)
		if e := b.check(coefficient); e != nil {
			return zero, e
		}
		if coefficient.Sign() != signs[band] {
			return zero, ErrConvergence
		}
		coefficient.Abs(coefficient)
		out.PiCoefficient.Add(out.PiCoefficient, coefficient)
		if e := b.check(out.PiCoefficient); e != nil {
			return zero, e
		}
		out.Bands = append(out.Bands, RadialGraphBandVolume{Band: band, HeightSign: signs[band], PiCoefficient: coefficient})
	}
	// Machin's identity pi=16 atan(1/5)-4 atan(1/239). Alternating
	// remainders give independent outward rational endpoints.
	target := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), uint(o.PrecisionBits)))
	scale := big.NewRat(1, 1)
	if out.PiCoefficient.Cmp(scale) > 0 {
		scale = out.PiCoefficient
	}
	tolerance := new(big.Rat).Quo(target, new(big.Rat).Mul(big.NewRat(20, 1), scale))
	if e := b.check(tolerance); e != nil {
		return zero, e
	}
	a, e := helicalAtanSmall(ctx, big.NewRat(1, 5), tolerance, 4096, b)
	if e != nil {
		return zero, e
	}
	c, e := helicalAtanSmall(ctx, big.NewRat(1, 239), tolerance, 4096, b)
	if e != nil {
		return zero, e
	}
	pi := [2]*big.Rat{new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(16, 1), a[0]), new(big.Rat).Mul(big.NewRat(4, 1), c[1])), new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(16, 1), a[1]), new(big.Rat).Mul(big.NewRat(4, 1), c[0]))}
	multiply := func(coefficient *big.Rat) (ExactParameterInterval, error) {
		lo, hi := new(big.Rat).Mul(coefficient, pi[0]), new(big.Rat).Mul(coefficient, pi[1])
		for _, q := range []*big.Rat{lo, hi} {
			if e := b.check(q); e != nil {
				return ExactParameterInterval{}, e
			}
		}
		return ExactParameterInterval{Lower: lo, Upper: hi}, nil
	}
	out.Volume, e = multiply(out.PiCoefficient)
	if e != nil {
		return zero, e
	}
	for i := range out.Bands {
		out.Bands[i].Volume, e = multiply(out.Bands[i].PiCoefficient)
		if e != nil {
			return zero, e
		}
	}
	if new(big.Rat).Sub(out.Volume.Upper, out.Volume.Lower).Cmp(target) > 0 || out.Volume.Lower.Sign() <= 0 {
		return zero, ErrConvergence
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	out.ExactOperations = b.operations
	return out, nil
}

// radialGraphHeightPolynomial recovers the already-proved physical height,
// not merely a scaled homogeneous numerator.
func radialGraphHeightPolynomial(r *SplineGraphRadialContours, b *polynomialBudget) (rationalPolynomial, []*big.Rat, error) {
	poly := rationalPolynomial{new(big.Rat).Set(r.heightScale)}
	square := make([]*big.Rat, len(r.radii))
	for i, radius := range r.radii {
		square[i] = new(big.Rat).Mul(radius, radius)
		if e := b.check(square[i]); e != nil {
			return nil, nil, e
		}
		next := make(rationalPolynomial, len(poly)+1)
		for j := range next {
			next[j] = new(big.Rat)
		}
		for j, c := range poly {
			term := new(big.Rat).Mul(c, square[i])
			if e := b.check(term); e != nil {
				return nil, nil, e
			}
			next[j].Sub(next[j], term)
			next[j+1].Add(next[j+1], c)
			for _, q := range []*big.Rat{next[j], next[j+1]} {
				if e := b.check(q); e != nil {
					return nil, nil, e
				}
			}
		}
		poly = next
	}
	return poly, square, nil
}
