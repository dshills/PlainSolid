package plaincadbridge

import (
	"math"
	"testing"
)

func TestObliqueToFaceUsesCircularExtent(t *testing.T) {
	b := New()
	r, s := math.Sqrt(.5), 1/math.Sqrt(6)
	normal := vector{1 / math.Sqrt(3), 1 / math.Sqrt(3), 1 / math.Sqrt(3)}
	origin := vector{0, 0, 1.6}
	var vertices []vector
	for _, p := range [][2]float64{{-10, -10}, {10, -10}, {10, 10}, {-10, 10}} {
		u, v := p[0], p[1]
		vertices = append(vertices, vector{u*r + v*s, -u*r + v*s, 1.6 - 2*v*s})
	}
	target := shape(t, b, map[string]any{"op": "prism", "outer": vertices, "offset": normal})
	q := map[string]any{"op": "circleToFace", "b": target.Handle, "origin": origin, "normal": normal,
		"center": vector{}, "direction": vector{0, 0, 1}, "radius": 1}
	tool := shape(t, b, q)
	near(t, tool.GeometryAssertions.Volume, math.Pi*1.6)
	invoke(t, b, map[string]any{"op": "mesh", "handle": tool.Handle})
	q["radius"] = 2
	rejectGroup(t, b, q)
	q["radius"], q["center"] = 1, vector{30, 30, -60}
	rejectGroup(t, b, q)
}
