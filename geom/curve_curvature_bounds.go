package geom

import (
	"context"
	"math"
	"math/big"
)

// CurveCurvatureCell encloses geometric curvature |C' cross C”|/|C'|^3.
// It is invariant under regular parameter scaling; Speed bounds retain the
// original parameter units. ProvenRegular means SpeedLower is strictly positive
// in exact arithmetic (it may round down to zero for extreme underflow).
// IdenticallyStationary certifies C'=0 throughout the cell, where curvature is
// undefined. Otherwise !ProvenRegular means unresolved, not proven singular.
// Unresolved/undefined curvature returns [0,+Inf]. Knot derivatives are one-sided.
type CurveCurvatureCell struct {
	Domain                               ParameterSpan
	SpeedLower, SpeedUpper               float64
	CurvatureLower, CurvatureUpper       float64
	ProvenRegular, IdenticallyStationary bool
}

// CurveCurvatureBounds retains valid cells on resource errors. Complete denotes
// exhaustive parameter coverage, not proven regularity. AllRegular requires
// both exhaustive coverage and every cell proven regular. Tightening
// MaxParameterWidth may resolve derivative-hull dependence overestimation.
type CurveCurvatureBounds struct {
	Cells                []CurveCurvatureCell
	Complete, AllRegular bool
	ExactOperations      int
}

// BoundCurveCurvature combines outward derivative intervals with exact rational
// norm/cross-product bounds. No point sampling or floating zero threshold is used.
func BoundCurveCurvature(c *BSpline, o CurveDerivativeBoundOptions) (CurveCurvatureBounds, error) {
	return BoundCurveCurvatureContext(context.Background(), c, o)
}

// BoundCurveCurvatureContext supports cancellation with valid partial bounds.
func BoundCurveCurvatureContext(ctx context.Context, c *BSpline, o CurveDerivativeBoundOptions) (CurveCurvatureBounds, error) {
	derivatives, e := BoundCurveDerivativesContext(ctx, c, o)
	return curvatureFromDerivatives(ctx, derivatives, o, e)
}

// BoundCurvature encloses the exact extracted iso curve's curvature magnitude.
func (c *SurfaceIsoCurve) BoundCurvature(o CurveDerivativeBoundOptions) (CurveCurvatureBounds, error) {
	return c.BoundCurvatureContext(context.Background(), o)
}

// BoundCurvatureContext supports cooperative cancellation.
func (c *SurfaceIsoCurve) BoundCurvatureContext(ctx context.Context, o CurveDerivativeBoundOptions) (CurveCurvatureBounds, error) {
	derivatives, e := c.BoundDerivativesContext(ctx, o)
	return curvatureFromDerivatives(ctx, derivatives, o, e)
}

func intervalNormSquared(v [3]exactInterval, b *polynomialBudget) (*big.Rat, *big.Rat, error) {
	lower, upper := new(big.Rat), new(big.Rat)
	for _, x := range v {
		lo, hi := new(big.Rat).Abs(x.lo), new(big.Rat).Abs(x.hi)
		if lo.Cmp(hi) > 0 {
			lo, hi = hi, lo
		}
		if x.lo.Sign() <= 0 && x.hi.Sign() >= 0 {
			lo = new(big.Rat)
		}
		lower.Add(lower, new(big.Rat).Mul(lo, lo))
		upper.Add(upper, new(big.Rat).Mul(hi, hi))
		if e := b.check(lower); e != nil {
			return nil, nil, e
		}
		if e := b.check(upper); e != nil {
			return nil, nil, e
		}
	}
	return lower, upper, nil
}
func curvatureFromDerivatives(ctx context.Context, d CurveDerivativeBounds, o CurveDerivativeBoundOptions, prior error) (r CurveCurvatureBounds, err error) {
	r.ExactOperations = d.ExactOperations
	if ctx == nil {
		return r, ErrInvalid
	}
	operations, bits := o.MaxOperations, o.MaxCoefficientBits
	if operations == 0 {
		operations = 4000000
	}
	if bits == 0 {
		bits = 16384
	}
	b := &polynomialBudget{maxOperations: operations - d.ExactOperations, maxBits: bits}
	defer func() { r.ExactOperations += b.operations }()
	allRegular := true
	for _, cell := range d.Cells {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		result := CurveCurvatureCell{Domain: cell.Domain, CurvatureUpper: math.Inf(1), SpeedUpper: cell.SpeedUpper}
		var first, second [3]exactInterval
		valid := true
		for axis := 0; axis < 3; axis++ {
			a, z := cell.First[axis], cell.Second[axis]
			if !finite(a.Min) || !finite(a.Max) || !finite(z.Min) || !finite(z.Max) {
				valid = false
				break
			}
			first[axis] = exactInterval{new(big.Rat).SetFloat64(a.Min), new(big.Rat).SetFloat64(a.Max)}
			second[axis] = exactInterval{new(big.Rat).SetFloat64(z.Min), new(big.Rat).SetFloat64(z.Max)}
		}
		if !valid {
			allRegular = false
			r.Cells = append(r.Cells, result)
			continue
		}
		speedMin, speedMax, e := intervalNormSquared(first, b)
		if e != nil {
			return r, e
		}
		result.SpeedLower, result.SpeedUpper = outwardSqrtRat(speedMin, true), outwardSqrtRat(speedMax, false)
		result.ProvenRegular = speedMin.Sign() > 0
		result.IdenticallyStationary = speedMax.Sign() == 0
		if result.ProvenRegular {
			cross, e := intervalCross(first, second, b)
			if e != nil {
				return r, e
			}
			crossMin, crossMax, e := intervalNormSquared(cross, b)
			if e != nil {
				return r, e
			}
			cube := func(x *big.Rat) *big.Rat { return new(big.Rat).Mul(x, new(big.Rat).Mul(x, x)) }
			low := new(big.Rat).Quo(crossMin, cube(speedMax))
			high := new(big.Rat).Quo(crossMax, cube(speedMin))
			if e := b.check(low); e != nil {
				return r, e
			}
			if e := b.check(high); e != nil {
				return r, e
			}
			result.CurvatureLower, result.CurvatureUpper = outwardSqrtRat(low, true), outwardSqrtRat(high, false)
		} else {
			allRegular = false
		}
		r.Cells = append(r.Cells, result)
	}
	r.Complete = d.Complete && prior == nil
	r.AllRegular = r.Complete && allRegular
	return r, prior
}
