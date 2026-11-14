package plaincadbridge

import (
	"bytes"
	"math"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestSteppedShaftSTEPBridge(t *testing.T) {
	for _, unit := range []float64{.001, .01} {
		f, err := geom.NewFrame(geom.Vec3{X: 3, Y: 4, Z: 5}, geom.Vec3{Y: 1}, geom.Vec3{X: -1})
		if err != nil {
			t.Fatal(err)
		}
		p := trimmed.SteppedShaft{Frame: f, ShaftRadius: 1.5, HeadRadius: 2.75, ShaftLength: 12, HeadHeight: 3,
			Socket: []geom.Vec2{{X: -1, Y: -1}, {X: 1, Y: -1}, {X: 1, Y: 1}, {X: -1, Y: 1}}, SocketFloor: 1.5}
		var source bytes.Buffer
		if err := step.WriteSteppedShaft(&source, step.SteppedShaftModel{Body: p, LengthUnit: unit}); err != nil {
			t.Fatal(err)
		}
		b := New()
		got, err := b.importSTEP(source.String())
		if err != nil {
			t.Fatal(err)
		}
		scale := unit / .001
		near(t, got.GeometryAssertions.Volume, (math.Pi*(1.5*1.5*12+2.75*2.75*3)-6)*scale*scale*scale)
		body := b.shapes[got.Handle]
		if body.analytic.shaft == nil {
			t.Fatal("expected native shaft recipe")
		}
		m, err := body.tessellate(request{})
		if err != nil {
			t.Fatal(err)
		}
		if err := m.ValidateSolid(); err != nil {
			t.Fatal(err)
		}
		if _, err := b.exportSTEP([]string{got.Handle}); err != nil {
			t.Fatal(err)
		}
	}
}
