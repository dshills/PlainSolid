package geom

import (
	"errors"
	"math"
	"testing"
)

func arcTestPatch(t *testing.T, f func(float64, float64) Vec3) *SurfacePatch {
	t.Helper()
	poles := []Vec3{f(0, 0), f(0, 1), f(1, 0), f(1, 1)}
	s, e := NewBSplineSurface(1, 1, 2, 2, poles, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func TestCertifiedSurfacePlaneStraightArcs(t *testing.T) {
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	cases := []struct {
		name     string
		f        func(float64, float64) Vec3
		boundary bool
		corners  bool
	}{
		{"interior", func(u, v float64) Vec3 { return Vec3{u, v, u - .375} }, false, false},
		{"diagonal", func(u, v float64) Vec3 { return Vec3{u, v, u - v} }, false, true},
		{"boundary", func(u, v float64) Vec3 { return Vec3{u, v, u} }, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := arcTestPatch(t, tc.f)
			r, e := CertifySurfacePlaneArcs(p, plane, SurfacePlaneArcOptions{})
			if e != nil || !r.Complete || len(r.Arcs) != 1 {
				t.Fatalf("%+v %v", r, e)
			}
			a := r.Arcs[0]
			if a.OnBoundary != tc.boundary || a.Certificate.WorldDerivative.Sign == 0 {
				t.Fatal(a)
			}
			for _, ep := range a.Endpoints {
				if (ep.CornerIndex >= 0) != tc.corners {
					t.Fatal(ep)
				}
				if ep.Bounds.Min.Z > 0 || ep.Bounds.Max.Z < 0 {
					t.Fatal(ep)
				}
			}
		})
	}
}
func TestCertifiedSurfacePlaneSingularAndDegenerate(t *testing.T) {
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	cases := []*SurfacePatch{arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, 0, u - .375} }), arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u + v} }), arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, (u - .5) * (v - .5)} })}
	s := quadraticContactSurface(t, 0)
	p, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cases = append(cases, p)
	for i, p := range cases {
		r, e := CertifySurfacePlaneArcs(p, plane, SurfacePlaneArcOptions{MaxDepth: 6})
		if !errors.Is(e, ErrConvergence) || r.Complete || len(r.Unresolved)+len(r.Pending) == 0 {
			t.Fatalf("case %d: %+v %v", i, r, e)
		}
	}
}
func TestCertifiedSurfacePlaneCoplanarAndBudget(t *testing.T) {
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	r, e := CertifySurfacePlaneArcs(p, plane, SurfacePlaneArcOptions{})
	if e != nil || !r.Complete || len(r.Coplanar) != 1 {
		t.Fatalf("%+v %v", r, e)
	}
	p = arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u - .37} })
	r, e = CertifySurfacePlaneArcs(p, plane, SurfacePlaneArcOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) || r.Complete || len(r.Pending) != 1 {
		t.Fatalf("%+v %v", r, e)
	}
}
func TestCertifiedSurfacePlaneParabola(t *testing.T) {
	// z=(u-.5)^2+v-.125; its bottom-edge roots are irrational.
	poles := []Vec3{}
	z := []float64{.125, -.375, .125}
	for i := 0; i < 3; i++ {
		for j := 0; j < 2; j++ {
			poles = append(poles, Vec3{float64(i) / 2, float64(j), z[i] + float64(j)})
		}
	}
	s, e := NewBSplineSurface(2, 1, 3, 2, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	r, e := CertifySurfacePlaneArcs(p, plane, SurfacePlaneArcOptions{})
	if e != nil || !r.Complete || len(r.Arcs) != 1 {
		t.Fatalf("%+v %v", r, e)
	}
	a := r.Arcs[0]
	if a.EdgeRootCounts[SurfaceEdgeVMin] != 2 || a.Certificate.FixedParameterAxis != 1 {
		t.Fatal(a)
	}
	for _, ep := range a.Endpoints {
		if ep.Bounds.Min.Z > 0 || ep.Bounds.Max.Z < 0 || ep.Parameters.Max.X-ep.Parameters.Min.X > 1e-8 {
			t.Fatal(ep)
		}
	}
}

func TestCertifiedSurfacePlaneRationalCylinder(t *testing.T) {
	s := quarterCylinder(t)
	p, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	plane, _ := NewPlane(Vec3{Z: .75}, Vec3{Z: 1})
	r, e := CertifySurfacePlaneArcs(p, plane, SurfacePlaneArcOptions{MaxDepth: 16})
	if e != nil || !r.Complete || len(r.Arcs) == 0 {
		t.Fatalf("%+v %v", r, e)
	}
	for _, a := range r.Arcs {
		for _, ep := range a.Endpoints {
			if ep.Bounds.Min.Z > .75 || ep.Bounds.Max.Z < .75 {
				t.Fatal(ep)
			}
		}
	}
}
func TestCertifiedSurfacePlaneRefinesAndCovers(t *testing.T) {
	s := quadraticContactSurface(t, -.12)
	p, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	r, e := CertifySurfacePlaneArcs(p, plane, SurfacePlaneArcOptions{MaxDepth: 10})
	if e != nil || !r.Complete || len(r.Arcs) < 4 {
		t.Fatalf("%+v %v", r, e)
	}
	// The closed circle is covered by individually monotone simple arcs.
	for i := 0; i < 100; i++ {
		theta := 2 * math.Pi * float64(i) / 100
		u, v := .5+math.Sqrt(.12)*math.Cos(theta), .5+math.Sqrt(.12)*math.Sin(theta)
		covered := false
		for _, a := range r.Arcs {
			d := a.Domain
			if u >= d.UMin && u <= d.UMax && v >= d.VMin && v <= d.VMax {
				covered = true
			}
		}
		if !covered {
			t.Fatalf("missing %g %g", u, v)
		}
	}
}

func TestCertifiedSurfacePlaneCornerTouchRemainsUnresolved(t *testing.T) {
	// The circle passes exactly through dyadic split corners. Neighboring cells
	// can certify arcs, but a cell that contains only that corner is not an arc.
	s := quadraticContactSurface(t, -.125)
	p, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	r, e := CertifySurfacePlaneArcs(p, plane, SurfacePlaneArcOptions{MaxDepth: 6})
	if !errors.Is(e, ErrConvergence) || r.Complete || len(r.Arcs) == 0 || len(r.Unresolved) == 0 {
		t.Fatalf("%+v %v", r, e)
	}
	trace, e := TraceSurfacePlane(p, plane, SurfaceTraceOptions{Arcs: SurfacePlaneArcOptions{ResolveSubdivisionCorners: true}})
	if e != nil || !trace.Complete || len(trace.Isolation.CornerContacts) == 0 || len(trace.Contours) != 1 || !trace.Contours[0].Closed {
		t.Fatal(trace.Complete, len(trace.Isolation.CornerContacts), len(trace.Contours), trace.Unresolved, e)
	}
}

func TestCertifiedSurfacePlaneMaximumCellWidth(t *testing.T) {
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u - .375} })
	r, e := CertifySurfacePlaneArcs(p, plane, SurfacePlaneArcOptions{MaxCellParameterWidth: .25})
	if e != nil || !r.Complete || len(r.Arcs) < 4 {
		t.Fatal(r, e)
	}
	for _, a := range r.Arcs {
		if a.Domain.UMax-a.Domain.UMin > .25 || a.Domain.VMax-a.Domain.VMin > .25 {
			t.Fatal(a)
		}
	}
	r, e = CertifySurfacePlaneArcs(p, plane, SurfacePlaneArcOptions{MaxCellParameterWidth: .01, MaxDepth: 2})
	if !errors.Is(e, ErrConvergence) || r.Complete || len(r.Unresolved) == 0 {
		t.Fatal(r, e)
	}
}
