package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestExactPolynomialDegreeEightMultiplicityAndOwnership(t *testing.T) {
	// (x-1)^2(x+2)^2(x²-2)(x²+1), including irrational roots
	// adjacent to repeats and an exact root at the requested endpoint.
	f := []*big.Rat{big.NewRat(1, 1)}
	for _, p := range [][]int64{{-1, 1}, {-1, 1}, {2, 1}, {2, 1}, {-2, 0, 1}, {1, 0, 1}} {
		q := make([]*big.Rat, len(f)+len(p)-1)
		for i := range q {
			q[i] = new(big.Rat)
		}
		for i, a := range f {
			for j, b := range p {
				q[i+j].Add(q[i+j], new(big.Rat).Mul(a, big.NewRat(b, 1)))
			}
		}
		f = q
	}
	ctx := context.Background()
	l, h := big.NewRat(-2, 1), big.NewRat(3, 2)
	o := ExactCurvePlaneEventOptions{}
	got, err := IsolateExactPolynomialRoots(ctx, f, l, h, o)
	if err != nil || len(got.Roots) != 4 {
		t.Fatal(got, err)
	}
	for i, row := range got.Roots {
		if row.Multiplicity != [4]int{2, 1, 2, 1}[i] {
			t.Fatal("multiplicity", i, row)
		}
		sign, err := row.Root.SignPolynomial(ctx, f, o)
		if err != nil || sign.Sign != 0 {
			t.Fatal("degree-eight root identity", i, sign, err)
		}
		iv := row.Root.Interval()
		if i != 0 && got.Roots[i-1].Root.Interval().Upper.Cmp(iv.Lower) >= 0 {
			t.Fatal("unseparated roots", i, iv)
		}
	}
	q := append(f, big.NewRat(1, 1))
	if out, err := IsolateExactPolynomialRoots(ctx, q, l, h, o); !errors.Is(err, ErrInvalid) || out.Roots != nil {
		t.Fatal("degree ceiling", out, err)
	}
	o.MaxOperations = got.Operations - 1
	if out, err := IsolateExactPolynomialRoots(ctx, f, l, h, o); !errors.Is(err, ErrConvergence) || out.Roots != nil {
		t.Fatal("atomic root budget", out, err)
	}
}
