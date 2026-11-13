package geom

import "math/big"

// SurfacePairIntervalOptions optionally fixes one of (u,v,s,t) at the exact
// rational value of FixedParameter. FixedAxis is 0..3 when UseFixedParameter.
// Zero arithmetic limits select 1M operations and 16384 coefficient bits.
type SurfacePairIntervalOptions struct {
	UseFixedParameter                 bool
	FixedAxis                         int
	FixedParameter                    float64
	MaxOperations, MaxCoefficientBits int
}

// SurfacePairIntervals encloses S(u,v)-T(s,t) and its full 3x4 Jacobian, whose
// columns are [Su,Sv,-Ts,-Tt]. All entries are outward bounds from exact rational
// arithmetic on stored coefficients. Fixed-coordinate sections retain all four
// Jacobian columns; a three-variable solver omits the fixed column.
// ExcludesContact proves a residual component cannot vanish. Otherwise no
// existence, uniqueness, or regularity conclusion follows from these intervals.
type SurfacePairIntervals struct {
	Residual        [3]CertifiedScalarRange
	Jacobian        [3][4]CertifiedScalarRange
	ExcludesContact bool
	ExactOperations int
}

func restrictedScalarRange(values []*big.Rat, nu, nv, axis int, t *big.Rat, b *polynomialBudget) (exactInterval, error) {
	if axis < 0 {
		return coefficientRange(values), nil
	}
	count, rows := nu, nv
	if axis == 1 {
		count, rows = nv, nu
	}
	result := make([]*big.Rat, rows)
	complement := new(big.Rat).Sub(big.NewRat(1, 1), t)
	for row := 0; row < rows; row++ {
		work := make([]*big.Rat, count)
		for i := range work {
			index := i*nv + row
			if axis == 1 {
				index = row*nv + i
			}
			work[i] = values[index]
		}
		for level := 1; level < count; level++ {
			for i := 0; i < count-level; i++ {
				value := new(big.Rat).Add(new(big.Rat).Mul(complement, work[i]), new(big.Rat).Mul(t, work[i+1]))
				if e := b.check(value); e != nil {
					return exactInterval{}, e
				}
				work[i] = value
			}
		}
		result[row] = work[0]
	}
	return coefficientRange(result), nil
}
func surfaceDifferentialIntervals(p *SurfacePatch, axis int, parameter float64, b *polynomialBudget) (position [3]exactInterval, derivative [3][2]exactInterval, err error) {
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	data, e := p.arcData(plane, b)
	if e != nil {
		return position, derivative, e
	}
	normalized := new(big.Rat)
	if axis >= 0 {
		d := p.Domain()
		lo, hi := d.UMin, d.UMax
		if axis == 1 {
			lo, hi = d.VMin, d.VMax
		}
		if !finite(parameter) || parameter < lo || parameter > hi {
			return position, derivative, ErrDomain
		}
		normalized.Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(parameter), new(big.Rat).SetFloat64(lo)), new(big.Rat).Sub(new(big.Rat).SetFloat64(hi), new(big.Rat).SetFloat64(lo)))
	}
	var base, du, dv [4]exactInterval
	for field := 0; field < 4; field++ {
		values := data.fields[field]
		base[field], e = restrictedScalarRange(values, p.countU, p.countV, axis, normalized, b)
		if e != nil {
			return position, derivative, e
		}
		u, e := scalarPartial(values, p.countU, p.countV, true, p.Domain(), b)
		if e != nil {
			return position, derivative, e
		}
		v, e := scalarPartial(values, p.countU, p.countV, false, p.Domain(), b)
		if e != nil {
			return position, derivative, e
		}
		du[field], e = restrictedScalarRange(u, p.countU-1, p.countV, axis, normalized, b)
		if e != nil {
			return position, derivative, e
		}
		dv[field], e = restrictedScalarRange(v, p.countU, p.countV-1, axis, normalized, b)
		if e != nil {
			return position, derivative, e
		}
	}
	denominator, e := intervalMultiply(base[3], base[3], b)
	if e != nil {
		return position, derivative, e
	}
	for world := 0; world < 3; world++ {
		position[world], e = intervalDivide(base[world], base[3], b)
		if e != nil {
			return position, derivative, e
		}
		center := new(big.Rat).SetFloat64(coordinate(p.bounds.Min, world))
		position[world], e = boundedExactInterval(new(big.Rat).Add(position[world].lo, center), new(big.Rat).Add(position[world].hi, center), b)
		if e != nil {
			return position, derivative, e
		}
		for direction := 0; direction < 2; direction++ {
			d := du
			if direction == 1 {
				d = dv
			}
			first, e := intervalMultiply(d[world], base[3], b)
			if e != nil {
				return position, derivative, e
			}
			second, e := intervalMultiply(base[world], d[3], b)
			if e != nil {
				return position, derivative, e
			}
			numerator, e := intervalSubtract(first, second, b)
			if e != nil {
				return position, derivative, e
			}
			derivative[world][direction], e = intervalDivide(numerator, denominator, b)
			if e != nil {
				return position, derivative, e
			}
		}
	}
	return position, derivative, nil
}

// BoundSurfacePairDifferentials provides certified residual/Jacobian hulls for
// two single Bezier rectangles, optionally on a fixed-coordinate 3D section.
// Internal knot lines must first be split/trimmed; otherwise ErrDomain results.
// Fixed-coordinate Bernstein collapse is exact, not a thin-box approximation.
func BoundSurfacePairDifferentials(a, c *SurfacePatch, options SurfacePairIntervalOptions) (result SurfacePairIntervals, err error) {
	if a == nil || c == nil || len(a.net) == 0 || len(c.net) == 0 {
		return result, ErrInvalid
	}
	for _, p := range []*SurfacePatch{a, c} {
		if p.countU != p.degreeU+1 || p.countV != p.degreeV+1 {
			return result, ErrDomain
		}
	}
	o := options
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || (o.UseFixedParameter && (o.FixedAxis < 0 || o.FixedAxis > 3 || !finite(o.FixedParameter))) {
		return result, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { result.ExactOperations = b.operations }()
	aa, ca := -1, -1
	if o.UseFixedParameter {
		if o.FixedAxis < 2 {
			aa = o.FixedAxis
		} else {
			ca = o.FixedAxis - 2
		}
	}
	ap, ad, e := surfaceDifferentialIntervals(a, aa, o.FixedParameter, b)
	if e != nil {
		return result, e
	}
	cp, cd, e := surfaceDifferentialIntervals(c, ca, o.FixedParameter, b)
	if e != nil {
		return result, e
	}
	for world := 0; world < 3; world++ {
		residual, e := intervalSubtract(ap[world], cp[world], b)
		if e != nil {
			return result, e
		}
		result.Residual[world] = certifiedRange(residual)
		result.ExcludesContact = result.ExcludesContact || residual.sign() != 0
		for direction := 0; direction < 2; direction++ {
			result.Jacobian[world][direction] = certifiedRange(ad[world][direction])
			negative := exactInterval{new(big.Rat).Neg(cd[world][direction].hi), new(big.Rat).Neg(cd[world][direction].lo)}
			result.Jacobian[world][direction+2] = certifiedRange(negative)
		}
	}
	return result, nil
}
