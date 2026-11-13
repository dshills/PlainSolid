package geom

import (
	"context"
	"errors"
	"testing"
)

func TestClipIsoCurveAffinePlaneParabola(t *testing.T) {
	ctx := context.Background()
	plane := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{.75, u, v} })
	chart, e := NewAffineSurfaceChart(ctx, plane, AffineSurfaceChartOptions{})
	if e != nil {
		t.Fatal(e)
	}
	makeCurve := func(height float64) *SurfaceIsoCurve {
		s, _ := NewBezier([]Vec3{{X: .75, Z: height - .25}, {X: .75, Y: .5, Z: height + .25}, {X: .75, Y: 1, Z: height - .25}}, nil)
		return isoIntersectionFixture(t, s)
	}
	c := makeCurve(25.0 / 1024)
	r, e := ClipIsoCurveToAffineChart(ctx, c, chart, SplinePlaneOptions{})
	if e != nil || !r.Complete || len(r.Spans) != 1 || len(r.Contacts) != 0 {
		t.Fatal(r, e)
	}
	span := r.Spans[0]
	if span.Start.Parameter.RatString() != "11/32" || span.End.Parameter.RatString() != "21/32" || span.Start.BoundaryMask != 4 || span.End.BoundaryMask != 4 || span.Start.UV[1].Sign() != 0 || span.Start.Point[2].Sign() != 0 {
		t.Fatal(span)
	}
	span.Start.Point[0].SetInt64(9)
	r, e = ClipIsoCurveToAffineChart(ctx, c, chart, SplinePlaneOptions{})
	if e != nil || r.Spans[0].Start.Point[0].RatString() != "3/4" {
		t.Fatal(r, e)
	}
	r, e = ClipIsoCurveToAffineChart(ctx, makeCurve(0), chart, SplinePlaneOptions{})
	if e != nil || !r.Complete || len(r.Spans) != 0 || len(r.Contacts) != 1 || !r.Contacts[0].TangentBoundary {
		t.Fatal(r, e)
	}
	r, e = ClipIsoCurveToAffineChart(ctx, makeCurve(-1), chart, SplinePlaneOptions{})
	if e != nil || !r.Complete || len(r.Spans) != 0 || len(r.Contacts) != 0 {
		t.Fatal(r, e)
	}
	r, e = ClipIsoCurveToAffineChart(ctx, makeCurve(.125), chart, SplinePlaneOptions{})
	if !errors.Is(e, ErrConvergence) || r.Complete {
		t.Fatal(r, e)
	}
	s, _ := NewBezier([]Vec3{{X: .75}, {X: .75, Y: 1}}, nil)
	r, e = ClipIsoCurveToAffineChart(ctx, isoIntersectionFixture(t, s), chart, SplinePlaneOptions{})
	if !errors.Is(e, ErrConvergence) || len(r.CoincidentBoundary) == 0 {
		t.Fatal(r, e)
	}
	_, e = ClipIsoCurveToAffineChart(ctx, c, chart, SplinePlaneOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, e = ClipIsoCurveToAffineChart(canceled, c, chart, SplinePlaneOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
