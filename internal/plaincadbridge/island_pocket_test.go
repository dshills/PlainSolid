package plaincadbridge

import (
	"math"
	"plainsolid/geom"
	"testing"
)

func TestIslandPocketFiniteSignedCaps(t *testing.T) {
	for _, axis := range []geom.Vec3{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}, {Z: 1}, {Z: -1}} {
		frame, _ := geom.FrameFromNormal(geom.Vec3{X: 3, Y: 5, Z: 7}, axis)
		for _, top := range []bool{false, true} {
			for _, reverseTool := range []bool{false, true} {
				b := New()
				outer := []vector{}
				for _, p := range []geom.Vec3{{X: -10, Y: -10}, {X: 10, Y: -10}, {X: 10, Y: 10}, {X: -10, Y: 10}} {
					outer = append(outer, coordinates(frame.ToWorld(p)))
				}
				stock := shape(t, b, map[string]any{"op": "prism", "outer": outer, "offset": coordinates(axis.Scale(8))})
				origin, normal, height := frame.Origin(), axis, 3.
				if top {
					origin = frame.ToWorld(geom.Vec3{Z: 5})
				}
				if reverseTool {
					origin = origin.Add(axis.Scale(3))
					normal = axis.Scale(-1)
				}
				tool := shape(t, b, map[string]any{"op": "mixedExtrude", "profile": map[string]any{"kind": "polygon", "origin": coordinates(origin), "normal": coordinates(normal), "xDirection": coordinates(frame.XAxis()), "outer": [][]float64{{-7, -7}, {7, -7}, {7, 7}, {-7, 7}}, "height": height, "circles": []any{map[string]any{"center": []float64{0, 0}, "radius": 3}}}})
				cut := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": tool.Handle})
				if math.Abs(cut.GeometryAssertions.Volume-(2612+27*math.Pi)) > 1e-9 || math.Abs(cut.GeometryAssertions.SurfaceArea-(1608+18*math.Pi)) > 1e-9 || cut.Bounds != stock.Bounds {
					t.Fatal(axis, top, reverseTool, cut, stock.Bounds)
				}
				p := b.shapes[cut.Handle].analytic.islandPocket
				wantFloor := 5.
				if !top {
					wantFloor = 3
				}
				actual := p.Frame.ToWorld(geom.Vec3{Z: p.Height - p.Depth})
				if actual != frame.ToWorld(geom.Vec3{Z: wantFloor}) {
					t.Fatal("wrong physical floor", actual, wantFloor)
				}
				if b.shapes[stock.Handle].result.GeometryAssertions.Volume != 3200 {
					t.Fatal("source mutated")
				}
			}
		}
	}
}

func TestIslandPocketFiniteRejectsAreAtomic(t *testing.T) {
	b := New()
	stock := shape(t, b, map[string]any{"op": "box", "min": vector{-10, -10, 0}, "max": vector{10, 10, 8}})
	for _, p := range [][3]float64{{3, 2, 0}, {-1, 10, 0}, {5, math.Nextafter(3, 0), 0}, {5, 3, 6}} {
		tool := shape(t, b, map[string]any{"op": "mixedExtrude", "profile": map[string]any{"kind": "polygon", "origin": vector{p[2], 0, p[0]}, "normal": vector{0, 0, 1}, "xDirection": vector{1, 0, 0}, "outer": [][]float64{{-7, -7}, {7, -7}, {7, 7}, {-7, 7}}, "height": p[1], "circles": []any{map[string]any{"center": []float64{0, 0}, "radius": 3}}}})
		count, next := len(b.shapes), b.next
		_, handled, e := b.rectangularIslandPocket(b.shapes[stock.Handle], b.shapes[tool.Handle])
		if !handled || e == nil || len(b.shapes) != count || b.next != next {
			t.Fatal("invalid finite cut published", p, handled, e)
		}
	}
}
