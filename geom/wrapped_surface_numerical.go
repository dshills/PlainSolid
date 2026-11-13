package geom

import (
	"math/big"
	"sync"
)

type wrappedSurfaceNumericalCache struct {
	once    sync.Once
	support *BSplineSurface
	err     error
}

func (w *WrappedSurfacePatch) numericalSupport() *BSplineSurface {
	if w == nil || w.patch == nil || w.numerical == nil {
		return nil
	}
	w.numerical.once.Do(func() { w.numerical.support, w.numerical.err = w.patch.ToBSplineSurface() })
	return w.numerical.support
}

// fastCanonicalUV computes the exact modulo of represented inputs before any
// float spline evaluation. Nonrepresentable reduced coordinates trigger the
// exact fallback, even if their rounded representatives would be in the domain.
func (w *WrappedSurfacePatch) fastCanonicalUV(u, v float64) (Vec2, bool, error) {
	if w == nil || w.patch == nil {
		return Vec2{}, false, ErrInvalid
	}
	if !finite(u) || !finite(v) {
		return Vec2{}, false, ErrDomain
	}
	d := w.patch.Domain()
	starts := [2]float64{d.UMin, d.VMin}
	ends := [2]float64{d.UMax, d.VMax}
	input := [2]float64{u, v}
	var result [2]float64
	for axis, x := range input {
		if w.period[axis] == nil {
			if x < starts[axis] || x > ends[axis] {
				return Vec2{}, false, ErrDomain
			}
			result[axis] = x
			continue
		}
		if x >= starts[axis] && x < ends[axis] {
			result[axis] = x
			continue
		}
		value := new(big.Rat).SetFloat64(x)
		delta := new(big.Rat).Sub(value, new(big.Rat).SetFloat64(starts[axis]))
		ratio := new(big.Rat).Quo(delta, w.period[axis])
		quotient, remainder := new(big.Int), new(big.Int)
		quotient.QuoRem(ratio.Num(), ratio.Denom(), remainder)
		if remainder.Sign() < 0 {
			quotient.Sub(quotient, big.NewInt(1))
		}
		reduced := new(big.Rat).Sub(value, new(big.Rat).Mul(new(big.Rat).SetInt(quotient), w.period[axis]))
		f, e := exactRepresentableFloat(reduced)
		if e != nil {
			return Vec2{}, false, nil
		}
		result[axis] = f
	}
	return Vec2{result[0], result[1]}, true, nil
}

// EvalApprox explicitly permits ordinary floating spline evaluation, which is
// not guaranteed correctly rounded. A cached lossless fundamental export is
// used only when the exact modulo-reduced UV also fits float64 losslessly.
// Otherwise evaluation falls back to Eval's nearest exact rational point.
func (w *WrappedSurfacePatch) EvalApprox(u, v float64) (Vec3, error) {
	uv, exact, e := w.fastCanonicalUV(u, v)
	if e != nil {
		return Vec3{}, e
	}
	if exact {
		if support := w.numericalSupport(); support != nil {
			return support.Eval(uv.X, uv.Y)
		}
	}
	return w.Eval(u, v)
}

// PartialsApprox is the corresponding explicitly approximate first-partial
// evaluator. It preserves original parameter units and exact phase reduction.
func (w *WrappedSurfacePatch) PartialsApprox(u, v float64) (Vec3, Vec3, error) {
	uv, exact, e := w.fastCanonicalUV(u, v)
	if e != nil {
		return Vec3{}, Vec3{}, e
	}
	if exact {
		if support := w.numericalSupport(); support != nil {
			return support.Partials(uv.X, uv.Y)
		}
	}
	return w.Partials(u, v)
}

type wrappedNumericalSurface struct{ surface *WrappedSurfacePatch }

func (s wrappedNumericalSurface) Eval(u, v float64) (Vec3, error) { return s.surface.EvalApprox(u, v) }
func (s wrappedNumericalSurface) Partials(u, v float64) (Vec3, Vec3, error) {
	return s.surface.PartialsApprox(u, v)
}

// NumericalSurface provides an explicit Surface adapter for approximate mass
// quadrature/tessellation. It retains exact periodic phase and seam validation;
// numerical samples are not substitutes for certified rational evaluations.
func (w *WrappedSurfacePatch) NumericalSurface() Surface {
	if w == nil || w.patch == nil {
		return nil
	}
	return wrappedNumericalSurface{w}
}
