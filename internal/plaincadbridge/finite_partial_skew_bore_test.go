package plaincadbridge

import (
	"encoding/base64"
	"math"
	"testing"
)

func TestFinitePartialSkewBoreScopeAndSTEP(t *testing.T) {
	b := New()
	stock := box(t, b, vector{}, vector{40, 40, 40})
	x := circle(t, b, vector{0, 16, 20}, vector{1, 0, 0}, 5, 40)
	z := circle(t, b, vector{20, 21, 40}, vector{0, 0, -1}, 4, 30)
	fullZ := circle(t, b, vector{20, 21, 40}, vector{0, 0, -1}, 4, 40)
	full, err := b.cutMany(stock.Handle, []string{x.Handle, fullZ.Handle})
	if err != nil {
		t.Fatal(err)
	}
	before := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": x.Handle})
	wantVolume := full.GeometryAssertions.Volume + 160*math.Pi
	wantArea := full.GeometryAssertions.SurfaceArea - 48*math.Pi
	near(t, overlapMeasure(t, b, before.Handle, z.Handle), before.GeometryAssertions.Volume-wantVolume)
	successive := shape(t, b, map[string]any{"op": "cut", "a": before.Handle, "b": z.Handle})
	group, err := b.cutMany(stock.Handle, []string{z.Handle, x.Handle})
	if err != nil || group.GeometryAssertions != successive.GeometryAssertions {
		t.Fatal(group, successive, err)
	}
	file, err := b.exportSTEP([]string{group.Handle})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(file.Base64)
	if err != nil || len(raw) >= 512*1024 {
		t.Fatal(len(raw), err)
	}
	got, err := b.importSTEP(string(raw))
	if err != nil || got.Bounds != stock.Bounds {
		t.Fatal(got, err)
	}
	near(t, got.GeometryAssertions.Volume, wantVolume)
	near(t, got.GeometryAssertions.SurfaceArea, wantArea)
	near(t, overlapMeasure(t, b, got.Handle, z.Handle), 0)
}
