package geom

import (
	"context"
	"errors"
	"testing"
)

func TestExactSurfaceKnotInsertionPreservesShapeAndExport(t *testing.T) {
	original, _ := NewSurfacePatch(quadraticContactSurface(t, -25.0/256), SurfacePatchOptions{})
	refined, e := original.InsertUKnot(.3125, 2, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	refined, e = refined.InsertUKnot(.6875, 2, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	refined, e = refined.InsertVKnot(.25, 2, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	refined, e = refined.InsertVKnot(.75, 2, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	proof, e := CompareExactSurfaces(context.Background(), original, refined, ExactSurfaceComparisonOptions{})
	if e != nil || !proof.Equal {
		t.Fatal(proof, e)
	}
	exported, e := refined.ToBSplineSurface()
	if e != nil {
		t.Fatal(e)
	}
	roundtrip, e := NewSurfacePatch(exported, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	proof, e = CompareExactSurfaces(context.Background(), original, roundtrip, ExactSurfaceComparisonOptions{})
	if e != nil || !proof.Equal {
		t.Fatal(proof, e)
	}
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	arcs, e := CertifySurfacePlaneArcs(refined, plane, SurfacePlaneArcOptions{})
	if e != nil && !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	exactEndpoints := 0
	for _, arc := range arcs.Arcs {
		for i := 0; i < 2; i++ {
			_, e := arc.ExactEndpoint(i, ArcExactEndpointOptions{})
			if e == nil {
				exactEndpoints++
			} else if !errors.Is(e, ErrConvergence) {
				t.Fatal(e)
			}
		}
	}
	if exactEndpoints == 0 {
		t.Fatal("no exact circle junctions")
	}
}
func TestExactSurfaceKnotInsertionLimits(t *testing.T) {
	p, _ := NewSurfacePatch(quarterCylinder(t), SurfacePatchOptions{})
	_, e := p.InsertUKnot(.3, 3, SurfacePatchOptions{})
	if !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	_, e = p.InsertVKnot(0, 1, SurfacePatchOptions{})
	if !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}
	_, e = p.InsertUKnot(.3, 1, SurfacePatchOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	q, e := p.InsertUKnot(.3, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	proof, e := CompareExactSurfaces(context.Background(), p, q, ExactSurfaceComparisonOptions{})
	if e != nil || !proof.Equal {
		t.Fatal(proof, e)
	}
}
