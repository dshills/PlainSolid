package plaincadbridge

import (
	"encoding/base64"
	"math"
	"testing"
)

func TestCircularLensSTEPBridge(t *testing.T) {
	for _, radius := range []float64{10, 12} {
		b := New()
		y := math.Sqrt(3) * radius / 2
		right, left := []float64{radius / 2, y}, []float64{radius / 2, -y}
		body := shape(t, b, map[string]any{"op": "mixedExtrude", "profile": map[string]any{
			"kind": "arcLoop", "origin": vector{}, "normal": vector{0, 0, 1}, "xDirection": vector{1, 0, 0}, "height": 10,
			"segments": []map[string]any{
				{"start": right, "end": left, "center": []float64{radius, 0}, "radius": radius, "startAngle": 2 * math.Pi / 3, "sweep": 2 * math.Pi / 3},
				{"start": left, "end": right, "center": []float64{0, 0}, "radius": radius, "startAngle": 5 * math.Pi / 3, "sweep": 2 * math.Pi / 3},
			},
		}})
		file, err := b.exportSTEP([]string{body.Handle})
		if err != nil {
			t.Fatal(radius, err)
		}
		raw, err := base64.StdEncoding.DecodeString(file.Base64)
		if err != nil {
			t.Fatal(err)
		}
		got, err := b.importSTEP(string(raw))
		if err != nil || got.Bounds != body.Bounds {
			t.Fatal(radius, got, err)
		}
		near(t, got.GeometryAssertions.Volume, 10*radius*radius*(2*math.Pi/3-math.Sqrt(3)/2))
		near(t, got.GeometryAssertions.SurfaceArea, body.GeometryAssertions.SurfaceArea)
		again, err := b.exportSTEP([]string{got.Handle})
		if err != nil || again.Base64 != file.Base64 {
			t.Fatal(radius, err)
		}
	}
}

func TestFragmentedCircularLensSTEPBridge(t *testing.T) {
	b := New()
	body := shape(t, b, map[string]any{"op": "mixedExtrude", "profile": map[string]any{
		"kind": "arcLoop", "origin": vector{}, "normal": vector{0, 0, 1}, "xDirection": vector{1, 0, 0}, "height": 10,
		"segments": []map[string]any{
			{"start": []float64{0, 0}, "end": []float64{6, -10.392304845413264}, "center": []float64{12, 0}, "radius": 12, "startAngle": 3.141592653589793, "sweep": 1.0471975511965983},
			{"start": []float64{6, -10.392304845413264}, "end": []float64{12, 0}, "center": []float64{0, 0}, "radius": 12, "startAngle": 5.235987755982988, "sweep": 1.0471975511965983},
			{"start": []float64{12, 0}, "end": []float64{6, 10.392304845413264}, "center": []float64{0, 0}, "radius": 12, "startAngle": 0, "sweep": 1.0471975511965983},
			{"start": []float64{6, 10.392304845413264}, "end": []float64{0, 0}, "center": []float64{12, 0}, "radius": 12, "startAngle": 2.094395102393195, "sweep": 1.0471975511965983},
		},
	}})
	file, err := b.exportSTEP([]string{body.Handle})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(file.Base64)
	if err != nil {
		t.Fatal(err)
	}
	got, err := b.importSTEP(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	near(t, got.GeometryAssertions.Volume, 1440*(2*math.Pi/3-math.Sqrt(3)/2))
	again, err := b.exportSTEP([]string{got.Handle})
	if err != nil || again.Base64 != file.Base64 {
		t.Fatal(err)
	}
}
