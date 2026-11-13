package geom

import (
	"context"
	"math"
	"testing"
)

func TestCylinderCommonWalls(t *testing.T) {
	for _, radii := range [][2]float64{{1, 2}, {2, 1}, {1, 1}, {1, 1.0001}} {
		center := Vec3{X: 1, Y: -2, Z: 3}
		axes := [2]Vec3{{X: 2}, {X: 3, Y: 4}}
		walls, e := NewCylinderCommonWalls(context.Background(), center, axes[0], axes[1], radii[0], radii[1], SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for _, w := range walls {
			owner, _, reverse := w.Ownership()
			bound, _ := w.HessianBounds()
			uu, _ := bound.UU.Float64()
			uv, _ := bound.UV.Float64()
			for _, phi := range []float64{-1.5, -.7, .1, .8, 1.5} {
				for _, v := range []float64{-1, -.3, .4, 1} {
					p, e := w.Eval(phi, v)
					du, dv, _ := w.Partials(phi, v)
					duu, duv, dvv, _ := w.SecondPartials(phi, v)
					x := p.Sub(center)
					if e != nil {
						t.Fatal(e)
					}
					for j, axis := range axes {
						n := x.Sub(axis.Scale(x.Dot(axis) / axis.Dot(axis)))
						d := n.Dot(n) - radii[j]*radii[j]
						if j == owner && math.Abs(d) > 1e-10 || j != owner && d > 1e-10 {
							t.Fatal("support", radii, owner, j, d)
						}
						if j == owner {
							normal := du.Cross(dv)
							if reverse {
								normal = normal.Scale(-1)
							}
							if normal.Dot(n) <= 0 {
								t.Fatal("orientation")
							}
						}
					}
					if duu.Length() > uu+1e-9 || duv.Length() > uv+1e-9 || dvv != (Vec3{}) {
						t.Fatal("bound")
					}
				}
			}
			const h = 1e-5
			before, _ := w.Eval(.2-h, .3)
			after, _ := w.Eval(.2+h, .3)
			du, _, _ := w.Partials(.2, .3)
			u0, v0, _ := w.Partials(.2-h, .3)
			u1, v1, _ := w.Partials(.2+h, .3)
			duu, duv, _, _ := w.SecondPartials(.2, .3)
			for _, d := range []Vec3{after.Sub(before).Scale(1 / (2 * h)).Sub(du), u1.Sub(u0).Scale(1 / (2 * h)).Sub(duu), v1.Sub(v0).Scale(1 / (2 * h)).Sub(duv)} {
				if d.Length() > 1e-8 {
					t.Fatal("jet", d)
				}
			}
		}
		for _, phi := range []float64{-math.Pi / 2, -.2, math.Pi / 2} {
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
		if radii[0] != radii[1] {
			smallOwner := 0
			if radii[1] < radii[0] {
				smallOwner = 1
			}
			for _, phi := range []float64{-math.Pi / 2, math.Pi / 2} {
				a, _ := walls[2*smallOwner].Eval(phi, .25)
				b, _ := walls[2*smallOwner+1].Eval(phi, .25)
				if a != b {
					t.Fatal("shared end line")
				}
				large := walls[2*(1-smallOwner)]
				a, _ = large.Eval(phi, -1)
				b, _ = large.Eval(phi, 1)
				if a != b {
					t.Fatal("collapsed large wall")
				}
			}
		}
	}
	if _, e := NewCylinderCommonWalls(context.Background(), Vec3{}, Vec3{X: 1}, Vec3{Y: 1}, 1, 2, SurfacePatchEvaluationOptions{MaxOperations: 1}); e == nil {
		t.Fatal("budget")
	}
}
