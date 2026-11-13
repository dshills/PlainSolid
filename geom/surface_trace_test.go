package geom

import (
	"errors"
	"math/big"
	"testing"
)

func TestTraceSurfacePlaneOpenAndClosed(t *testing.T) {
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u - .375} })
	r, e := TraceSurfacePlane(p, plane, SurfaceTraceOptions{})
	if e != nil || !r.Complete || len(r.Contours) != 1 || r.Contours[0].Closed || len(r.Contours[0].Arcs) != 1 {
		t.Fatalf("%+v %v", r, e)
	}
	s := quadraticContactSurface(t, -.12)
	p, _ = NewSurfacePatch(s, SurfacePatchOptions{})
	r, e = TraceSurfacePlane(p, plane, SurfaceTraceOptions{})
	if e != nil || !r.Complete || len(r.Contours) != 1 || !r.Contours[0].Closed || len(r.Contours[0].Arcs) != len(r.Isolation.Arcs) {
		t.Fatalf("%+v %v", r, e)
	}
	chain := r.Contours[0].Arcs
	for i, oriented := range chain {
		next := chain[(i+1)%len(chain)]
		exit, entry := 1, 0
		if oriented.Reversed {
			exit = 0
		}
		if next.Reversed {
			entry = 1
		}
		a, c := r.Isolation.Arcs[oriented.Index].Endpoints[exit].Parameters, r.Isolation.Arcs[next.Index].Endpoints[entry].Parameters
		if a.Max.X < c.Min.X || c.Max.X < a.Min.X || a.Max.Y < c.Min.Y || c.Max.Y < a.Min.Y {
			t.Fatal("disconnected traversal")
		}
	}
	used := map[int]bool{}
	for _, a := range r.Contours[0].Arcs {
		if used[a.Index] {
			t.Fatal("duplicate arc")
		}
		used[a.Index] = true
	}
}
func TestTraceSurfacePlaneRationalOpen(t *testing.T) {
	p, _ := NewSurfacePatch(quarterCylinder(t), SurfacePatchOptions{})
	plane, _ := NewPlane(Vec3{Z: .75}, Vec3{Z: 1})
	r, e := TraceSurfacePlane(p, plane, SurfaceTraceOptions{})
	if e != nil || !r.Complete || len(r.Contours) != 1 || r.Contours[0].Closed {
		t.Fatalf("%+v %v", r, e)
	}
}
func TestTraceSurfacePlaneUnresolvedAndBudget(t *testing.T) {
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, (u - .5) * (v - .5)} })
	r, e := TraceSurfacePlane(p, plane, SurfaceTraceOptions{Arcs: SurfacePlaneArcOptions{MaxDepth: 6}})
	if !errors.Is(e, ErrConvergence) || r.Complete {
		t.Fatalf("%+v %v", r, e)
	}
	s := quadraticContactSurface(t, -.12)
	p, _ = NewSurfacePatch(s, SurfacePatchOptions{})
	r, e = TraceSurfacePlane(p, plane, SurfaceTraceOptions{MaxJoinComparisons: 1})
	if !errors.Is(e, ErrConvergence) || r.Complete || len(r.Isolation.Arcs) == 0 {
		t.Fatalf("%+v %v", r, e)
	}
}
func TestExactSurfaceRootIdentityRejectsOverlappingDifferentRoots(t *testing.T) {
	// Both intervals overlap, but the exact roots differ by 2^-80, well below
	// a float ULP at 1. No spatial or rounded-parameter coincidence is sufficient.
	root := big.NewRat(1, 1)
	other := new(big.Rat).Add(root, new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 80)))
	a := &surfaceRootIdentity{axis: 0, fixed: big.NewRat(0, 1), lo: big.NewRat(0, 1), hi: big.NewRat(2, 1), start: 0, end: 1, polynomial: rationalPolynomial{new(big.Rat).Neg(root), big.NewRat(1, 1)}}
	c := *a
	c.polynomial = rationalPolynomial{new(big.Rat).Neg(other), big.NewRat(1, 1)}
	b := &polynomialBudget{maxOperations: 10000, maxBits: 1024}
	same, e := exactSurfaceRootMatch(a, &c, b)
	if e != nil || same {
		t.Fatalf("%v %v", same, e)
	}
	c.polynomial = clonePolynomial(a.polynomial)
	same, e = exactSurfaceRootMatch(a, &c, b)
	if e != nil || !same {
		t.Fatalf("%v %v", same, e)
	}
}

func TestTraceSurfacePlaneBoundaryAndKnotJoins(t *testing.T) {
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u} })
	r, e := TraceSurfacePlane(p, plane, SurfaceTraceOptions{})
	if e != nil || len(r.Contours) != 1 || r.Contours[0].Closed {
		t.Fatalf("%+v %v", r, e)
	}
	// A nonuniform knot cut creates two distinct local polynomial coordinate
	// systems. The endpoint equality must survive exact affine conversion.
	s, e := NewBSplineSurface(1, 1, 2, 3, []Vec3{{0, 0, -.375}, {0, .3, -.375}, {0, 1, -.375}, {1, 0, .625}, {1, .3, .625}, {1, 1, .625}}, []float64{0, 0, 1, 1}, []float64{0, 0, .3, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, _ = NewSurfacePatch(s, SurfacePatchOptions{})
	r, e = TraceSurfacePlane(p, plane, SurfaceTraceOptions{})
	if e != nil || len(r.Contours) != 1 || len(r.Contours[0].Arcs) != 2 || r.Contours[0].Closed {
		t.Fatalf("%+v %v", r, e)
	}
}

func TestTraceSurfacePlaneCanonicalInternalBoundary(t *testing.T) {
	// x=u; z=u²-.25. On u in [0,1] there is one regular vertical arc.
	poles := []Vec3{}
	for i, z := range []float64{-.25, -.25, .75} {
		for j := 0; j < 2; j++ {
			poles = append(poles, Vec3{float64(i) / 2, float64(j), z})
		}
	}
	s, e := NewBSplineSurface(2, 1, 3, 2, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	r, e := TraceSurfacePlane(p, plane, SurfaceTraceOptions{})
	if e != nil || !r.Complete || len(r.Contours) != 1 || r.Contours[0].Closed {
		t.Fatalf("%+v %v", r, e)
	}
	duplicate := false
	for i, j := range r.CanonicalArcs {
		if i != j {
			duplicate = true
		}
	}
	if !duplicate {
		t.Fatal("fixture did not create duplicate boundary arcs")
	}
	for _, entry := range r.Contours[0].Arcs {
		if r.CanonicalArcs[entry.Index] != entry.Index {
			t.Fatal("traversed duplicate")
		}
	}
}

func TestBoundaryArcIdentityRequiresEntireGeometry(t *testing.T) {
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	left := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u - 1} })
	right := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u} })
	l, _ := CertifySurfacePlaneArcs(left, plane, SurfacePlaneArcOptions{})
	r, _ := CertifySurfacePlaneArcs(right, plane, SurfacePlaneArcOptions{})
	a, c := l.Arcs[0], r.Arcs[0]
	// Supply matching parameter endpoint identities but incompatible spatial
	// homogeneous controls. Endpoint agreement alone must never be sufficient.
	c.Endpoints = a.Endpoints
	c.source = right
	budget := &polynomialBudget{maxOperations: 10000, maxBits: 1024}
	same, e := sameBoundaryArc(a, c, budget)
	if e != nil || same {
		t.Fatal(same, e)
	}
}
