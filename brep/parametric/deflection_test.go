package parametric

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/geom"
)

func TestRationalSplineDeflectionBound(t *testing.T) {
	poles := []geom.Vec3{{}, {Y: 1}, {X: 1, Z: 1}, {X: 1, Y: 1, Z: 2}}
	k := []float64{0, 0, 1, 1}
	s, e := geom.NewBSplineSurface(1, 1, 2, 2, poles, k, k, []float64{1, .5, 1.5, 1})
	if e != nil {
		t.Fatal(e)
	}
	for _, x := range []geom.Surface{s, MapSurface(s, geom.Scaling(geom.Vec3{X: 2, Y: 3, Z: 4}))} {
		const n = 16
		for i := range n {
			for j := range n {
				u, v := Interval{float64(i) / n, float64(i+1) / n}, Interval{float64(j) / n, float64(j+1) / n}
				b, e := SurfaceDeflectionBound(x, u, v)
				if e != nil || b <= 0 {
					t.Fatal(b, e)
				}
				a, _ := x.Eval(u.Min, v.Min)
				c, _ := x.Eval(u.Max, v.Min)
				d, _ := x.Eval(u.Max, v.Max)
				for k := 1; k < 9; k++ {
					f := float64(k) / 10
					for l := 1; l < 10-k; l++ {
						g := float64(l) / 10
						q := a.Scale(1 - f - g).Add(c.Scale(f)).Add(d.Scale(g))
						p, _ := x.Eval(u.at(f+g), v.at(g))
						if p.Distance(q) > b {
							t.Fatal(p.Distance(q), b)
						}
					}
				}
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = SurfaceDeflectionBoundContext(ctx, s, Interval{0, 1}, Interval{0, 1}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

func TestSplineDeflectionRejectsInteriorDiscontinuity(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	c, _ := geom.NewEllipse(f, 2, 1)
	p, _ := c.RationalBSpline()
	s, e := ExtrudeSpline(p, geom.Vec3{}, f.ZAxis(), geom.Vec3{Z: 2}, 1e-10)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DeflectionBound(8); !errors.Is(e, ErrDeflectionUnsupported) {
		t.Fatal(e)
	}
	a := s.Faces()[0].Surface
	b, e := SurfaceDeflectionBound(a, Interval{0, 1}, Interval{0, 1})
	if e != nil || math.IsInf(b, 0) || b <= 0 {
		t.Fatal(b, e)
	}
}
