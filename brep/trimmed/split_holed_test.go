package trimmed

import (
	"context"
	"math"
	"testing"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func TestSplitFaceWithExistingCertifiedCurvedHole(t *testing.T) {
	ctx := context.Background()
	b, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 8, Y: 8, Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	s, e := FromPlanar(b, 1e-10)
	if e != nil {
		t.Fatal(e)
	}
	f := s.faces[0]
	a, u, v := f.Outer[0].UVStart, f.Outer[1].UVStart, f.Outer[3].UVStart
	chart := func(x, y float64) geom.Vec2 { return add2(a, add2(scale2(sub2(u, a), x/8), scale2(sub2(v, a), y/8))) }
	square := func(x0, y0, x1, y1 float64) []geom.Vec2 {
		return []geom.Vec2{chart(x0, y0), chart(x1, y0), chart(x1, y1), chart(x0, y1)}
	}
	points := square(4, 1, 6, 2)
	curves := make([]PCurve, 4)
	for i, p := range points {
		q := points[(i+1)%4]
		ctrl := mix(p, q, .5)
		if i == 0 {
			ctrl = chart(5, .5)
		}
		curves[i] = QuadraticUV{p, ctrl, q}
	}
	s, hole, e := s.SplitFaceCurvedLoopContext(ctx, 0, curves)
	if e != nil || !hole.UVTopologyCertified {
		t.Fatal(hole, e)
	}
	for _, c := range []struct {
		uv   geom.Vec2
		want trimvalidate.Location
	}{{chart(5, 1.5), trimvalidate.Outside}, {chart(1, 5), trimvalidate.Inside},
		{chart(5, .75), trimvalidate.Boundary}, {chart(9, 2), trimvalidate.Outside}} {
		got, p, e := LocateInFaceTrim(ctx, s.faces[0], c.uv, trimvalidate.Options{})
		if e != nil || p.Status != trimvalidate.Certified || got != c.want {
			t.Fatal(c, got, p, e)
		}
	}
	check := func(s Solid) {
		t.Helper()
		r, e := s.ValidateTrimLoops(ctx, trimvalidate.Options{})
		if e != nil || !r.Complete || s.Topology().EulerCharacteristic != 2 {
			t.Fatal(r, s.Topology(), e)
		}
		m, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(m.Volume-64) > 1e-8 {
			t.Fatal(m, e)
		}
	}
	nested, h, e := s.SplitFaceLoop(0, square(3, .5, 7, 3))
	if e != nil || !h.UVTopologyCertified || len(nested.faces[h.Outer].Holes) != 1 || len(nested.faces[h.Inner].Holes) != 1 ||
		nested.faces[h.Inner].Holes[0][0].Edge != s.faces[0].Holes[0][0].Edge {
		t.Fatal(h, e)
	}
	check(nested)
	other, h, e := s.SplitFaceLoop(0, square(1, 5, 2, 6))
	if e != nil || !h.UVTopologyCertified || len(other.faces[0].Holes) != 2 {
		t.Fatal(h, e)
	}
	check(other)
	if _, _, e := s.SplitFaceLoop(0, square(5, .5, 7, 3)); e == nil {
		t.Fatal("accepted loop intersecting original hole")
	}
	out, cut, e := s.SplitFacePath(ctx, 0, 0, 2, []CutSegment{{Start: a, End: f.Outer[2].UVStart}})
	if e != nil || !cut.UVTopologyCertified || len(out.faces[cut.First].Holes)+len(out.faces[cut.Second].Holes) != 1 {
		t.Fatal(cut, e)
	}
	check(out)
	crossing := chart(5, 1.5)
	if _, _, e := s.SplitFacePath(ctx, 0, 0, 2, []CutSegment{{Start: a, End: crossing}, {Start: crossing, End: f.Outer[2].UVStart}}); e == nil {
		t.Fatal("accepted path intersecting original hole")
	}
}
