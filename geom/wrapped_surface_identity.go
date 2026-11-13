package geom

import "math/big"

// SameWrappedSurfaceRecipe identifies identical immutable rational supports
// and wrap domains. It compares stored coefficients directly and does not
// infer geometric equality across different knot bases.
func SameWrappedSurfaceRecipe(a, b *WrappedSurfacePatch) bool {
	if a == nil || b == nil || a.patch == nil || b.patch == nil {
		return false
	}
	if a == b {
		return true
	}
	if a.options.WrapU != b.options.WrapU || a.options.WrapV != b.options.WrapV {
		return false
	}
	p, q := a.patch, b.patch
	if p.degreeU != q.degreeU || p.degreeV != q.degreeV || p.countU != q.countU || p.countV != q.countV || len(p.net) != len(q.net) {
		return false
	}
	for _, pair := range [2][2][]float64{{p.knotsU, q.knotsU}, {p.knotsV, q.knotsV}} {
		if len(pair[0]) != len(pair[1]) {
			return false
		}
		for i, x := range pair[0] {
			if x != pair[1][i] {
				return false
			}
		}
	}
	// Homogeneous control weights may have been normalized once by native
	// export. Compare proportional rational control tuples without rounding.
	for i, h := range p.net {
		for k, x := range h {
			if new(big.Rat).Mul(x, q.net[0][3]).Cmp(new(big.Rat).Mul(q.net[i][k], p.net[0][3])) != 0 {
				return false
			}
		}
	}
	return true
}
