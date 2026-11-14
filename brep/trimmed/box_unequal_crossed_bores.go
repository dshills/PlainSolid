package trimmed

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// unequalBoreTrim retains the nonrational intersection of two perpendicular
// cylinders. Parameter u belongs to the smaller cylinder. Endpoint anchors
// only share bounded floating representatives; interior support is unchanged.
type unequalBoreTrim struct {
	a, b, sign, from, to float64
	large                bool
	start, end           geom.Vec2
}

func (c unequalBoreTrim) raw(t float64) (geom.Vec2, error) {
	if !finite(t) || t < 0 || t > 1 {
		return geom.Vec2{}, geom.ErrDomain
	}
	u := (1-t)*c.from + t*c.to
	s, k := math.Sincos(u)
	h := math.Sqrt((c.b-c.a)*(c.b+c.a) + c.a*c.a*k*k)
	if !finite(h) || h <= 0 {
		return geom.Vec2{}, geom.ErrDomain
	}
	if !c.large {
		return geom.Vec2{X: u, Y: c.sign * h}, nil
	}
	angle := math.Atan2(c.sign*h, c.a*s)
	if angle < 0 {
		angle += 2 * math.Pi
	}
	return geom.Vec2{X: angle, Y: c.a * k}, nil
}
func (c unequalBoreTrim) EvalUV(t float64) (geom.Vec2, error) {
	if t == 0 {
		return c.start, nil
	}
	if t == 1 {
		return c.end, nil
	}
	return c.raw(t)
}
func (c unequalBoreTrim) DerivativeUV(t float64) (geom.Vec2, error) {
	if _, e := c.raw(t); e != nil {
		return geom.Vec2{}, e
	}
	u := (1-t)*c.from + t*c.to
	s, k := math.Sincos(u)
	h := math.Sqrt((c.b-c.a)*(c.b+c.a) + c.a*c.a*k*k)
	scale := c.to - c.from
	if !c.large {
		return geom.Vec2{X: scale, Y: -c.sign * c.a * c.a * s * k / h * scale}, nil
	}
	return geom.Vec2{X: -c.sign * c.a * k / h * scale, Y: -c.a * s * scale}, nil
}
func (c unequalBoreTrim) BoundsUV() (geom.UVBox, error) {
	lo, hi := geom.Vec2{X: min(c.from, c.to), Y: 0}, geom.Vec2{X: max(c.from, c.to), Y: c.b}
	if c.sign < 0 {
		lo.Y, hi.Y = -hi.Y, -lo.Y
	}
	if c.large {
		// A whole half-chart bounds the oval without relying on rounded
		// inverse-trigonometric extrema near the equal-radius limit.
		lo, hi = geom.Vec2{X: 0, Y: -c.a}, geom.Vec2{X: math.Pi, Y: c.a}
		if c.sign < 0 {
			lo.X, hi.X = math.Pi, 2*math.Pi
		}
	}
	lo.X = math.Nextafter(min(lo.X, c.start.X, c.end.X), math.Inf(-1))
	lo.Y = math.Nextafter(min(lo.Y, c.start.Y, c.end.Y), math.Inf(-1))
	hi.X = math.Nextafter(max(hi.X, c.start.X, c.end.X), math.Inf(1))
	hi.Y = math.Nextafter(max(hi.Y, c.start.Y, c.end.Y), math.Inf(1))
	return geom.UVBox{Min: lo, Max: hi}, nil
}

// BoxUnequalCrossedBores retains the analytic nonrational intersection of
// full-through local X/Z cylinders. The signed-cardinal box spans [0,dims].
// Radii must be strictly separated and both complete intersections interior.
// Returned measures use converged surface quadrature, independently of mesh;
// they are numerical evaluations of retained analytic geometry.
func BoxUnequalCrossedBores(ctx context.Context, frame geom.Frame, dims, crossing geom.Vec3, xRadius, zRadius, tolerance float64) (BoxCrossedBoresResult, error) {
	fail := func(e error) (BoxCrossedBoresResult, error) { return BoxCrossedBoresResult{}, e }
	if ctx == nil || !dims.IsFinite() || !crossing.IsFinite() || !finite(xRadius) || !finite(zRadius) || !finite(tolerance) || tolerance <= 0 || min(xRadius, zRadius) <= 8*tolerance {
		return fail(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	if zRadius > xRadius {
		f, e := geom.NewFrame(frame.ToWorld(geom.Vec3{Y: dims.Y}), frame.ZAxis(), frame.XAxis())
		if e != nil {
			return fail(e)
		}
		return BoxUnequalCrossedBores(ctx, f, geom.Vec3{X: dims.Z, Y: dims.Y, Z: dims.X}, geom.Vec3{X: crossing.Z, Y: dims.Y - crossing.Y, Z: crossing.X}, zRadius, xRadius, tolerance)
	}
	a, b := zRadius, xRadius
	if b-a <= max(8*tolerance, 1024*0x1p-52*b) || !finite((b-a)*(b+a)) {
		return fail(geom.ErrDomain)
	}
	for _, v := range []geom.Vec3{frame.XAxis(), frame.YAxis(), frame.ZAxis()} {
		n := 0
		for _, q := range []float64{v.X, v.Y, v.Z} {
			if q != 0 {
				if q != 1 && q != -1 {
					return fail(geom.ErrDomain)
				}
				n++
			}
		}
		if n != 1 {
			return fail(geom.ErrDomain)
		}
	}
	for i, d := range []float64{dims.X, dims.Y, dims.Z} {
		c := []float64{crossing.X, crossing.Y, crossing.Z}[i]
		if c-b <= 8*tolerance || d-c-b <= 8*tolerance {
			return fail(geom.ErrDomain)
		}
	}
	builder := cornerCylinderBuilder{ctx: ctx, frame: frame, verticesByPoint: map[geom.Vec3]int{}, edgesByEnds: map[[2]int]int{}}
	represent := func(points []geom.Vec3) ([]geom.Vec3, error) {
		out := make([]geom.Vec3, len(points))
		for i, p := range points {
			q := frame.ToLocal(frame.ToWorld(p))
			if !q.IsFinite() || q.Sub(p).Length() > tolerance/8 {
				return nil, geom.ErrUnrepresentable
			}
			out[i] = q
		}
		return out, nil
	}
	wire := func(surface geom.Surface, points []geom.Vec3, uv []geom.Vec2, curves map[int]PCurve) ([]Use, error) {
		p, e := represent(points)
		if e != nil {
			return nil, e
		}
		return builder.wire(surface, p, uv, curves)
	}
	face := func(surface geom.Surface, points []geom.Vec3, uv []geom.Vec2, curves map[int]PCurve, reversed bool) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		loop, e := wire(surface, points, uv, curves)
		if e == nil {
			builder.faces = append(builder.faces, Face{Surface: surface, Outer: loop, Reversed: reversed})
		}
		return e
	}
	x, y, z := dims.X, dims.Y, dims.Z
	polys := [][]geom.Vec3{{{}, {Y: y}, {X: x, Y: y}, {X: x}}, {{Z: z}, {X: x, Z: z}, {X: x, Y: y, Z: z}, {Y: y, Z: z}}, {{}, {X: x}, {X: x, Z: z}, {Z: z}}, {{X: x}, {X: x, Y: y}, {X: x, Y: y, Z: z}, {X: x, Z: z}}, {{Y: y}, {Y: y, Z: z}, {X: x, Y: y, Z: z}, {X: x, Y: y}}, {{}, {Z: z}, {Y: y, Z: z}, {Y: y}}}
	for _, p := range polys {
		o := frame.ToWorld(p[0])
		dx, dy := frame.ToWorld(p[1]).Sub(o), frame.ToWorld(p[3]).Sub(o)
		f, e := geom.NewFrame(o, dx, dx.Cross(dy))
		if e != nil {
			return fail(e)
		}
		s, e := geom.NewPlaneSurface(f)
		if e != nil {
			return fail(e)
		}
		uv := make([]geom.Vec2, 4)
		for i, q := range p {
			v := f.ToLocal(frame.ToWorld(q))
			uv[i] = geom.Vec2{X: v.X, Y: v.Y}
		}
		if e = face(s, p, uv, nil, false); e != nil {
			return fail(e)
		}
	}
	cos, sin := []float64{1, 0, -1, 0, 1}, []float64{0, 1, 0, -1, 0}
	vector := func(v geom.Vec3) geom.Vec3 {
		return frame.XAxis().Scale(v.X).Add(frame.YAxis().Scale(v.Y)).Add(frame.ZAxis().Scale(v.Z))
	}
	sf, e := geom.NewFrame(frame.ToWorld(crossing), frame.XAxis(), frame.ZAxis())
	if e != nil {
		return fail(e)
	}
	small, e := geom.NewCylinderSurface(sf, a)
	if e != nil {
		return fail(e)
	}
	lf, e := geom.NewFrame(frame.ToWorld(crossing), frame.YAxis(), frame.XAxis())
	if e != nil {
		return fail(e)
	}
	large, e := geom.NewCylinderSurface(lf, b)
	if e != nil {
		return fail(e)
	}
	smallPoint := func(i int, h float64) geom.Vec3 { return crossing.Add(geom.Vec3{X: a * cos[i], Y: a * sin[i], Z: h}) }
	largePoint := func(i int, h float64) geom.Vec3 { return crossing.Add(geom.Vec3{X: h, Y: b * cos[i], Z: b * sin[i]}) }
	for axis := 0; axis < 2; axis++ {
		radius := a
		rx, ry, dir := geom.Vec3{X: 1}, geom.Vec3{Y: 1}, geom.Vec3{Z: 1}
		caps := [2]int{0, 1}
		low, high := -crossing.Z, z-crossing.Z
		point := smallPoint
		if axis == 1 {
			radius = b
			rx, ry, dir = geom.Vec3{Y: 1}, geom.Vec3{Z: 1}, geom.Vec3{X: 1}
			caps = [2]int{5, 3}
			low, high = -crossing.X, x-crossing.X
			point = largePoint
		}
		for k, h := range []float64{low, high} {
			cap := &builder.faces[caps[k]]
			plane := cap.Surface.(geom.PlaneSurface).Frame()
			indices := []int{0, 1, 2, 3}
			if k == 1 {
				indices = []int{0, 3, 2, 1}
			}
			points, uv := make([]geom.Vec3, 4), make([]geom.Vec2, 4)
			curves := map[int]PCurve{}
			center := plane.ToLocal(frame.ToWorld(crossing.Add(dir.Scale(h))))
			vx, vy := vector(rx).Scale(radius), vector(ry).Scale(radius)
			for j, i := range indices {
				points[j] = point(i, h)
				q := plane.ToLocal(frame.ToWorld(points[j]))
				uv[j] = geom.Vec2{X: q.X, Y: q.Y}
			}
			for j, i := range indices {
				from := float64(i) * math.Pi / 2
				to := from + math.Pi/2
				if k == 1 {
					if j == 0 {
						from = 2 * math.Pi
					}
					to = from - math.Pi/2
				}
				c := EllipseUV{Center: geom.Vec2{X: center.X, Y: center.Y}, Cos: geom.Vec2{X: vx.Dot(plane.XAxis()), Y: vx.Dot(plane.YAxis())}, Sin: geom.Vec2{X: vy.Dot(plane.XAxis()), Y: vy.Dot(plane.YAxis())}, From: from, To: to}
				for n, want := range []geom.Vec2{uv[j], uv[(j+1)%4]} {
					got, err := c.EvalUV(float64(n))
					if err != nil || math.Hypot(got.X-want.X, got.Y-want.Y) > tolerance {
						return fail(geom.ErrUnrepresentable)
					}
				}
				curves[j] = unionArcUV{c, uv[j], uv[(j+1)%4]}
			}
			loop, e := wire(cap.Surface, points, uv, curves)
			if e != nil {
				return fail(e)
			}
			cap.Holes = append(cap.Holes, loop)
		}
	}
	root := math.Sqrt((b - a) * (b + a))
	height := []float64{b, root, b, root, b}
	checked := func(c unequalBoreTrim) (PCurve, error) {
		for i, p := range []geom.Vec2{c.start, c.end} {
			q, e := c.raw(float64(i))
			if e != nil || math.Hypot(q.X-p.X, q.Y-p.Y) > tolerance {
				return nil, geom.ErrUnrepresentable
			}
		}
		return c, nil
	}
	for i := 0; i < 4; i++ {
		u, v := float64(i)*math.Pi/2, float64(i+1)*math.Pi/2
		for _, upper := range []bool{false, true} {
			lo0, lo1, hi0, hi1 := -crossing.Z, -crossing.Z, -height[i], -height[i+1]
			sign := -1.
			edge := 2
			from, to := v, u
			if upper {
				lo0, lo1, hi0, hi1 = height[i], height[i+1], z-crossing.Z, z-crossing.Z
				sign = 1
				edge = 0
				from, to = u, v
			}
			points := []geom.Vec3{smallPoint(i, lo0), smallPoint(i+1, lo1), smallPoint(i+1, hi1), smallPoint(i, hi0)}
			uv := []geom.Vec2{{X: u, Y: lo0}, {X: v, Y: lo1}, {X: v, Y: hi1}, {X: u, Y: hi0}}
			curve, e := checked(unequalBoreTrim{a, b, sign, from, to, false, uv[edge], uv[(edge+1)%4]})
			if e != nil {
				return fail(e)
			}
			if e = face(small, points, uv, map[int]PCurve{edge: curve}, true); e != nil {
				return fail(e)
			}
		}
	}
	for half := 0; half < 2; half++ {
		indices := []int{2 * half, 2*half + 1, 2*half + 2, 2*half + 2, 2*half + 1, 2 * half}
		points, uv := make([]geom.Vec3, 6), make([]geom.Vec2, 6)
		for j, i := range indices {
			h := -crossing.X
			if j >= 3 {
				h = x - crossing.X
			}
			points[j] = largePoint(i, h)
			uv[j] = geom.Vec2{X: float64(i) * math.Pi / 2, Y: h}
		}
		if e = face(large, points, uv, nil, true); e != nil {
			return fail(e)
		}
		f := &builder.faces[len(builder.faces)-1]
		sign := 1.
		order := []int{0, 3, 2, 1}
		if half == 1 {
			sign = -1
			order = []int{0, 1, 2, 3}
		}
		hp, huv := make([]geom.Vec3, 4), make([]geom.Vec2, 4)
		for j, i := range order {
			hp[j] = smallPoint(i, sign*height[i])
			theta := math.Atan2(sign*height[i], a*sin[i])
			if theta < 0 {
				theta += 2 * math.Pi
			}
			huv[j] = geom.Vec2{X: theta, Y: a * cos[i]}
		}
		curves := map[int]PCurve{}
		for j, i := range order {
			from := float64(i) * math.Pi / 2
			to := from + math.Pi/2
			if half == 0 {
				if j == 0 {
					from = 2 * math.Pi
				}
				to = from - math.Pi/2
			}
			curve, e := checked(unequalBoreTrim{a, b, sign, from, to, true, huv[j], huv[(j+1)%4]})
			if e != nil {
				return fail(e)
			}
			curves[j] = curve
		}
		hole, e := wire(large, hp, huv, curves)
		if e != nil {
			return fail(e)
		}
		f.Holes = append(f.Holes, hole)
	}
	solid, e := NewSolid(builder.vertices, builder.edges, builder.faces, tolerance)
	if e != nil {
		return fail(e)
	}
	mass, e := solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-10, MaxSubdivisions: 128})
	if e != nil {
		return fail(e)
	}
	volume := x * y * z
	area := 2 * (x*y + x*z + y*z)
	if !finite(volume) || !finite(area) {
		return fail(geom.ErrInvalid)
	}
	return BoxCrossedBoresResult{Solid: solid, RemovedVolume: volume - mass.Volume, SurfaceAreaDelta: mass.Area - area}, nil
}

// TessellateUnequalCrossedBores uses conforming directional refinement for
// the qualified radical-cylinder trim family. Caller limits and the original
// sampled deflection/normal checks are unchanged; no chord boundary substitutes
// for the retained analytic curves.
func (s Solid) TessellateUnequalCrossedBores(ctx context.Context, o parametric.TessellationOptions) (mesh.Mesh, error) {
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
	if len(s.faces) != 16 {
		return mesh.Mesh{}, geom.ErrDomain
	}
	radical := false
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
				case unequalBoreTrim:
					radical = true
				case nil, EllipseUV, unionArcUV:
				default:
					return mesh.Mesh{}, geom.ErrDomain
				}
			}
		}
	}
	if !radical {
		return mesh.Mesh{}, geom.ErrDomain
	}
	return s.tessellateAnalyticAdaptive(ctx, o)
}
