package geom

import "math/big"

// Transformed applies the represented affine coefficients to the exact
// homogeneous net. Control points remain rational and are not rounded.
// It retains the chart and its subdivision limits, including for singular
// placements. This supports exact hull and contact proofs in world space.
func (p *SurfacePatch) Transformed(m Affine) (*SurfacePatch, error) {
	if p == nil {
		return nil, ErrInvalid
	}
	out, _, e := p.TransformedWithWorkLimit(m, p.options.MaxOperations)
	return out, e
}

// TransformedWithWorkLimit is Transformed with an explicit operation limit
// for this call and a report of the work charged, including on failure.
func (p *SurfacePatch) TransformedWithWorkLimit(m Affine, maxOperations int) (_ *SurfacePatch, work int, err error) {
	if p == nil || len(p.net) == 0 || !m.IsFinite() {
		return nil, 0, ErrInvalid
	}
	if maxOperations < 1 {
		return nil, 0, ErrInvalid
	}
	if m == Identity() {
		return p, 0, nil
	}
	b := &polynomialBudget{maxOperations: maxOperations, maxBits: p.options.MaxCoefficientBits}
	defer func() { work = min(b.operations, b.maxOperations) }()
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
					return nil, 0, e
				}
			}
			out.net[i][j] = sum
		}
	}
	if e := out.computeBounds(b); e != nil {
		return nil, 0, e
	}
	out.operations = b.operations
	return &out, b.operations, nil
}
