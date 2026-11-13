package geom

import (
	"context"
	"errors"
	"math"
	"testing"
)

func TestAffineChartIntersections(t *testing.T) {
	ctx := context.Background()
	chart := func(f func(float64, float64) Vec3) *AffineSurfaceChart {
		t.Helper()
		c, e := NewAffineSurfaceChart(ctx, arcTestPatch(t, f), AffineSurfaceChartOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return c
	}
	a := chart(func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	tests := []struct {
		name string
		f    func(float64, float64) Vec3
		kind AffineChartIntersectionKind
	}{
		{"segment", func(u, v float64) Vec3 { return Vec3{.5, u, 2*v - 1} }, AffineChartsSegment},
		{"parallel", func(u, v float64) Vec3 { return Vec3{u, v, 1} }, AffineChartsDisjoint},
		{"coplanar", func(u, v float64) Vec3 { return Vec3{u + 3, v, 0} }, AffineChartsCoplanar},
		{"point", func(u, v float64) Vec3 { return Vec3{u + 1, v + 1, u - v} }, AffineChartsPoint},
		{"clipped empty", func(u, v float64) Vec3 { return Vec3{2, u, v - .5} }, AffineChartsDisjoint},
		{"near parallel", func(u, v float64) Vec3 { return Vec3{u, v, math.Ldexp(u-.5, -500)} }, AffineChartsSegment},
		{"oblique", func(u, v float64) Vec3 { return Vec3{u + v - .5, u - v + .5, v - .5} }, AffineChartsSegment},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := chart(tt.f)
			r, e := IntersectAffineSurfaceCharts(ctx, a, b, AffineChartIntersectionOptions{})
			if e != nil || !r.Complete || r.Kind != tt.kind {
				t.Fatal(r, e)
			}
			if r.Kind == AffineChartsPoint || r.Kind == AffineChartsSegment {
				for _, ep := range r.Endpoints {
					for _, c := range []*AffineSurfaceChart{a, b} {
						q, e := c.InvertExact(ep.Point)
						if e != nil || !q.Inside || !q.OnPlane {
							t.Fatal(q, e)
						}
					}
					if ep.BoundaryA == 0 && ep.BoundaryB == 0 {
						t.Fatal("unclipped endpoint")
					}
				}
			}
		})
	}
	b := chart(tests[0].f)
	r, e := IntersectAffineSurfaceCharts(ctx, a, b, AffineChartIntersectionOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if r.Endpoints[0].A[0].RatString() != "1/2" || r.Endpoints[0].BoundaryA != 4 || r.Endpoints[1].BoundaryA != 8 {
		t.Fatal(r)
	}
	swapped, e := IntersectAffineSurfaceCharts(ctx, b, a, AffineChartIntersectionOptions{})
	if e != nil || swapped.Kind != AffineChartsSegment {
		t.Fatal(swapped, e)
	}
	for _, ep := range swapped.Endpoints {
		if ep.Point[0].RatString() != "1/2" || ep.Point[2].Sign() != 0 {
			t.Fatal(ep)
		}
	}
	r.Endpoints[0].Point[0].SetInt64(99)
	s, e := IntersectAffineSurfaceCharts(ctx, a, b, AffineChartIntersectionOptions{})
	if e != nil || s.Endpoints[0].Point[0].RatString() != "1/2" {
		t.Fatal("alias", e)
	}
	_, e = IntersectAffineSurfaceCharts(ctx, a, b, AffineChartIntersectionOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, e = IntersectAffineSurfaceCharts(canceled, a, b, AffineChartIntersectionOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	_, e = IntersectAffineSurfaceCharts(ctx, nil, b, AffineChartIntersectionOptions{})
	if !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}
