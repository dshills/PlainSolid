package geom

import "math/big"

// Transform applies the represented affine matrix to the exact homogeneous
// control net without converting poles or weights to float64. The result is
// immutable and preserves the original parameter domain, knots and fixed-axis
// metadata. Singular maps, including collapse to a point, are supported.
// ExactOperations on the result reports this transform's work only. Options
// bound construction and subsequent evaluations; the source survives failures.
// Sequential transforms compose exactly as represented; this may differ from
// applying an Affine.Mul result whose matrix entries have rounded to float64.
func (c *SurfaceIsoCurve) Transform(m Affine, options SurfacePatchOptions) (*SurfaceIsoCurve, error) {
	if c == nil || c.patch == nil || !m.IsFinite() {
		return nil, ErrInvalid
	}
	o, e := patchOptions(options)
	if e != nil {
		return nil, e
	}
	source := c.patch
	if len(source.net) > o.MaxPoles {
		return nil, ErrConvergence
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	var matrix [3][4]*big.Rat
	for i := range matrix {
		for j := range matrix[i] {
			matrix[i][j] = new(big.Rat).SetFloat64(m[i][j])
			if e := b.check(matrix[i][j]); e != nil {
				return nil, e
			}
		}
	}
	net := make([]exactHomogeneous, len(source.net))
	for i, h := range source.net {
		for axis := 0; axis < 3; axis++ {
			value := new(big.Rat)
			for j := 0; j < 4; j++ {
				value.Add(value, new(big.Rat).Mul(matrix[axis][j], h[j]))
				if e := b.check(value); e != nil {
					return nil, e
				}
			}
			net[i][axis] = value
		}
		net[i][3] = new(big.Rat).Set(h[3])
		if e := b.check(net[i][3]); e != nil {
			return nil, e
		}
	}
	p := &SurfacePatch{degreeU: source.degreeU, degreeV: source.degreeV, countU: source.countU, countV: source.countV, knotsU: append([]float64(nil), source.knotsU...), knotsV: append([]float64(nil), source.knotsV...), net: net, options: o}
	if e := p.computeBounds(b); e != nil {
		return nil, e
	}
	p.operations = b.operations
	return &SurfaceIsoCurve{patch: p, fixedAxis: c.fixedAxis, fixedParameter: c.fixedParameter, operations: b.operations}, nil
}
