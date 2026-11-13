package geom

import "math/big"

func exactMappedKnots(knots []float64, oldLo, oldHi, newLo, newHi float64, reverse bool, b *polynomialBudget) ([]float64, error) {
	sourceOrigin := new(big.Rat).SetFloat64(oldLo)
	sourceWidth := new(big.Rat).Sub(new(big.Rat).SetFloat64(oldHi), sourceOrigin)
	targetOrigin := new(big.Rat).SetFloat64(newLo)
	targetWidth := new(big.Rat).Sub(new(big.Rat).SetFloat64(newHi), targetOrigin)
	result := make([]float64, len(knots))
	for i, k := range knots {
		parameter := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(k), sourceOrigin), sourceWidth)
		if reverse {
			parameter.Sub(big.NewRat(1, 1), parameter)
		}
		mapped := new(big.Rat).Add(targetOrigin, new(big.Rat).Mul(targetWidth, parameter))
		if e := b.check(mapped); e != nil {
			return nil, e
		}
		f, e := exactRepresentableFloat(mapped)
		if e != nil {
			return nil, e
		}
		index := i
		if reverse {
			index = len(result) - 1 - i
		}
		result[index] = f
	}
	return result, nil
}

// Reparameterize applies an exact affine map of both parameter coordinates to
// target, optionally reversing either direction. Homogeneous poles remain exact.
// Because this descriptor stores float knots, a nonrepresentable mapped knot
// returns ErrUnrepresentable instead of silently changing the parameterization.
func (p *SurfacePatch) Reparameterize(target ParameterRectangle, reverseU, reverseV bool, options SurfacePatchOptions) (*SurfacePatch, error) {
	o, e := patchOptions(options)
	if e != nil {
		return nil, e
	}
	if p == nil || len(p.net) == 0 || !target.Valid() {
		return nil, ErrInvalid
	}
	if len(p.net) > o.MaxPoles {
		return nil, ErrConvergence
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	d := p.Domain()
	ku, e := exactMappedKnots(p.knotsU, d.UMin, d.UMax, target.UMin, target.UMax, reverseU, b)
	if e != nil {
		return nil, e
	}
	kv, e := exactMappedKnots(p.knotsV, d.VMin, d.VMax, target.VMin, target.VMax, reverseV, b)
	if e != nil {
		return nil, e
	}
	net := make([]exactHomogeneous, len(p.net))
	for i := 0; i < p.countU; i++ {
		for j := 0; j < p.countV; j++ {
			u, v := i, j
			if reverseU {
				u = p.countU - 1 - i
			}
			if reverseV {
				v = p.countV - 1 - j
			}
			h := p.net[u*p.countV+v]
			for axis := 0; axis < 4; axis++ {
				if e := b.check(h[axis]); e != nil {
					return nil, e
				}
				net[i*p.countV+j][axis] = new(big.Rat).Set(h[axis])
			}
		}
	}
	return &SurfacePatch{degreeU: p.degreeU, degreeV: p.degreeV, countU: p.countU, countV: p.countV, knotsU: ku, knotsV: kv, net: net, bounds: p.bounds, options: o, operations: b.operations}, nil
}

// Reparameterize changes the curve domain exactly and optionally reverses it.
// Derivatives then use the new parameter. Nonrepresentable mapped knots fail.
func (c *SurfaceIsoCurve) Reparameterize(start, end float64, reverse bool, options SurfacePatchOptions) (*ExactSplineCurve, error) {
	if c == nil || c.patch == nil {
		return nil, ErrInvalid
	}
	p, e := c.patch.Reparameterize(ParameterRectangle{UMin: start, UMax: end, VMin: 0, VMax: 1}, reverse, false, options)
	if e != nil {
		return nil, e
	}
	return &SurfaceIsoCurve{patch: p, fixedAxis: c.fixedAxis, fixedParameter: c.fixedParameter, operations: p.operations}, nil
}
