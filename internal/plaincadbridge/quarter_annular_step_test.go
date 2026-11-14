package plaincadbridge

import (
	"bytes"
	"encoding/base64"
	"math"
	"testing"

	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestQuarterAnnularSTEPAndFurtherCut(t *testing.T) {
	b := New()
	base := shape(t, b, map[string]any{
		"op": "revolvePolygon", "origin": vector{}, "direction": vector{0, 1, 0},
		"outer": []vector{{}, {5, 0, 0}, {5, 10, 0}, {0, 10, 0}}, "angle": math.Pi / 2,
	})
	cut := func(from Shape, radius float64) Shape {
		tool := shape(t, b, map[string]any{"op": "circle", "origin": vector{0, -1, 0}, "normal": vector{0, 1, 0}, "radius": radius, "height": 12})
		return shape(t, b, map[string]any{"op": "cut", "a": from.Handle, "b": tool.Handle})
	}
	body := cut(base, 2)
	file, err := b.exportSTEP([]string{body.Handle})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(file.Base64)
	if err != nil || bytes.Count(raw, []byte("CYLINDRICAL_SURFACE")) != 2 {
		t.Fatal(err)
	}
	got, err := b.importSTEP(string(raw))
	if err != nil || got.Bounds != body.Bounds {
		t.Fatal(got, err)
	}
	near(t, got.GeometryAssertions.Volume, 52.5*math.Pi)
	m, err := b.shapes[got.Handle].tessellate(request{})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
	fresh, successive := cut(base, 3), cut(got, 3)
	want, err := b.exportSTEP([]string{fresh.Handle})
	if err != nil {
		t.Fatal(err)
	}
	out, err := b.exportSTEP([]string{successive.Handle})
	if err != nil || out.Base64 != want.Base64 {
		t.Fatal(err)
	}
}

func TestQuarterAnnularSTEPMillimeterNormalization(t *testing.T) {
	f, err := geom.NewFrame(geom.Vec3{X: 1, Y: 2, Z: 3}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	p := step.QuarterAnnularExtrusionBody{Frame: f, Inner: 2, Radius: 5, Height: 10}
	m := step.QuarterAnnularExtrusionModel{Body: p, LengthUnit: .01}
	var source bytes.Buffer
	if err = step.WriteQuarterAnnularExtrusion(&source, m); err != nil {
		t.Fatal(err)
	}
	b := New()
	got, err := b.importSTEP(source.String())
	if err != nil {
		t.Fatal(err)
	}
	near(t, got.GeometryAssertions.Volume, 52500*math.Pi)
	near(t, got.GeometryAssertions.SurfaceArea, (45.5*math.Pi+60)*100)
	if _, err := b.exportSTEP([]string{got.Handle}); err != nil {
		t.Fatal(err)
	}
}
