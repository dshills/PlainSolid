package geom

import (
	"context"
	"errors"
	"testing"
)

func TestDiscoverTubesFromStraightPlaneArc(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u - .375} })
	b := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	arcs, e := CertifySurfacePlaneArcs(a, plane, SurfacePlaneArcOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := DiscoverSurfacePlaneArcTubes(context.Background(), arcs.Arcs[0], a, b, .2, .8, SurfaceArcTubeDiscoveryOptions{})
	if e != nil || !r.Complete || len(r.Tubes) == 0 || len(r.Pending) != 0 || r.OriginalA != a.Domain() || r.OriginalB != b.Domain() {
		t.Fatal(r, e)
	}
	for _, tube := range r.Tubes {
		mid := tube.Span.Start/2 + tube.Span.End/2
		point, e := tube.Proof.Evaluate(SurfaceTubeEvaluationOptions{Parameter: mid})
		if e != nil || !point.Complete || !point.Bounds.Contains(Vec3{.375, mid, 0}) {
			t.Fatal(point, e)
		}
	}
}
func TestDiscoverTubesFromCurvedPlaneArc(t *testing.T) {
	a, _ := NewSurfacePatch(quadraticContactSurface(t, -.12), SurfacePatchOptions{})
	b := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	arcs, e := CertifySurfacePlaneArcs(a, plane, SurfacePlaneArcOptions{})
	if e != nil {
		t.Fatal(e)
	}
	arc := arcs.Arcs[0]
	lo := min(arc.Endpoints[0].Parameters.Max.Y, arc.Endpoints[1].Parameters.Max.Y)
	hi := max(arc.Endpoints[0].Parameters.Min.Y, arc.Endpoints[1].Parameters.Min.Y)
	if arc.Certificate.FixedParameterAxis == 1 {
		lo = min(arc.Endpoints[0].Parameters.Max.X, arc.Endpoints[1].Parameters.Max.X)
		hi = max(arc.Endpoints[0].Parameters.Min.X, arc.Endpoints[1].Parameters.Min.X)
	}
	start, end := lo+(hi-lo)/4, hi-(hi-lo)/4
	r, e := DiscoverSurfacePlaneArcTubes(context.Background(), arc, a, b, start, end, SurfaceArcTubeDiscoveryOptions{})
	if e != nil || !r.Complete || len(r.Tubes) == 0 {
		t.Fatal(r, e)
	}
	for _, tube := range r.Tubes {
		parameter := tube.Span.Start/2 + tube.Span.End/2
		point, e := tube.Proof.Evaluate(SurfaceTubeEvaluationOptions{Parameter: parameter})
		if e != nil {
			t.Fatal(e)
		}
		arcPoint, e := arc.Evaluate(parameter, ArcEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if _, ok := point.Bounds.Intersect(arcPoint.Bounds); !ok {
			t.Fatal(point, arcPoint)
		}
	}
}
func TestDiscoverPlaneArcTubesRejectsAndPreservesPending(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u - .375} })
	b := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	arcs, _ := CertifySurfacePlaneArcs(a, plane, SurfacePlaneArcOptions{})
	arc := arcs.Arcs[0]
	mismatch := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u - .25} })
	_, e := DiscoverSurfacePlaneArcTubes(context.Background(), arc, mismatch, b, .2, .8, SurfaceArcTubeDiscoveryOptions{})
	if !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	wrongPlane := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 1} })
	_, e = DiscoverSurfacePlaneArcTubes(context.Background(), arc, a, wrongPlane, .2, .8, SurfaceArcTubeDiscoveryOptions{})
	if !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	r, e := DiscoverSurfacePlaneArcTubes(context.Background(), arc, a, b, 0, 1, SurfaceArcTubeDiscoveryOptions{MaxDepth: 1, MaxIntervals: 4})
	if !errors.Is(e, ErrConvergence) || r.Complete || len(r.Pending) == 0 {
		t.Fatal(r, e)
	}
	r, e = DiscoverSurfacePlaneArcTubes(context.Background(), arc, a, b, .2, .8, SurfaceArcTubeDiscoveryOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) || r.Complete || len(r.Pending) == 0 {
		t.Fatal(r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = DiscoverSurfacePlaneArcTubes(ctx, arc, a, b, .2, .8, SurfaceArcTubeDiscoveryOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

func TestDiscoverCircleTubesFromExactRefinedTrace(t *testing.T) {
	original, _ := NewSurfacePatch(quadraticContactSurface(t, -25.0/256), SurfacePatchOptions{})
	refined := original
	for _, u := range []float64{.25, .3125, .6875, .75} {
		var e error
		refined, e = refined.InsertUKnot(u, 2, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
	}
	for _, v := range []float64{.25, .3125, .6875, .75} {
		var e error
		refined, e = refined.InsertVKnot(v, 2, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
	}
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	trace, e := TraceSurfacePlane(refined, plane, SurfaceTraceOptions{Arcs: SurfacePlaneArcOptions{ResolveSubdivisionCorners: true}})
	if e != nil {
		t.Fatal(e)
	}
	support := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	if len(trace.Contours) != 1 || !trace.Contours[0].Closed {
		t.Fatal(trace)
	}
	for _, arc := range trace.Isolation.Arcs {
		a, e := arc.ExactEndpoint(0, ArcExactEndpointOptions{})
		if e != nil {
			t.Fatal(e)
		}
		b, e := arc.ExactEndpoint(1, ArcExactEndpointOptions{})
		if e != nil {
			t.Fatal(e)
		}
		lo, exact := a.FreeParameter.Float64()
		if !exact {
			t.Fatal("nonrepresentable interval")
		}
		hi, exact := b.FreeParameter.Float64()
		if !exact {
			t.Fatal("nonrepresentable interval")
		}
		if hi < lo {
			lo, hi = hi, lo
		}
		result, e := DiscoverSurfacePlaneArcTubes(context.Background(), arc, original, support, lo, hi, SurfaceArcTubeDiscoveryOptions{})
		if e != nil || !result.Complete || len(result.Tubes) == 0 {
			t.Fatal(arc.Domain, result, e)
		}
	}
}
