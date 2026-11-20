package plaincadbridge

import (
	"encoding/base64"
	"math"
	"testing"
)

func TestBlindCrossedBoreScopeAndSTEP(t *testing.T) {
	b := New()
	stock := box(t, b, vector{0, 0, 0}, vector{20, 24, 18})
	x := circle(t, b, vector{0, 9, 7}, vector{1, 0, 0}, 3, 20)
	z := circle(t, b, vector{8, 9, 18}, vector{0, 0, -1}, 3, 16)
	before := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": x.Handle})
	near(t, overlapMeasure(t, b, before.Handle, z.Handle), 16*9*math.Pi-144)
	successive := shape(t, b, map[string]any{"op": "cut", "a": before.Handle, "b": z.Handle})
	wantVolume := 20.*24*18 - 36*9*math.Pi + 144
	wantArea := 2.*(20*24+20*18+24*18) + 36*6*math.Pi - 144 - 18*math.Pi
	for _, order := range [][]string{{x.Handle, z.Handle}, {z.Handle, x.Handle}} {
		out, err := b.cutMany(stock.Handle, order)
		if err != nil {
			t.Fatal(err)
		}
		near(t, out.GeometryAssertions.Volume, wantVolume)
		near(t, out.GeometryAssertions.SurfaceArea, wantArea)
		sameShape(t, out, successive)
		if out.Bounds != stock.Bounds {
			t.Fatal(out, stock)
		}
		first, err := b.exportSTEP([]string{out.Handle})
		if err != nil {
			t.Fatal(err)
		}
		next, err := b.exportSTEP([]string{successive.Handle})
		if err != nil {
			t.Fatal("different canonical STEP from the same cuts", err)
		}
		sameSTEP(t, b, first, next)
		data, err := base64.StdEncoding.DecodeString(first.Base64)
		if err != nil {
			t.Fatal(err)
		}
		recovered, err := b.importSTEP(string(data))
		if err != nil {
			t.Fatal(err)
		}
		sameShape(t, recovered, out)
		near(t, overlapMeasure(t, b, recovered.Handle, z.Handle), 0)
		floor, err := planarFace(t, b, recovered.Handle, vector{8, 9, 2}, vector{0, 0, 1})
		if err != nil {
			t.Fatal(err)
		}
		near(t, floor.Area, 9*math.Pi)
		near(t, floor.Normal[2], 1)
	}
}
