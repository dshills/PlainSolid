package boolean

import (
	"math"
	"testing"

	"plainsolid/geom"
)

// TestUnder covers the test crowded makes of an arc: whether a vertex of
// the face lies between the arc and its chord, where the polygon of the
// face's vertices would leave it on the wrong side.
func TestUnder(t *testing.T) {
	// A quarter of a circle of radius 2 about the origin, from (2, 0) to
	// (0, 2). Its chord is 2 − √2, about 0.586, from the arc at the middle.
	quarter := path{cos: geom.Vec2{X: 2}, sin: geom.Vec2{Y: 2}, t0: 0, t1: math.Pi / 2}
	at := func(r float64) geom.Vec2 { return geom.Vec2{X: r / math.Sqrt2, Y: r / math.Sqrt2} }
	for _, c := range []struct {
		name  string
		spots []geom.Vec2
		part  [2]float64
		want  bool
	}{
		{"a vertex between the arc and its chord", []geom.Vec2{at(1.9)}, [2]float64{0, math.Pi / 2}, true},
		{"a vertex inside the chord", []geom.Vec2{at(1.3)}, [2]float64{0, math.Pi / 2}, false},
		{"a vertex beyond the arc", []geom.Vec2{at(2.1)}, [2]float64{0, math.Pi / 2}, false},
		{"the ends of the arc are not counted", []geom.Vec2{{X: 2}, {Y: 2}}, [2]float64{0, math.Pi / 2}, false},
		{"a vertex the halved arc stands clear of", []geom.Vec2{at(1.9)}, [2]float64{0, math.Pi / 4}, false},
		{"walked the other way", []geom.Vec2{at(1.9)}, [2]float64{math.Pi / 2, 0}, true},
		{"a vertex on the chord", []geom.Vec2{{X: 1, Y: 1}}, [2]float64{0, math.Pi / 2}, true},
	} {
		if got := under(quarter, c.part[0], c.part[1], c.spots); got != c.want {
			t.Errorf("%s: got %v", c.name, got)
		}
	}
}
