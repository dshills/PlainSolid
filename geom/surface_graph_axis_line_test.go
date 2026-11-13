package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func rationalAxisLineGraphs(t *testing.T, axis int) (*SurfacePatch, *SurfacePatch) {
	t.Helper()
	var poles []Vec3
	for _, u := range []float64{0, 1} {
		for _, v := range []float64{0, 1} {
			coordinate := u
			if axis == 1 {
				coordinate = v
			}
			poles = append(poles, Vec3{u, v, u*v + 3*coordinate - 1})
		}
	}
	sa, _ := NewBSplineSurface(1, 1, 2, 2, poles, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	a, _ := NewSurfacePatch(sa, SurfacePatchOptions{})
	sb, _ := NewBSplineSurface(2, 2, 3, 3, make([]Vec3, 9), []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
	b, _ := NewSurfacePatch(sb, SurfacePatchOptions{})
	weights := []*big.Rat{big.NewRat(1, 1), big.NewRat(3, 2), big.NewRat(2, 1)}
	products := []*big.Rat{new(big.Rat), big.NewRat(1, 2), big.NewRat(2, 1)}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			wu, wv := big.NewRat(1, 1), big.NewRat(1, 1)
			xu, yv := big.NewRat(int64(i), 2), big.NewRat(int64(j), 2)
			if axis == 0 {
				wv = weights[j]
				yv = products[j]
			} else {
				wu = weights[i]
				xu = products[i]
			}
			b.net[i*3+j] = exactHomogeneous{new(big.Rat).Mul(xu, wv), new(big.Rat).Mul(wu, yv), new(big.Rat).Mul(xu, yv), new(big.Rat).Mul(wu, wv)}
		}
	}
	if e := b.computeBounds(&polynomialBudget{maxOperations: 100000, maxBits: 16384}); e != nil {
		t.Fatal(e)
	}
	a, _ = a.InsertUKnot(.5, 1, SurfacePatchOptions{})
	b, _ = b.InsertVKnot(.25, 1, SurfacePatchOptions{})
	return a, b
}
func TestExactNonKnotRationalGraphAxisLine(t *testing.T) {
	for axis := 0; axis < 2; axis++ {
		a, b := rationalAxisLineGraphs(t, axis)
		for swap, pair := range [2][2]*SurfacePatch{{a, b}, {b, a}} {
			r, e := CertifySplineGraphAxisLine(context.Background(), pair[0], pair[1], axis, GraphPointContactOptions{})
			sign := 1
			if swap == 1 {
				sign = -1
			}
			if e != nil || !r.Proven || r.OnKnot || r.Coordinate.Cmp(big.NewRat(1, 3)) != 0 || r.LowerSide != -sign || r.UpperSide != sign || len(r.CertifiedCells) != 4 {
				t.Fatal(r, e)
			}
			for _, q := range []*big.Rat{big.NewRat(1, 4), big.NewRat(1, 3), big.NewRat(2, 3)} {
				uv := [2]*big.Rat{big.NewRat(2, 5), big.NewRat(2, 5)}
				uv[axis] = q
				x, e := pair[0].EvaluateRational(context.Background(), uv[0], uv[1], SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				y, e := pair[1].EvaluateRational(context.Background(), uv[0], uv[1], SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				if new(big.Rat).Sub(x.Point[2], y.Point[2]).Sign() != sign*q.Cmp(big.NewRat(1, 3)) {
					t.Fatal("independent exact side")
				}
			}
		}
	}
}
func TestGraphAxisLineRejectsVaryingRootAndNonlinear(t *testing.T) {
	a, b := rationalAxisLineGraphs(t, 0)
	// Tiny perturbation proportional to v bends the zero set off a fixed U.
	tiny := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 80))
	for i := range a.net {
		a.net[i][2] = new(big.Rat).Add(a.net[i][2], new(big.Rat).Mul(tiny, a.net[i][1]))
	}
	r, e := CertifySplineGraphAxisLine(context.Background(), a, b, 0, GraphPointContactOptions{})
	if !errors.Is(e, ErrConvergence) || r.Proven {
		t.Fatal(r, e)
	}
	a, b = graphContactFixture(t, false, true)
	r, e = CertifySplineGraphAxisLine(context.Background(), a, b, 0, GraphPointContactOptions{})
	if !errors.Is(e, ErrConvergence) || r.Proven {
		t.Fatal(r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = CertifySplineGraphAxisLine(ctx, a, b, 0, GraphPointContactOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	r, e = CertifySplineGraphAxisLine(context.Background(), a, b, 0, GraphPointContactOptions{MaxOperations: 1})
	if e == nil || r.Proven {
		t.Fatal(r, e)
	}
}
