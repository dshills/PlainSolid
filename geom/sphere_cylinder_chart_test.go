package geom

import (
	"context"
	"math"
	"math/big"
	"testing"
)

func TestSphereCylinderWallCharts(t *testing.T) {
	r := big.NewRat
	ctx := context.Background()
	charts, e := NewSphereCylinderWallCharts(ctx, Vec3{X: 2}, 2, Vec3{}, Vec3{Z: 1}, 1, [2]*big.Rat{r(-2, 1), r(2, 1)}, SurfacePatchEvaluationOptions{})
	if e != nil || len(charts) != 4 {
		t.Fatal(len(charts), e)
	}
	for _, c := range charts {
		for _, u := range []float64{.1, .3, .7, .9} {
			p, e := c.Eval(u)
			if e != nil {
				t.Fatal(e)
			}
			if math.Abs(p.X*p.X+p.Y*p.Y-1) > 1e-12 || math.Abs((p.X-2)*(p.X-2)+p.Y*p.Y+p.Z*p.Z-4) > 1e-12 {
				t.Fatal(p)
			}
			d, e := c.Derivative(u)
			if e != nil {
				t.Fatal(e)
			}
			if math.Abs(p.X*d.X+p.Y*d.Y) > 1e-11 || math.Abs((p.X-2)*d.X+p.Y*d.Y+p.Z*d.Z) > 1e-11 || d.X <= 0 {
				t.Fatal(p, d)
			}
			eps := 1e-6
			a, _ := c.Eval(u - eps)
			b, _ := c.Eval(u + eps)
			fd := b.Sub(a).Scale(1 / (2 * eps))
			if d.Sub(fd).Length() > 1e-6 {
				t.Fatal(d, fd)
			}
		}
		for i := 0; i < 2; i++ {
			bounds, e := c.EndpointBounds(ctx, i, SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			event, _ := c.EndpointEvent(i)
			if event.HeightSign == 0 && (bounds[2][0].Sign() != 0 || bounds[2][1].Sign() != 0) {
				t.Fatal(event, bounds)
			}
			if event.ProjectionExtreme != 0 && bounds[1][0].Sign() != 0 {
				t.Fatal(event, bounds)
			}
			if event.ProjectionValue != nil {
				event.ProjectionValue.SetInt64(999)
				again, _ := c.EndpointEvent(i)
				if again.ProjectionValue.Cmp(r(999, 1)) == 0 {
					t.Fatal("mutable provenance")
				}
			}
		}
		if _, e = c.Derivative(0); e == nil {
			t.Fatal("singular endpoint parameter")
		}
	}
	caps, e := NewSphereCylinderWallCharts(ctx, Vec3{X: 2}, 2, Vec3{}, Vec3{Z: -3}, 1, [2]*big.Rat{r(-1, 3), r(-1, 6)}, SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, c := range caps {
		for i := 0; i < 2; i++ {
			ev, _ := c.EndpointEvent(i)
			bounds, e := c.EndpointBounds(ctx, i, SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			want := r(1, 2)
			if ev.CapMask == 1 {
				want = r(1, 1)
			} else if ev.CapMask != 2 {
				t.Fatal(ev)
			}
			if bounds[2][0].Cmp(want) != 0 || bounds[2][1].Cmp(want) != 0 {
				t.Fatal(ev, bounds)
			}
		}
	}
}
