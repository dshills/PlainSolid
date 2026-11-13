package plaincadbridge

import (
	"math"
	"testing"
)

func TestOrthogonalAxialEdgeTreatmentPreservesMixedCornerVolume(t *testing.T) {
	b := New()
	stock := shape(t, b, map[string]any{"op": "prism", "offset": vector{0, 0, 10},
		"outer": []vector{{0, 0, 0}, {20, 0, 0}, {20, 10, 0}, {10, 10, 0}, {10, 20, 0}, {0, 20, 0}}})
	edges := []any{edgeRef(vector{20, 10, 0}, vector{20, 10, 10}), edgeRef(vector{10, 10, 0}, vector{10, 10, 10})}
	q := map[string]any{"op": "fillet", "handle": stock.Handle, "normal": vector{0, 1, 0},
		"direction": vector{0, 0, 1}, "edges": edges, "radius": 1}
	fillet := shape(t, b, q)
	near(t, fillet.GeometryAssertions.Volume, 3000)
	near(t, fillet.GeometryAssertions.SurfaceArea, 1400-20*(2-math.Pi/2))
	invoke(t, b, map[string]any{"op": "mesh", "handle": fillet.Handle, "linearDeflection": .05, "angularDeflection": .1})
	q["op"], q["distance"] = "chamfer", 1
	chamfer := shape(t, b, q)
	near(t, chamfer.GeometryAssertions.Volume, 3000)
	near(t, chamfer.GeometryAssertions.SurfaceArea, 1400-20*(2-math.Sqrt2))
	q["distance"] = 5
	rejectGroup(t, b, q)
}
