package geom

import (
	"context"
	"math"
	"testing"
)

func TestSphereToolCapPatch(t *testing.T) {
	for _, d := range []float64{0, .5, 1, 2.5} {
		p, e := NewSphereToolCapPatch(context.Background(), 5, 1, d, SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		upper, _ := NewSphereToolUpperPatch(context.Background(), 5, 1, d, SurfacePatchEvaluationOptions{})
		b, _ := p.HessianBounds()
		uu, _ := b.UU.Float64()
		uv, _ := b.UV.Float64()
		vv, _ := b.VV.Float64()
		for _, u := range []float64{0, .4, 1.8, math.Pi, 5.7} {
			for _, v := range []float64{0, .25, .8, 1} {
				x, e := p.Eval(u, v)
				if e != nil || math.Abs(x.Dot(x)-25) > 1e-12 {
					t.Fatal(x, e)
				}
				a, c, e := p.Partials(u, v)
				aa, ac, cc, ee := p.SecondPartials(u, v)
				if e != nil || ee != nil || aa.Length() > uu+1e-10 || ac.Length() > uv+1e-10 || cc.Length() > vv+1e-10 {
					t.Fatal(d, u, v, e, ee)
				}
				if v == 0 {
					if a != (Vec3{}) || aa != (Vec3{}) || x.X != d || x.Y != 0 || math.Abs(x.Z-math.Sqrt(25-d*d)) > 1e-12 {
						t.Fatal("collapse", x, a, aa)
					}
				} else if a.Cross(c).Dot(x) >= 0 {
					t.Fatal("orientation", x, a, c)
				}
				if v == 1 {
					rim, e := upper.Eval(u, 0)
					if e != nil || x.Distance(rim) > 1e-12 {
						t.Fatal("shared rim", x, rim, e)
					}
				}
				if v > 0 && v < 1 {
					eps := 1e-5
					am, cm, _ := p.Partials(u-eps, v)
					ap, cp, _ := p.Partials(u+eps, v)
					_, vm, _ := p.Partials(u, v-eps)
					_, vp, _ := p.Partials(u, v+eps)
					if ap.Sub(am).Scale(1/(2*eps)).Sub(aa).Length() > 1e-7 || cp.Sub(cm).Scale(1/(2*eps)).Sub(ac).Length() > 1e-7 || vp.Sub(vm).Scale(1/(2*eps)).Sub(cc).Length() > 1e-7 {
						t.Fatal("Hessian", d, u, v)
					}
				}
			}
		}
		b.UV.SetInt64(0)
		fresh, _ := p.HessianBounds()
		if fresh.UV.Sign() <= 0 {
			t.Fatal("mutable")
		}
	}
	if _, e := NewSphereToolCapPatch(context.Background(), 3, 1, 2, SurfacePatchEvaluationOptions{}); e != ErrDomain {
		t.Fatal(e)
	}
	if _, e := NewSphereToolCapPatch(context.Background(), 5, 1, 2, SurfacePatchEvaluationOptions{MaxOperations: 1}); e == nil {
		t.Fatal("budget")
	}
}
