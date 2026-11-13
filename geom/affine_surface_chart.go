package geom

import (
	"context"
	"math/big"
)

// AffineSurfaceChartOptions bounds exact affine-chart detection. ProjectionAxes
// defaults to an automatically selected nonsingular coordinate pair. Zero
// limits select 8192 cells, 8M operations and 16384 coefficient bits.
type AffineSurfaceChartOptions struct {
	ProjectionAxes                              [2]int
	MaxCells, MaxOperations, MaxCoefficientBits int
}

// AffineSurfaceChart is an immutable certificate that the source patch equals
// one rank-two affine parameter map on its complete rectangle. Its rational
// coefficients need not fit float64. Inversion/evaluation can extrapolate outside
// the rectangle; domain containment is only a diagnostic.
type AffineSurfaceChart struct {
	domain       ParameterRectangle
	origin, u, v [3]*big.Rat // normalized unit-domain affine coefficients
	axes         [2]int
	determinant  *big.Rat
	options      AffineSurfaceChartOptions
	operations   int
}

// AffineChartInverse gives caller-owned rational original UV and outward bounds.
// OnPlane checks all three model coordinates exactly, not a distance tolerance.
// Inside is true only for an on-plane point in the closed original rectangle.
// For off-plane points, UV is a coordinate-axis projection inverse, not generally
// the orthogonal nearest point. Returned rationals never alias chart storage.
type AffineChartInverse struct {
	U, V             *big.Rat
	UBounds, VBounds CertifiedScalarRange
	OnPlane, Inside  bool
	ExactOperations  int
}

// NewAffineSurfaceChart certifies a rational spline chart by comparing it with
// the exact affine map proposed by three corners. ErrConvergence denotes a
// nonaffine or unproved chart; ErrDegenerate denotes rank-deficient corner axes.
func NewAffineSurfaceChart(ctx context.Context, p *SurfacePatch, o AffineSurfaceChartOptions) (*AffineSurfaceChart, error) {
	if ctx == nil || p == nil || len(p.net) == 0 {
		return nil, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	automatic := o.ProjectionAxes == [2]int{}
	if !automatic && (o.ProjectionAxes[0] < 0 || o.ProjectionAxes[0] > 2 || o.ProjectionAxes[1] < 0 || o.ProjectionAxes[1] > 2 || o.ProjectionAxes[0] == o.ProjectionAxes[1]) {
		return nil, ErrInvalid
	}
	if o.MaxCells == 0 {
		o.MaxCells = 8192
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxCells < 1 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return nil, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	chart := &AffineSurfaceChart{domain: p.Domain(), options: o}
	indices := [3]int{0, (p.countU - 1) * p.countV, p.countV - 1}
	var corners [3][3]*big.Rat
	for i, index := range indices {
		h := p.net[index]
		for axis := 0; axis < 3; axis++ {
			corners[i][axis] = new(big.Rat).Quo(h[axis], h[3])
			if e := b.check(corners[i][axis]); e != nil {
				return nil, e
			}
		}
	}
	for axis := 0; axis < 3; axis++ {
		chart.origin[axis] = new(big.Rat).Set(corners[0][axis])
		chart.u[axis] = new(big.Rat).Sub(corners[1][axis], corners[0][axis])
		chart.v[axis] = new(big.Rat).Sub(corners[2][axis], corners[0][axis])
		if e := b.check(chart.u[axis]); e != nil {
			return nil, e
		}
		if e := b.check(chart.v[axis]); e != nil {
			return nil, e
		}
	}
	choices := [][2]int{o.ProjectionAxes}
	if automatic {
		choices = [][2]int{{0, 1}, {0, 2}, {1, 2}}
	}
	for _, axes := range choices {
		i, j := axes[0], axes[1]
		det := new(big.Rat).Sub(new(big.Rat).Mul(chart.u[i], chart.v[j]), new(big.Rat).Mul(chart.u[j], chart.v[i]))
		if e := b.check(det); e != nil {
			return nil, e
		}
		if det.Sign() != 0 {
			chart.axes = axes
			chart.determinant = det
			break
		}
	}
	if chart.determinant == nil {
		return nil, ErrDegenerate
	}
	net := make([]exactHomogeneous, 4)
	for i := 0; i < 2; i++ {
		for j := 0; j < 2; j++ {
			index := 2*i + j
			for axis := 0; axis < 3; axis++ {
				value := new(big.Rat).Set(chart.origin[axis])
				if i == 1 {
					value.Add(value, chart.u[axis])
				}
				if j == 1 {
					value.Add(value, chart.v[axis])
				}
				if e := b.check(value); e != nil {
					return nil, e
				}
				net[index][axis] = value
			}
			net[index][3] = big.NewRat(1, 1)
		}
	}
	d := chart.domain
	candidate := &SurfacePatch{degreeU: 1, degreeV: 1, countU: 2, countV: 2, knotsU: []float64{d.UMin, d.UMin, d.UMax, d.UMax}, knotsV: []float64{d.VMin, d.VMin, d.VMax, d.VMax}, net: net, options: p.options}
	if e := candidate.computeBounds(b); e != nil {
		return nil, e
	}
	remaining := o.MaxOperations - b.operations
	if remaining < 1 {
		return nil, ErrConvergence
	}
	comparison, e := CompareExactSurfaces(ctx, p, candidate, ExactSurfaceComparisonOptions{MaxCells: o.MaxCells, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return nil, e
	}
	if !comparison.Complete || !comparison.Equal {
		return nil, ErrConvergence
	}
	chart.operations = b.operations + comparison.ExactOperations
	return chart, nil
}

// Domain returns the certified source parameter rectangle.
func (c *AffineSurfaceChart) Domain() ParameterRectangle {
	if c == nil {
		return ParameterRectangle{}
	}
	return c.domain
}

// ProjectionAxes identifies the exact nonsingular minor used for inversion.
func (c *AffineSurfaceChart) ProjectionAxes() [2]int {
	if c == nil {
		return [2]int{-1, -1}
	}
	return c.axes
}

// ExactOperations reports chart construction work only.
func (c *AffineSurfaceChart) ExactOperations() int {
	if c == nil {
		return 0
	}
	return c.operations
}

// Invert interprets the represented model point exactly and permits extrapolation.
func (c *AffineSurfaceChart) Invert(point Vec3) (AffineChartInverse, error) {
	if !point.IsFinite() {
		return AffineChartInverse{}, ErrInvalid
	}
	return c.InvertExact([3]*big.Rat{new(big.Rat).SetFloat64(point.X), new(big.Rat).SetFloat64(point.Y), new(big.Rat).SetFloat64(point.Z)})
}

// InvertExact solves the projected affine equations and checks model-plane
// membership exactly. It accepts points outside the domain. Inputs are read-only
// and all returned rationals belong to the caller. Nil inputs are invalid.
func (c *AffineSurfaceChart) InvertExact(point [3]*big.Rat) (AffineChartInverse, error) {
	if c == nil {
		return AffineChartInverse{}, ErrInvalid
	}
	return c.InvertExactContext(context.Background(), point, AffineChartInverseOptions{MaxOperations: c.options.MaxOperations, MaxCoefficientBits: c.options.MaxCoefficientBits})
}

// InvertExactContext supplies cancellation and independent exact arithmetic
// limits for inversion. It retains InvertExact's extrapolation semantics.
func (c *AffineSurfaceChart) InvertExactContext(ctx context.Context, point [3]*big.Rat, o AffineChartInverseOptions) (r AffineChartInverse, err error) {
	if ctx == nil || c == nil || c.determinant == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 100000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	var delta [3]*big.Rat
	for axis, x := range point {
		if x == nil {
			return r, ErrInvalid
		}
		if e := b.check(x); e != nil {
			return r, e
		}
		delta[axis] = new(big.Rat).Sub(x, c.origin[axis])
		if e := b.check(delta[axis]); e != nil {
			return r, e
		}
	}
	i, j := c.axes[0], c.axes[1]
	u := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(delta[i], c.v[j]), new(big.Rat).Mul(delta[j], c.v[i])), c.determinant)
	v := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(c.u[i], delta[j]), new(big.Rat).Mul(c.u[j], delta[i])), c.determinant)
	if e := b.check(u); e != nil {
		return r, e
	}
	if e := b.check(v); e != nil {
		return r, e
	}
	r.OnPlane = true
	for axis := 0; axis < 3; axis++ {
		expected := new(big.Rat).Add(new(big.Rat).Mul(c.u[axis], u), new(big.Rat).Mul(c.v[axis], v))
		if e := b.check(expected); e != nil {
			return r, e
		}
		if expected.Cmp(delta[axis]) != 0 {
			r.OnPlane = false
		}
	}
	r.Inside = r.OnPlane && u.Sign() >= 0 && u.Cmp(big.NewRat(1, 1)) <= 0 && v.Sign() >= 0 && v.Cmp(big.NewRat(1, 1)) <= 0
	r.U = parameterMap(ParameterSpan{Start: c.domain.UMin, End: c.domain.UMax}, u)
	r.V = parameterMap(ParameterSpan{Start: c.domain.VMin, End: c.domain.VMax}, v)
	if e := b.check(r.U); e != nil {
		return r, e
	}
	if e := b.check(r.V); e != nil {
		return r, e
	}
	r.UBounds = certifiedRange(exactInterval{r.U, r.U})
	r.VBounds = certifiedRange(exactInterval{r.V, r.V})
	return r, nil
}

// EvalExact evaluates the affine map at arbitrary exact original UV, including
// extrapolated points. All results are caller-owned. It does not test Inside.
func (c *AffineSurfaceChart) EvalExact(u, v *big.Rat) ([3]*big.Rat, error) {
	var result [3]*big.Rat
	if c == nil || c.determinant == nil || u == nil || v == nil {
		return result, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: c.options.MaxOperations, maxBits: c.options.MaxCoefficientBits}
	if e := b.check(u); e != nil {
		return result, e
	}
	if e := b.check(v); e != nil {
		return result, e
	}
	normalize := func(x *big.Rat, lo, hi float64) *big.Rat {
		return new(big.Rat).Quo(new(big.Rat).Sub(x, new(big.Rat).SetFloat64(lo)), new(big.Rat).Sub(new(big.Rat).SetFloat64(hi), new(big.Rat).SetFloat64(lo)))
	}
	s, t := normalize(u, c.domain.UMin, c.domain.UMax), normalize(v, c.domain.VMin, c.domain.VMax)
	for axis := 0; axis < 3; axis++ {
		result[axis] = new(big.Rat).Add(c.origin[axis], new(big.Rat).Add(new(big.Rat).Mul(c.u[axis], s), new(big.Rat).Mul(c.v[axis], t)))
		if e := b.check(result[axis]); e != nil {
			return result, e
		}
	}
	return result, nil
}
