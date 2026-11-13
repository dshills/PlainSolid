package plaincadbridge

import (
	"bytes"
	"math"
	"os"
	"testing"

	"plainsolid/exchange/step"
	"plainsolid/geom"
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

func TestSTEPRetainsExactSplitCylinderAsNativeStock(t *testing.T) {
	b := New()
	f, err := geom.FrameFromNormal(geom.Vec3{X: 10, Y: 20, Z: 30}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	d := step.SplitCylinderDescriptor{Name: "Cylinder", Frame: f, LengthUnit: .001, Radius: 8, Height: 12}
	for _, write := range []func(*bytes.Buffer, step.SplitCylinderDescriptor) error{
		func(out *bytes.Buffer, source step.SplitCylinderDescriptor) error {
			return step.WriteSplitCylinderModel(out, source)
		},
		func(out *bytes.Buffer, source step.SplitCylinderDescriptor) error {
			return step.WriteSplitCylinderProduct(out, source)
		},
	} {
		var out bytes.Buffer
		if err := write(&out, d); err != nil {
			t.Fatal(err)
		}
		imported, err := b.importSTEP(out.String())
		if err != nil {
			t.Fatal(err)
		}
		near(t, imported.GeometryAssertions.Volume, 768*math.Pi)
		for i, coordinate := range (vector{2, 12, 30}) {
			near(t, imported.Bounds.Min[i], coordinate)
		}
		for i, coordinate := range (vector{18, 28, 42}) {
			near(t, imported.Bounds.Max[i], coordinate)
		}
		tool := circle(t, b, vector{12, 20, 30}, vector{0, 0, 1}, 2, 6)
		cut, err := b.cutMany(imported.Handle, []string{tool.Handle})
		if err != nil {
			t.Fatal(err)
		}
		near(t, cut.GeometryAssertions.Volume, 744*math.Pi)
		if _, err := b.exportSTEP([]string{cut.Handle}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSTEPFittedEnclosureWithPlanarInnerWire(t *testing.T) {
	b := New()
	first := box(t, b, vector{0, 0, 0}, vector{30, 10, 5})
	second := box(t, b, vector{0, 0, 10}, vector{30, 10, 15})
	outer := box(t, b, vector{-4, -4, -4}, vector{34, 14, 7})
	inner := box(t, b, vector{-2, -2, -2}, vector{32, 12, 9})
	fitted := shape(t, b, map[string]any{"op": "cut", "a": outer.Handle, "b": inner.Handle})
	output, err := b.exportSTEP([]string{first.Handle, second.Handle, fitted.Handle})
	if err != nil {
		t.Fatal(err)
	}
	if len(output.After) != 3 || output.After[2].SolidCount != 1 {
		t.Fatal(output.After)
	}
	for axis := range 3 {
		near(t, output.After[2].Bounds.Min[axis], fitted.Bounds.Min[axis])
		near(t, output.After[2].Bounds.Max[axis], fitted.Bounds.Max[axis])
	}
	near(t, output.After[2].Volume, 3240)
	near(t, output.After[2].SurfaceArea, 3464)
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
