package plaincadbridge

import (
	"encoding/base64"
	"math"
	"testing"
)

func TestUnequalCrossedBoreScopeAndSTL(t *testing.T) {
	b := New()
	stock := box(t, b, vector{0, 0, 0}, vector{20, 24, 18})
	x := circle(t, b, vector{0, 12, 9}, vector{1, 0, 0}, 4, 20)
	z := circle(t, b, vector{8, 12, 18}, vector{0, 0, -1}, 2, 18)
	first := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": x.Handle})
	next := shape(t, b, map[string]any{"op": "cut", "a": first.Handle, "b": z.Handle})
	// Independent pinned OCCT Boolean on the same box and cylinder dimensions.
	const wantVolume, wantArea = 7505.7782390605544, 3027.2650793521734
	if m := next.GeometryAssertions; !m.Valid || m.SolidCount != 1 ||
		math.Abs(m.Volume-wantVolume) > 1e-5 || math.Abs(m.SurfaceArea-wantArea) > 1e-5 {
		t.Fatal(m)
	}
	near(t, overlapMeasure(t, b, first.Handle, z.Handle), first.GeometryAssertions.Volume-next.GeometryAssertions.Volume)
	for _, tools := range [][]string{{x.Handle, z.Handle}, {z.Handle, x.Handle}, {z.Handle, z.Handle, x.Handle}} {
		grouped, err := b.cutMany(stock.Handle, tools)
		if err != nil || grouped.GeometryAssertions != next.GeometryAssertions || grouped.Bounds != stock.Bounds {
			t.Fatal(grouped, err)
		}
		near(t, overlapMeasure(t, b, grouped.Handle, x.Handle), 0)
		near(t, overlapMeasure(t, b, grouped.Handle, z.Handle), 0)
	}
	mesh, err := b.shapes[next.Handle].tessellate(request{})
	if err != nil {
		t.Fatal(err)
	}
	if err := mesh.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
	exported, err := b.exportSTEP([]string{next.Handle})
	if err != nil {
		t.Fatal(err)
	}
	bytes, err := base64.StdEncoding.DecodeString(exported.Base64)
	if err != nil {
		t.Fatal(err)
	}
	imported, err := b.importSTEP(string(bytes))
	if err != nil || imported.Bounds != stock.Bounds || imported.GeometryAssertions != next.GeometryAssertions {
		t.Fatal(imported, err)
	}
	near(t, overlapMeasure(t, b, imported.Handle, x.Handle), 0)
	reexported, err := b.exportSTEP([]string{imported.Handle})
	if err != nil || reexported.Base64 != exported.Base64 {
		t.Fatal("noncanonical re-export", err)
	}
}
