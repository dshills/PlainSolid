package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
)

func TestRawObliquePlaneTraceRetainsEquation(t *testing.T) {
	support := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{5 * u, 5 * v, -u - 2*v} })
	raw, e := NewRawSupportPlane(Vec3{}, Vec3{5, 0, -1}, Vec3{0, 5, -2}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	bounds, e := support.RawPlaneBounds(raw)
	if e != nil || bounds.Relation != PlanePatchCoplanar {
		t.Fatal(bounds, e)
	}
	ordinary, _ := NewPlane(Vec3{}, Vec3{1, 2, 5})
	ordinaryBounds, e := support.PlaneBounds(ordinary)
	if e != nil || ordinaryBounds.Relation == PlanePatchCoplanar {
		t.Fatal("fixture must detect normalized coefficient rounding", ordinaryBounds, e)
	}
	source := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{5 * u, 5 * v, -2*v - .375} })
	trace, e := TraceSurfaceRawPlane(source, raw, SurfaceTraceOptions{})
	if e != nil || !trace.Complete || len(trace.Isolation.Arcs) != 1 {
		t.Fatal(trace, e)
	}
	arc := trace.Isolation.Arcs[0]
	for i := 0; i < 2; i++ {
		ep, e := arc.ExactEndpoint(i, ArcExactEndpointOptions{})
		if e != nil || ep.Parameters[0].Cmp(big.NewRat(3, 8)) != 0 {
			t.Fatal(ep, e)
		}
		eq := raw.Coefficients()
		value := exactPlaneHomogeneousValue(eq, exactHomogeneous{ep.Point[0], ep.Point[1], ep.Point[2], big.NewRat(1, 1)})
		if value.Sign() != 0 {
			t.Fatal(value)
		}
	}
	evaluated, e := arc.Evaluate(.5, ArcEvaluationOptions{})
	if e != nil || evaluated.Parameters.Min.X > .375 || evaluated.Parameters.Max.X < .375 {
		t.Fatal(evaluated, e)
	}
	enclosure, e := arc.EncloseInterval(context.Background(), .25, .75, SurfaceArcIntervalOptions{})
	if e != nil || !enclosure.HasBounds || enclosure.Parameters.Min.X > .375 || enclosure.Parameters.Max.X < .375 {
		t.Fatal(enclosure, e)
	}
	discovery, e := DiscoverSurfacePlaneArcTubes(context.Background(), arc, source, support, .25, .75, SurfaceArcTubeDiscoveryOptions{Padding: .2})
	if e != nil || !discovery.Complete || len(discovery.Tubes) == 0 {
		t.Fatal(discovery, e)
	}
	coefficients := raw.Coefficients()
	coefficients[0].SetInt64(0)
	again, e := support.RawPlaneBounds(raw)
	if e != nil || again.Relation != PlanePatchCoplanar {
		t.Fatal(again, e)
	}
}
func TestRawAffineChartNonrepresentableFrameAndLimits(t *testing.T) {
	original := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{5 * u, 5 * v, -u - 2*v} })
	shifted, e := original.Transformed(Translation(Vec3{X: math.Ldexp(1, -80)}))
	if e != nil {
		t.Fatal(e)
	}
	trimmed, e := shifted.Trim(ParameterRectangle{.1, .9, .2, .8})
	if e != nil {
		t.Fatal(e)
	}
	chart, e := NewAffineSurfaceChart(context.Background(), trimmed, AffineSurfaceChartOptions{})
	if e != nil {
		t.Fatal(e)
	}
	raw, e := chart.RawSupportPlane(SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	bounds, e := trimmed.RawPlaneBounds(raw)
	if e != nil || bounds.Relation != PlanePatchCoplanar {
		t.Fatal(bounds, e)
	}
	trace, e := TraceSurfaceRawPlane(trimmed, raw, SurfaceTraceOptions{})
	if e != nil || len(trace.Isolation.Coplanar) != 1 {
		t.Fatal(trace, e)
	}
	_, e = NewRawSupportPlane(Vec3{}, Vec3{X: 1}, Vec3{X: 2}, SurfacePatchOptions{})
	if !errors.Is(e, ErrDegenerate) {
		t.Fatal(e)
	}
	_, e = chart.RawSupportPlane(SurfacePatchOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = TraceSurfaceRawPlaneContext(canceled, trimmed, raw, SurfaceTraceOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	_, e = TraceSurfaceRawPlane(trimmed, RawSupportPlane{}, SurfaceTraceOptions{})
	if !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}
