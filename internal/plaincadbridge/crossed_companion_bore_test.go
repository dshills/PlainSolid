package plaincadbridge

import (
	"encoding/base64"
	"math"
	"testing"
)

func TestCrossedCompanionBoreScopeAndSTEP(t *testing.T) {
	b := New()
	stock := box(t, b, vector{}, vector{40, 40, 40})
	x := circle(t, b, vector{0, 20, 20}, vector{1, 0, 0}, 3, 40)
	z := circle(t, b, vector{20, 20, 40}, vector{0, 0, -1}, 3, 40)
	y := circle(t, b, vector{32, 0, 8}, vector{0, 1, 0}, 2, 40)
	pair := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": x.Handle})
	pair = shape(t, b, map[string]any{"op": "cut", "a": pair.Handle, "b": z.Handle})
	near(t, overlapMeasure(t, b, pair.Handle, y.Handle), 160*math.Pi)
	cut := shape(t, b, map[string]any{"op": "cut", "a": pair.Handle, "b": y.Handle})
	batch, err := b.cutMany(stock.Handle, []string{y.Handle, z.Handle, x.Handle, y.Handle})
	if err != nil || batch.GeometryAssertions != cut.GeometryAssertions {
		t.Fatal(batch, cut, err)
	}
	near(t, cut.GeometryAssertions.Volume, 64000-880*math.Pi+144)
	near(t, cut.GeometryAssertions.SurfaceArea, 9600+596*math.Pi-144)
	file, err := b.exportSTEP([]string{batch.Handle})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(file.Base64)
	if err != nil {
		t.Fatal(err)
	}
	got, err := b.importSTEP(string(raw))
	if err != nil || got.Bounds != stock.Bounds {
		t.Fatal(got, err)
	}
	near(t, got.GeometryAssertions.Volume, cut.GeometryAssertions.Volume)
	near(t, got.GeometryAssertions.SurfaceArea, cut.GeometryAssertions.SurfaceArea)
	near(t, overlapMeasure(t, b, got.Handle, y.Handle), 0)
}
