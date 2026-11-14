package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
)

func TestCertifiedCurveDistanceLineAndEndpoints(t *testing.T) {
	c, _ := NewBezier([]Vec3{{}, {X: 1}}, nil)
	for _, tc := range []struct {
		query    Vec3
		distance float64
	}{{Vec3{.3, 2, 0}, 4}, {Vec3{2, 0, 0}, 1}, {Vec3{.3, 0, 0}, 0}} {
		r, e := BoundPointCurveDistance(tc.query, c, CurveDistanceOptions{SquaredTolerance: 1e-7})
		if e != nil || !r.Complete || !r.HasWitness || r.SquaredDistanceMin > tc.distance || r.SquaredDistanceMax < tc.distance || r.SquaredDistanceMax-r.SquaredDistanceMin > 1.000001e-7 {
			t.Fatal(tc, r, e)
		}
	}
}
func TestCertifiedCurveDistanceRationalCircleKnots(t *testing.T) {
	circle, _ := NewCircle(Vec3{}, Vec3{Z: 1}, 2)
	c, _ := circle.RationalBSpline()
	c, _ = c.InsertKnot(.13, 1)
	r, e := BoundPointCurveDistance(Vec3{3, 3, 1}, c, CurveDistanceOptions{SquaredTolerance: 1e-3})
	distance := (math.Sqrt(18)-2)*(math.Sqrt(18)-2) + 1
	if e != nil || !r.Complete || r.SquaredDistanceMin > distance+1e-12 || r.SquaredDistanceMax < distance-1e-12 {
		t.Fatal(r, e)
	}
}
func TestCertifiedCurveDistanceFailureBounds(t *testing.T) {
	c, _ := NewBezier([]Vec3{{}, {1, 1, 0}, {2, 0, 0}}, nil)
	r, e := BoundPointCurveDistance(Vec3{.3, .2, 1}, c, CurveDistanceOptions{MaxCells: 1})
	if !errors.Is(e, ErrConvergence) || r.Complete || r.SquaredDistanceMin > r.SquaredDistanceMax {
		t.Fatal(r, e)
	}
	c, _ = NewBSpline(1, []Vec3{{}, {X: 1}, {X: 2}, {X: 3}}, []float64{0, 0, .5, .5, 1, 1}, nil)
	if _, e = BoundPointCurveDistance(Vec3{}, c, CurveDistanceOptions{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}

func TestExactRationalCurveDistanceUnrounded(t *testing.T) {
	ctx := context.Background()
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	iso, e := p.IsoCurve(1, 0, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	eps := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 120))
	query := [3]*big.Rat{new(big.Rat).Add(big.NewRat(1, 1), eps), new(big.Rat), new(big.Rat)}
	r, e := BoundPointExactCurveDistanceRational(ctx, query, iso, CurveDistanceOptions{SquaredTolerance: 1e-90})
	want := new(big.Rat).Mul(eps, eps)
	if e != nil || !r.Complete || !r.HasWitness || r.Parameter != 1 || r.Squared.Lower.Cmp(want) != 0 || r.Squared.Upper.Cmp(want) != 0 || r.Witness[0].Cmp(big.NewRat(1, 1)) != 0 || r.Witness[1].Sign() != 0 {
		t.Fatal("exact rational endpoint distance to original iso curve", r, e, want)
	}
}
