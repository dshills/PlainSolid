package geom

import (
	"context"
	"errors"
	"testing"
)

func TestSurfaceArcIntervalStraightAndClipping(t *testing.T) {
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u - .375} })
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	cert, e := CertifySurfacePlaneArcs(p, plane, SurfacePlaneArcOptions{})
	if e != nil {
		t.Fatal(e)
	}
	arc := cert.Arcs[0]
	r, e := arc.EncloseInterval(context.Background(), .2, .7, SurfaceArcIntervalOptions{})
	if e != nil || !r.HasBounds || !r.Complete || r.Excluded || r.Parameters.Min.Y != .2 || r.Parameters.Max.Y != .7 {
		t.Fatal(r, e)
	}
	for _, v := range []float64{.2, .4, .7} {
		if !r.Bounds.Contains(Vec3{.375, v, 0}) || r.Parameters.Min.X > .375 || r.Parameters.Max.X < .375 {
			t.Fatal(r, v)
		}
	}
	r, e = arc.EncloseInterval(context.Background(), -1, .3, SurfaceArcIntervalOptions{})
	if e != nil || !r.HasBounds || r.Parameters.Min.Y != 0 || r.Parameters.Max.Y != .3 {
		t.Fatal(r, e)
	}
	r, e = arc.EncloseInterval(context.Background(), 2, 3, SurfaceArcIntervalOptions{})
	if e != nil || !r.Excluded || !r.Complete {
		t.Fatal(r, e)
	}
	r, e = arc.EncloseInterval(context.Background(), -1, 0, SurfaceArcIntervalOptions{})
	if !errors.Is(e, ErrConvergence) || !r.HasBounds || r.Parameters.Min.Y != 0 || r.Parameters.Max.Y != 0 {
		t.Fatal(r, e)
	}
}
func TestSurfaceArcIntervalCircleKnownExactPoints(t *testing.T) {
	p, _ := NewSurfacePatch(quadraticContactSurface(t, -.125), SurfacePatchOptions{})
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	cert, e := CertifySurfacePlaneArcs(p, plane, SurfacePlaneArcOptions{})
	if e != nil && !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	found := 0
	for _, point := range []Vec3{{.25, .25, 0}, {.25, .75, 0}, {.75, .25, 0}, {.75, .75, 0}} {
		for _, arc := range cert.Arcs {
			d := arc.source.Domain()
			if point.X < d.UMin || point.X > d.UMax || point.Y < d.VMin || point.Y > d.VMax {
				continue
			}
			free := point.Y
			if arc.Certificate.FixedParameterAxis == 1 {
				free = point.X
			}
			r, e := arc.EncloseInterval(context.Background(), free-.01, free+.01, SurfaceArcIntervalOptions{MaxIterations: 1, ParameterTolerance: 1e-12, BoxTolerance: 1e-12})
			if e != nil && !errors.Is(e, ErrConvergence) {
				t.Fatal(r, e)
			}
			if !r.HasBounds || r.Excluded || !r.Bounds.Contains(point) || r.Parameters.Min.X > point.X || r.Parameters.Max.X < point.X || r.Parameters.Min.Y > point.Y || r.Parameters.Max.Y < point.Y {
				t.Fatal(point, r, e)
			}
			found++
		}
	}
	if found < 4 {
		t.Fatal("missing known points", found)
	}
}
func TestSurfaceArcIntervalLimitsAndCancel(t *testing.T) {
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u - .375} })
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	cert, _ := CertifySurfacePlaneArcs(p, plane, SurfacePlaneArcOptions{})
	arc := cert.Arcs[0]
	r, e := arc.EncloseInterval(distanceCancelContext(2), .2, .7, SurfaceArcIntervalOptions{})
	if !errors.Is(e, context.Canceled) || !r.HasBounds || r.Complete {
		t.Fatal(r, e)
	}
	r, e = arc.EncloseInterval(context.Background(), .2, .7, SurfaceArcIntervalOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) || !r.HasBounds || r.Complete {
		t.Fatal(r, e)
	}
	_, e = arc.EncloseInterval(context.Background(), .3, .3, SurfaceArcIntervalOptions{})
	if !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}

func TestSurfaceArcIntervalCanExcludeBeyondActualProjection(t *testing.T) {
	p, _ := NewSurfacePatch(quadraticContactSurface(t, -.12), SurfacePatchOptions{})
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	cert, e := CertifySurfacePlaneArcs(p, plane, SurfacePlaneArcOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, arc := range cert.Arcs {
		lo := arc.source.Domain().VMin
		bound := min(arc.Endpoints[0].Parameters.Min.Y, arc.Endpoints[1].Parameters.Min.Y)
		if arc.Certificate.FixedParameterAxis == 1 {
			lo = arc.source.Domain().UMin
			bound = min(arc.Endpoints[0].Parameters.Min.X, arc.Endpoints[1].Parameters.Min.X)
		}
		if bound <= lo {
			continue
		}
		r, e := arc.EncloseInterval(context.Background(), lo, lo+(bound-lo)/2, SurfaceArcIntervalOptions{})
		if e != nil || !r.Excluded || !r.Complete {
			t.Fatal(r, e)
		}
		return
	}
	t.Fatal("no projection gap fixture")
}
