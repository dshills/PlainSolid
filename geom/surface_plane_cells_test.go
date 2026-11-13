package geom

import (
	"errors"
	"testing"
)

func quadraticContactSurface(t *testing.T, offset float64) *BSplineSurface {
	t.Helper()
	coefficients := []float64{.25, -.25, .25}
	poles := make([]Vec3, 9)
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			poles[i*3+j] = Vec3{float64(i) / 2, float64(j) / 2, coefficients[i] + coefficients[j] + offset}
		}
	}
	surface, e := NewBSplineSurface(2, 2, 3, 3, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	return surface
}
func coveredByCells(result SurfacePlaneIsolation, u, v float64) bool {
	for _, cell := range result.Candidates {
		r := cell.Domain
		if u >= r.UMin && u <= r.UMax && v >= r.VMin && v <= r.VMax {
			return true
		}
	}
	for _, r := range result.Coplanar {
		if u >= r.UMin && u <= r.UMax && v >= r.VMin && v <= r.VMax {
			return true
		}
	}
	for _, r := range result.Pending {
		if u >= r.UMin && u <= r.UMax && v >= r.VMin && v <= r.VMax {
			return true
		}
	}
	return false
}
func TestSurfacePlaneTangentCandidateCoverage(t *testing.T) {
	surface := quadraticContactSurface(t, 0)
	patch, e := NewSurfacePatch(surface, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	plane, _ := NewPlane(Vec3{}, Vec3{0, 0, 1})
	result, e := BoundSurfacePlane(patch, plane, SurfacePlaneIsolationOptions{ParameterTolerance: 1.0 / 16})
	if e != nil || !result.Complete || !coveredByCells(result, .5, .5) || len(result.Coplanar) != 0 {
		t.Fatal(result, e)
	}
	for _, cell := range result.Candidates {
		if !cell.AtRequestedResolution || cell.Domain.UMax-cell.Domain.UMin > 1.0/16 || cell.Domain.VMax-cell.Domain.VMin > 1.0/16 {
			t.Fatal(cell)
		}
	}
	clear, e := NewSurfacePatch(quadraticContactSurface(t, .01), SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	result, e = BoundSurfacePlane(clear, plane, SurfacePlaneIsolationOptions{})
	if e != nil || !result.Complete || len(result.Candidates) != 0 || len(result.Coplanar) != 0 {
		t.Fatal(result, e)
	}
}
func TestSurfacePlaneSaddleAndKnots(t *testing.T) {
	surface, _ := NewBSplineSurface(1, 1, 2, 2, []Vec3{{0, 0, .25}, {0, 1, -.25}, {1, 0, -.25}, {1, 1, .25}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	patch, _ := NewSurfacePatch(surface, SurfacePatchOptions{})
	plane, _ := NewPlane(Vec3{}, Vec3{0, 0, 1})
	result, e := BoundSurfacePlane(patch, plane, SurfacePlaneIsolationOptions{ParameterTolerance: .125})
	if e != nil || !result.Complete {
		t.Fatal(result, e)
	}
	for i := 0; i <= 32; i++ {
		u := float64(i) / 32
		if !coveredByCells(result, u, .5) || !coveredByCells(result, .5, u) {
			t.Fatal("lost saddle branch", u)
		}
	}
	kink, _ := NewBSplineSurface(1, 1, 3, 2, []Vec3{{0, 0, .5}, {0, 1, .5}, {.5, 0, 0}, {.5, 1, 0}, {1, 0, .5}, {1, 1, .5}}, []float64{0, 0, .5, 1, 1}, []float64{0, 0, 1, 1}, nil)
	patch, _ = NewSurfacePatch(kink, SurfacePatchOptions{})
	result, e = BoundSurfacePlane(patch, plane, SurfacePlaneIsolationOptions{ParameterTolerance: .125})
	if e != nil || !result.Complete {
		t.Fatal(result, e)
	}
	for i := 0; i <= 16; i++ {
		if !coveredByCells(result, .5, float64(i)/16) {
			t.Fatal("lost knot contact")
		}
	}
	for _, cell := range result.Candidates {
		if cell.Domain.UMin < .5 && cell.Domain.UMax > .5 {
			t.Fatal("cell crosses original knot", cell)
		}
	}
}
func TestSurfacePlaneCoplanarAndResourceDiagnostics(t *testing.T) {
	plane, _ := NewPlane(Vec3{}, Vec3{0, 0, 1})
	flat, _ := NewBSplineSurface(1, 1, 2, 2, []Vec3{{0, 0, 0}, {0, 1, 0}, {1, 0, 0}, {1, 1, 0}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, []float64{1, 2, 3, 4})
	patch, _ := NewSurfacePatch(flat, SurfacePatchOptions{})
	result, e := BoundSurfacePlane(patch, plane, SurfacePlaneIsolationOptions{})
	if e != nil || !result.Complete || len(result.Coplanar) != 1 || len(result.Candidates) != 0 {
		t.Fatal(result, e)
	}
	patch, _ = NewSurfacePatch(quadraticContactSurface(t, 0), SurfacePatchOptions{})
	for _, options := range []SurfacePlaneIsolationOptions{{MaxDepth: 1}, {MaxCells: 1}, {MaxOperations: 1}, {MaxQueuedPoles: 4}} {
		result, e = BoundSurfacePlane(patch, plane, options)
		if !errors.Is(e, ErrConvergence) || result.Complete || !coveredByCells(result, .5, .5) {
			t.Fatal(options, result, e)
		}
	}
}
