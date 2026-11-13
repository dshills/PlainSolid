package geom

import (
	"context"
	"math/big"
)

// GraphQuadraticEllipseHint encodes C+Lu*u+Qu*u²+Lv*v+Qv*v²+M*u*v in
// original UV coordinates, in that order. Its quadratic matrix must be positive
// definite and its minimum strictly negative. All coefficients must be nonnil.
type GraphQuadraticEllipseHint struct{ Coefficients [6]*big.Rat }

// SplineGraphQuadraticEllipse proves one complete simple closed contact curve.
// ProjectionRadiusSquared are the squared coordinate extents about Center,
// computed by exact Schur complement (not the principal semi-axis lengths).
// Rational conic coefficients do not imply rational control-point export: a
// rational point on this conic is needed for rational parametrization.
type SplineGraphQuadraticEllipse struct {
	Proven                          bool
	Ellipse                         GraphQuadraticEllipseHint
	Center, ProjectionRadiusSquared [2]*big.Rat
	Domain                          ParameterRectangle
	InsideSide, OutsideSide         int
	Graph                           SurfaceInjectivity
	CertifiedCells                  []ParameterRectangle
	ExactOperations                 int
}

// CertifySplineGraphQuadraticEllipse proves strict full-domain containment and
// a common definite quadratic factor of every union-cell height numerator.
// A strict Bernstein residual excludes all other contact. The ellipse need not
// remain Euclidean elliptical after the original injective XY chart mapping.
func CertifySplineGraphQuadraticEllipse(ctx context.Context, a, c *SurfacePatch, h GraphQuadraticEllipseHint, o GraphPointContactOptions) (r SplineGraphQuadraticEllipse, err error) {
	if ctx == nil || a == nil || c == nil || len(a.net) == 0 || len(c.net) == 0 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxCells < 1 || o.MaxDepth < 0 {
		return r, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = budget.operations }()
	for i, x := range h.Coefficients {
		if x == nil {
			return r, ErrInvalid
		}
		if e := budget.check(x); e != nil {
			return r, e
		}
		r.Ellipse.Coefficients[i] = new(big.Rat).Set(x)
	}
	q := r.Ellipse.Coefficients
	determinant := new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(q[2], q[4])), new(big.Rat).Mul(q[5], q[5]))
	if q[2].Sign() <= 0 || determinant.Sign() <= 0 {
		return r, ErrConvergence
	}
	r.Center[0] = new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(q[5], q[3]), new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(q[4], q[1]))), determinant)
	r.Center[1] = new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(q[5], q[1]), new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(q[2], q[3]))), determinant)
	minimum := new(big.Rat).Add(q[0], new(big.Rat).Quo(new(big.Rat).Add(new(big.Rat).Mul(q[1], r.Center[0]), new(big.Rat).Mul(q[3], r.Center[1])), big.NewRat(2, 1)))
	if minimum.Sign() >= 0 {
		return r, ErrConvergence
	}
	for axis, diagonal := range []*big.Rat{q[4], q[2]} {
		r.ProjectionRadiusSquared[axis] = new(big.Rat).Quo(new(big.Rat).Mul(new(big.Rat).Mul(big.NewRat(-4, 1), minimum), diagonal), determinant)
		for _, value := range []*big.Rat{r.Center[axis], r.ProjectionRadiusSquared[axis]} {
			if e := budget.check(value); e != nil {
				return r, e
			}
		}
	}
	difference, e := orderingDifference(ctx, a, c, o.MaxCells, budget)
	if e != nil {
		return r, e
	}
	r.Domain = difference.Domain()
	bounds := [2][2]float64{{r.Domain.UMin, r.Domain.UMax}, {r.Domain.VMin, r.Domain.VMax}}
	for axis := 0; axis < 2; axis++ {
		for end := 0; end < 2; end++ {
			distance := new(big.Rat).Sub(r.Center[axis], new(big.Rat).SetFloat64(bounds[axis][end]))
			if end == 1 {
				distance.Neg(distance)
			}
			square := new(big.Rat).Mul(distance, distance)
			if e := budget.check(square); e != nil {
				return r, e
			}
			if distance.Sign() <= 0 || square.Cmp(r.ProjectionRadiusSquared[axis]) <= 0 {
				return r, ErrConvergence
			}
		}
	}
	cells, e := graphContactBezierCells(ctx, difference, o.MaxCells, budget)
	if e != nil {
		return r, e
	}
	sign := 0
	for _, cell := range cells {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		local := quadraticGraphCellCoefficients(q, cell.Domain())
		residual, e := divideGraphQuadraticBernstein(ctx, cell, local, budget)
		if e != nil {
			return r, e
		}
		if residual == nil {
			return r, ErrConvergence
		}
		for _, h := range residual.net {
			s := h[2].Sign()
			if s == 0 || (sign != 0 && s != sign) {
				return r, ErrConvergence
			}
			sign = s
		}
		r.CertifiedCells = append(r.CertifiedCells, cell.Domain())
	}
	if sign == 0 {
		return r, ErrConvergence
	}
	remaining := o.MaxOperations - budget.operations
	if remaining < 1 {
		return r, ErrConvergence
	}
	r.Graph, e = CertifySurfaceInjectivity(a, SurfaceInjectivityOptions{ProjectionAxes: [2]int{0, 1}, MaxCells: o.MaxCells, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
	budget.operations += r.Graph.ExactOperations
	if e != nil {
		return r, e
	}
	if !r.Graph.ProvenInjective {
		return r, ErrConvergence
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	r.InsideSide = -sign
	r.OutsideSide = sign
	r.Proven = true
	return r, nil
}

// quadraticGraphCellCoefficients composes original UV with normalized cell UV.
func quadraticGraphCellCoefficients(q [6]*big.Rat, d ParameterRectangle) (out [6]*big.Rat) {
	u, v := new(big.Rat).SetFloat64(d.UMin), new(big.Rat).SetFloat64(d.VMin)
	wu := new(big.Rat).Sub(new(big.Rat).SetFloat64(d.UMax), u)
	wv := new(big.Rat).Sub(new(big.Rat).SetFloat64(d.VMax), v)
	out[0] = new(big.Rat).Set(q[0])
	for _, term := range []*big.Rat{new(big.Rat).Mul(q[1], u), new(big.Rat).Mul(q[2], new(big.Rat).Mul(u, u)), new(big.Rat).Mul(q[3], v), new(big.Rat).Mul(q[4], new(big.Rat).Mul(v, v)), new(big.Rat).Mul(q[5], new(big.Rat).Mul(u, v))} {
		out[0].Add(out[0], term)
	}
	out[1] = new(big.Rat).Mul(wu, new(big.Rat).Add(q[1], new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(q[2], u)), new(big.Rat).Mul(q[5], v))))
	out[3] = new(big.Rat).Mul(wv, new(big.Rat).Add(q[3], new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(q[4], v)), new(big.Rat).Mul(q[5], u))))
	out[2] = new(big.Rat).Mul(q[2], new(big.Rat).Mul(wu, wu))
	out[4] = new(big.Rat).Mul(q[4], new(big.Rat).Mul(wv, wv))
	out[5] = new(big.Rat).Mul(q[5], new(big.Rat).Mul(wu, wv))
	return
}

// DiscoverSplineGraphQuadraticEllipse extracts a genuine quadratic from the
// first union cell, normalizes its overall sign and independently certifies the
// original supports. Rotated, elevated-degree and rational forms are supported.
func DiscoverSplineGraphQuadraticEllipse(ctx context.Context, a, c *SurfacePatch, o GraphPointContactOptions) (r SplineGraphQuadraticEllipse, err error) {
	if ctx == nil || a == nil || c == nil || len(a.net) == 0 || len(c.net) == 0 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxCells < 1 || o.MaxDepth < 0 {
		return r, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations += budget.operations }()
	difference, e := orderingDifference(ctx, a, c, o.MaxCells, budget)
	if e != nil {
		return r, e
	}
	cells, e := graphContactBezierCells(ctx, difference, o.MaxCells, budget)
	if e != nil {
		return r, e
	}
	if len(cells) == 0 {
		return r, ErrConvergence
	}
	local, proved, e := graphQuadraticCoefficients(cells[0], budget)
	if e != nil {
		return r, e
	}
	if !proved || local[2].Sign() == 0 {
		return r, ErrConvergence
	}
	if local[2].Sign() < 0 {
		for _, x := range local {
			x.Neg(x)
		}
	}
	d := cells[0].Domain()
	u, v := new(big.Rat).SetFloat64(d.UMin), new(big.Rat).SetFloat64(d.VMin)
	wu := new(big.Rat).Sub(new(big.Rat).SetFloat64(d.UMax), u)
	wv := new(big.Rat).Sub(new(big.Rat).SetFloat64(d.VMax), v)
	q := [6]*big.Rat{}
	q[2] = new(big.Rat).Quo(local[2], new(big.Rat).Mul(wu, wu))
	q[4] = new(big.Rat).Quo(local[4], new(big.Rat).Mul(wv, wv))
	q[5] = new(big.Rat).Quo(local[5], new(big.Rat).Mul(wu, wv))
	q[1] = new(big.Rat).Sub(new(big.Rat).Quo(local[1], wu), new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(q[2], u)), new(big.Rat).Mul(q[5], v)))
	q[3] = new(big.Rat).Sub(new(big.Rat).Quo(local[3], wv), new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(q[4], v)), new(big.Rat).Mul(q[5], u)))
	q[0] = new(big.Rat)
	composed := quadraticGraphCellCoefficients(q, d)
	q[0].Sub(local[0], composed[0])
	for _, x := range q {
		if e := budget.check(x); e != nil {
			return r, e
		}
	}
	o.MaxOperations -= budget.operations
	if o.MaxOperations < 1 {
		return r, ErrConvergence
	}
	return CertifySplineGraphQuadraticEllipse(ctx, a, c, GraphQuadraticEllipseHint{Coefficients: q}, o)
}
