package trimmed

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestLeftHandedTwoStartBoreRegression(t *testing.T) {
	f, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	profile := []geom.Vec2{{X: 2, Y: 1}, {X: 2.5, Y: 1.125}, {X: 2, Y: 1.25}}
	var volumes []float64
	for _, hand := range []geom.HelixHandedness{geom.LeftHanded, geom.RightHanded} {
		s, e := GrooveTubeWithHelicalStarts(context.Background(), f, 4, 2, 5, profile, 1, .25, 2.25, hand, 2)
		if e != nil {
			t.Fatal(hand, e)
		}
		if len(s.Faces()[1].Holes) != 2 {
			t.Fatal("lost start")
		}
		p, e := s.MassProperties(context.Background(), parametric.IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		volumes = append(volumes, p.Volume)
		var encoded bytes.Buffer
		if e := WriteJSON(&encoded, s); e != nil {
			t.Fatal(e)
		}
		loaded, e := ReadJSON(&encoded, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		if len(loaded.Faces()[1].Holes) != 2 {
			t.Fatal("roundtrip")
		}
	}
	if math.Abs(volumes[0]-volumes[1]) > 1e-6 {
		t.Fatal("hand parity", volumes)
	}
}
