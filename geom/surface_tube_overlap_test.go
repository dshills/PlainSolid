package geom

import "testing"

func TestSurfaceTubeOverlapDifferentAxes(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{s, s, t - .5} })
	// Left graph uses u with broad s; right graph uses s with broad u.
	la, lc := ParameterRectangle{.2, .6, 0, 1}, ParameterRectangle{0, 1, 0, 1}
	ra, rc := ParameterRectangle{0, 1, 0, 1}, ParameterRectangle{.4, .8, 0, 1}
	o := SurfaceTubeOverlapOptions{LeftFreeAxis: 0, RightFreeAxis: 2, WitnessAxis: 0, WitnessParameter: .5}
	r, e := CertifySurfaceTubeOverlap(a, b, la, lc, ra, rc, o)
	if e != nil || !r.SameLocalBranch || r.Excluded {
		t.Fatal(r, e)
	}
	o.WitnessAxis = 2
	r, e = CertifySurfaceTubeOverlap(a, b, la, lc, ra, rc, o)
	if e != nil || !r.SameLocalBranch {
		t.Fatal(r, e)
	}
	// Witness at overlap boundary does not prove an endpoint junction.
	o.WitnessAxis = 0
	o.WitnessParameter = .4
	r, e = CertifySurfaceTubeOverlap(a, b, la, lc, ra, rc, o)
	if e != nil || r.SameLocalBranch || r.Excluded {
		t.Fatal(r, e)
	}
}
func TestSurfaceTubeOverlapDistinctBranches(t *testing.T) {
	poles := []Vec3{}
	for i := 0; i < 2; i++ {
		for j, z := range []float64{.24, -.26, .24} {
			poles = append(poles, Vec3{float64(i), float64(j) / 2, z})
		}
	}
	s, _ := NewBSplineSurface(1, 2, 2, 3, poles, []float64{0, 0, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
	a, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{s, t, 0} })
	r, e := CertifySurfaceTubeOverlap(a, b, ParameterRectangle{.2, .6, .35, .45}, ParameterRectangle{0, 1, 0, 1}, ParameterRectangle{0, 1, .55, .65}, ParameterRectangle{.4, .8, 0, 1}, SurfaceTubeOverlapOptions{LeftFreeAxis: 0, RightFreeAxis: 2, WitnessAxis: 0, WitnessParameter: .5})
	if e != nil || r.SameLocalBranch || !r.Excluded || !r.Left.ProvenBranch || !r.Right.ProvenBranch {
		t.Fatal(r, e)
	}
}
