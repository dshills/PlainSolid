package geom

import (
	"context"
	"math"
	"math/big"
	"testing"
)

func TestDualCircleExitChart(t *testing.T) {
	r := big.NewRat
	for _, pole := range []*big.Rat{r(1, 3), r(-1, 3), r(0, 1)} {
		p, e := NewDualCircleExitChart(context.Background(), r(2, 1), r(7, 3), pole, SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		bound, _ := p.HessianBounds()
		uu, _ := bound.UU.Float64()
		uv, _ := bound.UV.Float64()
		lift, e := p.SphereLiftBounds(context.Background(), r(4, 1), SurfacePatchEvaluationOptions{})
		if e != nil || lift.UU.Cmp(bound.UU) <= 0 || lift.VV.Sign() <= 0 {
			t.Fatal(lift, e)
		}
		for _, u := range []float64{0, .4, 1.5, 3.2, 5.7} {
			for _, v := range []float64{0, .3, 1} {
				x, e := p.Eval(u, v)
				if e != nil || x.Dot(x) < 2-1e-12 || x.Dot(x) > 7.0/3+1e-12 {
					t.Fatal(x, e)
				}
				if v == 0 && math.Abs(x.Dot(x)-2) > 1e-12 || v == 1 && math.Abs(x.Dot(x)-7.0/3) > 1e-12 {
					t.Fatal("circle", x)
				}
				a, b, e := p.Partials(u, v)
				aa, ab, bb, ee := p.SecondPartials(u, v)
				if e != nil || ee != nil || a.Cross(b).Z >= 0 || aa.Length() > uu+1e-12 || ab.Length() > uv+1e-12 || bb != (Vec3{}) {
					t.Fatal(a, b, aa, ab, bb, e, ee)
				}
				if v > 0 && v < 1 {
					h := 1e-5
					am, bm, _ := p.Partials(u-h, v)
					ap, bp, _ := p.Partials(u+h, v)
					if ap.Sub(am).Scale(1/(2*h)).Sub(aa).Length() > 1e-6 || bp.Sub(bm).Scale(1/(2*h)).Sub(ab).Length() > 1e-6 {
						t.Fatal("derivative")
					}
				}
			}
		}
		inner, _, offset, _ := p.Parameters()
		inner.SetInt64(99)
		offset.SetInt64(99)
		x, _, z, _ := p.Parameters()
		if x.Cmp(r(2, 1)) != 0 || z.Cmp(pole) != 0 {
			t.Fatal("mutable")
		}
	}
	if _, e := NewDualCircleExitChart(context.Background(), r(1, 9), r(1, 1), r(1, 3), SurfacePatchEvaluationOptions{}); e != ErrDomain {
		t.Fatal(e)
	}
	if _, e := NewDualCircleExitChart(context.Background(), r(2, 1), r(3, 1), r(1, 3), SurfacePatchEvaluationOptions{MaxOperations: 1}); e == nil {
		t.Fatal("budget")
	}
}
