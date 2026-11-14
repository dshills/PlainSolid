package geom

import (
	"errors"
	"fmt"
	"math/big"
	"testing"
)

// Independent original binomial formula, retained only as a test/bench oracle.
func originalBernsteinPower(coefficients []*big.Rat, b *polynomialBudget) (rationalPolynomial, error) {
	n := len(coefficients) - 1
	p := make(rationalPolynomial, n+1)
	for k := 0; k <= n; k++ {
		p[k] = new(big.Rat)
		for i := 0; i <= k; i++ {
			f := new(big.Int).Mul(new(big.Int).Binomial(int64(n), int64(i)), new(big.Int).Binomial(int64(n-i), int64(k-i)))
			term := new(big.Rat).Mul(coefficients[i], new(big.Rat).SetInt(f))
			if (k-i)%2 == 1 {
				term.Neg(term)
			}
			p[k].Add(p[k], term)
			if e := b.check(p[k]); e != nil {
				return nil, e
			}
		}
	}
	return trimPolynomial(p), nil
}
func powerFactorInput(n int) []*big.Rat {
	out := make([]*big.Rat, n+1)
	for i := range out {
		out[i] = big.NewRat(int64((i%3-1)*(i*i+7)), int64(3*i+5))
	}
	return out
}
func TestSmallBernsteinPowerPreservesProofAccounting(t *testing.T) {
	for n := 0; n <= 10; n++ {
		values := powerFactorInput(n)
		for _, large := range []bool{false, true} {
			if large {
				values[n] = new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 90))
			}
			for _, bits := range []int{64, 128} {
				for limit := 1; limit <= (n+1)*(n+2)/2+1; limit++ {
					a := &polynomialBudget{maxOperations: limit, maxBits: bits}
					b := &polynomialBudget{maxOperations: limit, maxBits: bits}
					x, e := bernsteinToPower(values, a)
					y, f := originalBernsteinPower(values, b)
					if (e == nil) != (f == nil) || (e != nil && !errors.Is(e, f)) || a.operations != b.operations || len(x) != len(y) {
						t.Fatal(n, large, bits, limit, a.operations, b.operations, e, f)
					}
					for i := range x {
						if x[i].Cmp(y[i]) != 0 {
							t.Fatal("different exact coefficient", n, i, x[i], y[i])
						}
					}
				}
			}
		}
	}
	for n := range smallBernsteinPowerFactors {
		for k := 0; k <= n; k++ {
			for i := 0; i <= k; i++ {
				want := new(big.Int).Mul(new(big.Int).Binomial(int64(n), int64(i)), new(big.Int).Binomial(int64(n-i), int64(k-i)))
				if !want.IsInt64() || want.Int64() != smallBernsteinPowerFactors[n][k][i] {
					t.Fatal(n, k, i, want)
				}
			}
		}
	}
}
func BenchmarkBernsteinPowerFactors(b *testing.B) {
	for _, n := range []int{6, 8} {
		for _, old := range []bool{false, true} {
			label := "table"
			if old {
				label = "original"
			}
			b.Run(fmt.Sprintf("degree%d/%s", n, label), func(b *testing.B) {
				values := powerFactorInput(n)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					budget := &polynomialBudget{maxOperations: 10000, maxBits: 4096}
					var e error
					if old {
						_, e = originalBernsteinPower(values, budget)
					} else {
						_, e = bernsteinToPower(values, budget)
					}
					if e != nil {
						b.Fatal(e)
					}
				}
			})
		}
	}
}
