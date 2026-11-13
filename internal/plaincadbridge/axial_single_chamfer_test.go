package plaincadbridge

import (
	"math"
	"testing"
)

func TestSingleConcaveAxialChamferDispatch(t *testing.T) {
	b := New()
	stock := shape(t, b, map[string]any{"op": "prism", "offset": vector{0, 0, 10}, "outer": []vector{{0, 0, 0}, {20, 0, 0}, {20, 10, 0}, {10, 10, 0}, {10, 20, 0}, {0, 20, 0}}})
	q := map[string]any{"op": "chamfer", "handle": stock.Handle, "normal": vector{0, 1, 0}, "edges": []any{edgeRef(vector{10, 10, 0}, vector{10, 10, 10})}, "distance": 1}
	// Without an authored sweep direction, retain the strict supporting-chain gate.
	rejectGroup(t, b, q)
	q["direction"] = vector{0, 0, 1}
	out := shape(t, b, q)
	near(t, out.GeometryAssertions.Volume, 3005)
	near(t, out.GeometryAssertions.SurfaceArea, 1401+10*(math.Sqrt2-2))
	invoke(t, b, map[string]any{"op": "mesh", "handle": out.Handle, "linearDeflection": .1, "angularDeflection": .2})
	q["direction"] = vector{1, 0, 0}
	rejectGroup(t, b, q)
}
