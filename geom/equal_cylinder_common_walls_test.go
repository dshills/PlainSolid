package geom

import (
	"context"
	"math"
	"testing"
)

func TestEqualCylinderCommonWalls(t *testing.T) {
	center := Vec3{X: 1, Y: 2, Z: 3}
	u, v := Vec3{X: 3, Y: 4}, Vec3{X: 4, Y: 3}
	walls, e := NewEqualCylinderCommonWalls(context.Background(), center, u, v, 2, SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, w := range walls {
		owner, _, reverse := w.Ownership()
		axis, other := u, v
		if owner == 1 {
			axis, other = v, u
		}
		b, _ := w.HessianBounds()
		// Independent centered differences check both first and mixed jets.
		const h = 1e-5
		p0, _ := w.Eval(.2-h, .3)
		p1, _ := w.Eval(.2+h, .3)
		d0, _ := w.Eval(.2, .3-h)
		d1, _ := w.Eval(.2, .3+h)
		du, dv, _ := w.Partials(.2, .3)
		u0, v0, _ := w.Partials(.2-h, .3)
		u1, v1, _ := w.Partials(.2+h, .3)
		uu, uv, _, _ := w.SecondPartials(.2, .3)
		for _, delta := range []Vec3{p1.Sub(p0).Scale(1 / (2 * h)).Sub(du), d1.Sub(d0).Scale(1 / (2 * h)).Sub(dv), u1.Sub(u0).Scale(1 / (2 * h)).Sub(uu), v1.Sub(v0).Scale(1 / (2 * h)).Sub(uv)} {
			if delta.Length() > 1e-8 {
				t.Fatal("finite difference", delta)
			}
		}
		UU, _ := b.UU.Float64()
		UV, _ := b.UV.Float64()
		for _, phi := range []float64{-1, -.3, .6, 1.2} {
			for _, t0 := range []float64{-1, -.4, .7, 1} {
				p, e := w.Eval(phi, t0)
				a, c, _ := w.Partials(phi, t0)
				aa, ac, cc, _ := w.SecondPartials(phi, t0)
				x := p.Sub(center)
				normal := x.Sub(axis.Scale(x.Dot(axis) / axis.Dot(axis)))
				otherNormal := x.Sub(other.Scale(x.Dot(other) / other.Dot(other)))
				if e != nil || math.Abs(normal.Dot(normal)-4) > 1e-10 || otherNormal.Dot(otherNormal) > 4+1e-10 || aa.Length() > UU+1e-10 || ac.Length() > UV+1e-10 || cc != (Vec3{}) {
					t.Fatal(p, e)
				}
				n := a.Cross(c)
				if reverse {
					n = n.Scale(-1)
				}
				if n.Dot(normal) <= 0 {
					t.Fatal("orientation", owner, reverse)
				}
			}
		}
	}
	for _, phi := range []float64{-1, .2, 1} {
		for i, su := range []float64{-1, 1} {
			for j, sv := range []float64{-1, 1} {
				a, _ := walls[i].Eval(phi, sv)
				b, _ := walls[2+j].Eval(phi, su)
				if a != b {
					t.Fatal("seam", a, b)
				}
			}
		}
	}
	for _, phi := range []float64{-math.Pi / 2, math.Pi / 2} {
		first, _ := walls[0].Eval(phi, 0)
		for _, w := range walls {
			for _, t0 := range []float64{-1, 0, 1} {
				p, _ := w.Eval(phi, t0)
				_, dv, _ := w.Partials(phi, t0)
				if p != first || dv != (Vec3{}) {
					t.Fatal("pole", p, first, dv)
				}
			}
		}
	}
	if _, e := NewEqualCylinderCommonWalls(context.Background(), center, u, v, 2, SurfacePatchEvaluationOptions{MaxOperations: 1}); e == nil {
		t.Fatal("budget")
	}
}
