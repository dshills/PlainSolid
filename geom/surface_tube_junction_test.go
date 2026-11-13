package geom

import "testing"

func TestSurfaceTubeJunctionSameBranch(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{s, .4, t - .3} })
	left, right := ParameterRectangle{.2, .5, 0, 1}, ParameterRectangle{.5, .8, 0, 1}
	other := ParameterRectangle{0, 1, 0, 1}
	r, e := CertifySurfaceTubeJunction(a, b, left, other, right, other, SurfaceTubeJunctionOptions{})
	if e != nil || !r.Connected || r.Excluded || r.Parameter != .5 {
		t.Fatal(r, e)
	}
	r, e = CertifySurfaceTubeJunction(a, b, right, other, left, other, SurfaceTubeJunctionOptions{})
	if e != nil || !r.Connected {
		t.Fatal(r, e)
	}
}
func TestSurfaceTubeJunctionDistinctRoots(t *testing.T) {
	// z=(v-.5)^2-.01 gives two disjoint parallel branches, v=.4 and .6.
	poles := []Vec3{}
	for i := 0; i < 2; i++ {
		for j, z := range []float64{.24, -.26, .24} {
			poles = append(poles, Vec3{float64(i), float64(j) / 2, z})
		}
	}
	s, e := NewBSplineSurface(1, 2, 2, 3, poles, []float64{0, 0, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	a, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{s, t, 0} })
	left, right := ParameterRectangle{.2, .5, .35, .45}, ParameterRectangle{.5, .8, .55, .65}
	other := ParameterRectangle{0, 1, 0, 1}
	r, e := CertifySurfaceTubeJunction(a, b, left, other, right, other, SurfaceTubeJunctionOptions{})
	if e != nil || r.Connected || !r.Excluded || !r.Left.ProvenBranch || !r.Right.ProvenBranch {
		t.Fatal(r, e)
	}
	// Both boxes on the lower branch must instead join.
	right.VMin, right.VMax = .35, .45
	r, e = CertifySurfaceTubeJunction(a, b, left, other, right, other, SurfaceTubeJunctionOptions{})
	if e != nil || !r.Connected || r.Excluded {
		t.Fatal(r, e)
	}
}

func TestSurfaceTubeJunctionUnresolvedBoundaryOverlap(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{s, .4, t - .3} })
	other := ParameterRectangle{0, 1, 0, 1}
	r, e := CertifySurfaceTubeJunction(a, b, ParameterRectangle{.2, .5, 0, .4}, other, ParameterRectangle{.5, .8, .4, 1}, other, SurfaceTubeJunctionOptions{})
	if e != nil || r.Connected || r.Excluded {
		t.Fatal(r, e)
	}
}
