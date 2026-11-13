package geom

import "math/big"

// ParameterBounds encloses the entire certified arc. The free coordinate
// is bounded by its endpoints. For the dependent coordinate, interval
// bounds on the implicit derivative tighten the source cell when its
// extrema cannot lie on the contour.
func (a SurfacePlaneArc) ParameterBounds() (UVBox, error) {
	if a.source == nil {
		return UVBox{}, ErrInvalid
	}
	x, y := a.Endpoints[0].Parameters, a.Endpoints[1].Parameters
	d := a.Domain
	b := UVBox{Vec2{d.UMin, d.VMin}, Vec2{d.UMax, d.VMax}}
	budget := &polynomialBudget{maxOperations: 1000000, maxBits: 16384}
	data, e := a.source.arcData(a.plane, budget)
	if e != nil {
		return UVBox{}, e
	}
	fixed := a.Certificate.FixedParameterAxis
	dep, free := data.du[4], data.dv[4]
	if fixed == 1 {
		dep, free = free, dep
	}
	slope, e := intervalDivide(exactInterval{new(big.Rat).Neg(free.hi), new(big.Rat).Neg(free.lo)}, dep, budget)
	if e != nil {
		return UVBox{}, e
	}
	if fixed == 0 {
		b.Min.Y, b.Max.Y = min(x.Min.Y, y.Min.Y), max(x.Max.Y, y.Max.Y)
		if slope.lo.Sign() >= 0 || slope.hi.Sign() <= 0 {
			b.Min.X, b.Max.X = min(x.Min.X, y.Min.X), max(x.Max.X, y.Max.X)
		}
	} else {
		b.Min.X, b.Max.X = min(x.Min.X, y.Min.X), max(x.Max.X, y.Max.X)
		if slope.lo.Sign() >= 0 || slope.hi.Sign() <= 0 {
			b.Min.Y, b.Max.Y = min(x.Min.Y, y.Min.Y), max(x.Max.Y, y.Max.Y)
		}
	}
	return b, nil
}

// ArcEvaluationOptions bounds exact section-root evaluation. Zero values select
// 1e-9 UV and spatial box widths, 1M operations, 16384 bits, 4096 subdivisions.
type ArcEvaluationOptions struct {
	ParameterTolerance, BoxTolerance                   float64
	MaxOperations, MaxCoefficientBits, MaxSubdivisions int
}

// SurfaceArcEvaluation encloses an exact point on the implicit intersection.
// Point is the midpoint of Bounds, not an exact point on the surface/plane.
// Coordinate errors are bounded by the corresponding distances to box faces.
// Derivative is the arc's certified spatial-coordinate derivative diagnostic.
// DependentDerivative encloses d(dependent parameter)/d(free parameter).
type SurfaceArcEvaluation struct {
	Parameters                    UVBox
	Bounds                        Box
	Point                         Vec3
	Derivative                    SurfaceArcCertificate
	DependentDerivative           CertifiedScalarRange
	ExactOperations, Subdivisions int
}

func boxRepresentative(b Box) Vec3 {
	mid := func(lo, hi float64) float64 { return min(hi, max(lo, lo/2+hi/2)) }
	return Vec3{mid(b.Min.X, b.Max.X), mid(b.Min.Y, b.Max.Y), mid(b.Min.Z, b.Max.Z)}
}

// EndpointEvaluation returns a representative and enclosure of an algebraic arc
// endpoint without rounding its independent coordinate before solving. Endpoints
// of contours produced by TraceSurfacePlane share identical canonical boxes.
// The derivative certificate is global to the arc; DependentDerivative is not
// populated by this operation.
func (a SurfacePlaneArc) EndpointEvaluation(index int) (SurfaceArcEvaluation, error) {
	if a.source == nil || index < 0 || index > 1 {
		return SurfaceArcEvaluation{}, ErrInvalid
	}
	ep := a.Endpoints[index]
	return SurfaceArcEvaluation{Parameters: ep.Parameters, Bounds: ep.Bounds, Point: boxRepresentative(ep.Bounds), Derivative: a.Certificate}, nil
}

// Evaluate treats freeParameter as its exact binary rational value. The free
// coordinate is v when FixedParameterAxis=0, and u otherwise. ErrDomain means
// the selected isoparametric line contains no point of this arc.
func (a SurfacePlaneArc) Evaluate(freeParameter float64, o ArcEvaluationOptions) (SurfaceArcEvaluation, error) {
	if !finite(freeParameter) {
		return SurfaceArcEvaluation{}, ErrInvalid
	}
	return a.EvaluateExact(new(big.Rat).SetFloat64(freeParameter), o)
}

// EvaluateExact isolates the unique plane root on an exact rational section.
// It neither extrapolates outside the arc nor assumes the rounded endpoint
// parameter is on the arc. Returned boxes use outward-rounded exact hulls.
func (a SurfacePlaneArc) EvaluateExact(freeParameter *big.Rat, options ArcEvaluationOptions) (result SurfaceArcEvaluation, err error) {
	if a.source == nil || freeParameter == nil {
		return result, ErrInvalid
	}
	o := options
	if o.ParameterTolerance == 0 {
		o.ParameterTolerance = 1e-9
	}
	if o.BoxTolerance == 0 {
		o.BoxTolerance = 1e-9
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 4096
	}
	if !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 || !finite(o.BoxTolerance) || o.BoxTolerance <= 0 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxSubdivisions < 1 {
		return result, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { result.ExactOperations = b.operations }()
	if e := b.check(freeParameter); e != nil {
		return result, e
	}
	p := a.source
	d := p.Domain()
	fixed := a.Certificate.FixedParameterAxis
	start, end, dependentStart, dependentEnd := d.VMin, d.VMax, d.UMin, d.UMax
	if fixed == 1 {
		start, end, dependentStart, dependentEnd = d.UMin, d.UMax, d.VMin, d.VMax
	}
	lower, upper := new(big.Rat).SetFloat64(start), new(big.Rat).SetFloat64(end)
	if freeParameter.Cmp(lower) < 0 || freeParameter.Cmp(upper) > 0 {
		return result, ErrDomain
	}
	t := new(big.Rat).Quo(new(big.Rat).Sub(freeParameter, lower), new(big.Rat).Sub(upper, lower))
	count, other := p.countU, p.countV
	if fixed == 1 {
		count, other = p.countV, p.countU
	}
	net := make([]exactHomogeneous, count)
	for i := 0; i < count; i++ {
		row := make([]exactHomogeneous, other)
		for j := range row {
			index := i*p.countV + j
			if fixed == 1 {
				index = j*p.countV + i
			}
			row[j] = p.net[index]
		}
		point, e := restrictExactBezier(row, t, t, b)
		if e != nil {
			return result, e
		}
		net[i] = point[0]
	}
	coeff := make([]*big.Rat, count)
	equation := exactPlaneCoefficients(a.plane)
	for i, h := range net {
		v := exactPlaneHomogeneousValue(equation, h)
		if e := b.check(v); e != nil {
			return result, e
		}
		coeff[i] = v
	}
	poly, e := bernsteinToPower(coeff, b)
	if e != nil {
		return result, e
	}
	accept := func(lo, hi *big.Rat) (bool, error) {
		span := ParameterSpan{Start: dependentStart, End: dependentEnd}
		dl, dh := rationalFloatBound(parameterMap(span, lo), true), rationalFloatBound(parameterMap(span, hi), false)
		if dh-dl > o.ParameterTolerance {
			return false, nil
		}
		part, e := restrictExactBezier(net, lo, hi, b)
		if e != nil {
			return false, e
		}
		temp := SurfacePatch{net: part}
		if e = temp.computeBounds(b); e != nil {
			return false, e
		}
		size := temp.bounds.Size()
		if size.X > o.BoxTolerance || size.Y > o.BoxTolerance || size.Z > o.BoxTolerance {
			return false, nil
		}
		fl, fh := rationalFloatBound(freeParameter, true), rationalFloatBound(freeParameter, false)
		if fh-fl > o.ParameterTolerance {
			return false, ErrConvergence
		}
		result.Parameters = UVBox{Vec2{dl, fl}, Vec2{dh, fh}}
		if fixed == 1 {
			result.Parameters = UVBox{Vec2{fl, dl}, Vec2{fh, dh}}
		}
		result.Bounds = temp.bounds
		result.Point = boxRepresentative(temp.bounds)
		return true, nil
	}
	// The certificate already proves strict monotonicity in this coordinate
	// on the entire cell, so exact bisection can replace per-section Sturm
	// isolation while retaining the same enclosure guarantees.
	lo, hi := big.NewRat(0, 1), big.NewRat(1, 1)
	a0, e := polynomialValue(poly, lo, b)
	if e != nil {
		return result, e
	}
	a1, e := polynomialValue(poly, hi, b)
	if e != nil {
		return result, e
	}
	switch {
	case a0.Sign() == 0:
		hi = lo
	case a1.Sign() == 0:
		lo = hi
	case a0.Sign() == a1.Sign():
		return result, ErrDomain
	}
	for {
		ok, e := accept(lo, hi)
		if e != nil {
			return result, e
		}
		if ok {
			break
		}
		if lo.Cmp(hi) == 0 || result.Subdivisions >= o.MaxSubdivisions {
			return result, ErrConvergence
		}
		result.Subdivisions++
		mid := new(big.Rat).Quo(new(big.Rat).Add(lo, hi), big.NewRat(2, 1))
		value, e := polynomialValue(poly, mid, b)
		if e != nil {
			return result, e
		}
		switch {
		case value.Sign() == 0:
			lo, hi = mid, mid
		case value.Sign() == a0.Sign():
			lo = mid
		default:
			hi = mid
		}
	}
	data, e := p.arcData(a.plane, b)
	if e != nil {
		return result, e
	}
	fd, fr := data.du[4], data.dv[4]
	if fixed == 1 {
		fd, fr = fr, fd
	}
	slope, e := intervalDivide(exactInterval{new(big.Rat).Neg(fr.hi), new(big.Rat).Neg(fr.lo)}, fd, b)
	if e != nil {
		return result, e
	}
	result.DependentDerivative = certifiedRange(slope)
	result.Derivative = a.Certificate
	return result, nil
}
