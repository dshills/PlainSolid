package geom

import (
	"context"
	"errors"
	"testing"
)

func TestQuadraticSurfacePlaneTangency(t *testing.T) {
	k := []float64{0, 0, 0, 1, 1, 1}
	u := [3]float64{.25, -.5, 1} // (1.5u-.5)^2
	v := [3]float64{.25, -.25, .25}
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	o := SurfaceTraceOptions{Arcs: SurfacePlaneArcOptions{IsolateQuadraticTangencies: true, MaxDepth: 4}}
	for _, sign := range []int{-1, 1} {
		poles := []Vec3{}
		for i := range 3 {
			for j := range 3 {
				poles = append(poles, Vec3{float64(i) / 2, float64(j) / 2, float64(sign) * (u[i] + v[j])})
			}
		}
		s, e := NewBSplineSurface(2, 2, 3, 3, poles, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		p, e := NewSurfacePatch(s, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		trace, e := TraceSurfacePlane(p, plane, o)
		if e != nil || !trace.Complete || len(trace.Contours) != 0 || len(trace.Isolation.Tangencies) != 1 {
			t.Fatal(trace, e)
		}
		x := trace.Isolation.Tangencies[0]
		if x.Side != sign || x.UV.Min.X > 1./3 || x.UV.Max.X < 1./3 || x.UV.Max.X <= x.UV.Min.X ||
			x.UV.Min.Y != .5 || x.UV.Max.Y != .5 || x.Bounds.Min.Z != 0 || x.Bounds.Max.Z != 0 {
			t.Fatal(x)
		}
		// A strictly positive local minimum is not a tangent contact.
		high, _ := NewPlane(Vec3{Z: -float64(sign) * .03}, Vec3{Z: 1})
		r, e := TraceSurfacePlane(p, high, SurfaceTraceOptions{Arcs: SurfacePlaneArcOptions{IsolateQuadraticTangencies: true}})
		if e != nil || !r.Complete || len(r.Isolation.Tangencies) != 0 || len(r.Contours) != 0 {
			t.Fatal(r, e)
		}
		// A negative stationary value produces a regular curve, not a point.
		low, _ := NewPlane(Vec3{Z: float64(sign) / 32}, Vec3{Z: 1})
		r, e = TraceSurfacePlane(p, low, SurfaceTraceOptions{Arcs: SurfacePlaneArcOptions{IsolateQuadraticTangencies: true, ResolveSubdivisionCorners: true}})
		if e != nil || len(r.Isolation.Tangencies) != 0 || len(r.Contours) != 1 || !r.Contours[0].Closed {
			t.Fatal(r, e)
		}
		r, e = TraceSurfacePlane(p, plane, SurfaceTraceOptions{Arcs: SurfacePlaneArcOptions{MaxDepth: 4}})
		if !errors.Is(e, ErrConvergence) || r.Complete {
			t.Fatal("legacy unresolved mode", r, e)
		}
	}
}

func TestSplineIntersectionContextPreservesPending(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := quadraticContactSurface(t, -.12)
	p, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	pl, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	trace, e := TraceSurfacePlaneContext(ctx, p, pl, SurfaceTraceOptions{})
	if !errors.Is(e, context.Canceled) || trace.Complete || len(trace.Isolation.Pending) != 1 ||
		trace.Isolation.Pending[0] != p.Domain() {
		t.Fatal(trace, e)
	}
	line, _ := NewLine(Vec3{X: .5, Y: .5}, Vec3{Z: 1})
	r, e := IntersectLineSplineSurfaceContext(ctx, line, p, LineSplineSurfaceOptions{MinParameter: -1, MaxParameter: 1})
	if !errors.Is(e, context.Canceled) || r.Complete || len(r.Pending) != 1 || r.Pending[0].Cell != p.Domain() {
		t.Fatal(r, e)
	}
}
