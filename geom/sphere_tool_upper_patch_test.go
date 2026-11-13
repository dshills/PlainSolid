package geom

import (
	"context"
	"math"
	"testing"
)

func TestSphereToolUpperPatch(t *testing.T) {
	for _, d := range []float64{0, .5, 1, 2.5} {
		p, e := NewSphereToolUpperPatch(context.Background(), 5, 1, d, SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		b, _ := p.HessianBounds()
		uu, _ := b.UU.Float64()
		uv, _ := b.UV.Float64()
		vv, _ := b.VV.Float64()
		eq, _ := NewSphereToolEquator(context.Background(), 5, d, SurfacePatchEvaluationOptions{})
		for _, u := range []float64{0, .3, 1.7, math.Pi, 5.4} {
			for _, v := range []float64{0, .2, .7, 1} {
				x, e := p.Eval(u, v)
				if e != nil || math.Abs(x.Dot(x)-25) > 1e-11 {
					t.Fatal(d, u, v, x, e)
				}
				a, c, e := p.Partials(u, v)
				if e != nil || a.Cross(c).Dot(x) >= 0 || math.Abs(a.Dot(x)) > 1e-10 || math.Abs(c.Dot(x)) > 1e-10 {
					t.Fatal(d, u, v, a, c, e)
				}
				aa, ac, cc, e := p.SecondPartials(u, v)
				if e != nil || aa.Length() > uu+1e-10 || ac.Length() > uv+1e-10 || cc.Length() > vv+1e-10 {
					t.Fatal(d, u, v, aa, ac, cc, e)
				}
				if v == 0 && math.Abs(math.Hypot(x.X-d, x.Y)-1) > 1e-12 {
					t.Fatal("rim", x)
				}
				if v == 1 {
					q, _, _, _, e := eq.Evaluate(u)
					if e != nil || x.Z != 0 || math.Hypot(x.X-q.X, x.Y-q.Y) > 1e-12 {
						t.Fatal("equator", x, q, e)
					}
				}
				if v > 0 && v < 1 {
					h := 1e-5
					am, cm, _ := p.Partials(u-h, v)
					ap, cp, _ := p.Partials(u+h, v)
					if ap.Sub(am).Scale(1/(2*h)).Sub(aa).Length() > 1e-6 || cp.Sub(cm).Scale(1/(2*h)).Sub(ac).Length() > 1e-6 {
						t.Fatal("angular Hessian", d, u, v)
					}
					_, vm, _ := p.Partials(u, v-h)
					_, vp, _ := p.Partials(u, v+h)
					if vp.Sub(vm).Scale(1/(2*h)).Sub(cc).Length() > 1e-6 {
						t.Fatal("radial Hessian", d, u, v)
					}
				}
			}
		}
		// Full endpoints remain regular; returned exact bounds cannot mutate source.
		b.UU.SetInt64(0)
		again, _ := p.HessianBounds()
		if again.UU.Sign() <= 0 {
			t.Fatal("mutable bounds")
		}
	}
	if _, e := NewSphereToolUpperPatch(context.Background(), 3, 1, 2, SurfacePatchEvaluationOptions{}); e != ErrDomain {
		t.Fatal(e)
	}
	if _, e := NewSphereToolUpperPatch(context.Background(), 5, 1, 2, SurfacePatchEvaluationOptions{MaxOperations: 1}); e == nil {
		t.Fatal("budget")
	}
}
