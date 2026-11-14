package geom

import (
	"context"
	"math/big"
)

// RadialGraphTensor stores exact rational coefficients of pi. SecondMoment is
// Qij=integral Xi*Xj dV in WORLD coordinates. OriginInertia is trace(Q)I-Q,
// about the world origin, not the center of mass. Matrices are symmetric and
// all entries are independently owned rational values.
type RadialGraphTensor struct {
	SecondMomentPiCoefficients  [3][3]*big.Rat
	OriginInertiaPiCoefficients [3][3]*big.Rat
}
type RadialGraphBandTensor struct {
	Band   int
	Tensor RadialGraphTensor
}
type RadialGraphMassTensor struct {
	Mass            RadialGraphMassMoments
	Total           RadialGraphTensor
	Bands           []RadialGraphBandTensor
	ExactOperations int
}

// MassTensorBands completes the original polynomial-graph material moments.
// Source degree is <=8 per chart axis; exact height products are <=16 per
// axis, angular means <=16 in q=radius^2, and weighted integrands <=20 in q.
// All arithmetic shares one quota with mass/centroid integration. The positive
// constant-denominator and complete uncut band gates remain unchanged; callers
// must separately prove material ownership, as for MassMomentsBands.
func (r *SplineGraphRadialContours) MassTensorBands(ctx context.Context, bands []int, o RadialGraphMomentOptions) (RadialGraphMassTensor, error) {
	zero := RadialGraphMassTensor{}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 4096
	}
	mass, e := r.MassMomentsBands(ctx, bands, o)
	if e != nil {
		return zero, e
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits, operations: mass.ExactOperations}
	h, e := radialHeightPowers(ctx, r, b)
	if e != nil {
		return zero, e
	}
	sum, e := radialPowerAdd(h[0], h[1], b)
	if e != nil {
		return zero, e
	}
	xsum, ysum := newRadialPower(len(sum)+1, len(sum[0])), newRadialPower(len(sum), len(sum[0])+1)
	for i, row := range sum {
		for j, c := range row {
			xsum[i+1][j].Set(c)
			ysum[i][j+1].Set(c)
		}
	}
	square, e := radialPowerMultiply(ctx, h[0], h[0], b)
	if e != nil {
		return zero, e
	}
	for _, pair := range [][2]int{{0, 1}, {1, 1}} {
		p, e := radialPowerMultiply(ctx, h[pair[0]], h[pair[1]], b)
		if e != nil {
			return zero, e
		}
		square, e = radialPowerAdd(square, p, b)
		if e != nil {
			return zero, e
		}
	}
	height, radii, e := radialGraphHeightPolynomial(r, b)
	if e != nil {
		return zero, e
	}
	var primitives [3]rationalPolynomial
	for i, p := range []radialPower{xsum, ysum, square} {
		mean, e := radialAngularPower(ctx, p, r.center, b)
		if e != nil {
			return zero, e
		}
		divisor := 2
		if i == 2 {
			divisor = 3
		}
		primitives[i], e = radialMomentPrimitive(height, mean, divisor, b)
		if e != nil {
			return zero, e
		}
	}
	totals := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
	out := RadialGraphMassTensor{Mass: mass}
	for _, band := range mass.Planar.Bands {
		if e = ctx.Err(); e != nil {
			return zero, e
		}
		lo := new(big.Rat)
		if band.Band > 0 {
			lo = radii[band.Band-1]
		}
		var values [3]*big.Rat
		for i, p := range primitives {
			a, e := radialMomentValue(p, lo, b)
			if e != nil {
				return zero, e
			}
			c, e := radialMomentValue(p, radii[band.Band], b)
			if e != nil {
				return zero, e
			}
			values[i] = new(big.Rat).Sub(c, a)
			if band.HeightSign < 0 {
				values[i].Neg(values[i])
			}
			if e = b.check(values[i]); e != nil {
				return zero, e
			}
			totals[i].Add(totals[i], values[i])
			if e = b.check(totals[i]); e != nil {
				return zero, e
			}
		}
		if values[2].Sign() <= 0 {
			return zero, ErrConvergence
		}
		tensor, e := radialMomentTensor(band.Moment, values, b)
		if e != nil {
			return zero, e
		}
		out.Bands = append(out.Bands, RadialGraphBandTensor{Band: band.Band, Tensor: tensor})
	}
	out.Total, e = radialMomentTensor(mass.Planar.Total, totals, b)
	if e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	out.ExactOperations = b.operations
	return out, nil
}

func radialMomentPrimitive(height, mean rationalPolynomial, divisor int, b *polynomialBudget) (rationalPolynomial, error) {
	if len(mean) == 0 {
		mean = rationalPolynomial{new(big.Rat)}
	}
	n := len(height) + len(mean) - 1
	if n > 21 {
		return nil, ErrConvergence
	}
	product := make(rationalPolynomial, n)
	for i := range product {
		product[i] = new(big.Rat)
	}
	for i, a := range height {
		for j, c := range mean {
			term := new(big.Rat).Mul(a, c)
			if e := b.check(term); e != nil {
				return nil, e
			}
			product[i+j].Add(product[i+j], term)
			if e := b.check(product[i+j]); e != nil {
				return nil, e
			}
		}
	}
	primitive := make(rationalPolynomial, n+1)
	primitive[0] = new(big.Rat)
	for i, c := range product {
		primitive[i+1] = new(big.Rat).Quo(c, big.NewRat(int64(divisor*(i+1)), 1))
		if e := b.check(primitive[i+1]); e != nil {
			return nil, e
		}
	}
	return primitive, nil
}
func radialMomentValue(p rationalPolynomial, q *big.Rat, b *polynomialBudget) (*big.Rat, error) {
	v := new(big.Rat)
	for i := len(p) - 1; i >= 0; i-- {
		v.Mul(v, q)
		if e := b.check(v); e != nil {
			return nil, e
		}
		v.Add(v, p[i])
		if e := b.check(v); e != nil {
			return nil, e
		}
	}
	return v, nil
}
func radialMomentTensor(xy RadialGraphPlanarMoment, mixed [3]*big.Rat, b *polynomialBudget) (RadialGraphTensor, error) {
	zero := RadialGraphTensor{}
	out := RadialGraphTensor{}
	values := [3][3]*big.Rat{{xy.SecondMomentPiCoefficients[0], xy.SecondMomentPiCoefficients[1], mixed[0]}, {nil, xy.SecondMomentPiCoefficients[2], mixed[1]}, {nil, nil, mixed[2]}}
	for i := range 3 {
		for j := i; j < 3; j++ {
			if e := b.check(values[i][j]); e != nil {
				return zero, e
			}
			out.SecondMomentPiCoefficients[i][j] = new(big.Rat).Set(values[i][j])
			out.SecondMomentPiCoefficients[j][i] = new(big.Rat).Set(values[i][j])
		}
	}
	for i := range 3 {
		for j := i; j < 3; j++ {
			v := new(big.Rat)
			if i == j {
				for k := range 3 {
					if k != i {
						v.Add(v, values[k][k])
						if e := b.check(v); e != nil {
							return zero, e
						}
					}
				}
			} else {
				v.Neg(values[i][j])
				if e := b.check(v); e != nil {
					return zero, e
				}
			}
			out.OriginInertiaPiCoefficients[i][j] = new(big.Rat).Set(v)
			out.OriginInertiaPiCoefficients[j][i] = new(big.Rat).Set(v)
		}
	}
	return out, nil
}
