package geom

import (
	"errors"
	"testing"
)

func TestSurfaceArcExactFreeEndpointIrrationalDependent(t *testing.T) {
	p, _ := NewSurfacePatch(quadraticContactSurface(t, -.12), SurfacePatchOptions{})
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	result, e := CertifySurfacePlaneArcs(p, plane, SurfacePlaneArcOptions{})
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, arc := range result.Arcs {
		for end := 0; end < 2; end++ {
			_, full := arc.ExactEndpoint(end, ArcExactEndpointOptions{})
			free, e := arc.ExactFreeEndpoint(end, ArcExactEndpointOptions{})
			if errors.Is(full, ErrConvergence) && e == nil {
				found = true
				id := arc.Endpoints[end].identity
				if id.axis != 1-arc.Certificate.FixedParameterAxis || free.Cmp(id.fixed) != 0 {
					t.Fatal("unbound free coordinate")
				}
				free.SetInt64(999)
				again, e := arc.ExactFreeEndpoint(end, ArcExactEndpointOptions{})
				if e != nil || again.Cmp(id.fixed) != 0 {
					t.Fatal("mutable witness")
				}
			}
		}
	}
	if !found {
		t.Fatal("fixture needs irrational dependent endpoint with rational fixed free coordinate")
	}
	if _, e = (SurfacePlaneArc{}).ExactFreeEndpoint(0, ArcExactEndpointOptions{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	if _, e = result.Arcs[0].ExactFreeEndpoint(2, ArcExactEndpointOptions{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}
