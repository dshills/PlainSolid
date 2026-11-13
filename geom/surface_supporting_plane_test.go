package geom

import (
	"context"
	"errors"
	"testing"
)

func TestSurfaceSupportingPlaneBoundaryPatterns(t *testing.T) {
	for _, tc := range []struct {
		f            func(float64, float64) float64
		side, corner uint8
		coincident   bool
	}{{func(u, v float64) float64 { return u * (1 + v) }, 1, 0, false}, {func(u, v float64) float64 { return u + v }, 0, 1, false}, {func(u, v float64) float64 { return 1 + u*v }, 0, 0, false}, {func(u, v float64) float64 { return 0 }, 0, 0, true}} {
		p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, tc.f(u, v)} })
		r, e := CertifySurfaceSupportingPlane(context.Background(), p, Vec3{}, Vec3{1, 0, 0}, Vec3{0, 1, 0}, SurfaceSupportingPlaneOptions{})
		if e != nil || !r.Complete || r.Coincident != tc.coincident || r.SideMask != tc.side || r.CornerMask != tc.corner || (!tc.coincident && r.HalfspaceSign != 1) {
			t.Fatal(r, e)
		}
	}
}
func TestSurfaceSupportingPlaneObliqueRawDeterminant(t *testing.T) {
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, v - u + u*v} })
	r, e := CertifySurfaceSupportingPlane(context.Background(), p, Vec3{}, Vec3{1, 1, 0}, Vec3{0, 1, 1}, SurfaceSupportingPlaneOptions{})
	if e != nil || !r.Complete || r.HalfspaceSign != 1 || r.SideMask != 5 || r.CornerMask != 0 {
		t.Fatal(r, e)
	}
	r, e = CertifySurfaceSupportingPlane(context.Background(), p, Vec3{}, Vec3{0, 1, 1}, Vec3{1, 1, 0}, SurfaceSupportingPlaneOptions{})
	if e != nil || r.HalfspaceSign != -1 || r.SideMask != 5 {
		t.Fatal(r, e)
	}
}
func TestSurfaceSupportingPlaneRejectsInteriorAndMixed(t *testing.T) {
	for _, z := range [][]float64{{.25, -.25, .25}, {-1, 0, 1}} {
		poles := make([]Vec3, 6)
		for i := 0; i < 3; i++ {
			for j := 0; j < 2; j++ {
				poles[2*i+j] = Vec3{float64(i) / 2, float64(j), z[i]}
			}
		}
		s, _ := NewBSplineSurface(2, 1, 3, 2, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 1, 1}, nil)
		p, _ := NewSurfacePatch(s, SurfacePatchOptions{})
		r, e := CertifySurfaceSupportingPlane(context.Background(), p, Vec3{}, Vec3{1, 0, 0}, Vec3{0, 1, 0}, SurfaceSupportingPlaneOptions{})
		if !errors.Is(e, ErrConvergence) || r.Complete || r.HalfspaceSign != 0 || r.SideMask != 0 {
			t.Fatal(r, e)
		}
	}
}
func TestSurfaceSupportingPlaneBudgetAndOrientation(t *testing.T) {
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u * v} })
	r, e := CertifySurfaceSupportingPlane(context.Background(), p, Vec3{}, Vec3{1, 0, 0}, Vec3{0, 1, 0}, SurfaceSupportingPlaneOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) || r.Complete {
		t.Fatal(r, e)
	}
	_, e = CertifySurfaceSupportingPlane(context.Background(), p, Vec3{}, Vec3{1, 0, 0}, Vec3{2, 0, 0}, SurfaceSupportingPlaneOptions{})
	if !errors.Is(e, ErrDegenerate) {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = CertifySurfaceSupportingPlane(ctx, p, Vec3{}, Vec3{1, 0, 0}, Vec3{0, 1, 0}, SurfaceSupportingPlaneOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	injective, e := CertifySurfaceInjectivity(p, SurfaceInjectivityOptions{})
	if e != nil || !injective.ProvenInjective || injective.ProjectionOrientation != 1 {
		t.Fatal(injective, e)
	}
	injective, e = CertifySurfaceInjectivity(p, SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 0}})
	if e != nil || !injective.ProvenInjective || injective.ProjectionOrientation != -1 {
		t.Fatal(injective, e)
	}
}

func TestSurfaceSupportingPlaneSubdividesStrictPositive(t *testing.T) {
	poles := []Vec3{{0, 0, .375}, {0, 1, .375}, {.5, 0, -.125}, {.5, 1, -.125}, {1, 0, .375}, {1, 1, .375}}
	s, _ := NewBSplineSurface(2, 1, 3, 2, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 1, 1}, nil)
	p, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	r, e := CertifySurfaceSupportingPlane(context.Background(), p, Vec3{}, Vec3{1, 0, 0}, Vec3{0, 1, 0}, SurfaceSupportingPlaneOptions{})
	if e != nil || !r.Complete || r.HalfspaceSign != 1 || r.SideMask != 0 || r.CornerMask != 0 || r.VisitedCells < 3 {
		t.Fatal(r, e)
	}
}
