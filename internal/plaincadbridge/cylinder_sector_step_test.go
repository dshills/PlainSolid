package plaincadbridge

import (
	"bytes"
	"encoding/base64"
	"math"
	"os"
	"testing"

	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestCylinderSectorSTEPBridge(t *testing.T) {
	for _, angle := range []float64{math.Pi / 2, -4 * math.Pi / 3} {
		b := New()
		body := shape(t, b, map[string]any{
			"op": "revolvePolygon", "origin": vector{}, "direction": vector{0, 1, 0},
			"outer": []vector{{}, {5, 0, 0}, {5, 10, 0}, {0, 10, 0}}, "angle": angle,
		})
		file, err := b.exportSTEP([]string{body.Handle})
		if err != nil {
			t.Fatal(angle, err)
		}
		raw, err := base64.StdEncoding.DecodeString(file.Base64)
		if err != nil {
			t.Fatal(err)
		}
		got, err := b.importSTEP(string(raw))
		if err != nil {
			t.Fatal(angle, err)
		}
		near(t, got.GeometryAssertions.Volume, 125*math.Abs(angle))
		for i := range got.Bounds.Min {
			near(t, got.Bounds.Min[i], body.Bounds.Min[i])
			near(t, got.Bounds.Max[i], body.Bounds.Max[i])
		}
		again, err := b.exportSTEP([]string{got.Handle})
		if err != nil || again.Base64 != file.Base64 {
			t.Fatal(angle, err)
		}
		if angle == math.Pi/2 {
			tool := shape(t, b, map[string]any{"op": "circle", "origin": vector{0, 11, 0}, "normal": vector{0, -1, 0}, "radius": 2, "height": 11})
			cut := shape(t, b, map[string]any{"op": "cut", "a": got.Handle, "b": tool.Handle})
			near(t, cut.GeometryAssertions.Volume, 52.5*math.Pi)
		}
	}
}

func TestCylinderSectorSTEPBridgeIndependentAndUnits(t *testing.T) {
	b := New()
	raw, err := os.ReadFile("../../exchange/step/testdata/cylinder-sector.step")
	if err != nil {
		t.Fatal(err)
	}
	got, err := b.importSTEP(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.exportSTEP([]string{got.Handle}); err != nil {
		t.Fatal(err)
	}

	f, err := geom.NewFrame(geom.Vec3{X: 1, Y: 2, Z: 3}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	var source bytes.Buffer
	p := step.CylinderSectorBody{Frame: f, Radius: 5, Height: 10, Angle: math.Pi / 2}
	if err := step.WriteCylinderSectors(&source, step.CylinderSectorModel{Bodies: []step.CylinderSectorBody{p}, LengthUnit: .01}); err != nil {
		t.Fatal(err)
	}
	got, err = b.importSTEP(source.String())
	if err != nil {
		t.Fatal(err)
	}
	near(t, got.GeometryAssertions.Volume, 62500*math.Pi)
	if _, err := b.exportSTEP([]string{got.Handle}); err != nil {
		t.Fatal(err)
	}
}
