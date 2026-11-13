package geom

import (
	"context"
	"math/big"
	"testing"
)

func planeContactPolynomialCurve(t *testing.T, coeff []*big.Rat, singular bool) *ExactSplineCurve {
	t.Helper()
	n := len(coeff) - 1
	s, e := NewBezier(make([]Vec3, n+1), nil)
	if e != nil {
		t.Fatal(e)
	}
	c := isoIntersectionFixture(t, s)
	for i := 0; i <= n; i++ {
		y := new(big.Rat)
		for k := 0; k <= i; k++ {
			ratio := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(k)), new(big.Int).Binomial(int64(n), int64(k)))
			y.Add(y, new(big.Rat).Mul(coeff[k], ratio))
		}
		for j := 0; j < c.patch.countV; j++ {
			h := &c.patch.net[i*c.patch.countV+j]
			h[0] = big.NewRat(int64(i), int64(n))
			if singular {
				h[0] = new(big.Rat).Set(y)
			}
			h[1] = new(big.Rat).Set(y)
		}
	}
	return c
}
func TestAlgebraicPlaneContactRegularAndSingular(t *testing.T) {
	plane := [4]*big.Rat{nil, big.NewRat(1, 1), nil, nil}
	cases := []struct {
		name     string
		coeff    []*big.Rat
		singular bool
		order    int
	}{
		{"transverse", []*big.Rat{big.NewRat(-1, 2), new(big.Rat), big.NewRat(1, 1)}, false, 1},
		{"regular tangent", []*big.Rat{big.NewRat(1, 4), new(big.Rat), big.NewRat(-1, 1), new(big.Rat), big.NewRat(1, 1)}, false, 2},
		{"singular touch", []*big.Rat{big.NewRat(1, 4), new(big.Rat), big.NewRat(-1, 1), new(big.Rat), big.NewRat(1, 1)}, true, 2},
		{"inflection tangent", []*big.Rat{big.NewRat(-1, 8), new(big.Rat), big.NewRat(3, 4), new(big.Rat), big.NewRat(-3, 2), new(big.Rat), big.NewRat(1, 1)}, false, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := planeContactPolynomialCurve(t, tc.coeff, tc.singular)
			r, e := ClassifyExactCurvePlaneContact(context.Background(), c, plane, big.NewRat(1, 2), big.NewRat(1, 1), ExactCurvePlaneEventOptions{})
			if e != nil || !r.Proven || r.Order != tc.order || r.Singular != tc.singular || r.Transverse != (tc.order == 1) || r.Tangent != (!tc.singular && tc.order > 1) || r.Crossing != (tc.order%2 == 1) || r.Touching != (tc.order%2 == 0) || r.Boundary {
				t.Fatal(r, e)
			}
		})
	}
	c := planeContactPolynomialCurve(t, []*big.Rat{new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, false)
	r, e := ClassifyExactCurvePlaneContact(context.Background(), c, plane, new(big.Rat), big.NewRat(1, 1), ExactCurvePlaneEventOptions{})
	if e != nil || !r.Proven || !r.Boundary || !r.Tangent || r.Crossing || r.Touching {
		t.Fatal(r, e)
	}
}
func TestAlgebraicPlaneTangencyNearSplitAndLimits(t *testing.T) {
	c := planeContactPolynomialCurve(t, []*big.Rat{big.NewRat(1, 4), new(big.Rat), big.NewRat(-1, 1), new(big.Rat), big.NewRat(1, 1)}, false)
	epsilon := new(big.Rat).SetFrac(big.NewInt(-1), new(big.Int).Lsh(big.NewInt(1), 80))
	plane := [4]*big.Rat{nil, big.NewRat(1, 1), nil, epsilon}
	r, e := ClassifyExactCurvePlaneContact(context.Background(), c, plane, big.NewRat(1, 2), big.NewRat(1, 1), ExactCurvePlaneEventOptions{})
	if e == nil || r.Proven {
		t.Fatal("nearby plane has two distinct contacts", r, e)
	}
	plane[3] = nil
	r, e = ClassifyExactCurvePlaneContact(context.Background(), c, plane, big.NewRat(1, 2), big.NewRat(1, 1), ExactCurvePlaneEventOptions{MaxOperations: 1})
	if e == nil || r.Proven {
		t.Fatal(r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, e = ClassifyExactCurvePlaneContact(ctx, c, plane, big.NewRat(1, 2), big.NewRat(1, 1), ExactCurvePlaneEventOptions{})
	if e != context.Canceled || r.Proven {
		t.Fatal(r, e)
	}
}
