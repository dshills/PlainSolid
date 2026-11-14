package testgraphfixture

import (
	"math/big"
	"plainsolid/geom"
)

// TripleRadialRadii are strictly nested and have represented 3/5,4/5 stations.
func TripleRadialRadii() []*big.Rat {
	return []*big.Rat{big.NewRat(5, 64), big.NewRat(15, 64), big.NewRat(25, 64)}
}

// TripleRadialHeightControls independently constructs degree-eight Bernstein
// coefficients from the expanded radial cubic. Elevation permits exact i/8 XY
// poles; -1225/256 cancels the odd tensor Bernstein denominators. No coefficient
// is converted unless its binary float representation is exact.
func TripleRadialHeightControls() ([9][9]float64, error) {
	var result [9][9]float64
	radius := TripleRadialRadii()
	var squared [3]*big.Rat
	sum, pairs, product := new(big.Rat), new(big.Rat), big.NewRat(1, 1)
	for i, r := range radius {
		squared[i] = new(big.Rat).Mul(r, r)
		sum.Add(sum, squared[i])
		product.Mul(product, squared[i])
	}
	for i := 0; i < 3; i++ {
		for j := i + 1; j < 3; j++ {
			pairs.Add(pairs, new(big.Rat).Mul(squared[i], squared[j]))
		}
	}
	var axis [4][9]*big.Rat
	for power := 0; power <= 3; power++ {
		for i := 0; i <= 8; i++ {
			value := new(big.Rat)
			for k := 0; k <= 2*power && k <= i; k++ {
				factor := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(k)), new(big.Int).Binomial(8, int64(k)))
				factor.Mul(factor, new(big.Rat).SetInt(new(big.Int).Binomial(int64(2*power), int64(k))))
				shift := big.NewRat(1, 1)
				for n := 0; n < 2*power-k; n++ {
					shift.Mul(shift, big.NewRat(-1, 2))
				}
				factor.Mul(factor, shift)
				value.Add(value, factor)
			}
			axis[power][i] = value
		}
	}
	for i := 0; i <= 8; i++ {
		for j := 0; j <= 8; j++ {
			radial := func(n int) *big.Rat {
				v := new(big.Rat)
				for k := 0; k <= n; k++ {
					term := new(big.Rat).Mul(axis[k][i], axis[n-k][j])
					term.Mul(term, new(big.Rat).SetInt(new(big.Int).Binomial(int64(n), int64(k))))
					v.Add(v, term)
				}
				return v
			}
			z := new(big.Rat).Sub(radial(3), new(big.Rat).Mul(sum, radial(2)))
			z.Add(z, new(big.Rat).Mul(pairs, radial(1)))
			z.Sub(z, product)
			z.Mul(z, big.NewRat(-1225, 256))
			f, exact := z.Float64()
			if !exact {
				return result, geom.ErrUnrepresentable
			}
			result[i][j] = f
		}
	}
	return result, nil
}
func TripleRadialGraphs() (*geom.BSplineSurface, *geom.BSplineSurface, error) {
	height, e := TripleRadialHeightControls()
	if e != nil {
		return nil, nil, e
	}
	k := []float64{0, 0, 1, 1}
	a, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, k, k, nil)
	if e != nil {
		return nil, nil, e
	}
	knots := make([]float64, 18)
	for i := 9; i < 18; i++ {
		knots[i] = 1
	}
	var poles []geom.Vec3
	for i := 0; i <= 8; i++ {
		for j := 0; j <= 8; j++ {
			poles = append(poles, geom.Vec3{X: float64(i) / 8, Y: float64(j) / 8, Z: height[i][j]})
		}
	}
	b, e := geom.NewBSplineSurface(8, 8, 9, 9, poles, knots, knots, nil)
	return a, b, e
}
