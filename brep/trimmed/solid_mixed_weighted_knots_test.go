package trimmed

import (
	"math/big"
	"testing"
)

// Scaling chart coordinates by 81=3^4 exactly clears the odd denominators
// introduced by bisecting the degree-four Mobius weights 1,2,4,8,16. This is
// a change of source parameter units, not a rounded or enlarged physical solid.

// Independent exact de Boor evaluation in the ORIGINAL authored knot domain.
// This does not use the candidate Bezier normalization under test.
func authoredMixedSplinePoint(t *testing.T, q SplineUV, parameter *big.Rat) [2]*big.Rat {
	t.Helper()
	poles, weights, knots := q.Curve.Poles(), q.Curve.Weights(), q.Curve.Knots()
	p := q.Curve.Degree()
	span := len(poles) - 1
	for i := p; i < len(poles); i++ {
		if parameter.Cmp(ratInset(knots[i+1])) < 0 {
			span = i
			break
		}
	}
	d := make([][3]*big.Rat, p+1)
	for j := 0; j <= p; j++ {
		i := span - p + j
		w := big.NewRat(1, 1)
		if len(weights) > 0 {
			w = ratInset(weights[i])
		}
		d[j] = [3]*big.Rat{new(big.Rat).Mul(ratInset(poles[i].X), w), new(big.Rat).Mul(ratInset(poles[i].Y), w), new(big.Rat).Set(w)}
	}
	for r := 1; r <= p; r++ {
		for j := p; j >= r; j-- {
			lo := ratInset(knots[j+span-p])
			hi := ratInset(knots[j+span-r+1])
			a := new(big.Rat).Quo(new(big.Rat).Sub(parameter, lo), new(big.Rat).Sub(hi, lo))
			b := new(big.Rat).Sub(big.NewRat(1, 1), a)
			for k := 0; k < 3; k++ {
				d[j][k] = new(big.Rat).Add(new(big.Rat).Mul(b, d[j-1][k]), new(big.Rat).Mul(a, d[j][k]))
			}
		}
	}
	return [2]*big.Rat{new(big.Rat).Quo(d[p][0], d[p][2]), new(big.Rat).Quo(d[p][1], d[p][2])}
}
