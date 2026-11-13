package geom

import (
	"context"
	"math/big"
)

// WrappedSurfaceOptions requires exact seam identity. ContinuityOrder 0 proves
// position closure; order 1 additionally proves first partials match. Interior
// knot smoothness is unchanged. Zero limits select 4096 poles, 4M operations and 16384 bits.
type WrappedSurfaceOptions struct {
	WrapU, WrapV                                                 bool
	ContinuityOrder, MaxPoles, MaxOperations, MaxCoefficientBits int
}

// SurfaceSeamCertificate marks proved conditions per axis (0=U,1=V).
// Unwrapped axes remain false; limits never produce a positive incomplete proof.
type SurfaceSeamCertificate struct {
	PositionMatched, FirstPartialsMatched [2]bool
	ExactOperations                       int
}

// WrappedSurfacePatch periodically evaluates an immutable exact clamped patch.
// Seams select the lower/right parameter side. It does not alter poles, knots,
// weights or the semantics of the original BSplineSurface/SurfacePatch.
type WrappedSurfacePatch struct {
	numerical *wrappedSurfaceNumericalCache
	patch     *SurfacePatch
	options   WrappedSurfaceOptions
	period    [2]*big.Rat
	seam      SurfaceSeamCertificate
}

func NewWrappedSurfacePatch(ctx context.Context, p *SurfacePatch, o WrappedSurfaceOptions) (w *WrappedSurfacePatch, r SurfaceSeamCertificate, err error) {
	if ctx == nil || p == nil || len(p.net) == 0 || (!o.WrapU && !o.WrapV) || o.ContinuityOrder < 0 || o.ContinuityOrder > 1 {
		return nil, r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, r, e
	}
	if o.MaxPoles == 0 {
		o.MaxPoles = 4096
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 4000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxPoles < 4 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return nil, r, ErrInvalid
	}
	if len(p.net) > o.MaxPoles {
		return nil, r, ErrConvergence
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() {
		r.ExactOperations = budget.operations
		if w != nil {
			w.seam = r
		}
	}()
	d := p.Domain()
	domains := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	for axis, wrap := range []bool{o.WrapU, o.WrapV} {
		if !wrap {
			continue
		}
		curves := [2]*SurfaceIsoCurve{}
		for side := 0; side < 2; side++ {
			left := o.MaxOperations - budget.operations
			if left < 1 {
				return nil, r, ErrConvergence
			}
			curve, e := p.IsoCurve(axis, domains[axis][side], SurfacePatchOptions{MaxPoles: o.MaxPoles, MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits})
			if e != nil {
				return nil, r, e
			}
			budget.operations += curve.operations
			curves[side] = curve
		}
		left := o.MaxOperations - budget.operations
		if left < 1 {
			return nil, r, ErrConvergence
		}
		identity, e := CompareExactCurvesContext(ctx, curves[0], curves[1], ExactCurveComparisonOptions{MaxSpans: o.MaxPoles, MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits})
		budget.operations += identity.ExactOperations
		if e != nil {
			return nil, r, e
		}
		if !identity.Complete || !identity.Equal {
			return nil, r, ErrDegenerate
		}
		r.PositionMatched[axis] = true
		if o.ContinuityOrder == 1 {
			matched, e := exactSurfaceSeamDerivative(ctx, p, axis, budget)
			if e != nil {
				return nil, r, e
			}
			if !matched {
				return nil, r, ErrDegenerate
			}
			r.FirstPartialsMatched[axis] = true
		}
	}
	w = &WrappedSurfacePatch{patch: p, options: o, numerical: &wrappedSurfaceNumericalCache{}}
	for axis, wrap := range []bool{o.WrapU, o.WrapV} {
		if wrap {
			w.period[axis] = new(big.Rat).Sub(new(big.Rat).SetFloat64(domains[axis][1]), new(big.Rat).SetFloat64(domains[axis][0]))
		}
	}
	return w, r, nil
}

// FundamentalPatch returns the immutable original clamped support.
func (w *WrappedSurfacePatch) FundamentalPatch() *SurfacePatch {
	if w == nil {
		return nil
	}
	return w.patch
}
func (w *WrappedSurfacePatch) SeamCertificate() SurfaceSeamCertificate {
	if w == nil {
		return SurfaceSeamCertificate{}
	}
	return w.seam
}
func (w *WrappedSurfacePatch) Periods() (r [2]*big.Rat) {
	if w == nil {
		return
	}
	for i, p := range w.period {
		if p != nil {
			r[i] = new(big.Rat).Set(p)
		}
	}
	return
}

// EvaluateRational reduces exact rational UV modulo wrapped domain lengths,
// preserving phase for negative and extremely large inputs. Unwrapped axes
// retain the original closed-domain validation. Returned jets use original units.
func (w *WrappedSurfacePatch) EvaluateRational(ctx context.Context, u, v *big.Rat, o SurfacePatchEvaluationOptions) (r RationalSurfacePatchJet, err error) {
	if ctx == nil || w == nil || w.patch == nil || u == nil || v == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations += budget.operations }()
	d := w.patch.Domain()
	start := [2]float64{d.UMin, d.VMin}
	input := [2]*big.Rat{u, v}
	var reduced [2]*big.Rat
	for axis, x := range input {
		if e := budget.check(x); e != nil {
			return r, e
		}
		reduced[axis] = new(big.Rat).Set(x)
		if w.period[axis] == nil {
			continue
		}
		origin := new(big.Rat).SetFloat64(start[axis])
		delta := new(big.Rat).Sub(x, origin)
		ratio := new(big.Rat).Quo(delta, w.period[axis])
		if e := budget.check(ratio); e != nil {
			return r, e
		}
		quotient, remainder := new(big.Int), new(big.Int)
		quotient.QuoRem(ratio.Num(), ratio.Denom(), remainder)
		if remainder.Sign() < 0 {
			quotient.Sub(quotient, big.NewInt(1))
		}
		reduced[axis].Sub(x, new(big.Rat).Mul(new(big.Rat).SetInt(quotient), w.period[axis]))
		if e := budget.check(reduced[axis]); e != nil {
			return r, e
		}
	}
	remaining := o.MaxOperations - budget.operations
	if remaining < 1 {
		return r, ErrConvergence
	}
	return w.patch.EvaluateRational(ctx, reduced[0], reduced[1], SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
}
func (w *WrappedSurfacePatch) Eval(u, v float64) (Vec3, error) {
	if !finite(u) || !finite(v) {
		return Vec3{}, ErrDomain
	}
	r, e := w.EvaluateRational(context.Background(), new(big.Rat).SetFloat64(u), new(big.Rat).SetFloat64(v), SurfacePatchEvaluationOptions{})
	if e != nil {
		return Vec3{}, e
	}
	return nearestRationalVector(r.Point)
}
func (w *WrappedSurfacePatch) Partials(u, v float64) (Vec3, Vec3, error) {
	if !finite(u) || !finite(v) {
		return Vec3{}, Vec3{}, ErrDomain
	}
	r, e := w.EvaluateRational(context.Background(), new(big.Rat).SetFloat64(u), new(big.Rat).SetFloat64(v), SurfacePatchEvaluationOptions{})
	if e != nil {
		return Vec3{}, Vec3{}, e
	}
	a, e := nearestRationalVector(r.Du)
	if e != nil {
		return Vec3{}, Vec3{}, e
	}
	b, e := nearestRationalVector(r.Dv)
	return a, b, e
}
func nearestRationalVector(p [3]*big.Rat) (Vec3, error) {
	x, _ := p[0].Float64()
	y, _ := p[1].Float64()
	z, _ := p[2].Float64()
	v := Vec3{x, y, z}
	if !v.IsFinite() {
		return Vec3{}, ErrConvergence
	}
	return v, nil
}

func exactSurfaceSeamDerivative(ctx context.Context, p *SurfacePatch, axis int, b *polynomialBudget) (bool, error) {
	stack := []*SurfacePatch{p}
	for len(stack) > 0 {
		if e := ctx.Err(); e != nil {
			return false, e
		}
		q := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		knots, degree, count := q.knotsV, q.degreeV, q.countV
		if axis == 1 {
			knots, degree, count = q.knotsU, q.degreeU, q.countU
		}
		if knot, ok := internalPatchKnot(knots, degree, count); ok {
			left, right, e := q.split(knot, axis == 1, b)
			if e != nil {
				return false, e
			}
			stack = append(stack, right, left)
			continue
		}
		normalKnots, normalDegree, normalCount := q.knotsU, q.degreeU, q.countU
		if axis == 1 {
			normalKnots, normalDegree, normalCount = q.knotsV, q.degreeV, q.countV
		}
		var h, d [2][4]rationalPolynomial
		for side := 0; side < 2; side++ {
			row, next := 0, 1
			lo, hi := normalKnots[normalDegree], normalKnots[normalDegree+1]
			if side == 1 {
				row, next = normalCount-1, normalCount-2
				lo, hi = normalKnots[normalCount-1], normalKnots[normalCount]
			}
			scale := new(big.Rat).Quo(big.NewRat(int64(normalDegree), 1), new(big.Rat).Sub(new(big.Rat).SetFloat64(hi), new(big.Rat).SetFloat64(lo)))
			index := func(row, i int) int {
				if axis == 0 {
					return row*q.countV + i
				}
				return i*q.countV + row
			}
			for field := 0; field < 4; field++ {
				values, derivative := make([]*big.Rat, count), make([]*big.Rat, count)
				for i := 0; i < count; i++ {
					values[i] = q.net[index(row, i)][field]
					delta := new(big.Rat).Sub(q.net[index(next, i)][field], values[i])
					if side == 1 {
						delta.Neg(delta)
					}
					derivative[i] = new(big.Rat).Mul(scale, delta)
					if e := b.check(derivative[i]); e != nil {
						return false, e
					}
				}
				var e error
				h[side][field], e = bernsteinToPower(values, b)
				if e != nil {
					return false, e
				}
				d[side][field], e = bernsteinToPower(derivative, b)
				if e != nil {
					return false, e
				}
			}
		}
		subtract := func(a, c rationalPolynomial) (rationalPolynomial, error) {
			out := make(rationalPolynomial, max(len(a), len(c)))
			for i := range out {
				out[i] = new(big.Rat)
				if i < len(a) {
					out[i].Set(a[i])
				}
				if i < len(c) {
					out[i].Sub(out[i], c[i])
				}
				if e := b.check(out[i]); e != nil {
					return nil, e
				}
			}
			return trimPolynomial(out), nil
		}
		for field := 0; field < 3; field++ {
			var numerator [2]rationalPolynomial
			for side := 0; side < 2; side++ {
				x, e := polynomialProduct(d[side][field], h[side][3], b)
				if e != nil {
					return false, e
				}
				y, e := polynomialProduct(h[side][field], d[side][3], b)
				if e != nil {
					return false, e
				}
				numerator[side], e = subtract(x, y)
				if e != nil {
					return false, e
				}
				weightSquared, e := polynomialProduct(h[1-side][3], h[1-side][3], b)
				if e != nil {
					return false, e
				}
				numerator[side], e = polynomialProduct(numerator[side], weightSquared, b)
				if e != nil {
					return false, e
				}
			}
			difference, e := subtract(numerator[0], numerator[1])
			if e != nil {
				return false, e
			}
			if len(difference) != 0 {
				return false, nil
			}
		}
	}
	return true, nil
}

// InsertKnot refines one fundamental-domain axis exactly and revalidates the
// requested seam order. Arithmetic limits apply cumulatively to refinement and
// seam verification. Failure leaves the original immutable wrapper unchanged.
// The supplied knot is not wrapped; it must be an original interior parameter.
func (w *WrappedSurfacePatch) InsertKnot(ctx context.Context, axis int, parameter float64, count int, o SurfacePatchOptions) (result *WrappedSurfacePatch, seam SurfaceSeamCertificate, err error) {
	if ctx == nil || w == nil || w.patch == nil || axis < 0 || axis > 1 {
		return nil, seam, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, seam, e
	}
	options, e := patchOptions(o)
	if e != nil {
		return nil, seam, e
	}
	var refined *SurfacePatch
	if axis == 0 {
		refined, e = w.patch.InsertUKnot(parameter, count, options)
	} else {
		refined, e = w.patch.InsertVKnot(parameter, count, options)
	}
	if e != nil {
		return nil, seam, e
	}
	remaining := options.MaxOperations - refined.operations
	if remaining < 1 {
		return nil, seam, ErrConvergence
	}
	wrapped := w.options
	wrapped.MaxPoles = options.MaxPoles
	wrapped.MaxOperations = remaining
	wrapped.MaxCoefficientBits = options.MaxCoefficientBits
	result, seam, err = NewWrappedSurfacePatch(ctx, refined, wrapped)
	seam.ExactOperations += refined.operations
	if result != nil {
		result.seam = seam
	}
	return
}

// Domain returns the fundamental rectangle; wrapped axes evaluate periodically
// beyond it. A nil wrapper returns an invalid zero rectangle.
func (w *WrappedSurfacePatch) Domain() ParameterRectangle {
	if w == nil || w.patch == nil {
		return ParameterRectangle{}
	}
	return w.patch.Domain()
}
