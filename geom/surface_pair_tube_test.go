package geom

import (
	"errors"
	"testing"
)

func TestSurfacePairTubeAffineUniformBranch(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	a, _ = a.Trim(ParameterRectangle{.2, .8, 0, 1})
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{s, .4, t - .3} })
	r, e := CertifySurfacePairTube(a, b, SurfacePairTubeOptions{FreeAxis: 0})
	if e != nil || !r.ProvenBranch || r.ExcludesContact || r.Test.ContractionBound != 0 {
		t.Fatal(r, e)
	}
	swapped, e := CertifySurfacePairTube(b, a, SurfacePairTubeOptions{FreeAxis: 2})
	if e != nil || !swapped.ProvenBranch || swapped.ExcludesContact {
		t.Fatal(swapped, e)
	}
	if r.FreeParameter.Min != .2 || r.FreeParameter.Max != .8 {
		t.Fatal(r)
	}
	for i := 0; i <= 10; i++ {
		free := .2 + .6*float64(i)/10
		want := [4]float64{free, .4, free, .3}
		for j, axis := range r.Test.ActiveAxes {
			bound := r.Test.KrawczykBounds[j]
			if want[axis] < bound.Min-1e-15 || want[axis] > bound.Max+1e-15 {
				t.Fatal(r, want)
			}
		}
	}
}
func TestSurfacePairTubeNonlinearAndRational(t *testing.T) {
	bowl, _ := NewSurfacePatch(quadraticContactSurface(t, 0), SurfacePatchOptions{})
	bowl, _ = bowl.Trim(ParameterRectangle{.35, .45, .4, .6})
	plane := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{s, .5, t - .1} })
	r, e := CertifySurfacePairTube(bowl, plane, SurfacePairTubeOptions{FreeAxis: 0})
	if e != nil || !r.ProvenBranch {
		t.Fatal(r, e)
	}
	cylinder, _ := NewSurfacePatch(quarterCylinder(t), SurfacePatchOptions{})
	cylinder, _ = cylinder.Trim(ParameterRectangle{.25, .35, 0, 1})
	plane = arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{2 * s, 2 * t, .75} })
	r, e = CertifySurfacePairTube(cylinder, plane, SurfacePairTubeOptions{FreeAxis: 0})
	if e != nil || !r.ProvenBranch {
		t.Fatal(r, e)
	}
}
func TestSurfacePairTubeRejectsCenterOnlyEvidence(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{.25 + .5*s, .5, t - .5} })
	section, e := CertifySurfacePairSection(a, b, SurfacePairIntervalOptions{UseFixedParameter: true, FixedAxis: 0, FixedParameter: .5})
	if e != nil || !section.ProvenUnique {
		t.Fatal(section, e)
	}
	tube, e := CertifySurfacePairTube(a, b, SurfacePairTubeOptions{FreeAxis: 0})
	if e != nil || tube.ProvenBranch || tube.ExcludesContact {
		t.Fatal(tube, e)
	}
	if _, e = CertifySurfacePairTube(a, b, SurfacePairTubeOptions{FreeAxis: 0, MaxOperations: 1}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	tube, e = CertifySurfacePairTube(a, a, SurfacePairTubeOptions{FreeAxis: 0})
	if e != nil || tube.ProvenBranch || tube.Test.CenterInvertible {
		t.Fatal(tube, e)
	}
}

func TestSurfaceTubeCanonicalFreeTangentOrientation(t *testing.T) {
	// Residual (u-s,v-s,.5-t) has canonical null vector (-1,-1,-1,0).
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{s, s, t - .5} })
	for _, axis := range []int{0, 1, 2} {
		pa, pb := a, b
		switch axis {
		case 0:
			pa, _ = a.Trim(ParameterRectangle{.2, .8, 0, 1})
		case 1:
			pa, _ = a.Trim(ParameterRectangle{0, 1, .2, .8})
		case 2:
			pb, _ = b.Trim(ParameterRectangle{.2, .8, 0, 1})
		}
		r, e := CertifySurfacePairTube(pa, pb, SurfacePairTubeOptions{FreeAxis: axis})
		if e != nil || !r.ProvenBranch || r.FreeTangentSign != -1 {
			t.Fatal(axis, r, e)
		}
	}
	// Reverse B's t parameter: determinant/cofactor orientation reverses.
	b = arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{s, s, .5 - t} })
	pa, _ := a.Trim(ParameterRectangle{.2, .8, 0, 1})
	r, e := CertifySurfacePairTube(pa, b, SurfacePairTubeOptions{FreeAxis: 0})
	if e != nil || r.FreeTangentSign != 1 {
		t.Fatal(r, e)
	}
}
