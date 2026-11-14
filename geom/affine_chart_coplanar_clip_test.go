package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
)

func coplanarFootprintChart(t *testing.T, f func(float64, float64) Vec3, domain ParameterRectangle, refine bool) *AffineSurfaceChart {
	t.Helper()
	poles := []Vec3{f(0, 0), f(0, 1), f(1, 0), f(1, 1)}
	s, e := NewBSplineSurface(1, 1, 2, 2, poles, []float64{domain.UMin, domain.UMin, domain.UMax, domain.UMax}, []float64{domain.VMin, domain.VMin, domain.VMax, domain.VMax}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if refine {
		p, e = p.InsertUKnot((domain.UMin+domain.UMax)/2, 1, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
	}
	c, e := NewAffineSurfaceChart(context.Background(), p, AffineSurfaceChartOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func TestCoplanarAffineChartExactFootprint(t *testing.T) {
	ctx := context.Background()
	unit := ParameterRectangle{UMax: 1, VMax: 1}
	embed := func(x, y float64) Vec3 { return Vec3{x, y, x + 2*y} }
	a := coplanarFootprintChart(t, embed, unit, true)
	for _, reversed := range []bool{false, true} {
		c := coplanarFootprintChart(t, func(s, v float64) Vec3 {
			if reversed {
				v = 1 - v
			}
			return embed(3*s, v+s)
		}, ParameterRectangle{UMin: 2, UMax: 4, VMin: -1, VMax: 2}, false)
		r, e := IntersectCoplanarAffineSurfaceCharts(ctx, a, c, AffineChartIntersectionOptions{})
		if e != nil || !r.Complete || r.Kind != CoplanarChartsArea || len(r.Vertices) != 4 || r.AreaA.Cmp(big.NewRat(5, 6)) != 0 || r.SquaredArea.Cmp(big.NewRat(25, 6)) != 0 {
			t.Fatal(r, e)
		}
		sign := 1
		if reversed {
			sign = -1
		}
		if r.RelativeOrientation != sign {
			t.Fatal("orientation", r.RelativeOrientation)
		}
		nonbinary := false
		for _, v := range r.Vertices {
			x, e := a.EvalExact(v.A[0], v.A[1])
			if e != nil {
				t.Fatal(e)
			}
			y, e := c.EvalExact(v.B[0], v.B[1])
			if e != nil {
				t.Fatal(e)
			}
			for axis := 0; axis < 3; axis++ {
				if x[axis].Cmp(y[axis]) != 0 || x[axis].Cmp(v.Point[axis]) != 0 {
					t.Fatal("original model identity")
				}
			}
			if v.A[0].Cmp(big.NewRat(1, 1)) == 0 && v.A[1].Cmp(big.NewRat(1, 3)) == 0 {
				nonbinary = true
				if v.B[0].Cmp(big.NewRat(8, 3)) != 0 || v.BoundaryA != 2 {
					t.Fatal(v)
				}
				mask := uint8(4)
				if reversed {
					mask = 8
				}
				if v.BoundaryB != mask {
					t.Fatal("original B side", v.BoundaryB)
				}
			}
		}
		if !nonbinary {
			t.Fatal("missing exact rational cut")
		}
		swapped, e := IntersectCoplanarAffineSurfaceCharts(ctx, c, a, AffineChartIntersectionOptions{})
		if e != nil || swapped.SquaredArea.Cmp(r.SquaredArea) != 0 || swapped.RelativeOrientation != sign {
			t.Fatal(swapped, e)
		}
		for _, cap := range []int{r.ExactOperations, r.ExactOperations - 1} {
			q, e := IntersectCoplanarAffineSurfaceCharts(ctx, a, c, AffineChartIntersectionOptions{MaxOperations: cap})
			if cap == r.ExactOperations {
				if e != nil || !q.Complete {
					t.Fatal(e)
				}
			} else if e == nil || q.Complete || len(q.Vertices) != 0 || q.AreaA != nil {
				t.Fatal("partial footprint")
			}
		}
		r.Vertices[0].Point[0].SetInt64(999)
		again, e := IntersectCoplanarAffineSurfaceCharts(ctx, a, c, AffineChartIntersectionOptions{})
		if e != nil || again.Vertices[0].Point[0].Cmp(big.NewRat(999, 1)) == 0 {
			t.Fatal("source alias")
		}
	}
}
func TestCoplanarAffineChartContactDimensions(t *testing.T) {
	ctx := context.Background()
	d := ParameterRectangle{UMax: 1, VMax: 1}
	f := func(u, v float64) Vec3 { return Vec3{u, v, 0} }
	a := coplanarFootprintChart(t, f, d, false)
	for _, tc := range []struct {
		x, y  float64
		kind  CoplanarChartContactKind
		count int
	}{{1, 0, CoplanarChartsSegment, 2}, {1, 1, CoplanarChartsPoint, 1}, {math.Nextafter(1, 2), 0, CoplanarChartsEmpty, 0}, {.25, .25, CoplanarChartsArea, 4}} {
		c := coplanarFootprintChart(t, func(u, v float64) Vec3 { return f(tc.x+u, tc.y+v) }, d, false)
		r, e := IntersectCoplanarAffineSurfaceCharts(ctx, a, c, AffineChartIntersectionOptions{})
		if e != nil || !r.Complete || r.Kind != tc.kind || len(r.Vertices) != tc.count {
			t.Fatal(tc, r, e)
		}
		if tc.kind != CoplanarChartsArea && r.SquaredArea.Sign() != 0 {
			t.Fatal("contact gained area")
		}
	}
	c := coplanarFootprintChart(t, func(u, v float64) Vec3 { return Vec3{u, v, u + 2*v + .125} }, d, false)
	if r, e := IntersectCoplanarAffineSurfaceCharts(ctx, a, c, AffineChartIntersectionOptions{}); !errors.Is(e, ErrDomain) || r.Complete || len(r.Vertices) != 0 {
		t.Fatal("noncoplanar claimed empty", r, e)
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := IntersectCoplanarAffineSurfaceCharts(cancelCtx, a, a, AffineChartIntersectionOptions{}); e != context.Canceled || r.Complete {
		t.Fatal(r, e)
	}
}

func TestCoplanarAffineChartOctagon(t *testing.T) {
	ctx := context.Background()
	d := ParameterRectangle{UMax: 1, VMax: 1}
	embed := func(x, y float64) Vec3 { return Vec3{x, y, x + 2*y} }
	a := coplanarFootprintChart(t, embed, d, false)
	b := coplanarFootprintChart(t, func(u, v float64) Vec3 { return embed(-.25+.75*u+.75*v, .5-.75*u+.75*v) }, d, true)
	r, e := IntersectCoplanarAffineSurfaceCharts(ctx, a, b, AffineChartIntersectionOptions{})
	if e != nil || !r.Complete || r.Kind != CoplanarChartsArea || len(r.Vertices) != 8 || r.AreaA.Cmp(big.NewRat(7, 8)) != 0 || r.SquaredArea.Cmp(big.NewRat(147, 32)) != 0 {
		t.Fatal(r, e)
	}
	// Four original square corners lose right triangles of area(1/4)^2/2.
	for _, v := range r.Vertices {
		if v.BoundaryA == 0 && v.BoundaryB == 0 {
			t.Fatal("unowned clipping vertex")
		}
	}
}
