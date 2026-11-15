package geom

import "math"

// PeriodicSplineAxis defines the physical parameter interval of one uniformly
// periodic tensor axis. The number of spans is the original count of poles in
// that direction. A nil axis uses the supplied ordinary B-spline knot vector.
type PeriodicSplineAxis struct{ Start, Period float64 }

// PeriodicBSplineSurface is an immutable rational tensor surface with one or
// two uniform cyclic axes. It retains the original pole net without requiring
// duplicated seam poles from the caller. A cyclic degree p axis is C^(p−1)
// across its seam, subject to arithmetic roundoff in the float64 evaluator.
type PeriodicBSplineSurface struct {
	surface *BSplineSurface
	count   [2]int
	axis    [2]PeriodicSplineAxis
	end     [2]float64
}

// NewPeriodicBSplineSurface interprets data.Poles and data.Weights in the
// original u-major order. Each periodic direction takes a nil or empty knot
// vector and supplies a cyclic definition through axisU or axisV. At least
// one direction must be periodic. Weights are positive and their represented
// values are preserved. Nil weights select a polynomial surface.
func NewPeriodicBSplineSurface(data BSplineSurfaceData, axisU, axisV *PeriodicSplineAxis) (*PeriodicBSplineSurface, error) {
	if axisU == nil && axisV == nil || data.CountU < 1 || data.CountV < 1 || data.CountU > len(data.Poles)/data.CountV || data.CountU*data.CountV != len(data.Poles) || data.Weights != nil && len(data.Weights) != len(data.Poles) {
		return nil, ErrInvalid
	}
	result := &PeriodicBSplineSurface{count: [2]int{data.CountU, data.CountV}}
	degrees := [2]int{data.DegreeU, data.DegreeV}
	counts := result.count
	knots := [2][]float64{data.KnotsU, data.KnotsV}
	for direction, axis := range [2]*PeriodicSplineAxis{axisU, axisV} {
		if axis == nil {
			if err := validateKnots(degrees[direction], counts[direction], knots[direction]); err != nil {
				return nil, err
			}
			continue
		}
		if len(knots[direction]) != 0 || counts[direction] > math.MaxInt-2*64-1 {
			return nil, ErrInvalid
		}
		end, err := uniformPeriodicEnd(degrees[direction], counts[direction], axis.Start, axis.Period)
		if err != nil {
			return nil, err
		}
		result.axis[direction], result.end[direction] = *axis, end
		knots[direction] = uniformPeriodicKnots(degrees[direction], counts[direction])
		counts[direction] += degrees[direction]
	}
	if counts[0] > math.MaxInt/counts[1] {
		return nil, ErrInvalid
	}
	poles := make([]Vec3, counts[0]*counts[1])
	weights := make([]float64, len(poles))
	for u := 0; u < counts[0]; u++ {
		for v := 0; v < counts[1]; v++ {
			original := (u%data.CountU)*data.CountV + v%data.CountV
			expanded := u*counts[1] + v
			poles[expanded], weights[expanded] = data.Poles[original], 1
			if data.Weights != nil {
				weights[expanded] = data.Weights[original]
			}
		}
	}
	surface, err := NewBSplineSurfaceWithExactWeights(data.DegreeU, data.DegreeV, counts[0], counts[1], poles, knots[0], knots[1], weights)
	if err != nil {
		return nil, err
	}
	result.surface = surface
	return result, nil
}

// Domain gives the fundamental physical interval of each cyclic axis and the
// ordinary bounded interval of a noncyclic axis. A nil or zero surface returns
// NaNs. Finite parameters outside a periodic interval wrap by that period.
func (s *PeriodicBSplineSurface) Domain() (u0, u1, v0, v1 float64) {
	if s == nil || s.surface == nil {
		return math.NaN(), math.NaN(), math.NaN(), math.NaN()
	}
	u0, u1, v0, v1 = s.surface.Domain()
	if s.axis[0].Period != 0 {
		u0, u1 = s.axis[0].Start, s.end[0]
	}
	if s.axis[1].Period != 0 {
		v0, v1 = s.axis[1].Start, s.end[1]
	}
	return u0, u1, v0, v1
}

// Data returns independent copies of the original net and both cyclic axis
// definitions. A noncyclic axis returns nil and retains its knot vector.
func (s *PeriodicBSplineSurface) Data() (BSplineSurfaceData, *PeriodicSplineAxis, *PeriodicSplineAxis) {
	if s == nil || s.surface == nil {
		return BSplineSurfaceData{}, nil, nil
	}
	data := BSplineSurfaceData{DegreeU: s.surface.degreeU, DegreeV: s.surface.degreeV, CountU: s.count[0], CountV: s.count[1]}
	data.Poles = make([]Vec3, 0, data.CountU*data.CountV)
	data.Weights = make([]float64, 0, cap(data.Poles))
	for u := 0; u < data.CountU; u++ {
		offset := u * s.surface.countV
		data.Poles = append(data.Poles, s.surface.poles[offset:offset+data.CountV]...)
		data.Weights = append(data.Weights, s.surface.weights[offset:offset+data.CountV]...)
	}
	var axes [2]*PeriodicSplineAxis
	for i, axis := range s.axis {
		if axis.Period != 0 {
			axes[i] = &PeriodicSplineAxis{Start: axis.Start, Period: axis.Period}
		}
	}
	if axes[0] == nil {
		data.KnotsU = append([]float64(nil), s.surface.knotsU...)
	}
	if axes[1] == nil {
		data.KnotsV = append([]float64(nil), s.surface.knotsV...)
	}
	return data, axes[0], axes[1]
}

func (s *PeriodicBSplineSurface) wrapped(u, v float64) (float64, float64, error) {
	if s == nil || s.surface == nil {
		return 0, 0, ErrDomain
	}
	parameters := [2]float64{u, v}
	for direction, axis := range s.axis {
		if axis.Period != 0 {
			parameter, err := wrapUniformSplineParameter(parameters[direction], axis.Start, s.end[direction], axis.Period, s.count[direction])
			if err != nil {
				return 0, 0, err
			}
			parameters[direction] = parameter
		}
	}
	return parameters[0], parameters[1], nil
}

func (s *PeriodicBSplineSurface) Eval(u, v float64) (Vec3, error) {
	a, b, err := s.wrapped(u, v)
	if err != nil {
		return Vec3{}, err
	}
	return s.surface.Eval(a, b)
}

// Partials evaluates the first derivatives in physical parameter units. A
// degree-one cyclic direction uses the right derivative at its seam.
func (s *PeriodicBSplineSurface) Partials(u, v float64) (Vec3, Vec3, error) {
	a, b, err := s.wrapped(u, v)
	if err != nil {
		return Vec3{}, Vec3{}, err
	}
	du, dv, err := s.surface.Partials(a, b)
	if err != nil {
		return Vec3{}, Vec3{}, err
	}
	derivatives := [2]Vec3{du, dv}
	for direction, axis := range s.axis {
		if axis.Period != 0 {
			derivative, err := scaleUniformSplineDerivative(derivatives[direction], s.count[direction], axis.Period)
			if err != nil {
				return Vec3{}, Vec3{}, err
			}
			derivatives[direction] = derivative
		}
	}
	return derivatives[0], derivatives[1], nil
}

func (s *PeriodicBSplineSurface) Normal(u, v float64) (Vec3, error) {
	du, dv, err := s.Partials(u, v)
	if err != nil {
		return Vec3{}, err
	}
	return du.Cross(dv).Unit()
}
