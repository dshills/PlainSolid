package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestExactCurveAlgebraicPlaneEvent(t *testing.T) {
	// C(t)=(t,t²,t²): t=sqrt(1/2) is not rational, yet z=y exactly.
	s, _ := NewBezier([]Vec3{{}, {X: .5}, {X: 1, Y: 1, Z: 1}}, nil)
	c := isoIntersectionFixture(t, s)
	event := [4]*big.Rat{nil, big.NewRat(1, 1), nil, big.NewRat(-1, 2)}
	lo, hi := big.NewRat(1, 2), big.NewRat(1, 1)
	for _, tc := range []struct {
		name string
		p    [4]*big.Rat
		sign int
	}{
		{"shared algebraic event", [4]*big.Rat{nil, nil, big.NewRat(3, 1), big.NewRat(-3, 2)}, 0},
		{"positive", [4]*big.Rat{big.NewRat(1, 1), nil, nil, big.NewRat(-2, 3)}, 1},
		{"negative", [4]*big.Rat{big.NewRat(1, 1), nil, nil, big.NewRat(-3, 4)}, -1},
		{"coincident test plane", [4]*big.Rat{nil, big.NewRat(1, 1), big.NewRat(-1, 1), nil}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, e := ClassifyExactCurvePlaneEvent(context.Background(), c, event, tc.p, lo, hi, ExactCurvePlaneEventOptions{})
			if e != nil || !r.Proven || r.Side != tc.sign {
				t.Fatal(r, e)
			}
			r.Lower.SetInt64(9)
			if lo.Cmp(big.NewRat(1, 2)) != 0 {
				t.Fatal("aliased bounds")
			}
		})
	}
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := ClassifyExactCurvePlaneEvent(cancelCtx, c, event, event, lo, hi, ExactCurvePlaneEventOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e := ClassifyExactCurvePlaneEvent(context.Background(), c, event, event, lo, hi, ExactCurvePlaneEventOptions{MaxOperations: 1}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
}
func TestExactCurvePlaneEventRequiresUniqueRoot(t *testing.T) {
	s, _ := NewBezier([]Vec3{{X: 1}, {X: -1}, {X: 1}}, nil)
	c := isoIntersectionFixture(t, s)
	event := [4]*big.Rat{big.NewRat(1, 1), nil, nil, big.NewRat(-1, 2)}
	if _, e := ClassifyExactCurvePlaneEvent(context.Background(), c, event, event, big.NewRat(0, 1), big.NewRat(1, 1), ExactCurvePlaneEventOptions{}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	// Repeated root t=1/2 is one distinct event, not two.
	event[3] = nil
	r, e := ClassifyExactCurvePlaneEvent(context.Background(), c, event, event, big.NewRat(0, 1), big.NewRat(1, 1), ExactCurvePlaneEventOptions{})
	if e != nil || !r.Proven || r.Side != 0 {
		t.Fatal(r, e)
	}
}

func TestExactWeightedCurvePlaneEvent(t *testing.T) {
	s, _ := NewBezier([]Vec3{{}, {X: .5}, {X: 1, Y: 1, Z: 1}}, []float64{1, 2, 1})
	c := isoIntersectionFixture(t, s)
	event := [4]*big.Rat{nil, big.NewRat(1, 1), nil, big.NewRat(-1, 3)}
	test := [4]*big.Rat{nil, nil, big.NewRat(7, 1), big.NewRat(-7, 3)}
	r, e := ClassifyExactCurvePlaneEvent(context.Background(), c, event, test, big.NewRat(0, 1), big.NewRat(1, 1), ExactCurvePlaneEventOptions{})
	if e != nil || !r.Proven || r.Side != 0 {
		t.Fatal(r, e)
	}
	// An arbitrarily small rational plane offset is not incidence.
	test[3].Add(test[3], new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 80)))
	r, e = ClassifyExactCurvePlaneEvent(context.Background(), c, event, test, big.NewRat(0, 1), big.NewRat(1, 1), ExactCurvePlaneEventOptions{})
	if e != nil || !r.Proven || r.Side != 1 {
		t.Fatal(r, e)
	}
}
