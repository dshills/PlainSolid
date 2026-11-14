package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestObliqueCavityExactTorusCharts(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	pose := [3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 10)}, {r(0, 1), r(1, 1), r(0, 1), r(1, 5)}, {r(-4, 5), r(0, 1), r(3, 5), r(1, 8)}}
	s, e := NewObliqueCylinderTorusCavityMaterial(ctx, 3, 1, 8, -6, 6, pose, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	eval := func(p *ExactBezierSurface, u, v *big.Rat) [3]*big.Rat {
		t.Helper()
		j, e := p.ExactPatch().EvaluateRational(ctx, u, v, SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return j.Point
	}
	same := func(a, b [3]*big.Rat) {
		t.Helper()
		for k := range a {
			if a[k].Cmp(b[k]) != 0 {
				t.Fatal("unequal exact seam", a, b)
			}
		}
	}
	for u := 0; u < 4; u++ {
		for v := 0; v < 4; v++ {
			p, e := s.TorusQuadrant(ctx, u, v, SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			pu, _ := s.TorusQuadrant(ctx, (u+1)%4, v, SurfacePatchOptions{})
			pv, _ := s.TorusQuadrant(ctx, u, (v+1)%4, SurfacePatchOptions{})
			for _, q := range []*big.Rat{r(0, 1), r(1, 3), r(1, 1)} {
				same(eval(p, r(1, 1), q), eval(pu, r(0, 1), q))
				same(eval(p, q, r(1, 1)), eval(pv, q, r(0, 1)))
				point := eval(p, q, r(2, 7))
				location, mask, e := s.ClassifyRational(ctx, point, SurfacePatchOptions{})
				if e != nil || location != BoreBoundary || mask != TorusCylinderTorus {
					t.Fatal(location, mask, e)
				}
			}
			recipe, e := p.ExactPatch().ExportExactBezierRecipe()
			if e != nil {
				t.Fatal(e)
			}
			reloaded, e := ImportExactBezierSurfaceRecipe(ctx, recipe, SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			j, e := reloaded.EvaluateRational(ctx, r(1, 3), r(2, 7), SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			same(eval(p, r(1, 3), r(2, 7)), j.Point)
		}
	}
	// Independent rational quarter-circle formula at t=1/3: cos=4/5,sin=3/5.
	p, _ := s.TorusQuadrant(ctx, 0, 0, SurfacePatchOptions{})
	local := [3]*big.Rat{r(76, 25), r(57, 25), r(3, 5)}
	var want [3]*big.Rat
	for i := range want {
		want[i] = new(big.Rat).Set(pose[i][3])
		for k := 0; k < 3; k++ {
			want[i].Add(want[i], new(big.Rat).Mul(pose[i][k], local[k]))
		}
	}
	same(eval(p, r(1, 3), r(1, 3)), want)
	// The same exact chart point is the t=0 root of the original cavity's
	// world-space line polynomial, without frame normalization or rounding.
	budget := &polynomialBudget{maxOperations: 100000, maxBits: 4096}
	polys, err := s.polynomials(ctx, want, [3]*big.Rat{r(1, 1), r(2, 1), r(-1, 1)}, budget)
	if err != nil || len(polys) == 0 || polys[0][0].Sign() != 0 {
		t.Fatal("line binding", err)
	}

	if _, e = s.TorusQuadrant(ctx, 4, 0, SurfacePatchOptions{}); e == nil {
		t.Fatal("quadrant")
	}
	if _, e = s.TorusQuadrant(ctx, 0, 0, SurfacePatchOptions{MaxOperations: 1}); e == nil {
		t.Fatal("budget")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = s.TorusQuadrant(canceled, 0, 0, SurfacePatchOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
