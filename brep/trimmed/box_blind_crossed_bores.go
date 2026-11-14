package trimmed

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// BlindCrossedBoreFloorFace is the retained planar floor face index.
const BlindCrossedBoreFloorFace = 6

// BoxBlindCrossedBores removes a through X bore and finite Z bore of equal
// radius. The Z opening is on the lower cap unless oppositeCap is true. Its
// floor must lie strictly beyond the complete cylinder intersection.
func BoxBlindCrossedBores(ctx context.Context, frame geom.Frame, dimensions, crossing geom.Vec3, radius, depth float64, oppositeCap bool, tolerance float64) (BoxCrossedBoresResult, error) {
	return boxFiniteCrossedBores(ctx, frame, dimensions, crossing, radius, depth, oppositeCap, true, tolerance)
}

func boxFiniteCrossedBores(ctx context.Context, frame geom.Frame, dimensions, crossing geom.Vec3, radius, depth float64, oppositeCap, blind bool, tolerance float64) (BoxCrossedBoresResult, error) {
	fail := func(e error) (BoxCrossedBoresResult, error) { return BoxCrossedBoresResult{}, e }
	if ctx == nil || !dimensions.IsFinite() || !crossing.IsFinite() || !finite(radius) || !finite(tolerance) || tolerance <= 0 || radius <= 8*tolerance {
		return fail(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	for _, v := range []geom.Vec3{frame.XAxis(), frame.YAxis(), frame.ZAxis()} {
		n := 0
		for _, x := range []float64{v.X, v.Y, v.Z} {
			if x != 0 {
				if x != 1 && x != -1 {
					return fail(geom.ErrDomain)
				}
				n++
			}
		}
		if n != 1 {
			return fail(geom.ErrDomain)
		}
	}
	for i, d := range []float64{dimensions.X, dimensions.Y, dimensions.Z} {
		c := []float64{crossing.X, crossing.Y, crossing.Z}[i]
		if c-radius <= 8*tolerance || d-c-radius <= 8*tolerance {
			return fail(geom.ErrDomain)
		}
	}
	if blind && (!finite(depth) || depth <= 0 || depth >= dimensions.Z-8*tolerance) {
		return fail(geom.ErrDomain)
	}
	if !blind {
		depth = dimensions.Z
	}
	floor := depth
	if oppositeCap {
		floor = dimensions.Z - depth
	}
	if blind && ((!oppositeCap && floor <= crossing.Z+radius+8*tolerance) || (oppositeCap && floor >= crossing.Z-radius-8*tolerance)) {
		return fail(geom.ErrDomain)
	}
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
			return fail(e)
		}
		surface, e := geom.NewPlaneSurface(f)
		if e != nil {
			return fail(e)
		}
		uv := make([]geom.Vec2, 4)
		for i, q := range p {
			l := f.ToLocal(frame.ToWorld(q))
			uv[i] = geom.Vec2{X: l.X, Y: l.Y}
		}
		if e = b.face(surface, p, uv, nil, false); e != nil {
			return fail(e)
		}
	}
	cos := []float64{1, 0, -1, 0, 1}
	sin := []float64{0, 1, 0, -1, 0}
	for axis := 0; axis < 2; axis++ {
		// X cylinder radial axes are local Z,-Y; Z cylinder uses local X,Y.
		radialX, radialY, direction := geom.Vec3{X: 1}, geom.Vec3{Y: 1}, geom.Vec3{Z: 1}
		low, high := -crossing.Z, z-crossing.Z
		caps := [2]int{0, 1}
		if blind && axis == 0 {
			if oppositeCap {
				low = floor - crossing.Z
			} else {
				high = floor - crossing.Z
			}
		}
		if axis == 1 {
			radialX, radialY, direction = geom.Vec3{Z: 1}, geom.Vec3{Y: -1}, geom.Vec3{X: 1}
			low, high = -crossing.X, x-crossing.X
			caps = [2]int{5, 3}
		}
		worldVector := func(v geom.Vec3) geom.Vec3 {
			return frame.XAxis().Scale(v.X).Add(frame.YAxis().Scale(v.Y)).Add(frame.ZAxis().Scale(v.Z))
		}
		f, e := geom.NewFrame(frame.ToWorld(crossing), worldVector(radialX), worldVector(direction))
		if e != nil {
			return fail(e)
		}
		surface, e := geom.NewCylinderSurface(f, radius)
		if e != nil {
			return fail(e)
		}
		point := func(i int, h float64) geom.Vec3 {
			return crossing.Add(radialX.Scale(radius * cos[i])).Add(radialY.Scale(radius * sin[i])).Add(direction.Scale(h))
		}
		for k, h := range []float64{low, high} {
			isFloor := blind && axis == 0 && ((!oppositeCap && k == 1) || (oppositeCap && k == 0))
			cap := &b.faces[caps[k]]
			if isFloor {
				normal := worldVector(direction)
				if !oppositeCap {
					normal = normal.Scale(-1)
				}
				ff, err := geom.NewFrame(frame.ToWorld(crossing.Add(direction.Scale(h))), worldVector(radialX), normal)
				if err != nil {
					return fail(err)
				}
				ps, err := geom.NewPlaneSurface(ff)
				if err != nil {
					return fail(err)
				}
				b.faces = append(b.faces, Face{Surface: ps})
				cap = &b.faces[len(b.faces)-1]
			}
			plane := cap.Surface.(geom.PlaneSurface).Frame()
			points := make([]geom.Vec3, 4)
			uv := make([]geom.Vec2, 4)
			curves := map[int]PCurve{}
			// Lower outward normal is -axis, so its hole runs +angle.
			index := []int{0, 1, 2, 3}
			if k == 1 {
				index = []int{0, 3, 2, 1}
			}
			center := plane.ToLocal(frame.ToWorld(crossing.Add(direction.Scale(h))))
			vx, vy := worldVector(radialX).Scale(radius), worldVector(radialY).Scale(radius)
			for j, i := range index {
				points[j] = point(i, h)
				q := plane.ToLocal(frame.ToWorld(points[j]))
				uv[j] = geom.Vec2{X: q.X, Y: q.Y}
			}
			for j, i := range index {
				a := float64(i) * math.Pi / 2
				to := a + math.Pi/2
				if k == 1 {
					if j == 0 {
						a = 2 * math.Pi
					}
					to = a - math.Pi/2
				}
				curves[j] = EllipseUV{Center: geom.Vec2{X: center.X, Y: center.Y}, Cos: geom.Vec2{X: vx.Dot(plane.XAxis()), Y: vx.Dot(plane.YAxis())}, Sin: geom.Vec2{X: vy.Dot(plane.XAxis()), Y: vy.Dot(plane.YAxis())}, From: a, To: to}
			}
			for j, raw := range curves {
				c := raw.(EllipseUV)
				for k, want := range []geom.Vec2{uv[j], uv[(j+1)%4]} {
					got, err := c.EvalUV(float64(k))
					if err != nil || math.Hypot(got.X-want.X, got.Y-want.Y) > tolerance {
						return fail(geom.ErrUnrepresentable)
					}
				}
				curves[j] = unionArcUV{c, uv[j], uv[(j+1)%4]}
			}
			wire, e := b.wire(cap.Surface, points, uv, curves)
			if e != nil {
				return fail(e)
			}
			if isFloor {
				cap.Outer = wire
			} else {
				cap.Holes = append(cap.Holes, wire)
			}
		}
		// On the Z cylinder the other cylinder removes |z| < r|cos(u)|;
		// the X cylinder has the identical equation with x replacing z.
		// Quarter charts fix cos(u)'s sign. Their analytic rims lie in the
		// planes x=+z or x=-z and share endpoint IDs across both cylinders,
		// including the two tangent critical vertices at y=+r and y=-r.
		for i := 0; i < 4; i++ {
			a, c := float64(i)*math.Pi/2, float64(i+1)*math.Pi/2
			sign := 1.
			if i == 1 || i == 2 {
				sign = -1
			}
			for _, upper := range []bool{false, true} {
				u0, u1 := radius*math.Abs(cos[i]), radius*math.Abs(cos[i+1])
				lo0, lo1, hi0, hi1 := low, low, -u0, -u1
				coeff := -radius * sign
				if upper {
					lo0, lo1, hi0, hi1 = u0, u1, high, high
					coeff = radius * sign
				}
				points := []geom.Vec3{point(i, lo0), point(i+1, lo1), point(i+1, hi1), point(i, hi0)}
				uv := []geom.Vec2{{X: a, Y: lo0}, {X: c, Y: lo1}, {X: c, Y: hi1}, {X: a, Y: hi0}}
				curve := EllipseUV{Linear: geom.Vec2{X: 1}, Cos: geom.Vec2{Y: coeff}, From: c, To: a}
				edge := 2
				if upper {
					edge = 0
					curve.From, curve.To = a, c
				}
				for k, want := range []geom.Vec2{uv[edge], uv[(edge+1)%4]} {
					got, err := curve.EvalUV(float64(k))
					if err != nil || math.Hypot(got.X-want.X, got.Y-want.Y) > tolerance {
						return fail(geom.ErrUnrepresentable)
					}
				}
				if e = b.face(surface, points, uv, map[int]PCurve{edge: unionArcUV{curve, uv[edge], uv[(edge+1)%4]}}, true); e != nil {
					return fail(e)
				}
			}
		}
	}
	solid, e := NewSolid(b.vertices, b.edges, b.faces, tolerance)
	if e != nil {
		return fail(e)
	}
	removed := math.Pi*radius*radius*(x+depth) - 16*radius*radius*radius/3
	delta := 2*math.Pi*radius*(x+depth) - 16*radius*radius - 4*math.Pi*radius*radius
	if blind {
		delta += 2 * math.Pi * radius * radius
	}
	if !finite(removed) || !finite(delta) {
		return fail(geom.ErrInvalid)
	}
	return BoxCrossedBoresResult{solid, removed, delta}, nil
}

// TessellateBlindCrossedBores meshes the retained finite crossing topology.
func (s Solid) TessellateBlindCrossedBores(ctx context.Context, o parametric.TessellationOptions) (mesh.Mesh, error) {
	if ctx == nil || !finite(o.Deflection) || o.Deflection <= 0 || !finite(o.Angle) || o.Angle < 0 || o.Angle > math.Pi || o.MaxSubdivisions < 0 || o.MaxSubdivisions > 1024 || o.MaxTriangles < 0 {
		return mesh.Mesh{}, geom.ErrInvalid
	}
	if o.AnalyticBound {
		return mesh.Mesh{}, parametric.ErrDeflectionUnsupported
	}
	if e := ctx.Err(); e != nil {
		return mesh.Mesh{}, e
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 128
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 1000000
	}
	if len(s.faces) != 23 {
		return mesh.Mesh{}, geom.ErrDomain
	}
	found := false
	for _, f := range s.faces {
		base, _ := parametric.SurfaceMapping(f.Surface)
		switch base.(type) {
		case geom.PlaneSurface, geom.CylinderSurface:
		default:
			return mesh.Mesh{}, geom.ErrDomain
		}
		for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
			for _, u := range loop {
				switch u.Curve.(type) {
				case unionArcUV:
					found = true
				case nil, EllipseUV:
				default:
					return mesh.Mesh{}, geom.ErrDomain
				}
			}
		}
	}
	if !found {
		return mesh.Mesh{}, geom.ErrDomain
	}
	if m, ok, e := s.compactCylinderMesh(ctx, o); ok {
		return m, e
	}
	return s.tessellateAnalyticAdaptive(ctx, o)
}
