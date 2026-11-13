package geom

import (
	"errors"
	"testing"
)

func TestDiscoverSurfaceTubeAffine(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{s, s, t - .5} })
	o := SurfaceTubeDiscoveryOptions{FreeAxis: 0, Parameter: .5, DependentBox: SurfacePairParameterBox{a.Domain(), b.Domain()}}
	r, e := DiscoverSurfacePairTube(a, b, o)
	if e != nil || !r.Found || !r.Seed.ProvenUnique || !r.Tube.ProvenBranch || r.Box.A.UMin >= .5 || r.Box.A.UMax <= .5 {
		t.Fatal(r, e)
	}
}
func TestDiscoverSurfaceTubeShrinksFreeSpan(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{s, s, t - .5} })
	seed := SurfacePairParameterBox{ParameterRectangle{0, 1, .45, .55}, ParameterRectangle{.45, .55, 0, 1}}
	r, e := DiscoverSurfacePairTube(a, b, SurfaceTubeDiscoveryOptions{FreeAxis: 0, Parameter: .5, InitialFreeWidth: .8, DependentBox: seed, MaxAttempts: 20})
	if e != nil || !r.Found || r.Attempted <= 3 || r.Box.A.UMax-r.Box.A.UMin >= .1 {
		t.Fatal(r, e)
	}
}
func TestDiscoverSurfaceTubeRationalAndLimits(t *testing.T) {
	a, _ := NewSurfacePatch(quarterCylinder(t), SurfacePatchOptions{})
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{2 * s, 2 * t, .75} })
	o := SurfaceTubeDiscoveryOptions{FreeAxis: 0, Parameter: .3, DependentBox: SurfacePairParameterBox{a.Domain(), b.Domain()}}
	r, e := DiscoverSurfacePairTube(a, b, o)
	if e != nil || !r.Found {
		t.Fatal(r, e)
	}
	o.MaxOperations = 1
	r, e = DiscoverSurfacePairTube(a, b, o)
	if !errors.Is(e, ErrConvergence) || r.Found {
		t.Fatal(r, e)
	}
	o.MaxOperations = 0
	o.Parameter = 0
	if _, e = DiscoverSurfacePairTube(a, b, o); !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}
}
