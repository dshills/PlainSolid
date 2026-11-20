package plaincadbridge

import (
	"encoding/base64"
	"math"
	"testing"
)

func TestTripleCrossedBoreScopeAndSTEP(t *testing.T) {
	b := New()
	stock := box(t, b, vector{2, 4, -2}, vector{34, 40, 38})
	x := circle(t, b, vector{2, 19, 16}, vector{1, 0, 0}, 3, 32)
	y := circle(t, b, vector{17, 40, 16}, vector{0, -1, 0}, 3, 36)
	z := circle(t, b, vector{17, 19, -2}, vector{0, 0, 1}, 3, 40)
	pair := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": x.Handle})
	pair = shape(t, b, map[string]any{"op": "cut", "a": pair.Handle, "b": z.Handle})
	removed := 108*9*math.Pi - 216*math.Sqrt2
	volume := 32.*36*40 - removed
	area := 2.*(32*36+32*40+36*40) + (108*6-54)*math.Pi - 216*math.Sqrt2
	near(t, overlapMeasure(t, b, pair.Handle, y.Handle), pair.GeometryAssertions.Volume-volume)
	successive := shape(t, b, map[string]any{"op": "cut", "a": pair.Handle, "b": y.Handle})
	grouped, err := b.cutMany(stock.Handle, []string{z.Handle, y.Handle, x.Handle, y.Handle})
	if err != nil || grouped.Bounds != stock.Bounds {
		t.Fatal(grouped, successive, err)
	}
	sameShape(t, grouped, successive)
	near(t, grouped.GeometryAssertions.Volume, volume)
	near(t, grouped.GeometryAssertions.SurfaceArea, area)
	file, err := b.exportSTEP([]string{grouped.Handle})
	if err != nil {
		t.Fatal(err)
	}
	data, err := base64.StdEncoding.DecodeString(file.Base64)
	if err != nil {
		t.Fatal(err)
	}
	got, err := b.importSTEP(string(data))
	if err != nil || got.Bounds != stock.Bounds {
		t.Fatal(got, err)
	}
	sameShape(t, got, grouped)
	for _, tool := range []Shape{x, y, z} {
		near(t, overlapMeasure(t, b, got.Handle, tool.Handle), 0)
	}
}
