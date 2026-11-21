package plaincadbridge

import (
	"encoding/base64"
	"math"
	"reflect"
	"testing"
)

func TestFiniteDisjointBoresSTEP(t *testing.T) {
	b := New()
	stock := box(t, b, vector{2, -3, 5}, vector{22, 21, 23})
	x := circle(t, b, vector{23, 7, 15}, vector{-1, 0, 0}, 1, 5)
	y := circle(t, b, vector{12, -4, 15}, vector{0, 1, 0}, 1, 5)
	z := circle(t, b, vector{18, 15, 24}, vector{0, 0, -1}, 2.5, 20)
	cut := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": x.Handle})
	cut = shape(t, b, map[string]any{"op": "cut", "a": cut.Handle, "b": y.Handle})
	successive := shape(t, b, map[string]any{"op": "cut", "a": cut.Handle, "b": z.Handle})
	grouped, err := b.cutMany(stock.Handle, []string{z.Handle, y.Handle, x.Handle})
	if err != nil {
		t.Fatal(err)
	}
	wantVolume := 20.*24*18 - math.Pi*(4+4+18*6.25)
	wantArea := 2.*(20*24+20*18+24*18) + 2*math.Pi*(4+4+2.5*(18-2.5))
	first, err := b.exportSTEP([]string{successive.Handle})
	if err != nil {
		t.Fatal(err)
	}
	second, err := b.exportSTEP([]string{grouped.Handle})
	if err != nil {
		t.Fatal("non-canonical STEP for grouped and successive cuts", err)
	}
	sameSTEP(t, b, first, second)
	data, err := base64.StdEncoding.DecodeString(first.Base64)
	if err != nil {
		t.Fatal(err)
	}
	imported, err := b.importSTEP(string(data))
	if err != nil {
		t.Fatal(err)
	}
	for _, out := range []Shape{successive, grouped, imported} {
		near(t, out.GeometryAssertions.Volume, wantVolume)
		near(t, out.GeometryAssertions.SurfaceArea, wantArea)
		if out.Bounds != stock.Bounds {
			t.Fatalf("cut changed stock bounds: %+v", out.Bounds)
		}
	}
	for _, tool := range []Shape{x, y, z} {
		before, be := b.overlapVolume(successive.Handle, tool.Handle)
		after, ae := b.overlapVolume(imported.Handle, tool.Handle)
		if (be == nil) != (ae == nil) || be == nil && !reflect.DeepEqual(before, after) || be != nil && be.Error() != ae.Error() {
			t.Fatal("roundtrip changed overlap contract", before, be, after, ae)
		}
	}
	mesh, err := b.shapes[imported.Handle].tessellate(request{})
	if err != nil {
		t.Fatal(err)
	}
	if err := mesh.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
}
