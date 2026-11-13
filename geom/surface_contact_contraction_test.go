package geom

import (
	"errors"
	"testing"
)

func TestWeakSurfaceExtremumInteriorCoefficientIsNotContact(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	poles := []Vec3{{0, 0, 1}, {0, 1, 1}, {.5, 0, 0}, {.5, 1, 0}, {1, 0, 1}, {1, 1, 1}}
	s, _ := NewBSplineSurface(2, 1, 3, 2, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 1, 1}, nil)
	b, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	_, excluded, _, e := weakSurfaceContactBounds(a, b, SurfacePairParameterBox{a.Domain(), b.Domain()}, 1000000, 16384)
	if e != nil || !excluded {
		t.Fatal(excluded, e)
	}
}
func TestTubeCoverageClosedBoundaryLineAndUncoveredTail(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{2*u - 1, v, 0} })
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{2*s - 1, t, 2*s - 1} })
	tube := SurfacePairTubeRegion{Box: SurfacePairParameterBox{A: ParameterRectangle{UMin: .4, UMax: .6, VMin: .2, VMax: .8}, B: ParameterRectangle{UMin: .4, UMax: .6, VMin: .1, VMax: .9}}, FreeAxis: 1}
	candidate := SurfacePairParameterBox{A: ParameterRectangle{UMin: .5, UMax: 1, VMin: 0, VMax: .2}, B: ParameterRectangle{UMin: .5, UMax: 1, VMin: .2, VMax: 1}}
	r, e := CertifySurfacePairCellInTubes(a, b, candidate, []SurfacePairTubeRegion{tube}, SurfaceTubeUnionCoverageOptions{})
	if e != nil || !r.Covered || len(r.BoundaryCovered) == 0 {
		t.Fatal(r, e)
	}
	// An actual line portion outside the verified free interval must remain open.
	candidate.A.VMin, candidate.A.VMax = .2, .95
	candidate.B.VMin = 0
	r, e = CertifySurfacePairCellInTubes(a, b, candidate, []SurfacePairTubeRegion{tube}, SurfaceTubeUnionCoverageOptions{MaxDepth: 3, MaxCells: 128, MaxOperations: 1000000})
	if !errors.Is(e, ErrConvergence) || r.Covered {
		t.Fatal(r, e)
	}
}
