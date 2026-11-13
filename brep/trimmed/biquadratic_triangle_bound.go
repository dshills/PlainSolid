package trimmed

import (
	"math/big"
	"plainsolid/geom"
)

type trianglePower4 [5][5]*big.Rat

func zeroTrianglePower4() trianglePower4 {
	var p trianglePower4
	for i := 0; i <= 4; i++ {
		for j := 0; j <= 4-i; j++ {
			p[i][j] = new(big.Rat)
		}
	}
	return p
}
func multiplyTrianglePower4(a, b trianglePower4) trianglePower4 {
	r := zeroTrianglePower4()
	for i := 0; i <= 4; i++ {
		for j := 0; j <= 4-i; j++ {
			if a[i][j].Sign() == 0 {
				continue
			}
			for k := 0; k <= 4-i-j; k++ {
				for l := 0; l <= 4-i-j-k; l++ {
					if b[k][l].Sign() != 0 {
						r[i+k][j+l].Add(r[i+k][j+l], new(big.Rat).Mul(a[i][j], b[k][l]))
					}
				}
			}
		}
	}
	return r
}

// biquadraticCorrespondenceBound composes a certified polynomial patch with
// UV(lambda) and subtracts the model triangle's affine interpolation. Exact
// total-degree-four triangular Bernstein controls enclose that residual. The
// barycentric correspondence covers both images, proving both directions.
func biquadraticCorrespondenceBound(p *geom.SurfacePatch, uv [3]geom.Vec2, model [3]geom.Vec3) (*big.Rat, bool) {
	coefficients, ok := p.BiquadraticCoefficients()
	if !ok {
		return nil, false
	}
	d := p.Domain()
	du := new(big.Rat).Sub(ratInset(d.UMax), ratInset(d.UMin))
	dv := new(big.Rat).Sub(ratInset(d.VMax), ratInset(d.VMin))
	var powers [2][3]trianglePower4
	for axis := 0; axis < 2; axis++ {
		values := [3]*big.Rat{}
		for i, v := range uv {
			value, origin, width := v.X, d.UMin, du
			if axis == 1 {
				value, origin, width = v.Y, d.VMin, dv
			}
			values[i] = new(big.Rat).Quo(new(big.Rat).Sub(ratInset(value), ratInset(origin)), width)
		}
		linear := zeroTrianglePower4()
		linear[0][0].Set(values[0])
		linear[1][0].Sub(values[1], values[0])
		linear[0][1].Sub(values[2], values[0])
		powers[axis][0] = zeroTrianglePower4()
		powers[axis][0][0][0].SetInt64(1)
		powers[axis][1] = linear
		powers[axis][2] = multiplyTrianglePower4(linear, linear)
	}
	var residual [3]trianglePower4
	for k := range residual {
		residual[k] = zeroTrianglePower4()
	}
	for a := 0; a <= 2; a++ {
		for b := 0; b <= 2; b++ {
			term := multiplyTrianglePower4(powers[0][a], powers[1][b])
			for i := 0; i <= 4; i++ {
				for j := 0; j <= 4-i; j++ {
					for k := 0; k < 3; k++ {
						residual[k][i][j].Add(residual[k][i][j], new(big.Rat).Mul(term[i][j], coefficients[a][b][k]))
					}
				}
			}
		}
	}
	target := [3]exactBoundVector{exactBoundPoint(model[0]), exactBoundPoint(model[1]), exactBoundPoint(model[2])}
	for k := 0; k < 3; k++ {
		residual[k][0][0].Sub(residual[k][0][0], target[0][k])
		residual[k][1][0].Sub(residual[k][1][0], new(big.Rat).Sub(target[1][k], target[0][k]))
		residual[k][0][1].Sub(residual[k][0][1], new(big.Rat).Sub(target[2][k], target[0][k]))
	}
	falling := func(n, k int) int64 {
		r := int64(1)
		for i := 0; i < k; i++ {
			r *= int64(n - i)
		}
		return r
	}
	bound := new(big.Rat)
	// s^a*t^b has control (i)_a*(j)_b/(4)_(a+b) at index(i,j).
	for i := 0; i <= 4; i++ {
		for j := 0; j <= 4-i; j++ {
			var control exactBoundVector
			for k := range control {
				control[k] = new(big.Rat)
			}
			for a := 0; a <= i; a++ {
				for b := 0; b <= j; b++ {
					factor := big.NewRat(falling(i, a)*falling(j, b), falling(4, a+b))
					for k := 0; k < 3; k++ {
						control[k].Add(control[k], new(big.Rat).Mul(residual[k][a][b], factor))
					}
				}
			}
			squared := boundDot(control, control)
			if squared.Cmp(bound) > 0 {
				bound.Set(squared)
			}
		}
	}
	return bound, true
}
