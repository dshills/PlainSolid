package trimmed

import (
	"math/big"

	"plainsolid/geom"
)

// A grid rectangle has triangles (lower left, lower right, upper right)
// and (lower left, upper right, upper left). In barycentric coordinates
// (1-s-t,s,t) their normalized UV maps are (s+t,t) and (s,s+t).
func rationalBiquadraticGridHalf(d geom.ParameterRectangle, uv [3][2]*big.Rat) (int, bool) {
	lo := [2]*big.Rat{ratInset(d.UMin), ratInset(d.VMin)}
	hi := [2]*big.Rat{ratInset(d.UMax), ratInset(d.VMax)}
	if uv[0][0].Cmp(lo[0]) != 0 || uv[0][1].Cmp(lo[1]) != 0 || uv[1][0].Cmp(hi[0]) != 0 || uv[2][1].Cmp(hi[1]) != 0 {
		return 0, false
	}
	if uv[1][1].Cmp(lo[1]) == 0 && uv[2][0].Cmp(hi[0]) == 0 {
		return 0, true
	}
	if uv[1][1].Cmp(hi[1]) == 0 && uv[2][0].Cmp(lo[0]) == 0 {
		return 1, true
	}
	return 0, false
}

// Four factorial clears every degree-four monomial-to-Bernstein denominator.
const gridTrianglePowerScale = 24

// Each table entry gives one normalized tensor power monomial's contribution
// to one triangular degree-four control, scaled by gridTrianglePowerScale.
var gridTrianglePowerFactors = func() [2][5][5][3][3]int64 {
	var factors [2][5][5][3][3]int64
	falling := func(n, k int) int64 {
		p := int64(1)
		for i := 0; i < k; i++ {
			p *= int64(n - i)
		}
		return p
	}
	for half := range factors {
		for i := 0; i <= 4; i++ {
			for j := 0; j <= 4-i; j++ {
				for u := 0; u <= 2; u++ {
					for v := 0; v <= 2; v++ {
						power := u
						if half == 1 {
							power = v
						}
						for k := 0; k <= power; k++ {
							a, b := k, u+v-k
							if half == 1 {
								a, b = u+k, v-k
							}
							if a > i || b > j {
								continue
							}
							multiplier := int64(1)
							if power == 2 && k == 1 {
								multiplier = 2
							}
							factors[half][i][j][u][v] += multiplier * falling(i, a) * falling(j, b) * (gridTrianglePowerScale / falling(4, a+b))
						}
					}
				}
			}
		}
	}
	return factors
}()

// Keep the common denominator and scaled integers on the fast path bounded.
// Larger input or combined denominators use the general rational expression.
func gridTriangleCommonDenominator(values []*big.Rat) (*big.Int, bool) {
	const maxBits = 4096
	denominator := big.NewInt(1)
	var common big.Int
	for _, x := range values {
		d := x.Denom()
		if x.Num().BitLen() > maxBits || d.BitLen() > maxBits {
			return nil, false
		}
		if d.BitLen() == 1 || d.Cmp(denominator) == 0 {
			continue
		}
		common.GCD(nil, nil, denominator, d)
		denominator.Quo(denominator, &common)
		denominator.Mul(denominator, d)
		if denominator.BitLen() > maxBits {
			return nil, false
		}
	}
	return denominator, true
}

// The lower-degree homogeneous polynomial is pulled back onto the right
// triangle and written in degree-four Bernstein form. Multiplying its weight
// field by the affine target and elevating H to degree five use the same
// factors beta[k]/5 at each Bernstein index beta. Work in a common integer
// scale until the final norm bound. Uniqueness of Bernstein coefficients gives
// the same degree-five residual controls and maximum as the general proof.
func rationalBiquadraticGridTriangleBound(coefficients [3][3][4]*big.Rat, minimumWeight *big.Rat, model [3][3]*big.Rat, half int) (*big.Rat, bool) {
	var flat [36]*big.Rat
	for i := range coefficients {
		for j, h := range coefficients[i] {
			copy(flat[4*(3*i+j):], h[:])
		}
	}
	denominator, ok := gridTriangleCommonDenominator(flat[:])
	if !ok {
		return nil, false
	}
	var coordinates [9]*big.Rat
	for i, p := range model {
		copy(coordinates[3*i:], p[:])
	}
	modelDenominator, ok := gridTriangleCommonDenominator(coordinates[:])
	if !ok {
		return nil, false
	}
	var powers [3][3][4]big.Int
	var target [3][3]big.Int
	var quotient, factor, term big.Int
	setNumerator := func(result *big.Int, value *big.Rat, denominator *big.Int) {
		if value.Sign() != 0 {
			quotient.Quo(denominator, value.Denom())
			result.Mul(&quotient, value.Num())
		}
	}
	for i := range coefficients {
		for j, h := range coefficients[i] {
			for k, x := range h {
				setNumerator(&powers[i][j][k], x, denominator)
			}
		}
	}
	for i, p := range model {
		for axis, x := range p {
			setNumerator(&target[i][axis], x, modelDenominator)
		}
	}

	var controls [5][5][4]big.Int
	for i := 0; i <= 4; i++ {
		for j := 0; j <= 4-i; j++ {
			c := &controls[i][j]
			for u, row := range gridTrianglePowerFactors[half][i][j] {
				for v, multiplier := range row {
					if multiplier == 0 {
						continue
					}
					factor.SetInt64(multiplier)
					for k := range c {
						c[k].Add(&c[k], term.Mul(&powers[u][v][k], &factor))
					}
				}
			}
			for axis := 0; axis < 3; axis++ {
				c[axis].Mul(&c[axis], modelDenominator)
			}
		}
	}

	var maximum, squared big.Int
	for i := 0; i <= 5; i++ {
		for j := 0; j <= 5-i; j++ {
			var control [3]big.Int
			for vertex, count := range [3]int{5 - i - j, i, j} {
				if count == 0 {
					continue
				}
				a, b := i, j
				if vertex == 1 {
					a--
				}
				if vertex == 2 {
					b--
				}
				c := &controls[a][b]
				factor.SetInt64(int64(count))
				for axis := range control {
					term.Mul(&c[3], &target[vertex][axis])
					term.Sub(&c[axis], &term)
					control[axis].Add(&control[axis], term.Mul(&term, &factor))
				}
			}
			squared.SetInt64(0)
			for axis := range control {
				squared.Add(&squared, term.Mul(&control[axis], &control[axis]))
			}
			if squared.Cmp(&maximum) > 0 {
				maximum.Set(&squared)
			}
		}
	}
	// Divide the squared integer norm by the square of the common
	// coefficient scale times the same proved minimum weight.
	denominator.Mul(denominator, modelDenominator)
	denominator.Mul(denominator, factor.SetInt64(5*gridTrianglePowerScale))
	denominator.Mul(denominator, minimumWeight.Num())
	denominator.Mul(denominator, denominator)
	term.Mul(minimumWeight.Denom(), minimumWeight.Denom())
	maximum.Mul(&maximum, &term)
	return new(big.Rat).SetFrac(&maximum, denominator), true
}
