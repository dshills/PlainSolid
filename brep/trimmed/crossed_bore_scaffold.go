package trimmed

import (
	"context"
	"plainsolid/geom"
)

// crossedStockBuilder preserves the original six rectangular stock faces.
func crossedStockBuilder(ctx context.Context, frame geom.Frame, dimensions geom.Vec3) (cornerCylinderBuilder, error) {
	b := cornerCylinderBuilder{ctx: ctx, frame: frame, verticesByPoint: map[geom.Vec3]int{}, edgesByEnds: map[[2]int]int{}}
	x, y, z := dimensions.X, dimensions.Y, dimensions.Z
	polygons := [][]geom.Vec3{
		{{}, {Y: y}, {X: x, Y: y}, {X: x}},
		{{Z: z}, {X: x, Z: z}, {X: x, Y: y, Z: z}, {Y: y, Z: z}},
		{{}, {X: x}, {X: x, Z: z}, {Z: z}},
		{{X: x}, {X: x, Y: y}, {X: x, Y: y, Z: z}, {X: x, Z: z}},
		{{Y: y}, {Y: y, Z: z}, {X: x, Y: y, Z: z}, {X: x, Y: y}},
		{{}, {Z: z}, {Y: y, Z: z}, {Y: y}},
	}
	for _, p := range polygons {
		a := frame.ToWorld(p[0])
		dx := frame.ToWorld(p[1]).Sub(a)
		dy := frame.ToWorld(p[3]).Sub(a)
		f, e := geom.NewFrame(a, dx, dx.Cross(dy))
		if e != nil {
			return cornerCylinderBuilder{}, e
		}
		surface, e := geom.NewPlaneSurface(f)
		if e != nil {
			return cornerCylinderBuilder{}, e
		}
		uv := make([]geom.Vec2, 4)
		for i, q := range p {
			l := f.ToLocal(frame.ToWorld(q))
			uv[i] = geom.Vec2{X: l.X, Y: l.Y}
		}
		if e = b.face(surface, p, uv, nil, false); e != nil {
			return cornerCylinderBuilder{}, e
		}
	}

	return b, nil
}

// crossedCapWire attaches an analytic circular opening with the caller's
// exact angular partition, preserving shared rim vertices and orientation.
func (b *cornerCylinderBuilder) crossedCapWire(frame geom.Frame, centerWorld, radialX, radialY geom.Vec3, radius float64, capID int, upper bool, angles []float64, point func(int) geom.Vec3, normalize func(geom.Vec3) (geom.Vec3, error)) error {
	cap := &b.faces[capID]
	plane := cap.Surface.(geom.PlaneSurface).Frame()
	center := plane.ToLocal(centerWorld)
	n := len(angles) - 1
	if n < 4 || n > 8 {
		return geom.ErrDomain
	}
	points := make([]geom.Vec3, n)
	uv := make([]geom.Vec2, n)
	curves := map[int]PCurve{}
	ids := make([]int, n)
	for j := range ids {
		ids[j] = j
		if upper {
			ids[j] = n - j
		}
	}
	for j, i := range ids {
		p, e := normalize(point(i))
		if e != nil {
			return e
		}
		points[j] = p
		q := plane.ToLocal(frame.ToWorld(p))
		uv[j] = geom.Vec2{X: q.X, Y: q.Y}
	}
	for j, i := range ids {
		end := i + 1
		if upper {
			end = i - 1
		}
		raw := EllipseUV{Center: geom.Vec2{X: center.X, Y: center.Y}, Cos: geom.Vec2{X: radius * radialX.Dot(plane.XAxis()), Y: radius * radialX.Dot(plane.YAxis())}, Sin: geom.Vec2{X: radius * radialY.Dot(plane.XAxis()), Y: radius * radialY.Dot(plane.YAxis())}, From: angles[i], To: angles[end]}
		curves[j] = unionArcUV{raw, uv[j], uv[(j+1)%n]}
	}
	wire, e := b.wire(cap.Surface, points, uv, curves)
	if e != nil {
		return e
	}
	cap.Holes = append(cap.Holes, wire)
	return nil
}
