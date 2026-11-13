package geom

import "math/big"

// Transformed applies the represented affine coefficients to the exact
// homogeneous net. Control points remain rational and are not rounded.
// It retains the chart and its subdivision limits, including for singular
// placements. This supports exact hull and contact proofs in world space.
func (p *SurfacePatch) Transformed(m Affine) (*SurfacePatch, error) {
	if p == nil || len(p.net) == 0 || !m.IsFinite() {
		return nil, ErrInvalid
	}
	if m == Identity() {
		return p, nil
	}
	b := &polynomialBudget{maxOperations: p.options.MaxOperations, maxBits: p.options.MaxCoefficientBits}
	out := *p
	out.net = make([]exactHomogeneous, len(p.net))
	var coef [3][4]*big.Rat
	for i, row := range m {
		for j, value := range row {
			coef[i][j] = new(big.Rat).SetFloat64(value)
		}
	}
	for i, h := range p.net {
		out.net[i][3] = h[3]
		for j := range 3 {
			sum := new(big.Rat)
			for k := range 4 {
				sum.Add(sum, new(big.Rat).Mul(h[k], coef[j][k]))
				if e := b.check(sum); e != nil {
					return nil, e
				}
			}
			out.net[i][j] = sum
		}
	}
	if e := out.computeBounds(b); e != nil {
		return nil, e
	}
	out.operations = b.operations
	return &out, nil
}
