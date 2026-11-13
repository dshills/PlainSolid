package plaincadbridge

import (
	"math"
	"testing"
)

func TestPlainCADRoundedHousingJoinPocketAndThroughProfiles(t *testing.T) {
	b := New()
	layer := func(x0, y0, x1, y1, z, h, r float64) Shape {
		return shape(t, b, map[string]any{"op": "mixedExtrude", "profile": map[string]any{
			"kind": "roundedRectangle", "origin": vector{0, 0, z}, "normal": vector{0, 0, 1}, "xDirection": vector{1, 0, 0},
			"min": [2]float64{x0, y0}, "max": [2]float64{x1, y1}, "radii": [4]float64{r, r, r, r}, "height": h,
		}})
	}
	cut := func(a, c Shape) Shape {
		return shape(t, b, map[string]any{"op": "cut", "a": a.Handle, "b": c.Handle})
	}
	area := func(w, h, r float64) float64 { return w*h - (4-math.Pi)*r*r }
	base := layer(-70, -54, 70, 54, 0, 10, 10)
	base = shape(t, b, map[string]any{"op": "capFillet", "handle": base.Handle, "origin": vector{0, 0, 10}, "normal": vector{0, 0, 1}, "radius": 1})
	base = shape(t, b, map[string]any{"op": "capChamfer", "handle": base.Handle, "origin": vector{}, "normal": vector{0, 0, -1}, "distance": 1})
	roof := layer(-50, -38, 50, 38, 9, 33, 12)
	joined := shape(t, b, map[string]any{"op": "fuse", "a": base.Handle, "b": roof.Handle})
	near(t, joined.GeometryAssertions.Volume-base.GeometryAssertions.Volume, area(100, 76, 12)*32)
	pocket := cut(joined, layer(-39, -28, 39, 28, 42, -18, 10))
	near(t, joined.GeometryAssertions.Volume-pocket.GeometryAssertions.Volume, area(78, 56, 10)*18)
	bearing := circle(t, b, vector{}, vector{0, 0, 1}, 20, 42)
	bored := cut(pocket, bearing)
	near(t, pocket.GeometryAssertions.Volume-bored.GeometryAssertions.Volume, math.Pi*400*24)
	holes := func(body Shape, points [][2]float64, z, r, height float64) Shape {
		handles := make([]string, 0, len(points))
		for _, p := range points {
			tool := circle(t, b, vector{p[0], p[1], z}, vector{0, 0, 1}, r, height)
			handles = append(handles, tool.Handle)
		}
		return shape(t, b, map[string]any{"op": "cutMany", "a": body.Handle, "tools": handles})
	}
	mounts := [][2]float64{{-58, -42}, {-58, 42}, {58, -42}, {58, 42}}
	cover := [][2]float64{{-42, -26}, {-42, 26}, {42, -26}, {42, 26}, {0, -33}, {0, 33}}
	beforeHoles := bored.GeometryAssertions.Volume
	bored = holes(bored, mounts, 0, 3.5, 43)
	bored = holes(bored, mounts, 10, 6, -4)
	bored = holes(bored, cover, 42, 2.2, -18)
	bored = holes(bored, cover, 42, 4, -2)
	mountRemoval := math.Pi * (3.5*3.5*6 + 36*4)
	coverRemoval := math.Pi * (2.2*2.2*16 + 16*2)
	near(t, beforeHoles-bored.GeometryAssertions.Volume, 4*mountRemoval+6*coverRemoval)
	for _, x := range []float64{-60, 60} {
		bored = cut(bored, layer(x-4, -15, x+4, 15, 0, 10, 3))
	}
	for _, x := range []float64{-27, -17, 17, 27} {
		bored = cut(bored, layer(x-1.25, -36, x+1.25, -30, 42, -2, 1))
	}
	m, err := b.shapes[bored.Handle].tessellate(request{
		LinearDeflection: optionalNumber{value: .5, set: true}, AngularDeflection: optionalNumber{value: .2, set: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	lines, err := b.shapes[bored.Handle].displayEdges(12000, 8000)
	if err != nil || len(lines.Segments)/6 <= 100 {
		t.Fatal("missing retained native display edges", len(lines.Segments)/6, err)
	}
	t.Logf("default mesh %d triangles, native display %d segments", len(m.Triangles), len(lines.Segments)/6)
	detail, err := b.shapes[bored.Handle].tessellate(request{
		LinearDeflection: optionalNumber{value: .08, set: true}, AngularDeflection: optionalNumber{value: .08, set: true},
	})
	if err != nil || len(detail.Triangles) <= len(m.Triangles) {
		t.Fatal("refined detail", len(detail.Triangles), err)
	}
	t.Logf("refined mesh %d triangles", len(detail.Triangles))
	if err := m.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
}
