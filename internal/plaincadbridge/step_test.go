package plaincadbridge

import (
	"math"
	"os"
	"testing"
)

func TestSTEPPlanarAndCylinderNativeRoundTrip(t *testing.T) {
	b := New()
	for _, s := range []Shape{box(t, b, vector{2, 3, 4}, vector{5, 7, 9}), circle(t, b, vector{10, 20, 30}, vector{0, -1, 0}, 2, 5)} {
		data, e := writeSTEPBody(b.shapes[s.Handle])
		if e != nil {
			t.Fatal(e)
		}
		got, e := b.importSTEP(string(data))
		if e != nil {
			t.Fatal(e)
		}
		near(t, got.GeometryAssertions.Volume, s.GeometryAssertions.Volume)
		near(t, got.GeometryAssertions.SurfaceArea, s.GeometryAssertions.SurfaceArea)
		for i := 0; i < 3; i++ {
			near(t, got.Bounds.Min[i], s.Bounds.Min[i])
			near(t, got.Bounds.Max[i], s.Bounds.Max[i])
		}
	}
	before := len(b.shapes)
	if _, e := b.importSTEP("ISO-10303-21; malformed"); e == nil || len(b.shapes) != before {
		t.Fatal("malformed import published")
	}
}
func TestSTEPExistingPlainCADImportFixture(t *testing.T) {
	// Source: pinned PlainCAD schema-v21.pcaddoc stepImport feature.
	data, e := os.ReadFile("testdata/plaincad-placed-bore.step")
	if e != nil {
		t.Fatal(e)
	}
	b := New()
	s, e := b.importSTEP(string(data))
	if e != nil {
		t.Fatal(e)
	}
	near(t, s.GeometryAssertions.Volume, 2000-40*math.Pi)
	if _, e = b.exportSTEP([]string{s.Handle}); e != nil {
		t.Fatal("imported boundary reexport", e)
	}
}
func TestSTEPSelectionCapsuleAndBore(t *testing.T) {
	b := New()
	q := mixedFixture("capsule")
	q.Center = []number{-10, 0}
	q.Right = []number{10, 0}
	q.Radius = 6
	q.InnerRadius = 5
	q.Height = 5
	q.Origin = &vector{0, 0, 0}
	q.Normal = &vector{0, 0, 1}
	q.XDirection = &vector{1, 0, 0}
	solid, v, a, e := newMixedProfile(q)
	if e != nil {
		t.Fatal(e)
	}
	rim, e := b.putAnalytic(solid, v, a)
	if e != nil {
		t.Fatal(e)
	}
	rim = shape(t, b, map[string]any{"op": "transform", "handle": rim.Handle, "matrix": [][]float64{{0, 1, 0, 30}, {1, 0, 0, 40}, {0, 0, -1, 50}}})
	stock := box(t, b, vector{0, 0, 0}, vector{20, 10, 8})
	tool := circle(t, b, vector{10, 5, 0}, vector{0, 0, 1}, 2, 8)
	plate := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": tool.Handle})
	plate = shape(t, b, map[string]any{"op": "transform", "handle": plate.Handle, "matrix": [][]float64{{0, -1, 0, -20}, {0, 0, 1, -30}, {1, 0, 0, -5}}})
	before := len(b.shapes)
	out, e := b.exportSTEP([]string{rim.Handle, plate.Handle})
	if e != nil {
		t.Fatal(e)
	}
	if len(out.After) != 2 || len(b.shapes) != before {
		t.Fatal("selection ownership", out)
	}
	near(t, out.After[0].Volume, (40+11*math.Pi)*5)
	near(t, out.After[1].Volume, (200-4*math.Pi)*8)
}
