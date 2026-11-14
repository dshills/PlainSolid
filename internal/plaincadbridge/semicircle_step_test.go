package plaincadbridge

import (
	"bytes"
	"encoding/base64"
	"math"
	"testing"

	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestSemicircleSTEPBridge(t *testing.T) {
	for _, height := range []float64{4, -4} {
		b := New()
		raw, err := b.mixedExtrude(mixedProfileRequest{
			Kind: "semicircle", Origin: &vector{2, 4, 8}, Normal: &vector{0, 0, 1}, XDirection: &vector{-1, 0, 0},
			Center: []number{1, 2}, Radius: 10, Height: number(height),
		})
		if err != nil {
			t.Fatal(err)
		}
		exported, err := b.exportSTEP([]string{raw.Handle})
		if err != nil {
			t.Fatal(height, err)
		}
		file, err := base64.StdEncoding.DecodeString(exported.Base64)
		if err != nil {
			t.Fatal(err)
		}
		got, err := b.importSTEP(string(file))
		if err != nil || got.Bounds != raw.Bounds {
			t.Fatal(height, raw, got, err)
		}
		near(t, got.GeometryAssertions.Volume, 200*math.Pi)
		near(t, got.GeometryAssertions.SurfaceArea, 80+140*math.Pi)
		again, err := b.exportSTEP([]string{got.Handle})
		if err != nil || again.Base64 != exported.Base64 {
			t.Fatal(height, err)
		}
	}
	frame, err := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	var external bytes.Buffer
	p := step.SemicircleExtrusionBody{Frame: frame, Center: geom.Vec2{X: 1, Y: 2}, Radius: 1, Height: .5}
	if err := step.WriteSemicircleExtrusion(&external, step.SemicircleExtrusionModel{Body: p, LengthUnit: .01}); err != nil {
		t.Fatal(err)
	}
	got, err := New().importSTEP(external.String())
	if err != nil {
		t.Fatal(err)
	}
	near(t, got.GeometryAssertions.Volume, 250*math.Pi)
}
