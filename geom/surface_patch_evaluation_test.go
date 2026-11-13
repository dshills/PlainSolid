package geom

import (
	"errors"
	"testing"
)

func TestSurfacePatchExactJetRationalAndKnots(t *testing.T) {
	source := quarterCylinder(t)
	source, _ = source.InsertUKnot(.3, 1)
	source, _ = source.InsertVKnot(.6, 1)
	p, e := NewSurfacePatch(source, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, uv := range []Vec2{{0, 0}, {.3, .6}, {.37, .41}, {1, 1}} {
		j, e := p.EvaluateJet(uv.X, uv.Y)
		if e != nil {
			t.Fatal(e)
		}
		point, _ := source.Eval(uv.X, uv.Y)
		du, dv, _ := source.Partials(uv.X, uv.Y)
		if point.Distance(j.Point) > 1e-13 || du.Distance(j.Du) > 1e-12 || dv.Distance(j.Dv) > 1e-12 {
			t.Fatal(j, point, du, dv)
		}
		if !j.PointBounds.Contains(j.Point) || !j.DuBounds.Contains(j.Du) || !j.DvBounds.Contains(j.Dv) {
			t.Fatal(j)
		}
	}
	if _, e = p.Eval(-1, 0); !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}
}
func TestExactHeightPatchJetRetainsRoundingBounds(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 1} })
	b := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 1.0 / (1 << 54)} })
	p, e := NewSurfaceHeightDifferencePatch(a, b, SurfaceHeightPatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	j, e := p.EvaluateJet(.3, .4)
	if e != nil || j.Point.Z != 1 || j.PointBounds.Min.Z >= 1 || j.PointBounds.Max.Z != 1 || j.Du.Z != 0 || j.Dv.Z != 0 {
		t.Fatal(j, e)
	}
}
