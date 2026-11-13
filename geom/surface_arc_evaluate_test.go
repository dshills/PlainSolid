package geom

import (
	"errors"
	"math"
	"math/big"
	"testing"
)

func TestSurfaceArcExactEvaluation(t *testing.T) {
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u - .375} })
	r, e := CertifySurfacePlaneArcs(p, plane, SurfacePlaneArcOptions{})
	if e != nil {
		t.Fatal(e)
	}
	a := r.Arcs[0]
	for i := 0; i <= 10; i++ {
		v := big.NewRat(int64(i), 10)
		point, e := a.EvaluateExact(v, ArcEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		assertExactPointInBox(t, [3]*big.Rat{big.NewRat(3, 8), v, big.NewRat(0, 1)}, point.Bounds)
		if point.DependentDerivative.Min != 0 || point.DependentDerivative.Max != 0 {
			t.Fatal(point)
		}
	}
	if _, e = a.Evaluate(-.1, ArcEvaluationOptions{}); !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}
	if _, e = a.Evaluate(.5, ArcEvaluationOptions{MaxOperations: 1}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
}
func TestSurfaceArcCurvedEvaluationAndCanonicalEndpoints(t *testing.T) {
	s := quadraticContactSurface(t, -.12)
	p, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	r, e := TraceSurfacePlane(p, plane, SurfaceTraceOptions{})
	if e != nil {
		t.Fatal(e)
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
		a, c := r.Isolation.Arcs[oriented.Index], r.Isolation.Arcs[next.Index]
		ep, e := a.EndpointEvaluation(exit)
		if e != nil {
			t.Fatal(e)
		}
		cp, e := c.EndpointEvaluation(entry)
		if e != nil {
			t.Fatal(e)
		}
		if ep.Point != cp.Point || ep.Bounds != cp.Bounds || ep.Parameters != cp.Parameters {
			t.Fatal("noncanonical endpoints")
		}
		lo, hi := a.Endpoints[0].Parameters, a.Endpoints[1].Parameters
		free := (lo.Min.Y + hi.Max.Y) / 2
		if a.Certificate.FixedParameterAxis == 1 {
			free = (lo.Min.X + hi.Max.X) / 2
		}
		value, e := a.Evaluate(free, ArcEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if value.Bounds.Min.Z > 0 || value.Bounds.Max.Z < 0 {
			t.Fatal(value)
		}
		x, y := value.Point.X-.5, value.Point.Y-.5
		residual := x*x + y*y - .12
		if residual < -1e-8 || residual > 1e-8 {
			t.Fatal(residual)
		}
	}
}
func TestSurfaceArcActualFreeDomain(t *testing.T) {
	// The half-circle arc projection occupies only part of the cell's free axis.
	s := quadraticContactSurface(t, -.12)
	p, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	r, e := CertifySurfacePlaneArcs(p, plane, SurfacePlaneArcOptions{})
	if e != nil {
		t.Fatal(e)
	}
	rejected := false
	for _, a := range r.Arcs {
		free := a.Domain.VMin
		if a.Certificate.FixedParameterAxis == 1 {
			free = a.Domain.UMin
		}
		_, e := a.Evaluate(free, ArcEvaluationOptions{})
		if errors.Is(e, ErrDomain) {
			rejected = true
		} else if e != nil {
			t.Fatal(e)
		}
	}
	if !rejected {
		t.Fatal("fixture did not test an outside projection")
	}
}

func TestArcRepresentativeSubnormal(t *testing.T) {
	p := Vec3{math.SmallestNonzeroFloat64, -math.SmallestNonzeroFloat64, math.MaxFloat64}
	if got := boxRepresentative(Box{p, p}); got != p {
		t.Fatalf("%v", got)
	}
}
