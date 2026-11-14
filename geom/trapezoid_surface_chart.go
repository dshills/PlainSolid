package geom

import (
	"context"
	"math/big"
)

// TrapezoidSurfaceChart proves the complete original rational surface equals
// O+A*s+B*(1+k*s)*t on normalized original coordinates. A and B are independent
// and 1+k*s is strictly positive on [0,1]. Thus its planar trapezoid is embedded
// and the original inverse is rational, not an affine-chart approximation.
// The parallel sides correspond to original U endpoints. Other orientations
// and nontrapezoidal planar maps are not inferred.
type TrapezoidSurfaceChart struct {
	domain     ParameterRectangle
	plane      *AffineSurfaceChart
	k          *big.Rat
	operations int
}

func NewTrapezoidSurfaceChart(ctx context.Context, p *SurfacePatch, o AffineSurfaceChartOptions) (result *TrapezoidSurfaceChart, err error) {
	if ctx == nil || p == nil || len(p.net) == 0 {
		return nil, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxCells == 0 {
		o.MaxCells = 8192
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxCells < 1 {
		return nil, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	corners := [4][3]*big.Rat{}
	for i, index := range []int{0, p.countV - 1, (p.countU - 1) * p.countV, len(p.net) - 1} {
		for j := 0; j < 3; j++ {
			h := p.net[index]
			if e := b.check(h[j]); e != nil {
				return nil, e
			}
			if e := b.check(h[3]); e != nil {
				return nil, e
			}
			corners[i][j] = new(big.Rat).Quo(h[j], h[3])
			if e := b.check(corners[i][j]); e != nil {
				return nil, e
			}
		}
	}
	var A, B, C [3]*big.Rat
	var k *big.Rat
	for j := 0; j < 3; j++ {
		A[j] = new(big.Rat).Sub(corners[2][j], corners[0][j])
		B[j] = new(big.Rat).Sub(corners[1][j], corners[0][j])
		C[j] = new(big.Rat).Sub(new(big.Rat).Sub(corners[3][j], corners[2][j]), B[j])
		for _, v := range []*big.Rat{A[j], B[j], C[j]} {
			if e := b.check(v); e != nil {
				return nil, e
			}
		}
		if B[j].Sign() != 0 && k == nil {
			k = new(big.Rat).Quo(C[j], B[j])
			if e := b.check(k); e != nil {
				return nil, e
			}
		}
	}
	if k == nil {
		return nil, ErrDegenerate
	}
	if new(big.Rat).Add(big.NewRat(1, 1), k).Sign() <= 0 {
		return nil, ErrConvergence
	}
	for j := 0; j < 3; j++ {
		q := new(big.Rat).Mul(k, B[j])
		if e := b.check(q); e != nil {
			return nil, e
		}
		if q.Cmp(C[j]) != 0 {
			return nil, ErrConvergence
		}
	}
	net := make([]exactHomogeneous, 4)
	affine := make([][4]*big.Rat, 4)
	for i := 0; i < 4; i++ {
		for j := 0; j < 3; j++ {
			net[i][j] = new(big.Rat).Set(corners[i][j])
			affine[i][j] = new(big.Rat).Set(corners[0][j])
			if i/2 == 1 {
				affine[i][j].Add(affine[i][j], A[j])
			}
			if i%2 == 1 {
				affine[i][j].Add(affine[i][j], B[j])
			}
		}
		net[i][3] = big.NewRat(1, 1)
		affine[i][3] = big.NewRat(1, 1)
	}
	d := p.Domain()
	candidate := &SurfacePatch{degreeU: 1, degreeV: 1, countU: 2, countV: 2, knotsU: []float64{d.UMin, d.UMin, d.UMax, d.UMax}, knotsV: []float64{d.VMin, d.VMin, d.VMax, d.VMax}, net: net, options: p.options}
	if e := candidate.computeBounds(b); e != nil {
		return nil, e
	}
	left := func() (int, error) {
		if e := ctx.Err(); e != nil {
			return 0, e
		}
		n := o.MaxOperations - b.operations
		if n < 1 {
			return 0, ErrConvergence
		}
		return n, nil
	}
	n, e := left()
	if e != nil {
		return nil, e
	}
	comparison, e := CompareExactSurfaces(ctx, p, candidate, ExactSurfaceComparisonOptions{MaxCells: o.MaxCells, MaxOperations: n, MaxCoefficientBits: o.MaxCoefficientBits})
	b.operations += comparison.ExactOperations
	if e != nil {
		return nil, e
	}
	if !comparison.Complete || !comparison.Equal {
		return nil, ErrConvergence
	}
	n, e = left()
	if e != nil {
		return nil, e
	}
	planar, e := NewExactHomogeneousBezierSurface(ctx, 1, 1, affine, SurfacePatchOptions{MaxOperations: n, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return nil, e
	}
	b.operations += planar.ExactOperations()
	n, e = left()
	if e != nil {
		return nil, e
	}
	o.MaxOperations = n
	chart, e := NewAffineSurfaceChart(ctx, planar, o)
	if e != nil {
		return nil, e
	}
	b.operations += chart.ExactOperations()
	return &TrapezoidSurfaceChart{domain: d, plane: chart, k: k, operations: b.operations}, nil
}
func (c *TrapezoidSurfaceChart) Domain() ParameterRectangle {
	if c == nil {
		return ParameterRectangle{}
	}
	return c.domain
}
func (c *TrapezoidSurfaceChart) ExactOperations() int {
	if c == nil {
		return 0
	}
	return c.operations
}
func (c *TrapezoidSurfaceChart) RawSupportPlane(o SurfacePatchOptions) (RawSupportPlane, error) {
	if c == nil || c.plane == nil {
		return RawSupportPlane{}, ErrInvalid
	}
	return c.plane.RawSupportPlane(o)
}

// ProjectIsoCurveToTrapezoidChart proves complete source-plane incidence, then
// constructs exact original UV as homogeneous polynomial products. This bounded
// slice supports one Bezier span of degree <=6 and strictly positive inverse
// denominator Bernstein coefficients on the entire source interval. Failure
// publishes no projected curve. The source parameter and orientation are kept.
func ProjectIsoCurveToTrapezoidChart(ctx context.Context, c *SurfaceIsoCurve, chart *TrapezoidSurfaceChart, o SurfacePatchOptions) (out *SurfaceIsoCurve, operations int, err error) {
	if ctx == nil || c == nil || c.patch == nil || chart == nil || chart.plane == nil {
		return nil, 0, ErrInvalid
	}
	opts, e := patchOptions(o)
	if e != nil {
		return nil, 0, e
	}
	p := c.patch
	if p.degreeU > 6 || p.countU != p.degreeU+1 || p.countU > opts.MaxPoles/4 {
		return nil, 0, ErrConvergence
	}
	b := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	defer func() { operations = b.operations }()
	if e := b.check(chart.k); e != nil {
		return nil, 0, e
	}
	q, e := projectIsoCurveChart(ctx, c, chart.plane, b)
	if e != nil {
		return nil, 0, e
	}
	var coeff [4][]*big.Rat
	for j := range coeff {
		coeff[j] = make([]*big.Rat, p.countU)
	}
	for i := 0; i < p.countU; i++ {
		h := q.patch.net[i*q.patch.countV]
		coeff[0][i] = h[0]
		coeff[1][i] = h[1]
		coeff[2][i] = h[3]
		coeff[3][i] = new(big.Rat).Add(h[3], new(big.Rat).Mul(chart.k, h[0]))
		if e := b.check(coeff[3][i]); e != nil {
			return nil, 0, e
		}
		if coeff[3][i].Sign() <= 0 {
			return nil, 0, ErrConvergence
		}
	}
	x, e := exactBernsteinScalarProduct(coeff[0], coeff[3], b)
	if e != nil {
		return nil, 0, e
	}
	y, e := exactBernsteinScalarProduct(coeff[1], coeff[2], b)
	if e != nil {
		return nil, 0, e
	}
	w, e := exactBernsteinScalarProduct(coeff[2], coeff[3], b)
	if e != nil {
		return nil, 0, e
	}
	d := chart.domain
	umin := new(big.Rat).SetFloat64(d.UMin)
	vmin := new(big.Rat).SetFloat64(d.VMin)
	us := new(big.Rat).Sub(new(big.Rat).SetFloat64(d.UMax), umin)
	vs := new(big.Rat).Sub(new(big.Rat).SetFloat64(d.VMax), vmin)
	net := make([]exactHomogeneous, 2*len(w))
	for i := range w {
		h := exactHomogeneous{new(big.Rat).Add(new(big.Rat).Mul(umin, w[i]), new(big.Rat).Mul(us, x[i])), new(big.Rat).Add(new(big.Rat).Mul(vmin, w[i]), new(big.Rat).Mul(vs, y[i])), new(big.Rat), w[i]}
		for _, v := range h {
			if e := b.check(v); e != nil {
				return nil, 0, e
			}
		}
		net[2*i], net[2*i+1] = h, h
	}
	lo, hi := c.Domain()
	degree := len(w) - 1
	knots := make([]float64, 2*(degree+1))
	for i := range knots {
		knots[i] = lo
		if i > degree {
			knots[i] = hi
		}
	}
	patch := &SurfacePatch{degreeU: degree, degreeV: 1, countU: degree + 1, countV: 2, knotsU: knots, knotsV: []float64{0, 0, 1, 1}, net: net, options: opts}
	if e := patch.computeBounds(b); e != nil {
		return nil, 0, e
	}
	return &SurfaceIsoCurve{patch: patch, fixedAxis: c.fixedAxis, fixedParameter: c.fixedParameter, operations: b.operations}, b.operations, nil
}
