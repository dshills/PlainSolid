package geom

import "testing"

func TestSurfaceXYInverseGraph(t *testing.T) {
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u + 2*v} })
	r, e := InvertSurfaceXY(p, .3, .4, SurfaceXYInverseOptions{})
	if e != nil || !r.Found || !r.Complete || r.UV.Min.X > .3 || r.UV.Max.X < .3 || r.Height.Min > 1.1 || r.Height.Max < 1.1 {
		t.Fatal(r, e)
	}
	r, e = InvertSurfaceXY(p, 2, .4, SurfaceXYInverseOptions{})
	if e != nil || !r.Excluded || !r.Complete {
		t.Fatal(r, e)
	}
}
func TestSurfaceXYInversePlanarBoundary(t *testing.T) {
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	for _, xy := range []Vec2{{.3, .4}, {0, 0}, {1, .4}} {
		r, e := InvertSurfaceXY(p, xy.X, xy.Y, SurfaceXYInverseOptions{})
		if e != nil || !r.Found || r.Height.Min != 0 || r.Height.Max != 0 {
			t.Fatal(xy, r, e)
		}
	}
}
