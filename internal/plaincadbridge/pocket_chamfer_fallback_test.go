package plaincadbridge

import (
	"plainsolid/geom"
	"testing"
)

func TestPocketChamferIndependentFallback(t *testing.T) {
	b := New()
	stock := box(t, b, vector{}, vector{40, 30, 10})
	tool := box(t, b, vector{15, 10, 8}, vector{25, 20, 10})
	pocket := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": tool.Handle})
	owner := b.shapes[pocket.Handle]
	native, ok, e := owner.planarCutBoundary()
	if e != nil || !ok {
		t.Fatal(e)
	}
	a, c := geom.Vec3{Z: 10}, geom.Vec3{X: 40, Z: 10}
	n := geom.Vec3{Z: 1}
	if _, e = chamferSupportingChain(native, a, c, n, 1); e == nil {
		t.Fatal("fixture no longer exercises unsupported cap-hole clip")
	}
	result := shape(t, b, map[string]any{"op": "chamfer", "handle": pocket.Handle, "edgeStart": coordinates(a), "edgeEnd": coordinates(c), "normal": coordinates(n), "distance": 1})
	near(t, result.GeometryAssertions.Volume, 11780)
	verifyRetainedPlanarBore(t, b, result)
	if b.shapes[pocket.Handle].analytic != owner.analytic || !b.shapes[pocket.Handle].edgeAvailable(a, c) {
		t.Fatal("source ownership changed")
	}
	if b.shapes[result.Handle].edgeAvailable(a, c) {
		t.Fatal("consumed edge revived")
	}
}
