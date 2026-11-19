package boolean

import "plainsolid/geom"

// chart is the planar coordinate domain in which the faces of one surface
// carrier are split (section 3). Splitting, region tests and the
// piece-based volumes are written against it, so that a new carrier
// arrives as a chart and not as a second path.
type chart interface {
	// locate gives the chart coordinates of a point on the carrier.
	locate(p geom.Vec3) geom.Vec2
	// lift gives the point of the carrier at chart coordinates q.
	lift(q geom.Vec2) geom.Vec3
	// outline gives the leg as a chain of chart points, both ends
	// included, fine enough for areas and crossing counts.
	outline(l leg) []geom.Vec2
	// heading gives the unit chart direction in which the leg leaves its
	// start, and its signed curvature there, positive to the left.
	heading(l leg) (geom.Vec2, float64)
	// flux integrates x·n / 3 over the region the loops bound, where n is
	// the carrier normal: the region's share of an enclosed volume.
	flux(loops [][]leg) float64
}

// planeChart is the chart of a plane: the frame coordinates x and y.
type planeChart plane

func (c planeChart) locate(p geom.Vec3) geom.Vec2 {
	q := c.frame.ToLocal(p)
	return geom.Vec2{X: q.X, Y: q.Y}
}
func (c planeChart) lift(q geom.Vec2) geom.Vec3 { return c.frame.ToWorld(geom.Vec3{X: q.X, Y: q.Y}) }
func (c planeChart) outline(l leg) []geom.Vec2 {
	return []geom.Vec2{c.locate(l.c.at(l.t0)), c.locate(l.c.at(l.t1))}
}
func (c planeChart) heading(l leg) (geom.Vec2, float64) {
	d := l.c.tangent(l.t0)
	if l.t1 < l.t0 {
		d = d.Scale(-1)
	}
	return unit2(geom.Vec2{X: d.Dot(c.frame.XAxis()), Y: d.Dot(c.frame.YAxis())}), 0
}
func (c planeChart) flux(loops [][]leg) float64 {
	return c.frame.Origin().Dot(c.frame.ZAxis()) * chartArea(c, loops) / 3
}

// ring is the chart polygon of a closed chain of legs.
func ring(c chart, loop []leg) []geom.Vec2 {
	var out []geom.Vec2
	for _, l := range loop {
		points := c.outline(l)
		out = append(out, points[:len(points)-1]...)
	}
	return out
}

// polygonArea is the signed area of a chart polygon, positive when it is
// wound counterclockwise.
func polygonArea(points []geom.Vec2) float64 {
	sum := 0.
	for i, p := range points {
		q := points[(i+1)%len(points)]
		sum += p.X*q.Y - q.X*p.Y
	}
	return sum / 2
}

// chartArea is the signed chart area the loops bound: outer loops count
// positive and holes negative.
func chartArea(c chart, loops [][]leg) float64 {
	sum := 0.
	for _, loop := range loops {
		sum += polygonArea(ring(c, loop))
	}
	return sum
}

func unit2(v geom.Vec2) geom.Vec2 {
	n := geom.Vec3{X: v.X, Y: v.Y}.Length()
	if n == 0 {
		return v
	}
	return geom.Vec2{X: v.X / n, Y: v.Y / n}
}
