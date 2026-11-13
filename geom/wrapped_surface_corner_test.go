package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestWrappedCornerPairing(t *testing.T) {
	ctx := context.Background()
	// Tensor cubic loops have equal nonzero first derivatives at both seams.
	controls := []float64{0, 1, -1, 0}
	var poles []Vec3
	for _, x := range controls {
		for _, y := range controls {
			poles = append(poles, Vec3{x, y, 0})
		}
	}
	s, e := NewBSplineSurface(3, 3, 4, 4, poles, []float64{0, 0, 0, 0, 1, 1, 1, 1}, []float64{0, 0, 0, 0, 1, 1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	w, _, e := NewWrappedSurfacePatch(ctx, p, WrappedSurfaceOptions{WrapU: true, WrapV: true, ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	for _, direction := range []float64{1, -1} {
		plane, _ := NewPlane(Vec3{}, Vec3{X: direction, Y: direction})
		var arcs [2]SurfacePlaneArc
		var endpoint [2]int
		for i, d := range []ParameterRectangle{{0, .125, .875, 1}, {.875, 1, 0, .125}} {
			patch, e := p.Trim(d)
			if e != nil {
				t.Fatal(e)
			}
			r, e := CertifySurfacePlaneArcs(patch, plane, SurfacePlaneArcOptions{})
			if e != nil || len(r.Arcs) != 1 {
				t.Fatal(r, e)
			}
			arcs[i] = r.Arcs[0]
			endpoint[i] = -1
			for end := 0; end < 2; end++ {
				ep, e := arcs[i].ExactEndpoint(end, ArcExactEndpointOptions{})
				if e != nil {
					t.Fatal(e)
				}
				if (ep.Parameters[0].Sign() == 0 || ep.Parameters[0].Cmp(big.NewRat(1, 1)) == 0) && (ep.Parameters[1].Sign() == 0 || ep.Parameters[1].Cmp(big.NewRat(1, 1)) == 0) {
					endpoint[i] = end
				}
			}
			if endpoint[i] < 0 {
				t.Fatal("missing exact corner")
			}
		}
		r, e := w.CompareArcCornerEndpoints(ctx, arcs[0], endpoint[0], arcs[1], endpoint[1], ArcExactEndpointOptions{})
		if e != nil || !r.Matched || !r.TransverseContinuation || r.Direction[0] != int(direction) || r.Direction[1] != -int(direction) {
			t.Fatal(r, e)
		}
		_, e = w.CompareArcCornerEndpoints(ctx, arcs[0], endpoint[0], arcs[0], endpoint[0], ArcExactEndpointOptions{})
		if !errors.Is(e, ErrConvergence) {
			t.Fatal(e)
		}
		_, e = w.CompareArcCornerEndpoints(ctx, arcs[0], endpoint[0], arcs[1], endpoint[1], ArcExactEndpointOptions{MaxOperations: 1})
		if !errors.Is(e, ErrConvergence) {
			t.Fatal(e)
		}
	}
}

func TestWrappedTorusCornerTrace(t *testing.T) {
	ctx := context.Background()
	w := wrappedTorusPatch(t)
	for _, direction := range []float64{1, -1} {
		p, _ := NewPlane(Vec3{}, Vec3{Y: direction, Z: direction})
		r, e := TraceWrappedSurfacePlaneContext(ctx, w, p, SurfaceTraceOptions{Arcs: SurfacePlaneArcOptions{ResolveSubdivisionCorners: true}})
		if e != nil || !r.Complete {
			t.Log(r.Unresolved, r.Fundamental.Unresolved)
			t.Fatal("torus trace", len(r.Contours), len(r.Fundamental.Isolation.Unresolved), len(r.Unresolved), e)
		}
		corners := 0
		for _, j := range r.SeamJoins {
			if j.Axis == 2 {
				corners++
				if !j.CornerCertificate.TransverseContinuation {
					t.Fatal(j)
				}
			}
		}
		if corners != 1 {
			t.Fatal("corner joins", corners)
		}
		for _, c := range r.Contours {
			if !c.Closed {
				t.Fatal("open torus section")
			}
		}
	}
}
