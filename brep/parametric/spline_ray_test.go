package parametric

import (
	"context"
	"testing"

	"plainsolid/geom"
)

func TestSplineRayFaceAncestry(t *testing.T) {
	k := []float64{0, 0, 1, 1}
	face := func(z float64) Face {
		s, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{Z: z}, {Y: 1, Z: z}, {X: 1, Z: z}, {X: 1, Y: 1, Z: z}}, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return Face{Surface: s, U: Interval{0, 1}, V: Interval{0, 1}}
	}
	faces := []Face{face(2), face(1)}
	r, e := IntersectSplineFaces(context.Background(), faces, geom.Vec3{X: .3, Y: .6}, geom.Vec3{Z: 1},
		SplineRayOptions{Line: geom.LineSplineSurfaceOptions{MinParameter: 0, MaxParameter: 3}})
	if e != nil || !r.Complete || len(r.Hits) != 2 || r.Hits[0].Face != 1 || r.Hits[1].Face != 0 {
		t.Fatal(r, e)
	}
	for i, h := range r.Hits {
		p := geom.Vec3{X: .3, Y: .6, Z: float64(i + 1)}
		if !h.Bounds.Contains(p) || h.ParameterMin > p.Z || h.ParameterMax < p.Z {
			t.Fatal(h, p)
		}
	}
}

func TestSplineSolidClassification(t *testing.T) {
	f, e := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	s, e := ExtrudeEllipse(f, 2, 1, geom.Vec3{Z: 3})
	if e != nil {
		t.Fatal(e)
	}
	for _, x := range []struct {
		p    geom.Vec3
		want Location
	}{
		{geom.Vec3{X: .2, Y: .3, Z: 1.3}, Inside},
		{geom.Vec3{X: 2.1, Y: .1, Z: 1.3}, Outside},
	} {
		p, e := s.ClassifySpline(context.Background(), x.p, 1e-8, SplineRayOptions{})
		if e != nil || p != x.want {
			t.Fatal(x, p, e)
		}
	}
}
