package geom

import "testing"

func TestSurfaceHeightDifferenceExactAndOutside(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u + v} })
	b := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u + v + .5} })
	r, e := ClassifySurfaceHeightDifference(a, b, Vec3{.25, .5, 1}, SurfaceHeightDifferenceOptions{})
	if e != nil || r.Relation != HeightBelow || r.PointVsA != HeightAbove || r.PointVsB != HeightBelow || r.HeightA.Min > .75 || r.HeightB.Max < 1.25 {
		t.Fatal(r, e)
	}
	r, e = ClassifySurfaceHeightDifference(a, a, Vec3{.25, .5, .75}, SurfaceHeightDifferenceOptions{})
	if e != nil || r.Relation != HeightBoundary || r.PointVsA != HeightBoundary {
		t.Fatal(r, e)
	}
	r, e = ClassifySurfaceHeightDifference(a, b, Vec3{2, .5, 0}, SurfaceHeightDifferenceOptions{})
	if e != nil || !r.OutsideA || !r.OutsideB || r.Relation != HeightUnresolved {
		t.Fatal(r, e)
	}
}
func TestSurfaceHeightDifferenceRationalCommonBasis(t *testing.T) {
	poles := []Vec3{{0, 0, 0}, {0, 1, 1}, {1, 0, 1}, {1, 1, 2}}
	s, e := NewBSplineSurface(1, 1, 2, 2, poles, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, []float64{1, 2, 2, 1})
	if e != nil {
		t.Fatal(e)
	}
	a, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	r, e := ClassifySurfaceHeightDifference(a, a, Vec3{.37, .41, .8}, SurfaceHeightDifferenceOptions{})
	if e != nil || r.Relation != HeightBoundary {
		t.Fatal(r, e)
	}
}
