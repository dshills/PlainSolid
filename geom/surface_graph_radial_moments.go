package geom

import (
	"context"
	"math/big"
)

// RadialGraphMomentOptions bounds exact rational arithmetic. Defaults are one
// million operations and 4096 coefficient bits; hard maxima are 128 million
// operations and 16384 bits (minimum 64 bits).
type RadialGraphMomentOptions struct{ MaxOperations, MaxCoefficientBits int }

// RadialGraphPlanarMoment contains coefficients of pi, not rounded measures.
// First and second moments integrate x,y and x*x,x*y,y*y against VOLUME.
// ProjectedArea is the XY footprint area, not curved boundary surface area.
// No Z centroid, XZ/YZ moments or complete inertia tensor is asserted.
type RadialGraphPlanarMoment struct {
	ProjectedAreaPiCoefficient *big.Rat
	VolumePiCoefficient        *big.Rat
	FirstMomentPiCoefficients  [2]*big.Rat
	SecondMomentPiCoefficients [3]*big.Rat // XX, XY, YY
	CentroidXY                 [2]*big.Rat // exact, pi cancels
}
type RadialGraphBandPlanarMoment struct {
	Band, HeightSign int
	Moment           RadialGraphPlanarMoment
}
type RadialGraphPlanarMoments struct {
	Total           RadialGraphPlanarMoment
	Bands           []RadialGraphBandPlanarMoment
	ExactOperations int
}

// PlanarMomentsBands integrates complete finite radial bands between the
// original graphs. As with BoundVolumeBands, callers must independently prove
// material ownership and absence of clipping. Only the height difference must
// be radial: the individual roofs may be nonradial. The same strictly positive
// constant-denominator gate is retained. Results own all rational values and
// errors return no partial table.
func (r *SplineGraphRadialContours) PlanarMomentsBands(ctx context.Context, bands []int, o RadialGraphMomentOptions) (RadialGraphPlanarMoments, error) {
	zero := RadialGraphPlanarMoments{}
	if ctx == nil || r == nil || r.center[0] == nil || len(bands) == 0 || len(bands) > len(r.radii) {
		return zero, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if r.heightScale == nil {
		return zero, ErrConvergence
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 4096
	}
	if o.MaxOperations < 1 || o.MaxOperations > 128000000 || o.MaxCoefficientBits < 64 || o.MaxCoefficientBits > 16384 {
		return zero, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	for _, q := range append([]*big.Rat{r.heightScale, r.center[0], r.center[1]}, r.radii...) {
		if e := b.check(q); e != nil {
			return zero, e
		}
	}
	poly, squares, e := radialGraphHeightPolynomial(r, b)
	if e != nil {
		return zero, e
	}
	// dA=pi*dq. The angular average of (x-cx)^2 and (y-cy)^2
	// is q/2, whereas that of (x-cx)(y-cy) is zero.
	var primitives [2]rationalPolynomial
	for power := range 2 {
		primitives[power] = make(rationalPolynomial, len(poly)+power+1)
		for j := range primitives[power] {
			primitives[power][j] = new(big.Rat)
		}
		for j, c := range poly {
			primitives[power][j+power+1].Quo(c, big.NewRat(int64(j+power+1), 1))
			if e = b.check(primitives[power][j+power+1]); e != nil {
				return zero, e
			}
		}
	}
	integral := func(poly rationalPolynomial, lo, hi *big.Rat) (*big.Rat, error) {
		values := [2]*big.Rat{new(big.Rat), new(big.Rat)}
		for i, q := range []*big.Rat{lo, hi} {
			for j := len(poly) - 1; j >= 0; j-- {
				values[i].Mul(values[i], q)
				if e := b.check(values[i]); e != nil {
					return nil, e
				}
				values[i].Add(values[i], poly[j])
				if e := b.check(values[i]); e != nil {
					return nil, e
				}
			}
		}
		out := new(big.Rat).Sub(values[1], values[0])
		if e := b.check(out); e != nil {
			return nil, e
		}
		return out, nil
	}
	moment := func(area, volume, radial *big.Rat) (RadialGraphPlanarMoment, error) {
		m := RadialGraphPlanarMoment{ProjectedAreaPiCoefficient: new(big.Rat).Set(area), VolumePiCoefficient: new(big.Rat).Set(volume)}
		for i, c := range r.center {
			m.CentroidXY[i] = new(big.Rat).Set(c)
			m.FirstMomentPiCoefficients[i] = new(big.Rat).Mul(c, volume)
			if e := b.check(m.FirstMomentPiCoefficients[i]); e != nil {
				return RadialGraphPlanarMoment{}, e
			}
		}
		half := new(big.Rat).Quo(radial, big.NewRat(2, 1))
		if e := b.check(half); e != nil {
			return RadialGraphPlanarMoment{}, e
		}
		for j, axes := range [][2]int{{0, 0}, {0, 1}, {1, 1}} {
			q := new(big.Rat).Mul(r.center[axes[0]], r.center[axes[1]])
			if e := b.check(q); e != nil {
				return RadialGraphPlanarMoment{}, e
			}
			q.Mul(q, volume)
			if e := b.check(q); e != nil {
				return RadialGraphPlanarMoment{}, e
			}
			if axes[0] == axes[1] {
				q.Add(q, half)
				if e := b.check(q); e != nil {
					return RadialGraphPlanarMoment{}, e
				}
			}
			m.SecondMomentPiCoefficients[j] = q
		}
		return m, nil
	}
	area, total, radial := new(big.Rat), new(big.Rat), new(big.Rat)
	out := RadialGraphPlanarMoments{}
	previous := -1
	for _, band := range bands {
		if e = ctx.Err(); e != nil {
			return zero, e
		}
		if band < 0 || band >= len(squares) || band <= previous {
			return zero, ErrInvalid
		}
		previous = band
		lo := new(big.Rat)
		if band > 0 {
			lo = squares[band-1]
		}
		hi := squares[band]
		a := new(big.Rat).Sub(hi, lo)
		if e = b.check(a); e != nil {
			return zero, e
		}
		v, e := integral(primitives[0], lo, hi)
		if e != nil {
			return zero, e
		}
		q, e := integral(primitives[1], lo, hi)
		if e != nil {
			return zero, e
		}
		sign := r.BandSigns()[band]
		if a.Sign() <= 0 || v.Sign() != sign || q.Sign() != sign {
			return zero, ErrConvergence
		}
		v.Abs(v)
		q.Abs(q)
		m, e := moment(a, v, q)
		if e != nil {
			return zero, e
		}
		out.Bands = append(out.Bands, RadialGraphBandPlanarMoment{Band: band, HeightSign: sign, Moment: m})
		area.Add(area, a)
		total.Add(total, v)
		radial.Add(radial, q)
		for _, x := range []*big.Rat{area, total, radial} {
			if e = b.check(x); e != nil {
				return zero, e
			}
		}
	}
	out.Total, e = moment(area, total, radial)
	if e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	out.ExactOperations = b.operations
	return out, nil
}
