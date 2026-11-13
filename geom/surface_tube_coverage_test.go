package geom

import (
	"errors"
	"testing"
)

func TestSurfaceTubeCoverageOutsideDependentSlabs(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{s, .5, t - .5} })
	tube := SurfacePairParameterBox{ParameterRectangle{.3, .7, .4, .6}, ParameterRectangle{.2, .8, .4, .6}}
	candidate := SurfacePairParameterBox{ParameterRectangle{.35, .65, 0, 1}, ParameterRectangle{0, 1, 0, 1}}
	r, e := CertifySurfacePairCellInTube(a, b, candidate, tube, SurfaceTubeCoverageOptions{})
	if e != nil || !r.Covered || len(r.OutsideExcluded) == 0 {
		t.Fatal(r, e)
	}
}
func TestSurfaceTubeCoverageRejectsOutsideBranch(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{s, .5, t - .5} })
	tube := SurfacePairParameterBox{ParameterRectangle{.3, .7, .4, .6}, ParameterRectangle{.2, .8, .4, .6}}
	candidate := SurfacePairParameterBox{ParameterRectangle{.2, .8, 0, 1}, ParameterRectangle{0, 1, 0, 1}}
	r, e := CertifySurfacePairCellInTube(a, b, candidate, tube, SurfaceTubeCoverageOptions{MaxDepth: 3})
	if !errors.Is(e, ErrConvergence) || r.Covered || len(r.Unresolved) == 0 {
		t.Fatal(r, e)
	}
}
func TestSurfaceTubeCoverageContainedAndBudget(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{s, .5, t - .5} })
	tube := SurfacePairParameterBox{ParameterRectangle{.3, .7, .4, .6}, ParameterRectangle{.2, .8, .4, .6}}
	r, e := CertifySurfacePairCellInTube(a, b, tube, tube, SurfaceTubeCoverageOptions{})
	if e != nil || !r.Covered || r.VisitedCells != 0 {
		t.Fatal(r, e)
	}
	candidate := SurfacePairParameterBox{ParameterRectangle{.35, .65, 0, 1}, ParameterRectangle{0, 1, 0, 1}}
	r, e = CertifySurfacePairCellInTube(a, b, candidate, tube, SurfaceTubeCoverageOptions{MaxCells: 1})
	if !errors.Is(e, ErrConvergence) || r.Covered || len(r.Pending) == 0 {
		t.Fatal(r, e)
	}
}
