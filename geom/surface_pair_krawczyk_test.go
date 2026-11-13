package geom

import (
	"errors"
	"testing"
)

func TestSurfaceSectionKrawczykAffine(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{s, .4, t - .3} })
	for _, fixed := range []int{0, 2} {
		r, e := CertifySurfacePairSection(a, b, SurfacePairIntervalOptions{UseFixedParameter: true, FixedAxis: fixed, FixedParameter: .6})
		if e != nil || !r.ProvenUnique || r.ExcludesContact || r.ContractionBound != 0 {
			t.Fatal(r, e)
		}
		for j, axis := range r.ActiveAxes {
			want := [4]float64{.6, .4, .6, .3}[axis]
			if r.KrawczykBounds[j].Min > want || r.KrawczykBounds[j].Max < want {
				t.Fatal(r)
			}
		}
	}
}
func TestSurfaceSectionKrawczykNonlinear(t *testing.T) {
	// Bowl and a plane parametrized in x,z. Fixing bowl u=.4 leaves one
	// exact contact (v=.5,s=.4,t=.11) in the supplied interior domains.
	bowl := quadraticContactSurface(t, 0)
	a, _ := NewSurfacePatch(bowl, SurfacePatchOptions{})
	a, _ = a.Trim(ParameterRectangle{.3, .5, .4, .6})
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{s, .5, t - .1} })
	r, e := CertifySurfacePairSection(a, b, SurfacePairIntervalOptions{UseFixedParameter: true, FixedAxis: 0, FixedParameter: .4})
	if e != nil || !r.ProvenUnique || r.ExcludesContact {
		t.Fatal(r, e)
	}
}
func TestSurfaceSectionKrawczykExclusionSingularAndBudget(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{s, 2, t - .3} })
	o := SurfacePairIntervalOptions{UseFixedParameter: true, FixedAxis: 0, FixedParameter: .5}
	r, e := CertifySurfacePairSection(a, b, o)
	if e != nil || !r.ExcludesContact || r.ProvenUnique {
		t.Fatal(r, e)
	}
	r, e = CertifySurfacePairSection(a, a, o)
	if e != nil || r.CenterInvertible || r.ProvenUnique {
		t.Fatal(r, e)
	}
	o.MaxOperations = 1
	if _, e = CertifySurfacePairSection(a, b, o); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
}

func TestSurfaceSectionKrawczykRationalCylinder(t *testing.T) {
	source := quarterCylinder(t)
	a, _ := NewSurfacePatch(source, SurfacePatchOptions{})
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{2 * s, 2 * t, .75} })
	r, e := CertifySurfacePairSection(a, b, SurfacePairIntervalOptions{UseFixedParameter: true, FixedAxis: 0, FixedParameter: .3})
	if e != nil || !r.ProvenUnique || r.ExcludesContact {
		t.Fatal(r, e)
	}
	p, _ := source.Eval(.3, .375)
	want := [4]float64{.3, .375, p.X / 2, p.Y / 2}
	for j, axis := range r.ActiveAxes {
		bound := r.KrawczykBounds[j]
		if want[axis] < bound.Min-1e-14 || want[axis] > bound.Max+1e-14 {
			t.Fatal(r, p)
		}
	}
}
