package geom

import "math/big"

type exactInterval struct{ lo, hi *big.Rat }

func boundedExactInterval(lo, hi *big.Rat, b *polynomialBudget) (exactInterval, error) {
	if e := b.check(lo); e != nil {
		return exactInterval{}, e
	}
	if e := b.check(hi); e != nil {
		return exactInterval{}, e
	}
	return exactInterval{lo, hi}, nil
}
func intervalSubtract(a, c exactInterval, b *polynomialBudget) (exactInterval, error) {
	return boundedExactInterval(new(big.Rat).Sub(a.lo, c.hi), new(big.Rat).Sub(a.hi, c.lo), b)
}
func intervalMultiply(a, c exactInterval, b *polynomialBudget) (exactInterval, error) {
	products := []*big.Rat{new(big.Rat).Mul(a.lo, c.lo), new(big.Rat).Mul(a.lo, c.hi), new(big.Rat).Mul(a.hi, c.lo), new(big.Rat).Mul(a.hi, c.hi)}
	minimum, maximum := products[0], products[0]
	for _, v := range products {
		if e := b.check(v); e != nil {
			return exactInterval{}, e
		}
		if v.Cmp(minimum) < 0 {
			minimum = v
		}
		if v.Cmp(maximum) > 0 {
			maximum = v
		}
	}
	return exactInterval{minimum, maximum}, nil
}
func intervalDivide(a, c exactInterval, b *polynomialBudget) (exactInterval, error) {
	if c.lo.Sign() <= 0 && c.hi.Sign() >= 0 {
		return exactInterval{}, ErrDegenerate
	}
	reciprocal, e := boundedExactInterval(new(big.Rat).Inv(c.hi), new(big.Rat).Inv(c.lo), b)
	if e != nil {
		return exactInterval{}, e
	}
	return intervalMultiply(a, reciprocal, b)
}
func (i exactInterval) sign() int {
	if i.lo.Sign() > 0 {
		return 1
	}
	if i.hi.Sign() < 0 {
		return -1
	}
	return 0
}
func coefficientRange(values []*big.Rat) exactInterval {
	minimum, maximum := values[0], values[0]
	for _, v := range values[1:] {
		if v.Cmp(minimum) < 0 {
			minimum = v
		}
		if v.Cmp(maximum) > 0 {
			maximum = v
		}
	}
	return exactInterval{minimum, maximum}
}
func scalarPartial(values []*big.Rat, countU, countV int, uDirection bool, domain ParameterRectangle, b *polynomialBudget) ([]*big.Rat, error) {
	degree, count, other := countU-1, countU, countV
	width := new(big.Rat).Sub(new(big.Rat).SetFloat64(domain.UMax), new(big.Rat).SetFloat64(domain.UMin))
	if !uDirection {
		degree, count, other = countV-1, countV, countU
		width.Sub(new(big.Rat).SetFloat64(domain.VMax), new(big.Rat).SetFloat64(domain.VMin))
	}
	scale := new(big.Rat).Quo(big.NewRat(int64(degree), 1), width)
	result := make([]*big.Rat, (count-1)*other)
	for i := 0; i < count-1; i++ {
		for j := 0; j < other; j++ {
			before, after, index := i*countV+j, (i+1)*countV+j, i*other+j
			if !uDirection {
				before, after, index = j*countV+i, j*countV+i+1, j*(count-1)+i
			}
			result[index] = new(big.Rat).Mul(scale, new(big.Rat).Sub(values[after], values[before]))
			if e := b.check(result[index]); e != nil {
				return nil, e
			}
		}
	}
	return result, nil
}

// CertifiedScalarRange encloses an exact rational expression. Sign remains
// exact even if conversion rounds a strictly signed endpoint to zero.
type CertifiedScalarRange struct {
	Min, Max float64
	Sign     int
}

func certifiedRange(interval exactInterval) CertifiedScalarRange {
	return CertifiedScalarRange{rationalFloatBound(interval.lo, true), rationalFloatBound(interval.hi, false), interval.sign()}
}

// SurfaceArcCertificate proves a regular implicit parameter graph and strict
// spatial monotonicity. FixedParameterAxis 0 means F_u is nonzero (free axis v);
// 1 means F_v is nonzero (free axis u). WorldAxis is 0=x,1=y,2=z.
// WorldDerivative encloses the chosen coordinate's derivative along the free
// parameter on F=0. Strict sign makes the spatial arc injective and regular.
type SurfaceArcCertificate struct {
	FixedParameterAxis, WorldAxis             int
	PlaneNumeratorDerivative, WorldDerivative CertifiedScalarRange
}
type surfaceArcData struct {
	fields       [5][]*big.Rat
	base, du, dv [5]exactInterval
}

func (p *SurfacePatch) arcData(plane Plane, b *polynomialBudget) (surfaceArcData, error) {
	data := surfaceArcData{}
	center := p.bounds.Min
	equation := exactPlaneCoefficients(plane)
	offset := new(big.Rat).Set(equation[3])
	for axis := 0; axis < 3; axis++ {
		offset.Add(offset, new(big.Rat).Mul(equation[axis], new(big.Rat).SetFloat64(coordinate(center, axis))))
	}

	for field := range data.fields {
		data.fields[field] = make([]*big.Rat, len(p.net))
	}
	for i, h := range p.net {
		data.fields[3][i] = h[3]
		f := new(big.Rat).Mul(offset, h[3])
		for axis := 0; axis < 3; axis++ {
			centered := new(big.Rat).Sub(h[axis], new(big.Rat).Mul(new(big.Rat).SetFloat64(coordinate(center, axis)), h[3]))
			if e := b.check(centered); e != nil {
				return data, e
			}
			data.fields[axis][i] = centered
			f.Add(f, new(big.Rat).Mul(centered, equation[axis]))
		}
		if e := b.check(f); e != nil {
			return data, e
		}
		data.fields[4][i] = f
	}
	domain := p.Domain()
	for field, values := range data.fields {
		data.base[field] = coefficientRange(values)
		du, e := scalarPartial(values, p.countU, p.countV, true, domain, b)
		if e != nil {
			return data, e
		}
		dv, e := scalarPartial(values, p.countU, p.countV, false, domain, b)
		if e != nil {
			return data, e
		}
		data.du[field], data.dv[field] = coefficientRange(du), coefficientRange(dv)
	}
	return data, nil
}
func (data surfaceArcData) certify(fixedAxis int, b *polynomialBudget) (SurfaceArcCertificate, bool, error) {
	fixed, free := data.du, data.dv
	if fixedAxis == 1 {
		fixed, free = data.dv, data.du
	}
	if fixed[4].sign() == 0 {
		return SurfaceArcCertificate{}, false, nil
	}
	weightSquared, e := intervalMultiply(data.base[3], data.base[3], b)
	if e != nil {
		return SurfaceArcCertificate{}, false, e
	}
	denominator, e := intervalMultiply(weightSquared, fixed[4], b)
	if e != nil {
		return SurfaceArcCertificate{}, false, e
	}
	for world := 0; world < 3; world++ {
		fixedProduct, e := intervalMultiply(fixed[world], data.base[3], b)
		if e != nil {
			return SurfaceArcCertificate{}, false, e
		}
		fixedWeight, e := intervalMultiply(data.base[world], fixed[3], b)
		if e != nil {
			return SurfaceArcCertificate{}, false, e
		}
		fixedNumerator, e := intervalSubtract(fixedProduct, fixedWeight, b)
		if e != nil {
			return SurfaceArcCertificate{}, false, e
		}
		freeProduct, e := intervalMultiply(free[world], data.base[3], b)
		if e != nil {
			return SurfaceArcCertificate{}, false, e
		}
		freeWeight, e := intervalMultiply(data.base[world], free[3], b)
		if e != nil {
			return SurfaceArcCertificate{}, false, e
		}
		freeNumerator, e := intervalSubtract(freeProduct, freeWeight, b)
		if e != nil {
			return SurfaceArcCertificate{}, false, e
		}
		first, e := intervalMultiply(freeNumerator, fixed[4], b)
		if e != nil {
			return SurfaceArcCertificate{}, false, e
		}
		second, e := intervalMultiply(fixedNumerator, free[4], b)
		if e != nil {
			return SurfaceArcCertificate{}, false, e
		}
		tangent, e := intervalSubtract(first, second, b)
		if e != nil {
			return SurfaceArcCertificate{}, false, e
		}
		if tangent.sign() == 0 {
			continue
		}
		derivative, e := intervalDivide(tangent, denominator, b)
		if e != nil {
			return SurfaceArcCertificate{}, false, e
		}
		return SurfaceArcCertificate{fixedAxis, world, certifiedRange(fixed[4]), certifiedRange(derivative)}, true, nil
	}
	return SurfaceArcCertificate{}, false, nil
}
