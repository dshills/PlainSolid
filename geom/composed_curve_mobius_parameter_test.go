package geom

import (
	"context"
	"math/big"
	"testing"
)

func mobiusCurveFixture(t *testing.T, reversed bool) *ExactComposedCurveCell {
	t.Helper()
	ctx := context.Background()
	// B(s)=(s²,s³), while A(t)=B(2t/(1+t)) or B(2(1-t)/(2-t)).
	numerator := rationalPolynomial{new(big.Rat), big.NewRat(2, 1)}
	denominator := rationalPolynomial{big.NewRat(1, 1), big.NewRat(1, 1)}
	if reversed {
		numerator = rationalPolynomial{big.NewRat(2, 1), big.NewRat(-2, 1)}
		denominator = rationalPolynomial{big.NewRat(2, 1), big.NewRat(-1, 1)}
	}
	budget := &polynomialBudget{maxOperations: 100000, maxBits: 16384}
	n2, _ := polynomialProduct(numerator, numerator, budget)
	n3, _ := polynomialProduct(n2, numerator, budget)
	d2, _ := polynomialProduct(denominator, denominator, budget)
	d3, _ := polynomialProduct(d2, denominator, budget)
	x, _ := polynomialProduct(n2, denominator, budget)
	powers := [4]rationalPolynomial{x, n3, nil, d3}
	net := make([][4]*big.Rat, 4)
	for i := range net {
		for axis, p := range powers {
			net[i][axis] = new(big.Rat)
			for j := 0; j <= i && j < len(p); j++ {
				ratio := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(j)), new(big.Int).Binomial(3, int64(j)))
				net[i][axis].Add(net[i][axis], new(big.Rat).Mul(p[j], ratio))
			}
		}
	}
	curve, e := NewExactHomogeneousBezier(ctx, net, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cell, e := NewExactComposedBezierCell(ctx, curve, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return cell
}
func TestComposedMobiusParameterNonlinearSpeed(t *testing.T) {
	ctx := context.Background()
	b := affineParameterFixture(t, false, false)
	for _, reverse := range []bool{false, true} {
		a := mobiusCurveFixture(t, reverse)
		m := [4]*big.Rat{new(big.Rat), big.NewRat(2, 1), big.NewRat(1, 1), big.NewRat(1, 1)}
		want := 1
		if reverse {
			m = [4]*big.Rat{big.NewRat(2, 1), big.NewRat(-2, 1), big.NewRat(2, 1), big.NewRat(-1, 1)}
			want = -1
		}
		r, e := CompareComposedCurvesMobiusParameter(ctx, a, b, m, ExactCurvePlaneEventOptions{})
		if e != nil || !r.Proven || !r.Equivalent || r.OrientationSign != want {
			t.Fatal(reverse, r, e)
		}
		for i := range m {
			m[i] = new(big.Rat).Neg(m[i])
		}
		r, e = CompareComposedCurvesMobiusParameter(ctx, a, b, m, ExactCurvePlaneEventOptions{})
		if e != nil || !r.Equivalent || r.OrientationSign != want {
			t.Fatal("negative denominator canonicalization", r, e)
		}
		r, e = CompareComposedCurvesMobiusParameter(ctx, a, b, m, ExactCurvePlaneEventOptions{MaxOperations: 1})
		if e == nil || r.Proven || r.OrientationSign != 0 {
			t.Fatal("atomic", r, e)
		}
	}
	a := mobiusCurveFixture(t, false)
	pole := [4]*big.Rat{new(big.Rat), big.NewRat(1, 1), big.NewRat(1, 2), big.NewRat(-1, 1)}
	r, e := CompareComposedCurvesMobiusParameter(ctx, a, b, pole, ExactCurvePlaneEventOptions{})
	if e != ErrDomain || r.Proven {
		t.Fatal("pole", r, e)
	}
	// Identity map matches endpoints but is not the nonlinear parameter relation.
	r, e = CompareComposedCurvesMobiusParameter(ctx, a, b, [4]*big.Rat{new(big.Rat), big.NewRat(1, 1), big.NewRat(1, 1), new(big.Rat)}, ExactCurvePlaneEventOptions{})
	if e != nil || !r.Proven || r.Equivalent {
		t.Fatal(r, e)
	}
}
