package plaincadbridge

import (
	"plainsolid/geom"
	"testing"
)

func TestPolygonHoleExactBoundsAndPlacement(t *testing.T) {
	for _, axis := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		f, err := geom.FrameFromNormal(geom.Vec3{}, axis)
		if err != nil {
			t.Fatal(err)
		}
		for _, height := range []float64{5, -5} {
			b := New()
			q := mixedProfileRequest{Kind: "polygon", Origin: ptrVector(vector{}), Normal: ptrVector(coordinates(f.ZAxis())), XDirection: ptrVector(coordinates(f.XAxis())), Height: number(height), Outer: [][]number{{0, 0}, {24, 0}, {24, 20}, {0, 20}}, Circles: []mixedProfileCircle{{Center: []number{8, 8}, Radius: 2}, {Center: []number{16, 12}, Radius: 3}}}
			s, err := b.mixedExtrude(q)
			if err != nil {
				t.Fatal(err)
			}
			var corners []geom.Vec3
			for _, xy := range q.Outer {
				for _, z := range []float64{0, height} {
					corners = append(corners, f.ToWorld(geom.Vec3{X: float64(xy[0]), Y: float64(xy[1]), Z: z}))
				}
			}
			expected, _ := geom.Bounds(corners)
			if s.Bounds != (Bounds{Min: coordinates(expected.Min), Max: coordinates(expected.Max)}) {
				t.Fatal(axis, height, s.Bounds, expected)
			}
			matrix := geom.Affine{{0, -1, 0, 30}, {1, 0, 0, -7}, {0, 0, 1, 9}}
			moved := shape(t, b, map[string]any{"op": "transform", "handle": s.Handle, "matrix": matrix})
			for i, p := range corners {
				corners[i] = matrix.Apply(p)
			}
			expected, _ = geom.Bounds(corners)
			if moved.Bounds != (Bounds{Min: coordinates(expected.Min), Max: coordinates(expected.Max)}) {
				t.Fatal("placed", moved.Bounds, expected)
			}
			original := b.shapes[s.Handle].result
			if original.Bounds != s.Bounds {
				t.Fatal("placement mutated source")
			}
		}
	}
}
func ptrVector(v vector) *vector { return &v }
