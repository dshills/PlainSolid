package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func repeatedRationalArcFixture(t *testing.T, transpose bool, mode string) (*SurfacePatch, *SurfacePatch) {
	t.Helper()
	// Bernstein coefficients of ((5+2u)v-(2+u))², with an original
	// independently evaluated affine projection in either chart order.
	z := [3][3]float64{{4, -6, 9}, {6, -8.5, 12}, {9, -12, 16}}
	if mode == "vanishing leading term" {
		z = [3][3]float64{{}, {}, {1, -1, 1}}
	}
	points, plane := make([]Vec3, 0, 9), make([]Vec3, 0, 9)
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			u, v := i, j
			if transpose {
				u, v = v, u
			}
			x := Vec3{X: float64(u) / 2, Y: float64(v) / 2}
			plane = append(plane, x)
			x.Z = z[u][v]
			if mode == "changed interior" && i == 1 && j == 1 {
				x.Z += 1.0 / (1 << 42)
			}
			points = append(points, x)
		}
	}
	knots := []float64{0, 0, 0, 1, 1, 1}
	mustPatch := func(p []Vec3) *SurfacePatch {
		t.Helper()
		s, e := NewBSplineSurface(2, 2, 3, 3, p, knots, knots, nil)
		if e != nil {
			t.Fatal(e)
		}
		q, e := NewSurfacePatch(s, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return q
	}
	return mustPatch(points), mustPatch(plane)
}

func TestDiscoverSplineGraphRepeatedArc(t *testing.T) {
	ctx := context.Background()
	for dependent := 0; dependent < 2; dependent++ {
		a, b := repeatedRationalArcFixture(t, dependent == 0, "")
		q, e := DiscoverSplineGraphRepeatedArc(ctx, a, b, dependent, GraphPointContactOptions{})
		if e != nil || !q.Proven || !q.Projection.ProvenInjective || q.HeightSign != 1 || q.DependentAxis != dependent {
			t.Fatal(q, e)
		}
		for _, x := range []*big.Rat{new(big.Rat), big.NewRat(1, 2), big.NewRat(4, 7), big.NewRat(1, 1)} {
			n := new(big.Rat).Add(big.NewRat(2, 1), x)
			d := new(big.Rat).Add(big.NewRat(5, 1), new(big.Rat).Mul(big.NewRat(2, 1), x))
			p, e := q.UVCurve.patch.EvaluateRational(ctx, x, new(big.Rat), SurfacePatchEvaluationOptions{})
			if e != nil || p.Point[dependent].Cmp(new(big.Rat).Quo(n, d)) != 0 || p.Point[1-dependent].Cmp(x) != 0 {
				t.Fatal("exact nonlinear original UV correspondence", p, e)
			}
			y := p.Point[dependent]
			for _, epsilon := range []*big.Rat{big.NewRat(-1, 16), new(big.Rat), big.NewRat(1, 16)} {
				uv := [2]*big.Rat{}
				uv[dependent], uv[1-dependent] = new(big.Rat).Add(y, epsilon), x
				z, e := a.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{})
				want := new(big.Rat).Mul(d, epsilon)
				want.Mul(want, want)
				if e != nil || z.Point[2].Cmp(want) != 0 {
					t.Fatal("independent repeated contact and material side", z, want, e)
				}
			}
		}
		if q.ModelEndpoints[0][1].Cmp(big.NewRat(2, 5)) != 0 || q.ModelEndpoints[1][1].Cmp(big.NewRat(3, 7)) != 0 {
			t.Fatal("original model terminals", q.ModelEndpoints)
		}
		r, e := DiscoverSplineGraphRepeatedArc(ctx, b, a, dependent, GraphPointContactOptions{})
		if e != nil || !r.Proven || r.HeightSign != -1 {
			t.Fatal("exact side in reverse operand order", r, e)
		}
	}
}

func TestRepeatedSplineGraphRejectsDifferentContactSet(t *testing.T) {
	ctx := context.Background()
	for _, mode := range []string{"changed interior", "vanishing leading term"} {
		a, b := repeatedRationalArcFixture(t, false, mode)
		if r, e := DiscoverSplineGraphRepeatedArc(ctx, a, b, 1, GraphPointContactOptions{}); !errors.Is(e, ErrConvergence) || r.Proven || r.UVCurve != nil {
			t.Fatal("misclassified contact set", mode, r, e)
		}
	}
	a, b := repeatedRationalArcFixture(t, false, "")
	if r, e := DiscoverSplineGraphRepeatedArc(ctx, a, b, 1, GraphPointContactOptions{MaxOperations: 1}); !errors.Is(e, ErrConvergence) || r.Proven {
		t.Fatal("operation bound", r, e)
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := DiscoverSplineGraphRepeatedArc(cancelCtx, a, b, 1, GraphPointContactOptions{}); !errors.Is(e, context.Canceled) || r.Proven {
		t.Fatal("cancellation", r, e)
	}
}
