package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func separableContactFixture(t *testing.T, up, vp rationalPolynomial) (*SurfacePatch, *SurfacePatch) {
	a, c := graphContactFixture(t, false, false)
	choose := func(n, k int) int64 {
		x := int64(1)
		for i := 1; i <= k; i++ {
			x = x * int64(n-i+1) / int64(i)
		}
		return x
	}
	bern := func(i int, p rationalPolynomial) *big.Rat {
		x := new(big.Rat)
		for k, v := range p {
			if k <= i {
				x.Add(x, new(big.Rat).Mul(v, big.NewRat(choose(i, k), choose(4, k))))
			}
		}
		return x
	}
	for i := 0; i < 5; i++ {
		for j := 0; j < 5; j++ {
			k := 5*i + j
			a.net[k][2] = new(big.Rat).Add(c.net[k][2], new(big.Rat).Add(bern(i, up), bern(j, vp)))
		}
	}
	if e := a.computeBounds(&polynomialBudget{maxOperations: 100000, maxBits: 16384}); e != nil {
		t.Fatal(e)
	}
	return a, c
}
func rootPower(t *testing.T, root *big.Rat, n int) rationalPolynomial {
	t.Helper()
	p := rationalPolynomial{big.NewRat(1, 1)}
	b := &polynomialBudget{maxOperations: 100000, maxBits: 16384}
	for i := 0; i < n; i++ {
		var e error
		p, e = polynomialProduct(p, rationalPolynomial{new(big.Rat).Neg(root), big.NewRat(1, 1)}, b)
		if e != nil {
			t.Fatal(e)
		}
	}
	return p
}
func TestSeparableGraphQuarticDegenerateContact(t *testing.T) {
	ctx := context.Background()
	u, v := big.NewRat(1, 3), big.NewRat(2, 5)
	a, c := separableContactFixture(t, rootPower(t, u, 4), rootPower(t, v, 4))
	uv := [2]*big.Rat{u, v}
	r, e := CertifySeparableSplineGraphPointContact(ctx, a, c, uv, SeparableGraphContactOptions{})
	if e != nil || !r.Proven || r.Side != 1 || r.Orders != ([2]int{4, 4}) {
		t.Fatal(r, e)
	}
	r, e = CertifySeparableSplineGraphPointContact(ctx, c, a, uv, SeparableGraphContactOptions{})
	if e != nil || !r.Proven || r.Side != -1 {
		t.Fatal(r, e)
	}
	// The previous strictly definite Hessian criterion must not fabricate proof
	// at a zero-curvature contact, even though the isolated zero is now proved.
	old, e := CertifySplineGraphPointContact(ctx, a, c, uv, GraphPointContactOptions{MaxDepth: 2})
	if !errors.Is(e, ErrConvergence) || old.Proven {
		t.Fatal(old, e)
	}
}
func TestSeparableGraphRejectsSaddleExtraRootAndLine(t *testing.T) {
	u, v := big.NewRat(1, 4), big.NewRat(1, 2)
	up, vp := rootPower(t, u, 4), rootPower(t, v, 4)
	negative := clonePolynomial(vp)
	for _, x := range negative {
		x.Neg(x)
	}
	extra, e := polynomialProduct(rootPower(t, u, 2), rootPower(t, big.NewRat(3, 4), 2), &polynomialBudget{maxOperations: 100000, maxBits: 16384})
	if e != nil {
		t.Fatal(e)
	}
	for _, pair := range [][2]rationalPolynomial{{up, negative}, {extra, vp}, {nil, vp}} {
		a, c := separableContactFixture(t, pair[0], pair[1])
		r, e := CertifySeparableSplineGraphPointContact(context.Background(), a, c, [2]*big.Rat{u, v}, SeparableGraphContactOptions{})
		if !errors.Is(e, ErrConvergence) || r.Proven {
			t.Fatal(r, e)
		}
	}
}
func TestSeparableGraphBoundaryAndLimits(t *testing.T) {
	a, c := separableContactFixture(t, rootPower(t, new(big.Rat), 3), rootPower(t, big.NewRat(1, 1), 4))
	uv := [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}
	r, e := CertifySeparableSplineGraphPointContact(context.Background(), a, c, uv, SeparableGraphContactOptions{})
	if e != nil || !r.Proven || r.Orders != ([2]int{3, 4}) {
		t.Fatal(r, e)
	}
	_, e = CertifySeparableSplineGraphPointContact(context.Background(), a, c, uv, SeparableGraphContactOptions{GraphPointContactOptions: GraphPointContactOptions{MaxOperations: 1}})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = CertifySeparableSplineGraphPointContact(canceled, a, c, uv, SeparableGraphContactOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
