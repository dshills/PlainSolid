package plaincadbridge

import (
	"context"
	"math"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

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
	if s.planar() {
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
	} else if s.analytic != nil && s.analytic.filletedCap != nil && q.Op == "capChamfer" {
		return b.chamferOppositeCap(s, q, normal)
	} else if s.analytic != nil && b.generalResult(s.analytic) {
		return b.generalCapPerimeter(s, q, normal, d)
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
		return b.putAnalyticBody(&analyticBody{solid: body, volume: s.result.GeometryAssertions.Volume - removed,
			area: area, filletedCap: &filletedCapStock{frame, size, d}})
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

func (b *Bridge) fusePlainCircularBoss(a stored, c *cylinderBody) (Shape, bool, error) {
	frame, err := geom.FrameFromNormal(geom.Vec3{}, c.frame.ZAxis())
	if err != nil || !cardinalFrame(frame) {
		return Shape{}, false, nil
	}
	bounds, ok := orthogonalPrism(a.solid, frame)
	if !ok {
		return Shape{}, false, nil
	}
	frame, err = geom.NewFrame(frame.ToWorld(bounds.Min), frame.XAxis(), frame.ZAxis())
	if err != nil {
		return Shape{}, false, nil
	}
	size := bounds.Max.Sub(bounds.Min)
	p := frame.ToLocal(c.frame.Origin())
	if p.Z != size.Z || frame.ToWorld(p) != c.frame.Origin() {
		return Shape{}, false, nil
	}
	boss := trimmed.ExtrusionCircle{Center: geom.Vec2{X: p.X, Y: p.Y}, Radius: c.radius}
	if min(p.X, p.Y, size.X-p.X, size.Y-p.Y) <= c.radius {
		return Shape{}, false, nil
	}
	solid, err := trimmed.BoxBoreAndBoss(context.Background(), frame, size, trimmed.ExtrusionCircle{}, boss, 0, c.height)
	if err != nil {
		return Shape{}, true, err
	}
	proof := a.result.GeometryAssertions
	result, err := b.putAnalytic(solid, proof.Volume+math.Pi*c.radius*c.radius*c.height, proof.SurfaceArea+2*math.Pi*c.radius*c.height)
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
