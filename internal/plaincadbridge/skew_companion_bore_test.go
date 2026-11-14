package plaincadbridge

import (
	"encoding/base64"
	"math"
	"testing"
)

func TestSkewCompanionBoreScopeAndSTEP(t *testing.T) {
	b := New()
	stock := box(t, b, vector{}, vector{30, 30, 30})
	x := circle(t, b, vector{0, 15, 15}, vector{1, 0, 0}, 4, 22)
	z := circle(t, b, vector{15, 15.5, 30}, vector{0, 0, -1}, 2, 22)
	y := circle(t, b, vector{24, 0, 6}, vector{0, 1, 0}, 1.5, 30)
	pair, err := b.cutMany(stock.Handle, []string{z.Handle, x.Handle})
	if err != nil {
		t.Fatal(err)
	}
	near(t, overlapMeasure(t, b, pair.Handle, y.Handle), 67.5*math.Pi)
	successive := shape(t, b, map[string]any{"op": "cut", "a": pair.Handle, "b": y.Handle})
	grouped, err := b.cutMany(stock.Handle, []string{y.Handle, x.Handle, z.Handle})
	if err != nil || grouped.GeometryAssertions != successive.GeometryAssertions {
		t.Fatal(grouped, successive, err)
	}
	wantVolume := pair.GeometryAssertions.Volume - 67.5*math.Pi
	wantArea := pair.GeometryAssertions.SurfaceArea + 85.5*math.Pi
	near(t, grouped.GeometryAssertions.Volume, wantVolume)
	near(t, grouped.GeometryAssertions.SurfaceArea, wantArea)
	file, err := b.exportSTEP([]string{grouped.Handle})
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
	near(t, overlapMeasure(t, b, got.Handle, y.Handle), 0)
}
