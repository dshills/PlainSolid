package geom

import (
	"context"
	"math"
	"math/big"
	"testing"
)

func TestSphereCylinderLensFans(t *testing.T) {
	r := big.NewRat
	for _, axis := range []Vec3{{Z: 1}, {Z: -2}} {
		fans, e := NewSphereCylinderLensFans(context.Background(), Vec3{X: 1}, 1, Vec3{}, axis, 1, r(0, 1), SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for _, v := range []float64{0, .2, .75, 1} {
			a, _ := fans[0].Eval(0, v)
			b, _ := fans[1].Eval(1, v)
			c, _ := fans[0].Eval(1, v)
			d, _ := fans[1].Eval(0, v)
			if a != b || c != d {
				t.Fatal("seam", a, b, c, d)
			}
		}
		area := 0.0
		for n, p := range fans {
			owner, hub, events, e := p.BoundaryProvenance()
			if e != nil || int(owner) != n || hub[0].Cmp(r(1, 2)) != 0 || events[0].RootSign == events[1].RootSign {
				t.Fatal(owner, hub, events, e)
			}
			b, _ := p.HessianBounds()
			uu, _ := b.UU.Float64()
			uv, _ := b.UV.Float64()
			for _, u := range []float64{0, .2, .6, 1} {
				for _, v := range []float64{0, .3, 1} {
					x, e := p.Eval(u, v)
					if e != nil || x.X*x.X+x.Y*x.Y > 1+1e-12 || (x.X-1)*(x.X-1)+x.Y*x.Y > 1+1e-12 {
						t.Fatal(x, e)
					}
					a, c, e := p.Partials(u, v)
					aa, ac, cc, ee := p.SecondPartials(u, v)
					if e != nil || ee != nil || aa.Length() > uu+1e-12 || ac.Length() > uv+1e-12 || cc != (Vec3{}) {
						t.Fatal(aa, ac, cc, e, ee)
					}
					if v > 0 && a.Cross(c).Dot(axis) >= 0 {
						t.Fatal("orientation", a, c)
					}
					if v == 0 && a != (Vec3{}) {
						t.Fatal("collapse", a)
					}
				}
			}
			const count = 2000
			for i := 0; i < count; i++ {
				u := (float64(i) + .5) / count
				a, c, _ := p.Partials(u, .5)
				area += a.Cross(c).Length() / count
			}
			hub[0].SetInt64(99)
			_, again, _, _ := p.BoundaryProvenance()
			if again[0].Cmp(r(1, 2)) != 0 {
				t.Fatal("mutable")
			}
		}
		want := 2*math.Pi/3 - math.Sqrt(3)/2
		if math.Abs(area-want) > 1e-6 {
			t.Fatal(area, want)
		}
	}
	if _, e := NewSphereCylinderLensFans(context.Background(), Vec3{}, 1, Vec3{}, Vec3{Z: 1}, 1, r(0, 1), SurfacePatchEvaluationOptions{}); e != ErrDomain {
		t.Fatal(e)
	}
}
