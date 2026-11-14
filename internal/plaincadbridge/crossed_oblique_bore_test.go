package plaincadbridge

import (
	"encoding/base64"
	"math"
	"testing"

	"plainsolid/geom"
)

func TestCrossedObliqueBoreScope(t *testing.T) {
	b := New()
	stock := box(t, b, vector{}, vector{20, 16, 40})
	cardinal := circle(t, b, vector{10, 8, -2}, vector{0, 0, 1}, 2, 44)
	oblique := circle(t, b, vector{-4, 8, 9.5}, vector{4, 0, 3}, 2, 35)
	one := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": cardinal.Handle})
	near(t, overlapMeasure(t, b, one.Handle, oblique.Handle), 100*math.Pi-160./3)
	out := shape(t, b, map[string]any{"op": "cut", "a": one.Handle, "b": oblique.Handle})
	near(t, out.GeometryAssertions.Volume, 12800-260*math.Pi+160./3)
	near(t, out.GeometryAssertions.SurfaceArea, 3520+242*math.Pi-80)
	group, err := b.cutMany(stock.Handle, []string{oblique.Handle, cardinal.Handle})
	if err != nil || group.GeometryAssertions != out.GeometryAssertions || group.Bounds != out.Bounds {
		t.Fatal(group, out, err)
	}
	near(t, overlapMeasure(t, b, group.Handle, oblique.Handle), 0)
	body := b.shapes[group.Handle].bore
	cap, err := body.planarFace(geom.Vec3{}, geom.Vec3{X: 1})
	if err != nil {
		t.Fatal(err)
	}
	near(t, cap.Area, 640-5*math.Pi)
	file, err := b.exportSTEP([]string{group.Handle})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(file.Base64)
	if err != nil {
		t.Fatal(err)
	}
	got, err := b.importSTEP(string(raw))
	if err != nil || got.Bounds != out.Bounds {
		t.Fatal(got, err)
	}
	near(t, got.GeometryAssertions.Volume, out.GeometryAssertions.Volume)
	near(t, got.GeometryAssertions.SurfaceArea, out.GeometryAssertions.SurfaceArea)
	near(t, overlapMeasure(t, b, got.Handle, oblique.Handle), 0)
}
