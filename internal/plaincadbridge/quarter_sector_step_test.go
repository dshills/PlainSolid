package plaincadbridge

import (
	"bytes"
	"context"
	"encoding/base64"
	"math"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestQuarterSectorSTEPBridge(t *testing.T) {
	frame, err := geom.FrameFromNormal(geom.Vec3{X: 4, Y: 8, Z: 12}, geom.Vec3{X: 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, height := range []float64{4, -4} {
		b := New()
		recipe := step.QuarterSectorExtrusionBody{Frame: frame, Center: geom.Vec2{X: 1, Y: 2}, Radius: 10, Height: math.Abs(height), Quarter: 1}
		segments, err := recipe.Segments()
		if err != nil {
			t.Fatal(err)
		}
		// Preserve a clockwise authored order and normalize only for exchange.
		arc := segments[1]
		arc.Start, arc.End = arc.End, arc.Start
		arc.StartAngle += arc.Sweep
		arc.Sweep = -arc.Sweep
		segments = []trimmed.ExtrusionSegment{arc, {Start: arc.End, End: arc.Center}, {Start: arc.Center, End: arc.Start}}
		s, err := trimmed.ExtrudeArcLoop(context.Background(), frame, segments, height, 1e-9)
		if err != nil {
			t.Fatal(err)
		}
		body, err := b.putArcLoop(s, frame, segments, height)
		if err != nil {
			t.Fatal(err)
		}
		f, err := b.exportSTEP([]string{body.Handle})
		if err != nil {
			t.Fatal(height, err)
		}
		raw, err := base64.StdEncoding.DecodeString(f.Base64)
		if err != nil {
			t.Fatal(err)
		}
		got, err := b.importSTEP(string(raw))
		if err != nil || got.Bounds != body.Bounds {
			t.Fatal(height, got, err)
		}
		near(t, got.GeometryAssertions.Volume, 100*math.Pi)
		near(t, got.GeometryAssertions.SurfaceArea, 80+70*math.Pi)
		again, err := b.exportSTEP([]string{got.Handle})
		if err != nil || again.Base64 != f.Base64 {
			t.Fatal(height, err)
		}
	}
	metric := step.QuarterSectorExtrusionModel{LengthUnit: .01, Body: step.QuarterSectorExtrusionBody{
		Frame: frame, Center: geom.Vec2{X: 1, Y: 2}, Radius: 1, Height: .5, Quarter: 3,
	}}
	var raw bytes.Buffer
	if err := step.WriteQuarterSectorExtrusion(&raw, metric); err != nil {
		t.Fatal(err)
	}
	got, err := New().importSTEP(raw.String())
	if err != nil {
		t.Fatal(err)
	}
	near(t, got.GeometryAssertions.Volume, 125*math.Pi)
}
