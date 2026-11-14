package plaincadbridge

import (
	"encoding/base64"
	"math"
	"testing"

	"plainsolid/geom"
)

func TestDisjointObliqueBoreScopeAndSTEP(t *testing.T) {
	b := New()
	stock := box(t, b, vector{}, vector{20, 20, 20})
	z := circle(t, b, vector{16, 16, 0}, vector{0, 0, 1}, 1, 20)
	o := circle(t, b, vector{1, 4, -4}, vector{1, 0, 4}, 1, 30)
	withoutZ := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": o.Handle})
	withoutOblique := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": z.Handle})
	near(t, overlapMeasure(t, b, withoutZ.Handle, z.Handle), 20*math.Pi)
	near(t, overlapMeasure(t, b, withoutOblique.Handle, o.Handle), 5*math.Sqrt(17)*math.Pi)
	one := shape(t, b, map[string]any{"op": "cut", "a": withoutZ.Handle, "b": z.Handle})
	two := shape(t, b, map[string]any{"op": "cut", "a": withoutOblique.Handle, "b": o.Handle})
	batch, err := b.cutMany(stock.Handle, []string{z.Handle, o.Handle})
	if err != nil || batch.GeometryAssertions != one.GeometryAssertions || batch.GeometryAssertions != two.GeometryAssertions {
		t.Fatal(batch, one, two, err)
	}
	v := 8000 - math.Pi*(20+5*math.Sqrt(17))
	a := 2400 + 2*math.Pi*(19+5*math.Sqrt(17)-math.Sqrt(17)/4)
	near(t, batch.GeometryAssertions.Volume, v)
	near(t, batch.GeometryAssertions.SurfaceArea, a)
	file, err := b.exportSTEP([]string{batch.Handle})
	if err != nil {
		t.Fatal(err)
	}
	s, err := base64.StdEncoding.DecodeString(file.Base64)
	if err != nil {
		t.Fatal(err)
	}
	got, err := b.importSTEP(string(s))
	if err != nil || got.Bounds != stock.Bounds {
		t.Fatal(got, err)
	}
	near(t, got.GeometryAssertions.Volume, v)
	near(t, got.GeometryAssertions.SurfaceArea, a)
	near(t, overlapMeasure(t, b, got.Handle, z.Handle), 0)
	cap, err := b.shapes[got.Handle].bore.planarFace(geom.Vec3{}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	near(t, cap.Area, 400-math.Pi*(1+math.Sqrt(17)/4))
}
