package geom

import (
	"math"
	"testing"
)

func TestHelicalProfileSurface(t *testing.T) {
	f, _ := NewFrame(Vec3{2, -1, 4}, Vec3{0, 1, 0}, Vec3{1, 0, 0})
	for _, hand := range []HelixHandedness{RightHanded, LeftHanded} {
		for _, end := range []Vec2{{3, 1}, {3, 0}, {2, 1}} {
			s, e := NewHelicalProfileSurface(f, Vec2{2, 0}, end, 4, hand, -2, 3)
			if e != nil {
				t.Fatal(e)
			}
			for _, u := range []float64{-.3, 0, .25, .7} {
				for _, v := range []float64{0, .2, .8, 1} {
					p, e := s.Eval(u, v)
					if e != nil {
						t.Fatal(e)
					}
					local := f.ToLocal(p)
					radius := 2 + (end.X-2)*v
					if math.Abs(math.Hypot(local.X, local.Y)-radius) > 1e-12 || math.Abs(local.Z-(4*u+end.Y*v)) > 1e-12 {
						t.Fatal(local)
					}
					du, dv, e := s.Partials(u, v)
					if e != nil {
						t.Fatal(e)
					}
					uu, uv, vv, e := s.SecondPartials(u, v)
					if e != nil || vv != (Vec3{}) || du.Cross(dv).Length() == 0 {
						t.Fatal(uu, uv, vv, e)
					}
					eps := 1e-6
					a, _ := s.Eval(u-eps, v)
					b, _ := s.Eval(u+eps, v)
					if b.Sub(a).Scale(.5/eps).Sub(du).Length() > 1e-7 {
						t.Fatal("u derivative")
					}
					da, va, _ := s.Partials(u-eps, v)
					db, vb, _ := s.Partials(u+eps, v)
					if db.Sub(da).Scale(.5/eps).Sub(uu).Length() > 1e-6 || vb.Sub(va).Scale(.5/eps).Sub(uv).Length() > 1e-7 {
						t.Fatal("second partials")
					}
					if v > 0 && v < 1 {
						a, _ = s.Eval(u, v-eps)
						b, _ = s.Eval(u, v+eps)
						if b.Sub(a).Scale(.5/eps).Sub(dv).Length() > 1e-7 {
							t.Fatal("v derivative")
						}
					}
					if v == 0 {
						h, _ := NewHelix(f, 2, 4, hand)
						hp, _ := h.Eval(u)
						if hp.Sub(p).Length() > 1e-12 {
							t.Fatal("boundary helix")
						}
					}
				}
			}
			rect := ParameterRectangle{-.4, .8, .1, .9}
			box, e := s.Bounds(rect)
			if e != nil {
				t.Fatal(e)
			}
			bu, buv, bv, e := s.SecondDerivativeBounds(rect)
			if e != nil || bv != 0 {
				t.Fatal(e)
			}
			for i := 0; i <= 20; i++ {
				for j := 0; j <= 20; j++ {
					u, v := -.4+1.2*float64(i)/20, .1+.8*float64(j)/20
					p, _ := s.Eval(u, v)
					if p.X < box.Min.X-1e-14 || p.X > box.Max.X+1e-14 || p.Y < box.Min.Y-1e-14 || p.Y > box.Max.Y+1e-14 || p.Z < box.Min.Z-1e-14 || p.Z > box.Max.Z+1e-14 {
						t.Fatal(p, box)
					}
					uu, uv, _, _ := s.SecondPartials(u, v)
					if uu.Length() > bu || uv.Length() > buv {
						t.Fatal("derivative bound")
					}
				}
			}
		}
	}
}
func TestHelicalProfileSurfaceInvalid(t *testing.T) {
	f, _ := NewFrame(Vec3{}, Vec3{1, 0, 0}, Vec3{0, 0, 1})
	if _, e := NewHelicalProfileSurface(f, Vec2{1, 0}, Vec2{1, 0}, 1, RightHanded, 0, 1); e == nil {
		t.Fatal("degenerate profile")
	}
	if _, e := NewHelicalProfileSurface(f, Vec2{1, 0}, Vec2{0, 1}, 1, RightHanded, 0, 1); e == nil {
		t.Fatal("axis endpoint")
	}
	s, _ := NewHelicalProfileSurface(f, Vec2{1, 0}, Vec2{2, 1}, 1, RightHanded, 0, 1)
	if _, e := s.Eval(2, .5); e == nil {
		t.Fatal("domain")
	}
	if _, e := s.Bounds(ParameterRectangle{0, 1, -.1, 1}); e == nil {
		t.Fatal("bounds domain")
	}
	if _, _, _, e := s.SecondDerivativeBounds(ParameterRectangle{0, 1, 0, 2}); e == nil {
		t.Fatal("derivative bounds domain")
	}
	if _, _, e := (HelicalProfileSurface{}).Partials(0, 0); e == nil {
		t.Fatal("zero")
	}
}
