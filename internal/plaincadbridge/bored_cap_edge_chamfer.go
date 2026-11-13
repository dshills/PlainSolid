package plaincadbridge

import (
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
)

// chamferLayeredBoreEdge handles one bottom longitudinal edge of an original
// rectangular planar stock. Every retained cylinder must be parallel to the
// extrusion and strictly clear the entire chamfer wedge. Original cutters are
// replayed against the exact convex planar base, preserving depth-layer rims.
func (b *Bridge) chamferLayeredBoreEdge(s stored, q request, edges [][2]geom.Vec3, normal geom.Vec3) (Shape, bool, error) {
	if s.bore == nil || q.Op != "chamfer" || len(edges) != 1 {
		return Shape{}, false, nil
	}
	fail := func(message string) (Shape, bool, error) { return Shape{}, true, unsupportedEdge(message) }
	body, e := inferEdgeStock(s.bore.base, edges[0][0], edges[0][1], normal)
	if e != nil {
		return Shape{}, false, nil
	}
	a, c := body.frame.ToLocal(edges[0][0]), body.frame.ToLocal(edges[0][1])
	if a.Y != 0 || c.Y != 0 || a.Z != 0 || c.Z != 0 || !((a.X == 0 && c.X == body.length) || (c.X == 0 && a.X == body.length)) {
		return Shape{}, false, nil
	}
	d := float64(q.Distance)
	if !finite(d) || d <= 0 || d >= body.width || d >= body.height {
		return fail("selected chamfer exceeds rectangular stock")
	}
	for _, tool := range s.bore.cylinders {
		axis := tool.frame.ZAxis()
		if axis != body.frame.ZAxis() && axis != body.frame.ZAxis().Scale(-1) {
			return fail("retained bore axis does not match cap")
		}
		p := body.frame.ToLocal(tool.frame.Origin())
		if body.frame.ToWorld(p) != tool.frame.Origin() {
			return fail("retained bore placement is not exactly representable")
		}
		clear := new(big.Rat).Sub(new(big.Rat).SetFloat64(p.Y), new(big.Rat).SetFloat64(tool.radius))
		clear.Sub(clear, new(big.Rat).SetFloat64(d))
		if clear.Sign() <= 0 {
			return fail("retained cylindrical footprint does not strictly clear chamfer wedge")
		}
	}
	yz := []geom.Vec2{{X: d}, {X: body.width}, {X: body.width, Y: body.height}, {Y: body.height}, {Y: d}}
	points := make([]geom.Vec3, len(yz))
	for i, p := range yz {
		local := geom.Vec3{Y: p.X, Z: p.Y}
		points[i] = body.frame.ToWorld(local)
		if body.frame.ToLocal(points[i]) != local {
			return fail("chamfer vertices are not exactly representable")
		}
	}
	base, e := brep.Prism(points, body.frame.XAxis().Scale(body.length))
	if e != nil {
		return Shape{}, true, e
	}
	replay, e := newBore(base, append([]*cylinderBody(nil), s.bore.cylinders...))
	if e != nil {
		return Shape{}, true, e
	}
	result, e := b.putBore(replay)
	return result, true, e
}
