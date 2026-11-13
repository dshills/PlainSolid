package geom

import (
	"errors"
	"math/big"
	"testing"
)

func TestSurfaceArcExactEndpointLinearRational(t *testing.T) {
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 3*u - 1} })
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	result, e := CertifySurfacePlaneArcs(p, plane, SurfacePlaneArcOptions{})
	if e != nil {
		t.Fatal(e)
	}
	arc := result.Arcs[0]
	for i := 0; i < 2; i++ {
		endpoint, e := arc.ExactEndpoint(i, ArcExactEndpointOptions{})
		if e != nil || endpoint.Parameters[0].Cmp(big.NewRat(1, 3)) != 0 || endpoint.Point[0].Cmp(big.NewRat(1, 3)) != 0 || endpoint.Point[2].Sign() != 0 {
			t.Fatal(endpoint, e)
		}
		endpoint.Parameters[0].SetInt64(8)
		again, e := arc.ExactEndpoint(i, ArcExactEndpointOptions{})
		if e != nil || again.Parameters[0].Cmp(big.NewRat(1, 3)) != 0 {
			t.Fatal(again, e)
		}
	}
}
func TestSurfaceArcExactEndpointDyadicAndIrrational(t *testing.T) {
	p, _ := NewSurfacePatch(quadraticContactSurface(t, -.12), SurfacePatchOptions{})
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	result, e := CertifySurfacePlaneArcs(p, plane, SurfacePlaneArcOptions{})
	if e != nil {
		t.Fatal(e)
	}
	unresolved := false
	for _, arc := range result.Arcs {
		for i := 0; i < 2; i++ {
			_, e := arc.ExactEndpoint(i, ArcExactEndpointOptions{})
			if errors.Is(e, ErrConvergence) {
				unresolved = true
			} else if e != nil {
				t.Fatal(e)
			}
		}
	}
	if !unresolved {
		t.Fatal("expected algebraic endpoint")
	}
	p = arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u - .375} })
	result, e = CertifySurfacePlaneArcs(p, plane, SurfacePlaneArcOptions{})
	if e != nil {
		t.Fatal(e)
	}
	endpoint, e := result.Arcs[0].ExactEndpoint(0, ArcExactEndpointOptions{})
	if e != nil || endpoint.Parameters[0].Cmp(big.NewRat(3, 8)) != 0 {
		t.Fatal(endpoint, e)
	}
	_, e = result.Arcs[0].ExactEndpoint(0, ArcExactEndpointOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
}

func TestSurfaceArcExactEndpointQuadraticRational(t *testing.T) {
	var poles []Vec3
	for i, z := range []float64{-1, -1, 8} {
		for _, v := range []float64{0, 1} {
			poles = append(poles, Vec3{float64(i) / 2, v, z})
		}
	}
	s, e := NewBSplineSurface(2, 1, 3, 2, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	result, e := CertifySurfacePlaneArcs(p, plane, SurfacePlaneArcOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, arc := range result.Arcs {
		for i := 0; i < 2; i++ {
			endpoint, e := arc.ExactEndpoint(i, ArcExactEndpointOptions{})
			if e != nil || endpoint.Parameters[0].Cmp(big.NewRat(1, 3)) != 0 {
				t.Fatal(endpoint, e)
			}
		}
	}
}
