package trimmed

import (
	"math/big"
	"plainsolid/geom"
)

// bilinearCorrespondenceBound encloses the exact quadratic error polynomial
// S(sum(lambda_i UV_i))-sum(lambda_i model_i) by its six triangular Bernstein
// controls. The same barycentric map covers both images, hence either directed
// Hausdorff distance is bounded, including represented model vertex rounding.
func bilinearCorrespondenceBound(p *geom.SurfacePatch, uv [3]geom.Vec2, model [3]geom.Vec3) (*big.Rat, bool) {
	coefficients, ok := p.BilinearCoefficients()
	if !ok {
		return nil, false
	}
	d := p.Domain()
	u0, v0 := ratInset(d.UMin), ratInset(d.VMin)
	du, dv := new(big.Rat).Sub(ratInset(d.UMax), u0), new(big.Rat).Sub(ratInset(d.VMax), v0)
	var x, y [3]*big.Rat
	var residual [3]exactBoundVector
	bound := new(big.Rat)
	for i := 0; i < 3; i++ {
		x[i] = new(big.Rat).Quo(new(big.Rat).Sub(ratInset(uv[i].X), u0), du)
		y[i] = new(big.Rat).Quo(new(big.Rat).Sub(ratInset(uv[i].Y), v0), dv)
		target := exactBoundPoint(model[i])
		for k := 0; k < 3; k++ {
			value := new(big.Rat).Set(coefficients[0][k])
			value.Add(value, new(big.Rat).Mul(coefficients[1][k], x[i]))
			value.Add(value, new(big.Rat).Mul(coefficients[2][k], y[i]))
			value.Add(value, new(big.Rat).Mul(coefficients[3][k], new(big.Rat).Mul(x[i], y[i])))
			residual[i][k] = value.Sub(value, target[k])
		}
		norm := boundDot(residual[i], residual[i])
		if norm.Cmp(bound) > 0 {
			bound.Set(norm)
		}
	}
	for i := 0; i < 3; i++ {
		for j := i + 1; j < 3; j++ {
			factor := new(big.Rat).Mul(new(big.Rat).Sub(x[i], x[j]), new(big.Rat).Sub(y[i], y[j]))
			var control exactBoundVector
			for k := 0; k < 3; k++ {
				control[k] = new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Add(residual[i][k], residual[j][k]), new(big.Rat).Mul(coefficients[3][k], factor)), big.NewRat(2, 1))
			}
			norm := boundDot(control, control)
			if norm.Cmp(bound) > 0 {
				bound.Set(norm)
			}
		}
	}
	return bound, true
}
