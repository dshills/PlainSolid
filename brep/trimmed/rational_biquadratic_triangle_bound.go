package trimmed

import (
	"math/big"
	"plainsolid/geom"
)

type trianglePower5 [6][6]*big.Rat

func zeroTrianglePower5() trianglePower5 {
	var p trianglePower5
	for i := 0; i <= 5; i++ {
		for j := 0; j <= 5-i; j++ {
			p[i][j] = new(big.Rat)
		}
	}
	return p
}
func multiplyTrianglePower5(a, b trianglePower5) trianglePower5 {
	r := zeroTrianglePower5()
	for i := 0; i <= 5; i++ {
		for j := 0; j <= 5-i; j++ {
			if a[i][j].Sign() == 0 {
				continue
			}
			for k := 0; k <= 5-i-j; k++ {
				for l := 0; l <= 5-i-j-k; l++ {
					if b[k][l].Sign() != 0 {
						r[i+k][j+l].Add(r[i+k][j+l], new(big.Rat).Mul(a[i][j], b[k][l]))
					}
				}
			}
		}
	}
	return r
}

// rationalBiquadraticCorrespondenceBound encloses H-W*T in total-degree-five
// triangular Bernstein form. Positive control weights prove W>=minimumWeight,
// so max|control|²/minimumWeight² bounds the exact residual |H/W-T|².
// The shared barycentric correspondence certifies both directed distances.
func rationalBiquadraticCorrespondenceBound(p *geom.SurfacePatch, uv [3]geom.Vec2, model [3]geom.Vec3) (*big.Rat, bool) {
	var exact [3][2]*big.Rat
	for i, v := range uv {
		exact[i] = [2]*big.Rat{ratInset(v.X), ratInset(v.Y)}
	}
	return rationalBiquadraticExactCorrespondenceBound(p, exact, model)
}

func rationalBiquadraticExactCorrespondenceBound(p *geom.SurfacePatch, uv [3][2]*big.Rat, model [3]geom.Vec3) (*big.Rat, bool) {
	exact := [3][3]*big.Rat{[3]*big.Rat(exactBoundPoint(model[0])), [3]*big.Rat(exactBoundPoint(model[1])), [3]*big.Rat(exactBoundPoint(model[2]))}
	return rationalBiquadraticRationalCorrespondenceBound(p, uv, exact)
}

func rationalBiquadraticRationalCorrespondenceBound(p *geom.SurfacePatch, uv [3][2]*big.Rat, model [3][3]*big.Rat) (*big.Rat, bool) {
	coefficients, minimumWeight, ok := p.RationalBiquadraticCoefficients()
	if !ok {
		return nil, false
	}
	d := p.Domain()
	du := new(big.Rat).Sub(ratInset(d.UMax), ratInset(d.UMin))
	dv := new(big.Rat).Sub(ratInset(d.VMax), ratInset(d.VMin))
	var powers [2][3]trianglePower5
	for axis := 0; axis < 2; axis++ {
		values := [3]*big.Rat{}
		for i, v := range uv {
			value, origin, width := v[0], d.UMin, du
			if axis == 1 {
				value, origin, width = v[1], d.VMin, dv
			}
			values[i] = new(big.Rat).Quo(new(big.Rat).Sub(value, ratInset(origin)), width)
		}
		linear := zeroTrianglePower5()
		linear[0][0].Set(values[0])
		linear[1][0].Sub(values[1], values[0])
		linear[0][1].Sub(values[2], values[0])
		powers[axis][0] = zeroTrianglePower5()
		powers[axis][0][0][0].SetInt64(1)
		powers[axis][1] = linear
		powers[axis][2] = multiplyTrianglePower5(linear, linear)
	}
	var homogeneous [4]trianglePower5
	for k := range homogeneous {
		homogeneous[k] = zeroTrianglePower5()
	}
	for a := 0; a <= 2; a++ {
		for b := 0; b <= 2; b++ {
			nonzero := false
			for k := 0; k < 4; k++ {
				nonzero = nonzero || coefficients[a][b][k].Sign() != 0
			}
			if !nonzero {
				continue
			}
			term := multiplyTrianglePower5(powers[0][a], powers[1][b])
			for i := 0; i <= 5; i++ {
				for j := 0; j <= 5-i; j++ {
					if term[i][j].Sign() == 0 {
						continue
					}
					for k := 0; k < 4; k++ {
						if coefficients[a][b][k].Sign() == 0 {
							continue
						}
						homogeneous[k][i][j].Add(homogeneous[k][i][j], new(big.Rat).Mul(term[i][j], coefficients[a][b][k]))
					}
				}
			}
		}
	}
	target := [3]exactBoundVector{exactBoundVector(model[0]), exactBoundVector(model[1]), exactBoundVector(model[2])}
	var residual [3]trianglePower5
	for k := 0; k < 3; k++ {
		affine := zeroTrianglePower5()
		affine[0][0].Set(target[0][k])
		affine[1][0].Sub(target[1][k], target[0][k])
		affine[0][1].Sub(target[2][k], target[0][k])
		product := multiplyTrianglePower5(homogeneous[3], affine)
		residual[k] = zeroTrianglePower5()
		for i := 0; i <= 5; i++ {
			for j := 0; j <= 5-i; j++ {
				residual[k][i][j].Sub(homogeneous[k][i][j], product[i][j])
			}
		}
	}
	bound := new(big.Rat)
	// s^a*t^b has control (i)_a*(j)_b/(5)_(a+b) at index(i,j).
	for i := 0; i <= 5; i++ {
		for j := 0; j <= 5-i; j++ {
			var control exactBoundVector
			for k := range control {
				control[k] = new(big.Rat)
			}
			for a := 0; a <= i; a++ {
				for b := 0; b <= j; b++ {
					factor := &triangleDegreeFiveFactors[i][j][a][b]
					for k := 0; k < 3; k++ {
						if residual[k][a][b].Sign() == 0 {
							continue
						}
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
	return bound.Quo(bound, new(big.Rat).Mul(minimumWeight, minimumWeight)), true
}

// Read-only exact power-to-Bernstein factors for total degree five. Receivers
// are always fresh rationals; no caller can mutate this shared table. Skipping
// exact zeros and reusing these constants leaves every certificate unchanged.
var triangleDegreeFiveFactors = func() [6][6][6][6]big.Rat {
	var out [6][6][6][6]big.Rat
	falling := func(n, k int) int64 {
		r := int64(1)
		for i := 0; i < k; i++ {
			r *= int64(n - i)
		}
		return r
	}
	for i := 0; i <= 5; i++ {
		for j := 0; j <= 5-i; j++ {
			for a := 0; a <= i; a++ {
				for b := 0; b <= j; b++ {
					out[i][j][a][b].SetFrac64(falling(i, a)*falling(j, b), falling(5, a+b))
				}
			}
		}
	}
	return out
}()
