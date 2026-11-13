package geom

import (
	"context"
	"errors"
	"testing"
)

func TestTraceWrappedSurfacePlaneCylinder(t *testing.T) {
	ctx := context.Background()
	w, _, e := NewWrappedSurfacePatch(ctx, wrappedCylinderPatch(t), WrappedSurfaceOptions{WrapU: true, ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	p, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	r, e := TraceWrappedSurfacePlaneContext(ctx, w, p, SurfaceTraceOptions{})
	if e != nil || !r.Complete || len(r.Contours) != 1 || !r.Contours[0].Closed || len(r.SeamJoins) != 1 {
		t.Fatal(r, e)
	}
	if len(r.Contours[0].Arcs) != len(r.Fundamental.Isolation.Arcs) {
		t.Fatal("lost evidence")
	}
	// Duplicated topology cannot silently choose one of several seam mates.
	ambiguous := r.Fundamental
	ambiguous.Contours = append(append([]SurfaceContour{}, ambiguous.Contours...), ambiguous.Contours[0])
	bad, e := assembleWrappedSurfaceTrace(ctx, w, ambiguous, nil, SurfaceTraceOptions{})
	if !errors.Is(e, ErrConvergence) || bad.Complete || len(bad.Contours) != 0 {
		t.Fatal("ambiguous graph accepted", bad, e)
	}
	// A vertical plane away from the periodic cut gives two open generators.
	p, _ = NewPlane(Vec3{}, Vec3{X: 1})
	r, e = TraceWrappedSurfacePlaneContext(ctx, w, p, SurfaceTraceOptions{})
	if e != nil || !r.Complete || len(r.Contours) != 2 {
		t.Fatal(r, e)
	}
	for _, c := range r.Contours {
		if c.Closed {
			t.Fatal("generator closed")
		}
	}
	raw, e := NewRawSupportPlane(Vec3{}, Vec3{X: 1}, Vec3{Y: 1}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e = TraceWrappedSurfaceRawPlaneContext(ctx, w, raw, SurfaceTraceOptions{})
	if e != nil || len(r.Contours) != 1 || !r.Contours[0].Closed {
		t.Fatal(r, e)
	}
	c0, _, _ := NewWrappedSurfacePatch(ctx, w.patch, WrappedSurfaceOptions{WrapU: true})
	r, e = TraceWrappedSurfaceRawPlaneContext(ctx, c0, raw, SurfaceTraceOptions{})
	if !errors.Is(e, ErrConvergence) || r.Complete || len(r.Contours) != 0 || len(r.Unresolved) == 0 {
		t.Fatal(r, e)
	}
	_, e = TraceWrappedSurfaceRawPlaneContext(ctx, w, raw, SurfaceTraceOptions{MaxJoinOperations: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, e = TraceWrappedSurfaceRawPlaneContext(canceled, w, raw, SurfaceTraceOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
