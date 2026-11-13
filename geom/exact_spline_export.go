package geom

import (
	"errors"
	"math/big"
)

// ErrUnrepresentable means a lossless float64 spline representation in the
// requested convention does not exist. It never authorizes rounded export.
var ErrUnrepresentable = errors.New("geom: exact spline coefficient is not representable as float64")

func exactRepresentableFloat(x *big.Rat) (float64, error) {
	if x == nil {
		return 0, ErrInvalid
	}
	f, _ := x.Float64()
	// Rat.Float64's reported exact flag is not sufficient on all Go versions:
	// underflow may report (0,true). Always verify the represented rational.
	if !finite(f) || new(big.Rat).SetFloat64(f).Cmp(x) != 0 {
		return 0, ErrUnrepresentable
	}
	return f, nil
}

// RepresentedExactScalar converts without rounding, including at subnormal
// limits. A finite float64 roundtrip must equal the supplied rational exactly.
func RepresentedExactScalar(x *big.Rat) (float64, error) { return exactRepresentableFloat(x) }

func exportExactNet(p *SurfacePatch) ([]Vec3, []float64, error) {
	if p == nil || len(p.net) == 0 {
		return nil, nil, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: p.options.MaxOperations, maxBits: p.options.MaxCoefficientBits}
	maximum := p.net[0][3]
	for _, h := range p.net {
		if h[3].Cmp(maximum) > 0 {
			maximum = h[3]
		}
	}
	poles := make([]Vec3, len(p.net))
	weights := make([]float64, len(p.net))
	for i, h := range p.net {
		var coordinates [3]float64
		for axis := 0; axis < 3; axis++ {
			value := new(big.Rat).Quo(h[axis], h[3])
			if e := budget.check(value); e != nil {
				return nil, nil, e
			}
			f, e := exactRepresentableFloat(value)
			if e != nil {
				return nil, nil, e
			}
			coordinates[axis] = f
		}
		poles[i] = Vec3{coordinates[0], coordinates[1], coordinates[2]}
		weight := new(big.Rat).Quo(h[3], maximum)
		if e := budget.check(weight); e != nil {
			return nil, nil, e
		}
		f, e := exactRepresentableFloat(weight)
		if e != nil {
			return nil, nil, e
		}
		if f <= 0 {
			return nil, nil, ErrUnrepresentable
		}
		weights[i] = f
	}
	return poles, weights, nil
}

// ToBSpline exports losslessly when all exact Euclidean poles and weights
// normalized by their maximum are representable float64 values. The maximum
// normalization matches BSpline's ordinary coefficient convention. Other weight
// scalings are not searched; failure may occur even if an unnormalized encoding
// could be exact. Original knots and domain are retained. No tolerance is used.
func (c *SurfaceIsoCurve) ToBSpline() (*BSpline, error) {
	if c == nil || c.patch == nil {
		return nil, ErrInvalid
	}
	poles, weights, e := exportExactNet(c.patch)
	if e != nil {
		return nil, e
	}
	control := make([]Vec3, c.patch.countU)
	w := make([]float64, len(control))
	for i := range control {
		control[i] = poles[i*c.patch.countV]
		w[i] = weights[i*c.patch.countV]
	}
	return &BSpline{degree: c.patch.degreeU, poles: control, weights: w, knots: append([]float64(nil), c.patch.knotsU...)}, nil
}

// ToBSplineSurface losslessly exports an immutable exact patch using the same
// maximum-weight normalization as ToBSpline, or returns ErrUnrepresentable.
// Knots and original parameter rectangle are preserved; nothing is rounded.
func (p *SurfacePatch) ToBSplineSurface() (*BSplineSurface, error) {
	poles, weights, e := exportExactNet(p)
	if e != nil {
		return nil, e
	}
	return &BSplineSurface{degreeU: p.degreeU, degreeV: p.degreeV, countU: p.countU, countV: p.countV, poles: poles, weights: weights, knotsU: append([]float64(nil), p.knotsU...), knotsV: append([]float64(nil), p.knotsV...)}, nil
}

// ExactPoint returns an exact represented point only if all three rational
// coordinates fit float64 without rounding. Otherwise ErrUnrepresentable is
// returned. Work uses this descriptor's per-operation construction limits.
func (c *SurfaceIsoCurve) ExactPoint(parameter float64) (Vec3, error) {
	if c == nil || c.patch == nil {
		return Vec3{}, ErrInvalid
	}
	lo, hi := c.Domain()
	if !finite(parameter) || parameter < lo || parameter > hi {
		return Vec3{}, ErrDomain
	}
	b := &polynomialBudget{maxOperations: c.patch.options.MaxOperations, maxBits: c.patch.options.MaxCoefficientBits}
	point, e := exactPatchPointAt(c.patch, parameter, 0, b)
	if e != nil {
		return Vec3{}, e
	}
	var values [3]float64
	for axis := range point {
		f, e := exactRepresentableFloat(point[axis])
		if e != nil {
			return Vec3{}, e
		}
		values[axis] = f
	}
	return Vec3{values[0], values[1], values[2]}, nil
}
