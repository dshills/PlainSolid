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

// radialAngularHeightSum returns the exact mean of ZA+ZB at squared radius q.
// For m,n>=0, mean(cos^(2m) sin^(2n)) equals
// C(2m,m) C(2n,n)/(4^(m+n) C(m+n,m)); all odd centered powers vanish.
func radialAngularHeightSum(ctx context.Context, r *SplineGraphRadialContours, b *polynomialBudget) (rationalPolynomial, error) {
	mean := make(rationalPolynomial, 9)
	for i := range mean {
		mean[i] = new(big.Rat)
	}
	var powers [2]rationalPolynomial
	for axis := range 2 {
		powers[axis] = make(rationalPolynomial, 9)
		powers[axis][0] = big.NewRat(1, 1)
		for i := 1; i < 9; i++ {
			powers[axis][i] = new(big.Rat).Mul(powers[axis][i-1], r.center[axis])
			if e := b.check(powers[axis][i]); e != nil {
				return nil, e
			}
		}
	}
	for _, p := range r.supports {
		if p == nil || p.degreeU > 8 || p.degreeV > 8 || p.countU != p.degreeU+1 || p.countV != p.degreeV+1 {
			return nil, ErrConvergence
		}
		w := p.net[0][3]
		if w.Sign() <= 0 {
			return nil, ErrConvergence
		}
		rows := make([]rationalPolynomial, p.countU)
		for i := range rows {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
			values := make([]*big.Rat, p.countV)
			for j := range values {
				h := p.net[i*p.countV+j]
				if h[3].Cmp(w) != 0 {
					return nil, ErrConvergence
				}
				for _, q := range []*big.Rat{h[2], h[3]} {
					if e := b.check(q); e != nil {
						return nil, e
					}
				}
				values[j] = new(big.Rat).Quo(h[2], w)
				if e := b.check(values[j]); e != nil {
					return nil, e
				}
			}
			var e error
			rows[i], e = bernsteinToPower(values, b)
			if e != nil {
				return nil, e
			}
		}
		for j := 0; j < p.countV; j++ {
			values := make([]*big.Rat, p.countU)
			for i := range values {
				values[i] = new(big.Rat)
				if j < len(rows[i]) {
					values[i].Set(rows[i][j])
				}
			}
			column, e := bernsteinToPower(values, b)
			if e != nil {
				return nil, e
			}
			for i, c := range column {
				if c.Sign() == 0 {
					continue
				}
				for a := 0; a <= i; a += 2 {
					for d := 0; d <= j; d += 2 {
						if e := ctx.Err(); e != nil {
							return nil, e
						}
						m, n := int64(a/2), int64(d/2)
						numerator := new(big.Int).Mul(new(big.Int).Binomial(2*m, m), new(big.Int).Binomial(2*n, n))
						denominator := new(big.Int).Mul(new(big.Int).Lsh(big.NewInt(1), uint(2*(m+n))), new(big.Int).Binomial(m+n, m))
						term := new(big.Rat).SetFrac(numerator, denominator)
						for _, factor := range []*big.Rat{c, new(big.Rat).SetInt(new(big.Int).Binomial(int64(i), int64(a))), new(big.Rat).SetInt(new(big.Int).Binomial(int64(j), int64(d))), powers[0][i-a], powers[1][j-d]} {
							term.Mul(term, factor)
							if e := b.check(term); e != nil {
								return nil, e
							}
						}
						mean[(a+d)/2].Add(mean[(a+d)/2], term)
						if e := b.check(mean[(a+d)/2]); e != nil {
							return nil, e
						}
					}
				}
			}
		}
	}
	return trimPolynomial(mean), nil
}
