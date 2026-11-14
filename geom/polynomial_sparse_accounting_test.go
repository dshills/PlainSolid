package geom

import (
	"errors"
	"math/big"
	"testing"
)

func originalDenseProduct(a, c rationalPolynomial, b *polynomialBudget) (rationalPolynomial, error) {
	if len(a) == 0 || len(c) == 0 {
		return nil, nil
	}
	p := make(rationalPolynomial, len(a)+len(c)-1)
	for i := range p {
		p[i] = new(big.Rat)
	}
	for i, x := range a {
		for j, y := range c {
			p[i+j].Add(p[i+j], new(big.Rat).Mul(x, y))
			if e := b.check(p[i+j]); e != nil {
				return nil, e
			}
		}
	}
	return trimPolynomial(p), nil
}
func sparseProductInput(large bool) rationalPolynomial {
	p := make(rationalPolynomial, 9)
	for i := range p {
		p[i] = new(big.Rat)
	}
	p[0] = big.NewRat(2, 3)
	p[4] = big.NewRat(-7, 11)
	p[8] = big.NewRat(13, 5)
	if large {
		p[4].SetInt(new(big.Int).Lsh(big.NewInt(1), 90))
	}
	return p
}
func TestSparsePolynomialProductPreservesAccounting(t *testing.T) {
	for _, large := range []bool{false, true} {
		for _, bits := range []int{64, 128} {
			a, c := sparseProductInput(large), sparseProductInput(false)
			for limit := 1; limit <= len(a)*len(c)+1; limit++ {
				b := &polynomialBudget{maxOperations: limit, maxBits: bits}
				d := &polynomialBudget{maxOperations: limit, maxBits: bits}
				x, e := polynomialProduct(a, c, b)
				y, f := originalDenseProduct(a, c, d)
				if (e == nil) != (f == nil) || (e != nil && !errors.Is(e, f)) || b.operations != d.operations || len(x) != len(y) {
					t.Fatal(large, bits, limit, b.operations, d.operations, e, f)
				}
				for i := range x {
					if x[i].Cmp(y[i]) != 0 {
						t.Fatal("coefficient", i)
					}
				}
			}
		}
	}
	a := sparseProductInput(false)
	b := &polynomialBudget{maxOperations: 1000, maxBits: 4096}
	p, e := polynomialProduct(a, a, b)
	if e != nil {
		t.Fatal(e)
	}
	p[0].SetInt64(999)
	if a[0].Cmp(big.NewRat(2, 3)) != 0 {
		t.Fatal("source alias")
	}
}
func BenchmarkSparsePolynomialProduct(b *testing.B) {
	for _, old := range []bool{false, true} {
		label := "zero-elision"
		if old {
			label = "original"
		}
		b.Run(label, func(b *testing.B) {
			a, c := sparseProductInput(false), sparseProductInput(false)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				budget := &polynomialBudget{maxOperations: 1000, maxBits: 4096}
				var e error
				if old {
					_, e = originalDenseProduct(a, c, budget)
				} else {
					_, e = polynomialProduct(a, c, budget)
				}
				if e != nil {
					b.Fatal(e)
				}
			}
		})
	}
}
func BenchmarkZeroBernsteinConversion(b *testing.B) {
	for _, old := range []bool{false, true} {
		label := "zero-elision"
		if old {
			label = "table-before"
		}
		b.Run(label, func(b *testing.B) {
			values := make([]*big.Rat, 9)
			for i := range values {
				values[i] = new(big.Rat)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				budget := &polynomialBudget{maxOperations: 1000, maxBits: 4096}
				var e error
				if old {
					_, e = tableBeforeZeroElision(values, budget)
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

// Previous checkpoint's factor-table conversion, without zero-term elision.
func tableBeforeZeroElision(values []*big.Rat, b *polynomialBudget) (rationalPolynomial, error) {
	n := len(values) - 1
	p := make(rationalPolynomial, n+1)
	for k := 0; k <= n; k++ {
		p[k] = new(big.Rat)
		for i := 0; i <= k; i++ {
			term := new(big.Rat).Mul(values[i], new(big.Rat).SetInt64(smallBernsteinPowerFactors[n][k][i]))
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
func TestZeroBernsteinPreservesAccounting(t *testing.T) {
	values := make([]*big.Rat, 9)
	for i := range values {
		values[i] = new(big.Rat)
	}
	for limit := 1; limit <= 46; limit++ {
		b := &polynomialBudget{maxOperations: limit, maxBits: 64}
		d := &polynomialBudget{maxOperations: limit, maxBits: 64}
		x, e := bernsteinToPower(values, b)
		y, f := tableBeforeZeroElision(values, d)
		if len(x) != 0 || len(y) != 0 || (e == nil) != (f == nil) || (e != nil && !errors.Is(e, f)) || b.operations != d.operations {
			t.Fatal(limit, b.operations, d.operations, e, f)
		}
	}
}
