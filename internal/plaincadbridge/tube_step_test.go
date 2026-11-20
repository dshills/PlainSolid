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

func TestFullTubeSTEPAndCoaxialCut(t *testing.T) {
	b := New()
	target := shape(t, b, map[string]any{"op": "circle", "origin": vector{}, "normal": vector{0, 1, 0}, "radius": 5, "height": 10})
	cut := func(from Shape, radius float64) Shape {
		tool := shape(t, b, map[string]any{"op": "circle", "origin": vector{0, 11, 0}, "normal": vector{0, -1, 0}, "radius": radius, "height": 11})
		return shape(t, b, map[string]any{"op": "cut", "a": from.Handle, "b": tool.Handle})
	}
	body := cut(target, 2)
	file, err := b.exportSTEP([]string{body.Handle})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(file.Base64)
	if err != nil || bytes.Count(raw, []byte("CYLINDRICAL_SURFACE")) != 2 {
		t.Fatal(err)
	}
	got, err := b.importSTEP(string(raw))
	if err != nil {
		t.Fatal(got, err)
	}
	for i := range got.Bounds.Min {
		near(t, got.Bounds.Min[i], body.Bounds.Min[i])
		near(t, got.Bounds.Max[i], body.Bounds.Max[i])
	}
	near(t, got.GeometryAssertions.Volume, 210*math.Pi)
	near(t, got.GeometryAssertions.SurfaceArea, 182*math.Pi)
	again, err := b.exportSTEP([]string{got.Handle})
	if err != nil || again.Base64 != file.Base64 {
		t.Fatal(err)
	}
	fresh, successive := cut(target, 3), cut(got, 3)
	want, err := b.exportSTEP([]string{fresh.Handle})
	if err != nil {
		t.Fatal(err)
	}
	out, err := b.exportSTEP([]string{successive.Handle})
	if err != nil || out.Base64 != want.Base64 {
		t.Fatal(err)
	}
}

func TestFullTubeSTEPIndependentAndUnits(t *testing.T) {
	b := New()
	raw, err := os.ReadFile("../../exchange/step/testdata/tube.step")
	if err != nil {
		t.Fatal(err)
	}
	got, err := b.importSTEP(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	near(t, got.GeometryAssertions.Volume, 32*math.Pi)
	if _, err = b.exportSTEP([]string{got.Handle}); err != nil {
		t.Fatal(err)
	}
	f, err := geom.NewFrame(geom.Vec3{X: 1, Y: 2, Z: 3}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	var source bytes.Buffer
	p := step.TubeBody{Frame: f, OuterRadius: 5, InnerRadius: 2, Height: 10}
	if err = step.WriteTubes(&source, step.TubeModel{Bodies: []step.TubeBody{p}, LengthUnit: .01}); err != nil {
		t.Fatal(err)
	}
	got, err = b.importSTEP(source.String())
	if err != nil {
		t.Fatal(err)
	}
	near(t, got.GeometryAssertions.Volume, 210000*math.Pi)
	a := b.shapes[got.Handle].analytic
	if a.sector == nil {
		native, readErr := step.ReadAnalyticBodies(bytes.NewReader(source.Bytes()), step.Limits{})
		if readErr != nil {
			t.Fatal("step reader", readErr)
		}
		_, outer, inner, height, ok := native.Bodies[0].Solid.ConcentricTubeParameters()
		t.Fatal("lost tube recipe", outer, inner, height, ok)
	}
	if _, err = b.exportSTEP([]string{got.Handle}); err != nil {
		t.Fatal(err)
	}
}
