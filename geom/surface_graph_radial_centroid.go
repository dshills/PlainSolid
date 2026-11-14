package geom

import (
	"context"
	"math/big"
)

// RadialGraphHeightMoment is exact: the first Z moment is pi times the
// coefficient, while the centroid is the rational ratio to volume/pi.
type RadialGraphHeightMoment struct{ FirstZPiCoefficient, CentroidZ *big.Rat }
type RadialGraphBandHeightMoment struct {
	Band   int
	Moment RadialGraphHeightMoment
}

// RadialGraphMassMoments extends planar moments to the original height first
// moment and thus all three centroid coordinates. It does not assert ZZ/XZ/YZ
// second moments or a complete inertia tensor. ExactOperations is cumulative.
type RadialGraphMassMoments struct {
	Planar          RadialGraphPlanarMoments
	Height          RadialGraphHeightMoment
	Bands           []RadialGraphBandHeightMoment
	ExactOperations int
}

// MassMomentsBands integrates the ORIGINAL polynomial heights, not a radial
// surrogate placed at z=0. Both original denominators must be positive constants.
// The individual heights may be nonradial: exact circular angular averaging of
// their sum, multiplied by the proved radial difference, gives the Z moment.
// Callers still prove complete band material ownership independently.
func (r *SplineGraphRadialContours) MassMomentsBands(ctx context.Context, bands []int, o RadialGraphMomentOptions) (RadialGraphMassMoments, error) {
	zero := RadialGraphMassMoments{}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 4096
	}
	planar, e := r.PlanarMomentsBands(ctx, bands, o)
	if e != nil {
		return zero, e
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits, operations: planar.ExactOperations}
	sum, e := radialAngularHeightSum(ctx, r, b)
	if e != nil {
		return zero, e
	}
	height, squares, e := radialGraphHeightPolynomial(r, b)
	if e != nil {
		return zero, e
	}
	product := make(rationalPolynomial, len(sum)+len(height)-1)
	for i := range product {
		product[i] = new(big.Rat)
	}
	for i, a := range height {
		for j, c := range sum {
			term := new(big.Rat).Mul(a, c)
			if e = b.check(term); e != nil {
				return zero, e
			}
			product[i+j].Add(product[i+j], term)
			if e = b.check(product[i+j]); e != nil {
				return zero, e
			}
		}
	}
	primitive := make(rationalPolynomial, len(product)+1)
	primitive[0] = new(big.Rat)
	for i, c := range product {
		primitive[i+1] = new(big.Rat).Quo(c, big.NewRat(2*int64(i+1), 1))
		if e = b.check(primitive[i+1]); e != nil {
			return zero, e
		}
	}
	eval := func(q *big.Rat) (*big.Rat, error) {
		v := new(big.Rat)
		for i := len(primitive) - 1; i >= 0; i-- {
			v.Mul(v, q)
			if e := b.check(v); e != nil {
				return nil, e
			}
			v.Add(v, primitive[i])
			if e := b.check(v); e != nil {
				return nil, e
			}
		}
		return v, nil
	}
	out := RadialGraphMassMoments{Planar: planar}
	total := new(big.Rat)
	for _, band := range planar.Bands {
		if e = ctx.Err(); e != nil {
			return zero, e
		}
		lo := new(big.Rat)
		if band.Band > 0 {
			lo = squares[band.Band-1]
		}
		a, e := eval(lo)
		if e != nil {
			return zero, e
		}
		c, e := eval(squares[band.Band])
		if e != nil {
			return zero, e
		}
		z := new(big.Rat).Sub(c, a)
		if band.HeightSign < 0 {
			z.Neg(z)
		}
		if e = b.check(z); e != nil {
			return zero, e
		}
		centroid := new(big.Rat).Quo(z, band.Moment.VolumePiCoefficient)
		if e = b.check(centroid); e != nil {
			return zero, e
		}
		out.Bands = append(out.Bands, RadialGraphBandHeightMoment{Band: band.Band, Moment: RadialGraphHeightMoment{FirstZPiCoefficient: z, CentroidZ: centroid}})
		total.Add(total, z)
		if e = b.check(total); e != nil {
			return zero, e
		}
	}
	out.Height = RadialGraphHeightMoment{FirstZPiCoefficient: total, CentroidZ: new(big.Rat).Quo(total, planar.Total.VolumePiCoefficient)}
	if e = b.check(out.Height.CentroidZ); e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	out.ExactOperations = b.operations
	return out, nil
}

// radialAngularHeightSum retains the original height sum, including nonradial terms.
func radialAngularHeightSum(ctx context.Context, r *SplineGraphRadialContours, b *polynomialBudget) (rationalPolynomial, error) {
	heights, e := radialHeightPowers(ctx, r, b)
	if e != nil {
		return nil, e
	}
	sum, e := radialPowerAdd(heights[0], heights[1], b)
	if e != nil {
		return nil, e
	}
	return radialAngularPower(ctx, sum, r.center, b)
}
