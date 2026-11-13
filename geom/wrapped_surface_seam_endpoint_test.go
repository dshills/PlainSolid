package geom

import (
	"context"
	"errors"
	"testing"
)

func TestWrappedArcSeamEndpoint(t *testing.T) {
	ctx := context.Background()
	p := wrappedCylinderPatch(t)
	w, _, e := NewWrappedSurfacePatch(ctx, p, WrappedSurfaceOptions{WrapU: true, ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	arcs, e := CertifySurfacePlaneArcs(p, plane, SurfacePlaneArcOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var left, right SurfacePlaneArc
	li, ri := -1, -1
	for _, a := range arcs.Arcs {
		for i := 0; i < 2; i++ {
			ep, e := a.ExactEndpoint(i, ArcExactEndpointOptions{})
			if e != nil {
				t.Fatal(e)
			}
			u, _ := ep.Parameters[0].Float64()
			if u == p.Domain().UMin {
				left, li = a, i
			}
			if u == p.Domain().UMax {
				right, ri = a, i
			}
		}
	}
	if li < 0 || ri < 0 {
		t.Fatal("missing seam ends")
	}
	r, e := w.CompareArcSeamEndpoints(ctx, left, li, right, ri, 0, ArcExactEndpointOptions{})
	if e != nil || !r.Matched || !r.TransverseContinuation || r.TangentSeamSign != 1 || r.LeftUpper {
		t.Fatal(r, e)
	}
	r.Left.Point[0].SetInt64(999)
	r, e = w.CompareArcSeamEndpoints(ctx, right, ri, left, li, 0, ArcExactEndpointOptions{})
	if e != nil || !r.Matched || !r.LeftUpper {
		t.Fatal(r, e)
	}
	_, e = w.CompareArcSeamEndpoints(ctx, left, li, left, li, 0, ArcExactEndpointOptions{})
	if !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}
	_, e = w.CompareArcSeamEndpoints(ctx, left, li, right, ri, 0, ArcExactEndpointOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, e = w.CompareArcSeamEndpoints(canceled, left, li, right, ri, 0, ArcExactEndpointOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	c0, _, e := NewWrappedSurfacePatch(ctx, p, WrappedSurfaceOptions{WrapU: true})
	if e != nil {
		t.Fatal(e)
	}
	r, e = c0.CompareArcSeamEndpoints(ctx, left, li, right, ri, 0, ArcExactEndpointOptions{})
	if e != nil || !r.Matched || r.TransverseContinuation {
		t.Fatal(r, e)
	}
	// A public endpoint box is only display data and cannot forge root identity.
	left.Endpoints[li].Parameters = right.Endpoints[ri].Parameters
	r, e = w.CompareArcSeamEndpoints(ctx, left, li, right, ri, 0, ArcExactEndpointOptions{})
	if e != nil || !r.Matched {
		t.Fatal(r, e)
	}
	shifted, e := p.Transformed(Translation(Vec3{X: 0.125}))
	if e != nil {
		t.Fatal(e)
	}
	wrong, e := CertifySurfacePlaneArcs(shifted, plane, SurfacePlaneArcOptions{})
	if e != nil {
		t.Fatal(e)
	}
	_, e = w.CompareArcSeamEndpoints(ctx, wrong.Arcs[0], 0, right, ri, 0, ArcExactEndpointOptions{})
	if !errors.Is(e, ErrDomain) {
		t.Fatal("foreign source accepted", e)
	}
	reversed, _ := NewPlane(Vec3{}, Vec3{Z: -1})
	wrong, e = CertifySurfacePlaneArcs(p, reversed, SurfacePlaneArcOptions{})
	if e != nil {
		t.Fatal(e)
	}
	_, e = w.CompareArcSeamEndpoints(ctx, wrong.Arcs[0], 0, right, ri, 0, ArcExactEndpointOptions{})
	if !errors.Is(e, ErrDomain) {
		t.Fatal("different oriented plane accepted", e)
	}

}
