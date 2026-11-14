package plaincadbridge

import (
	"encoding/base64"
	"math"
	"testing"
)

func TestCrossedBoreScopeAndSTEP(t *testing.T) {
	b := New()
	stock := box(t, b, vector{0, 0, 0}, vector{20, 24, 18})
	x := circle(t, b, vector{0, 9, 7}, vector{1, 0, 0}, 3, 20)
	z := circle(t, b, vector{8, 9, 18}, vector{0, 0, -1}, 3, 18)
	before := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": x.Handle})
	near(t, overlapMeasure(t, b, before.Handle, z.Handle), 18*9*math.Pi-144)
	successive := shape(t, b, map[string]any{"op": "cut", "a": before.Handle, "b": z.Handle})
	wantVolume := 20.*24*18 - 38*9*math.Pi + 144
	wantArea := 2.*(20*24+20*18+24*18) + 38*6*math.Pi - 144 - 36*math.Pi
	for _, order := range [][]string{{x.Handle, z.Handle}, {z.Handle, x.Handle}, {z.Handle, x.Handle, z.Handle}} {
		out, err := b.cutMany(stock.Handle, order)
		if err != nil {
			t.Fatal(err)
		}
		near(t, out.GeometryAssertions.Volume, wantVolume)
		near(t, out.GeometryAssertions.SurfaceArea, wantArea)
		if out.GeometryAssertions != successive.GeometryAssertions || out.Bounds != stock.Bounds {
			t.Fatal(out, successive)
		}
		// Direct and grouped operations retain the same support geometry.
		first, err := b.exportSTEP([]string{out.Handle})
		if err != nil {
			t.Fatal(err)
		}
		next, err := b.exportSTEP([]string{successive.Handle})
		if err != nil || first.Base64 != next.Base64 {
			t.Fatal("different canonical STEP from the same cuts", err)
		}
		bytes, err := base64.StdEncoding.DecodeString(first.Base64)
		if err != nil {
			t.Fatal(err)
		}
		recovered, err := b.importSTEP(string(bytes))
		if err != nil || recovered.GeometryAssertions != out.GeometryAssertions || recovered.Bounds != out.Bounds {
			t.Fatal(recovered, err)
		}
		near(t, overlapMeasure(t, b, recovered.Handle, z.Handle), 0)
		mesh, err := b.shapes[recovered.Handle].tessellate(request{})
		if err != nil {
			t.Fatal(err)
		}
		if err := mesh.ValidateSolid(); err != nil {
			t.Fatal(err)
		}
	}
}
