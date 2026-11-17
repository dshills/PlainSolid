package trimmed

import (
	"testing"

	"plainsolid/geom"
)

func TestAnalyticCircularCapSeedProvesOriginalDisks(t *testing.T) {
	frame, err := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	plane, err := geom.NewPlaneSurface(frame)
	if err != nil {
		t.Fatal(err)
	}
	loop := func(x, y, r float64, clockwise bool) []Use {
		t.Helper()
		circle := ExtrusionCircle{Center: geom.Vec2{X: x, Y: y}, Radius: r}
		s := r
		if clockwise {
			s = -r
		}
		p := [4]geom.Vec2{{X: x + r, Y: y}, {X: x, Y: y + s}, {X: x - r, Y: y}, {X: x, Y: y - s}}
		out := make([]Use, 4)
		for i := range out {
			a, b := p[i], p[(i+1)%4]
			arc, err := extrusionArc(circle, i, a, b, clockwise)
			if err != nil {
				t.Fatal(err)
			}
			out[i] = Use{UVStart: a, UVEnd: b, Curve: arc}
		}
		return out
	}
	f := Face{Surface: plane, Outer: loop(0, 0, 46, false), Holes: [][]Use{loop(0, 0, 16, true), loop(32, 22, 3.2, true)}}
	if !certifyAnalyticCircularCap(f) {
		t.Fatal("separated authored holes should qualify even past the cardinal chord")
	}
	for _, hole := range [][]Use{loop(44, 0, 2, true), loop(44.5, 0, 2, true), loop(18, 0, 2, true), loop(17.5, 0, 2, true)} {
		f.Holes[1] = hole
		if certifyAnalyticCircularCap(f) {
			t.Fatal("intersecting or tangent source disks qualified")
		}
	}
	f.Holes[1] = loop(32, 22, 3.2, true)
	f.Holes[1][1].Curve = nil
	if certifyAnalyticCircularCap(f) {
		t.Fatal("partial circle passed the original support check")
	}
}
