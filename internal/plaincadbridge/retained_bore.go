package plaincadbridge

import (
	"context"
	"math"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func rectangularBoredStock(body *boreBody, normal geom.Vec3, allowBlind bool) (geom.Frame, geom.Vec3, trimmed.ExtrusionCircle, bool) {
	if body == nil || len(body.cylinders) != 1 || body.layered != nil || body.union != nil || body.blind[0] && !allowBlind {
		return geom.Frame{}, geom.Vec3{}, trimmed.ExtrusionCircle{}, false
	}
	c := body.cylinders[0]
	if c.frame.ZAxis() != normal && c.frame.ZAxis() != normal.Scale(-1) {
		return geom.Frame{}, geom.Vec3{}, trimmed.ExtrusionCircle{}, false
	}
	f, err := geom.FrameFromNormal(geom.Vec3{}, normal)
	if err != nil || !cardinalFrame(f) {
		return geom.Frame{}, geom.Vec3{}, trimmed.ExtrusionCircle{}, false
	}
	stock, ok := orthogonalPrism(body.base, f)
	if !ok {
		return geom.Frame{}, geom.Vec3{}, trimmed.ExtrusionCircle{}, false
	}
	frame, err := geom.NewFrame(f.ToWorld(stock.Min), f.XAxis(), f.ZAxis())
	if err != nil {
		return geom.Frame{}, geom.Vec3{}, trimmed.ExtrusionCircle{}, false
	}
	center := frame.ToLocal(c.frame.Origin())
	return frame, stock.Max.Sub(stock.Min), trimmed.ExtrusionCircle{Center: geom.Vec2{X: center.X, Y: center.Y}, Radius: c.radius}, true
}

// fuseBoredStock proves the planar join and that its tool never fills the
// retained bore. A successful rectangular union replays the original cutter
// against the new native boundary.
func (b *Bridge) fuseBoredStock(a *boreBody, tool brep.Solid) (Shape, bool, error) {
	if len(a.cylinders) != 1 {
		return Shape{}, false, nil
	}
	c := a.cylinders[0]
	f, size, hole, ok := rectangularBoredStock(a, c.frame.ZAxis(), false)
	if !ok {
		return Shape{}, false, nil
	}
	extent, ok := orthogonalPrism(tool, f)
	if !ok || extent.Min.Z != 0 || extent.Max.Z != size.Z {
		return Shape{}, false, nil
	}
	nearX := max(extent.Min.X, min(extent.Max.X, hole.Center.X))
	nearY := max(extent.Min.Y, min(extent.Max.Y, hole.Center.Y))
	if math.Hypot(nearX-hole.Center.X, nearY-hole.Center.Y) <= hole.Radius+1e-8 {
		return Shape{}, false, nil
	}
	alongX := extent.Min.Y == 0 && extent.Max.Y == size.Y && extent.Min.X <= size.X && extent.Max.X >= 0
	alongY := extent.Min.X == 0 && extent.Max.X == size.X && extent.Min.Y <= size.Y && extent.Max.Y >= 0
	if !alongX && !alongY {
		return Shape{}, false, nil
	}
	lo := geom.Vec3{X: min(0, extent.Min.X), Y: min(0, extent.Min.Y)}
	hi := geom.Vec3{X: max(size.X, extent.Max.X), Y: max(size.Y, extent.Max.Y), Z: size.Z}
	if (hi.X-lo.X)*(hi.Y-lo.Y) <= size.X*size.Y {
		return Shape{}, false, nil
	}
	bounds, err := frameMap(f).ApplyBox(geom.Box{Min: lo, Max: hi})
	if err != nil {
		return Shape{}, true, err
	}
	union, err := brep.Box(bounds.Min, bounds.Max)
	if err != nil {
		return Shape{}, true, err
	}
	body, err := newBore(union, a.cylinders)
	if err != nil {
		return Shape{}, true, err
	}
	result, err := b.putBore(body)
	return result, true, err
}

// treatCapPerimeter resolves all four complete authored edges on one stock cap.
// A side added by a later join cannot silently replace a missing edge.
func (b *Bridge) treatCapPerimeter(s stored, q request) (Shape, error) {
	reject := func() (Shape, error) {
		return Shape{}, unsupportedEdge("original cap perimeter was lost or is ambiguous")
	}
	d := float64(q.Distance)
	if q.Op == "capFillet" {
		d = float64(q.Radius)
	}
	if q.Normal == nil || len(q.Edges) != 4 || d <= 0 {
		return reject()
	}
	normal, err := q.Normal.vec().Unit()
	if err != nil {
		return reject()
	}
	var frame geom.Frame
	var size geom.Vec3
	var hole trimmed.ExtrusionCircle
	if s.bore != nil {
		var ok bool
		frame, size, hole, ok = rectangularBoredStock(s.bore, normal, false)
		if !ok {
			return reject()
		}
	} else if s.planar() {
		f, err := geom.FrameFromNormal(geom.Vec3{}, normal)
		if err != nil || !cardinalFrame(f) {
			return reject()
		}
		stock, ok := orthogonalPrism(s.solid, f)
		if !ok {
			return reject()
		}
		frame, err = geom.NewFrame(f.ToWorld(stock.Min), f.XAxis(), f.ZAxis())
		if err != nil {
			return reject()
		}
		size = stock.Max.Sub(stock.Min)
	} else {
		return reject()
	}
	rim := [4]geom.Vec3{{Z: size.Z}, {X: size.X, Z: size.Z}, {X: size.X, Y: size.Y, Z: size.Z}, {Y: size.Y, Z: size.Z}}
	seen := [4]bool{}
	for _, ref := range q.Edges {
		if ref.Start == nil || ref.End == nil {
			return reject()
		}
		a, c := ref.Start.vec(), ref.End.vec()
		found := false
		for i, p := range rim {
			if !seen[i] && sameEnds(a, c, frame.ToWorld(p), frame.ToWorld(rim[(i+1)%len(rim)])) && s.edgeAvailable(a, c) {
				seen[i], found = true, true
				break
			}
		}
		if !found {
			return reject()
		}
	}
	if q.Op == "capFillet" {
		if hole.Radius != 0 {
			return Shape{}, unsupportedEdge("complete cap fillet requires qualified rectangular stock")
		}
		body, err := trimmed.TopFilletedBox(context.Background(), frame, size.X, size.Y, size.Z, d, 1e-9)
		if err != nil {
			return Shape{}, err
		}
		w, h := size.X, size.Y
		removed := 2*(w+h)*d*d*(1-math.Pi/4) - 4*d*d*d*(5./3-math.Pi/2)
		area := w*h + (w-2*d)*(h-2*d) + 2*(w+h)*(size.Z-d) +
			2*d*((w+h-4*d)*math.Pi/2+4*d)
		return b.putAnalytic(body, s.result.GeometryAssertions.Volume-removed, area)
	}
	body, err := trimmed.TopChamferedBox(context.Background(), frame, size.X, size.Y, size.Z, d, hole.Center, hole.Radius, 1e-9)
	if err != nil {
		return Shape{}, err
	}
	r := hole.Radius
	volume := s.result.GeometryAssertions.Volume - d*d*(size.X+size.Y) + 4*d*d*d/3
	area := 2*size.X*size.Y + 2*(size.X+size.Y)*(size.Z-d) -
		2*d*(size.X+size.Y) + 4*d*d + 2*(size.X+size.Y-2*d)*d*math.Sqrt2 + 2*math.Pi*r*(size.Z-r)
	return b.putAnalytic(body, volume, area)
}

// fuseCircularBoss sews a positive cylinder to the actual retained cap.
// The stock and added disk remain separate bounded recipe inputs.
func (b *Bridge) fuseCircularBoss(a *boreBody, c *cylinderBody) (Shape, bool, error) {
	f, size, hole, ok := rectangularBoredStock(a, c.frame.ZAxis(), true)
	if !ok {
		return Shape{}, false, nil
	}
	p := f.ToLocal(c.frame.Origin())
	if p.Z != size.Z || f.ToWorld(p) != c.frame.Origin() {
		return Shape{}, false, nil
	}
	if a.blind[0] && a.cylinders[0].frame.ZAxis() != f.ZAxis() {
		return Shape{}, false, nil
	}
	boss := trimmed.ExtrusionCircle{Center: geom.Vec2{X: p.X, Y: p.Y}, Radius: c.radius}
	s, err := trimmed.BoxBoreAndBoss(context.Background(), f, size, hole, boss, a.spans[0], c.height)
	if err != nil {
		return Shape{}, true, err
	}
	volume, area, err := a.measures()
	if err != nil {
		return Shape{}, true, err
	}
	result, err := b.putAnalytic(s, volume+math.Pi*c.radius*c.radius*c.height, area+2*math.Pi*c.radius*c.height)
	return result, true, err
}

func joinedRectangularStock(a, b brep.Solid) (brep.Solid, bool) {
	f, err := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	if err != nil {
		return brep.Solid{}, false
	}
	x, ok := orthogonalPrism(a, f)
	if !ok {
		return brep.Solid{}, false
	}
	y, ok := orthogonalPrism(b, f)
	if !ok {
		return brep.Solid{}, false
	}
	aMin, aMax, bMin, bMax := coordinates(x.Min), coordinates(x.Max), coordinates(y.Min), coordinates(y.Max)
	for axis := range aMin {
		if aMax[axis] < bMin[axis] || bMax[axis] < aMin[axis] {
			return brep.Solid{}, false
		}
	}
	different := 0
	for axis := range aMin {
		if aMin[axis] != bMin[axis] || aMax[axis] != bMax[axis] {
			different++
		}
	}
	if different > 1 {
		return brep.Solid{}, false
	}
	bounds, err := x.Union(y)
	if err != nil {
		return brep.Solid{}, false
	}
	s, err := brep.Box(bounds.Min, bounds.Max)
	return s, err == nil
}
