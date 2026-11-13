package geom

import (
	"context"
	"math/big"
)

// CurveDerivativeBoundOptions bounds knot-aligned derivative enclosure work.
// Zero limits select 4096 cells, 4M operations and 16384 coefficient bits.
// MaxParameterWidth optionally requests further midpoint subdivision.
type CurveDerivativeBoundOptions struct {
	MaxParameterWidth                           float64
	MaxCells, MaxOperations, MaxCoefficientBits int
}

// CurveDerivativeCell bounds actual derivatives with respect to the original
// curve parameter, not unit tangents or normalized Bezier coordinates. Bounds
// include one-sided endpoints of this knot span. ChordDeviationUpper bounds
// |C(t)-linear endpoint interpolation(t)| by sup|C”|*(End-Start)^2/8.
// It is a certified pointwise geometric chord-error bound for this cell only.
type CurveDerivativeCell struct {
	Domain                                             ParameterSpan
	First, Second                                      [3]CertifiedScalarRange
	SpeedUpper, AccelerationUpper, ChordDeviationUpper float64
}

// CurveDerivativeBounds is exhaustive only when Complete; partial cells remain
// valid on cancellation/limits but do not cover unprocessed curve portions.
type CurveDerivativeBounds struct {
	Cells           []CurveDerivativeCell
	Complete        bool
	ExactOperations int
}

func curveDerivativeCell(p *SurfacePatch, b *polynomialBudget) (CurveDerivativeCell, error) {
	d := p.Domain()
	result := CurveDerivativeCell{Domain: ParameterSpan{Start: d.UMin, End: d.UMax}}
	width := new(big.Rat).Sub(new(big.Rat).SetFloat64(d.UMax), new(big.Rat).SetFloat64(d.UMin))
	var ranges [3][4]exactInterval
	for axis := 0; axis < 4; axis++ {
		coefficients := make([]*big.Rat, p.countU)
		origin := new(big.Rat)
		if axis < 3 {
			origin.Quo(p.net[0][axis], p.net[0][3])
		}
		for i := range coefficients {
			h := p.net[i*p.countV]
			coefficients[i] = new(big.Rat).Set(h[axis])
			if axis < 3 {
				coefficients[i].Sub(coefficients[i], new(big.Rat).Mul(origin, h[3]))
			}
			if e := b.check(coefficients[i]); e != nil {
				return result, e
			}
		}
		for order := 0; order < 3; order++ {
			lo, hi := coefficients[0], coefficients[0]
			for _, v := range coefficients {
				if v.Cmp(lo) < 0 {
					lo = v
				}
				if v.Cmp(hi) > 0 {
					hi = v
				}
			}
			ranges[order][axis] = exactInterval{lo, hi}
			next := make([]*big.Rat, max(1, len(coefficients)-1))
			if len(coefficients) == 1 {
				next[0] = new(big.Rat)
			} else {
				scale := new(big.Rat).Quo(big.NewRat(int64(len(coefficients)-1), 1), width)
				for i := range next {
					next[i] = new(big.Rat).Mul(new(big.Rat).Sub(coefficients[i+1], coefficients[i]), scale)
					if e := b.check(next[i]); e != nil {
						return result, e
					}
				}
			}
			coefficients = next
		}
	}
	normFirst, normSecond := new(big.Rat), new(big.Rat)
	for axis := 0; axis < 3; axis++ {
		position, e := intervalDivide(ranges[0][axis], ranges[0][3], b)
		if e != nil {
			return result, e
		}
		product, e := intervalMultiply(position, ranges[1][3], b)
		if e != nil {
			return result, e
		}
		numerator, e := intervalSubtract(ranges[1][axis], product, b)
		if e != nil {
			return result, e
		}
		first, e := intervalDivide(numerator, ranges[0][3], b)
		if e != nil {
			return result, e
		}
		product, e = intervalMultiply(position, ranges[2][3], b)
		if e != nil {
			return result, e
		}
		numerator, e = intervalSubtract(ranges[2][axis], product, b)
		if e != nil {
			return result, e
		}
		product, e = intervalMultiply(first, ranges[1][3], b)
		if e != nil {
			return result, e
		}
		twice, e := intervalMultiply(product, exactInterval{big.NewRat(2, 1), big.NewRat(2, 1)}, b)
		if e != nil {
			return result, e
		}
		numerator, e = intervalSubtract(numerator, twice, b)
		if e != nil {
			return result, e
		}
		second, e := intervalDivide(numerator, ranges[0][3], b)
		if e != nil {
			return result, e
		}
		result.First[axis], result.Second[axis] = certifiedRange(first), certifiedRange(second)
		for i, v := range []exactInterval{first, second} {
			maximum := new(big.Rat).Abs(v.lo)
			if z := new(big.Rat).Abs(v.hi); z.Cmp(maximum) > 0 {
				maximum = z
			}
			square := new(big.Rat).Mul(maximum, maximum)
			if i == 0 {
				normFirst.Add(normFirst, square)
			} else {
				normSecond.Add(normSecond, square)
			}
		}
	}
	factor := new(big.Rat).Quo(new(big.Rat).Mul(width, width), big.NewRat(8, 1))
	chordSquared := new(big.Rat).Mul(normSecond, new(big.Rat).Mul(factor, factor))
	for _, v := range []*big.Rat{normFirst, normSecond, chordSquared} {
		if e := b.check(v); e != nil {
			return result, e
		}
	}
	result.SpeedUpper = outwardSqrtRat(normFirst, false)
	result.AccelerationUpper = outwardSqrtRat(normSecond, false)
	result.ChordDeviationUpper = outwardSqrtRat(chordSquared, false)
	return result, nil
}

// BoundCurveDerivatives certifies first/second derivatives on continuous clamped
// positive-weight rational spline spans. Derivative jumps at knots are retained
// as separate one-sided cells; the chord theorem is never applied across them.
func BoundCurveDerivatives(c *BSpline, o CurveDerivativeBoundOptions) (CurveDerivativeBounds, error) {
	return BoundCurveDerivativesContext(context.Background(), c, o)
}

// BoundCurveDerivativesContext checks cancellation between bounded cell steps.
func BoundCurveDerivativesContext(ctx context.Context, c *BSpline, o CurveDerivativeBoundOptions) (CurveDerivativeBounds, error) {
	if ctx == nil {
		return CurveDerivativeBounds{}, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return CurveDerivativeBounds{}, e
	}
	operations, bits := o.MaxOperations, o.MaxCoefficientBits
	if operations == 0 {
		operations = 4000000
	}
	if bits == 0 {
		bits = 16384
	}
	p, e := curveDistancePatch(c, operations, bits)
	if e != nil {
		return CurveDerivativeBounds{}, e
	}
	if operations <= p.operations {
		return CurveDerivativeBounds{ExactOperations: p.operations}, ErrConvergence
	}
	o.MaxOperations = operations - p.operations
	r, e := boundCurvePatchDerivatives(ctx, p, o)
	r.ExactOperations += p.operations
	return r, e
}

// BoundDerivatives encloses the exact extracted iso curve without rounded poles.
func (c *SurfaceIsoCurve) BoundDerivatives(o CurveDerivativeBoundOptions) (CurveDerivativeBounds, error) {
	return c.BoundDerivativesContext(context.Background(), o)
}

// BoundDerivativesContext supports cooperative cancellation for exact iso curves.
func (c *SurfaceIsoCurve) BoundDerivativesContext(ctx context.Context, o CurveDerivativeBoundOptions) (CurveDerivativeBounds, error) {
	if ctx == nil || c == nil || c.patch == nil {
		return CurveDerivativeBounds{}, ErrInvalid
	}
	return boundCurvePatchDerivatives(ctx, c.patch, o)
}
func boundCurvePatchDerivatives(ctx context.Context, p *SurfacePatch, o CurveDerivativeBoundOptions) (r CurveDerivativeBounds, err error) {
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 4000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if !finite(o.MaxParameterWidth) || o.MaxParameterWidth < 0 || o.MaxCells < 1 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	stack := []*SurfacePatch{p}
	for len(stack) > 0 {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		d := current.Domain()
		split, knot := internalPatchKnot(current.knotsU, current.degreeU, current.countU)
		if knot || (o.MaxParameterWidth > 0 && new(big.Rat).Sub(new(big.Rat).SetFloat64(d.UMax), new(big.Rat).SetFloat64(d.UMin)).Cmp(new(big.Rat).SetFloat64(o.MaxParameterWidth)) > 0) {
			if len(stack)+len(r.Cells)+2 > o.MaxCells {
				return r, ErrConvergence
			}
			if !knot {
				split = d.UMin + (d.UMax-d.UMin)/2
			}
			if split <= d.UMin || split >= d.UMax {
				return r, ErrConvergence
			}
			a, z, e := current.split(split, true, b)
			if e != nil {
				return r, e
			}
			stack = append(stack, z, a)
			continue
		}
		cell, e := curveDerivativeCell(current, b)
		if e != nil {
			return r, e
		}
		cell.Domain.Index = len(r.Cells)
		r.Cells = append(r.Cells, cell)
	}
	r.Complete = true
	return r, nil
}
