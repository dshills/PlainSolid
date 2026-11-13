package intersect

import (
	"context"
	"plainsolid/geom"
	"testing"
)

func TestDiscoverIsoPlaneClippedParabola(t *testing.T) {
	var poles []geom.Vec3
	for _, u := range []float64{0, 1} {
		for j, z := range []float64{25.0/1024 - .25, 25.0/1024 + .25, 25.0/1024 - .25} {
			poles = append(poles, geom.Vec3{X: u, Y: float64(j) / 2, Z: z})
		}
	}
	s, e := geom.NewBSplineSurface(1, 2, 2, 3, poles, []float64{0, 0, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{X: .75}, {X: .75, Z: 1}, {X: .75, Y: 1}, {X: .75, Y: 1, Z: 1}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	a, b := Patch{Surface: s, Max: geom.Vec2{X: 1, Y: 1}}, Patch{Surface: p, Max: geom.Vec2{X: 1, Y: 1}}
	for _, pair := range [][2]Patch{{a, b}, {b, a}} {
		result, e := DiscoverSplinePlanePairSections(context.Background(), pair[0], pair[1], geom.SurfacePatchOptions{}, SplinePlanePairDiscoveryOptions{})
		if e != nil || result.OpenContours != 1 || len(result.Terminals) != 2 || len(result.Seeds.Attempts) < 2 {
			t.Fatal(result, e)
		}
		for _, terminal := range result.Terminals {
			if !terminal.Certificate.Proven || terminal.Point[2].Sign() != 0 {
				t.Fatal(terminal)
			}
		}
	}
}
