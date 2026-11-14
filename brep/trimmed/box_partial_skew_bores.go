package trimmed

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// partialSkewTrim uses a regular factored trigonometric parameter at both
// turning points of the nonrational cylinder-cylinder intersection.
type partialSkewTrim struct {
	a, b, d, sx, sz, from, to float64
	xChart                    bool
	start, end                geom.Vec2
}

func (c partialSkewTrim) point(t float64) (geom.Vec3, geom.Vec3, error) {
	if !finite(t) || t < 0 || t > 1 {
		return geom.Vec3{}, geom.Vec3{}, geom.ErrDomain
	}
	u := (1-t)*c.from + t*c.to
	sn, co := math.Sincos(u)
	if u == 0 {
		sn, co = 0, 1
	}
	if u == math.Pi/2 {
		sn, co = 1, 0
	}
	l := c.a + c.b - c.d
	rx := math.Sqrt(l * (2*c.b - l*sn*sn))
	rz := math.Sqrt(l * (2*c.a - l*co*co))
	if rx <= 0 || rz <= 0 || !finite(rx) || !finite(rz) {
		return geom.Vec3{}, geom.Vec3{}, geom.ErrDomain
	}
	y := c.a - l*co*co
	if u == 0 {
		y = c.d - c.b
	}
	if u == math.Pi/2 {
		y = c.a
	}
	p := geom.Vec3{X: c.sx * sn * rx, Y: y, Z: c.sz * co * rz}
	v := geom.Vec3{X: c.sx * (co*rx - l*l*sn*sn*co/rx), Y: 2 * l * sn * co, Z: c.sz * (-sn*rz + l*l*co*co*sn/rz)}.Scale(c.to - c.from)
	return p, v, nil
}
func (c partialSkewTrim) raw(t float64) (geom.Vec2, error) {
	p, _, e := c.point(t)
	if e != nil {
		return geom.Vec2{}, e
	}
	if c.xChart {
		return geom.Vec2{X: math.Atan2(p.Z, p.Y), Y: p.X}, nil
	}
	return geom.Vec2{X: math.Atan2(p.X, c.d-p.Y), Y: p.Z}, nil
}

// PartialSkewAngularInterval exposes the shared regular angular parameter
// used by both cylinder trims for conservative finite-span interpolation.
func (c partialSkewTrim) PartialSkewAngularInterval() (float64, float64) {
	return c.from, c.to
}

func (c partialSkewTrim) EvalUV(t float64) (geom.Vec2, error) {
	if t == 0 {
		return c.start, nil
	}
	if t == 1 {
		return c.end, nil
	}
	return c.raw(t)
}
func (c partialSkewTrim) DerivativeUV(t float64) (geom.Vec2, error) {
	p, v, e := c.point(t)
	if e != nil {
		return geom.Vec2{}, e
	}
	if c.xChart {
		return geom.Vec2{X: (p.Y*v.Z - p.Z*v.Y) / (c.a * c.a), Y: v.X}, nil
	}
	return geom.Vec2{X: ((c.d-p.Y)*v.X + p.X*v.Y) / (c.b * c.b), Y: v.Z}, nil
}
func (c partialSkewTrim) BoundsUV() (geom.UVBox, error) {
	r := c.a
	if c.xChart {
		r = c.b
	}
	return geom.UVBox{Min: geom.Vec2{X: math.Nextafter(-math.Pi, math.Inf(-1)), Y: math.Nextafter(-r, math.Inf(-1))}, Max: geom.Vec2{X: math.Nextafter(math.Pi, math.Inf(1)), Y: math.Nextafter(r, math.Inf(1))}}, nil
}

// BoxPartialSkewBores removes full-through local X and Z cylinders whose
// transverse disks overlap only partially: |xRadius-zRadius|<|offset|<sum.
// X runs through (crossing.Y,crossing.Z), Z through (crossing.X,crossing.Y+offset).
// All four stock openings must be strictly interior. Only signed cardinal
// placements are supported. Measures integrate retained analytic geometry.
func BoxPartialSkewBores(ctx context.Context, frame geom.Frame, dims, crossing geom.Vec3, xRadius, zRadius, offset, tolerance float64) (BoxCrossedBoresResult, error) {
	fail := func(e error) (BoxCrossedBoresResult, error) { return BoxCrossedBoresResult{}, e }
	if ctx == nil || !dims.IsFinite() || !crossing.IsFinite() || !finite(xRadius) || !finite(zRadius) || !finite(offset) || !finite(tolerance) || tolerance <= 0 || min(xRadius, zRadius) <= 8*tolerance {
		return fail(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	if offset < 0 {
		f, e := geom.NewFrame(frame.ToWorld(geom.Vec3{Y: dims.Y, Z: dims.Z}), frame.XAxis(), frame.ZAxis().Scale(-1))
		if e != nil {
			return fail(e)
		}
		return BoxPartialSkewBores(ctx, f, dims, geom.Vec3{X: crossing.X, Y: dims.Y - crossing.Y, Z: dims.Z - crossing.Z}, xRadius, zRadius, -offset, tolerance)
	}
	a, b, d := xRadius, zRadius, offset
	pad := max(8*tolerance, 1024*0x1p-52*max(a, b, d))
	if d-math.Abs(a-b) <= pad || a+b-d <= pad || !finite(a+b) || a*a == 0 || b*b == 0 || !finite(a*a) || !finite(b*b) {
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
	for _, gap := range []float64{crossing.X - b, dims.X - crossing.X - b, crossing.Z - a, dims.Z - crossing.Z - a, crossing.Y - a, dims.Y - crossing.Y - a, crossing.Y + d - b, dims.Y - crossing.Y - d - b} {
		if !finite(gap) || gap <= pad {
			return fail(geom.ErrDomain)
		}
	}
	builder := cornerCylinderBuilder{ctx: ctx, frame: frame, verticesByPoint: map[geom.Vec3]int{}, edgesByEnds: map[[2]int]int{}}
	wire := func(s geom.Surface, p []geom.Vec3, uv []geom.Vec2, curves map[int]PCurve) ([]Use, error) {
		points := make([]geom.Vec3, len(p))
		for i, q := range p {
			points[i] = frame.ToLocal(frame.ToWorld(q))
			if points[i].Sub(q).Length() > tolerance/8 {
				return nil, geom.ErrUnrepresentable
			}
		}
		return builder.wire(s, points, uv, curves)
	}
	addFace := func(s geom.Surface, p []geom.Vec3, uv []geom.Vec2, curves map[int]PCurve, rev bool) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		w, e := wire(s, p, uv, curves)
		if e == nil {
			builder.faces = append(builder.faces, Face{Surface: s, Outer: w, Reversed: rev})
		}
		return e
	}
	x, y, z := dims.X, dims.Y, dims.Z
	polygons := [][]geom.Vec3{{{}, {Y: y}, {X: x, Y: y}, {X: x}}, {{Z: z}, {X: x, Z: z}, {X: x, Y: y, Z: z}, {Y: y, Z: z}}, {{}, {X: x}, {X: x, Z: z}, {Z: z}}, {{X: x}, {X: x, Y: y}, {X: x, Y: y, Z: z}, {X: x, Z: z}}, {{Y: y}, {Y: y, Z: z}, {X: x, Y: y, Z: z}, {X: x, Y: y}}, {{}, {Z: z}, {Y: y, Z: z}, {Y: y}}}
	for _, p := range polygons {
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
		if e = addFace(s, p, uv, nil, false); e != nil {
			return fail(e)
		}
	}
	vec := func(v geom.Vec3) geom.Vec3 {
		return frame.XAxis().Scale(v.X).Add(frame.YAxis().Scale(v.Y)).Add(frame.ZAxis().Scale(v.Z))
	}
	// The common loop follows four regular arcs. Reverse this order for the X
	// chart; the listed order is clockwise in the Z chart.
	arcs := []partialSkewTrim{{a: a, b: b, d: d, sx: 1, sz: 1, from: 0, to: math.Pi / 2}, {a: a, b: b, d: d, sx: 1, sz: -1, from: math.Pi / 2, to: 0}, {a: a, b: b, d: d, sx: -1, sz: -1, from: 0, to: math.Pi / 2}, {a: a, b: b, d: d, sx: -1, sz: 1, from: math.Pi / 2, to: 0}}
	originalArcs := arcs
	arcs = nil
	for _, c := range originalArcs {
		for i := 0; i < 2; i++ {
			q := c
			q.from = c.from + (c.to-c.from)*float64(i)/2
			q.to = c.from + (c.to-c.from)*float64(i+1)/2
			arcs = append(arcs, q)
		}
	}
	for axis := 0; axis < 2; axis++ {
		radius, center, rx, ry, dir := a, crossing, geom.Vec3{Y: 1}, geom.Vec3{Z: 1}, geom.Vec3{X: 1}
		caps := [2]int{5, 3}
		low, high := -crossing.X, x-crossing.X
		alpha := math.Acos((d - b) / a)
		if axis == 1 {
			radius = b
			center.Y += d
			rx, ry, dir = geom.Vec3{Y: -1}, geom.Vec3{X: 1}, geom.Vec3{Z: 1}
			caps = [2]int{0, 1}
			low, high = -crossing.Z, z-crossing.Z
			alpha = math.Acos((d - a) / b)
		}
		cut := (alpha + math.Pi) / 2
		angles := []float64{-cut, -math.Pi / 2, 0, math.Pi / 2, cut, math.Pi, 2*math.Pi - cut}
		f, e := geom.NewFrame(frame.ToWorld(center), vec(rx), vec(dir))
		if e != nil {
			return fail(e)
		}
		surface, e := geom.NewCylinderSurface(f, radius)
		if e != nil {
			return fail(e)
		}
		point := func(i int, h float64) geom.Vec3 {
			if i == 6 {
				i = 0
			}
			sn, co := math.Sincos(angles[i])
			return center.Add(rx.Scale(radius * co)).Add(ry.Scale(radius * sn)).Add(dir.Scale(h))
		}
		for capIndex, h := range []float64{low, high} {
			cap := &builder.faces[caps[capIndex]]
			plane := cap.Surface.(geom.PlaneSurface).Frame()
			indices := []int{0, 1, 2, 3, 4, 5}
			if capIndex == 1 {
				indices = []int{0, 5, 4, 3, 2, 1}
			}
			p, uv := make([]geom.Vec3, 6), make([]geom.Vec2, 6)
			curves := map[int]PCurve{}
			origin := plane.ToLocal(frame.ToWorld(center.Add(dir.Scale(h))))
			vx, vy := vec(rx).Scale(radius), vec(ry).Scale(radius)
			for j, i := range indices {
				p[j] = point(i, h)
				q := plane.ToLocal(frame.ToWorld(p[j]))
				uv[j] = geom.Vec2{X: q.X, Y: q.Y}
			}
			for j, i := range indices {
				from, to := angles[i], angles[i+1]
				if capIndex == 1 {
					if j == 0 {
						from = angles[6]
						to = angles[5]
					} else {
						to = angles[i-1]
					}
				}
				c := EllipseUV{Center: geom.Vec2{X: origin.X, Y: origin.Y}, Cos: geom.Vec2{X: vx.Dot(plane.XAxis()), Y: vx.Dot(plane.YAxis())}, Sin: geom.Vec2{X: vy.Dot(plane.XAxis()), Y: vy.Dot(plane.YAxis())}, From: from, To: to}
				for k, want := range []geom.Vec2{uv[j], uv[(j+1)%6]} {
					got, e := c.EvalUV(float64(k))
					if e != nil || math.Hypot(got.X-want.X, got.Y-want.Y) > tolerance {
						return fail(geom.ErrUnrepresentable)
					}
				}
				curves[j] = unionArcUV{c, uv[j], uv[(j+1)%6]}
			}
			w, e := wire(cap.Surface, p, uv, curves)
			if e != nil {
				return fail(e)
			}
			cap.Holes = append(cap.Holes, w)
		}
		for patch := 0; patch < 2; patch++ {
			lo, hi := 0, 4
			if patch == 1 {
				lo, hi = 4, 6
			}
			var indices []int
			for i := lo; i <= hi; i++ {
				indices = append(indices, i)
			}
			for i := hi; i >= lo; i-- {
				indices = append(indices, i)
			}
			p, uv := make([]geom.Vec3, len(indices)), make([]geom.Vec2, len(indices))
			for j, i := range indices {
				h := low
				if j > hi-lo {
					h = high
				}
				p[j] = point(i, h)
				uv[j] = geom.Vec2{X: angles[i], Y: h}
			}
			if e = addFace(surface, p, uv, nil, true); e != nil {
				return fail(e)
			}
			if patch == 1 {
				continue
			}
			p, uv = make([]geom.Vec3, len(arcs)), make([]geom.Vec2, len(arcs))
			curves := map[int]PCurve{}
			for j := 0; j < len(arcs); j++ {
				c := arcs[j]
				if axis == 0 {
					c = arcs[len(arcs)-1-j]
					c.from, c.to = c.to, c.from
				}
				c.xChart = axis == 0
				start, _, e := c.point(0)
				if e != nil {
					return fail(e)
				}
				p[j] = crossing.Add(start)
				u, e := c.raw(0)
				if e != nil {
					return fail(e)
				}
				uv[j] = u
				c.start = u
				c.end, e = c.raw(1)
				if e != nil {
					return fail(e)
				}
				curves[j] = c
			}
			// Neighboring arcs share one independently computed endpoint UV; prove
			// the raw supports before anchoring them to that representative.
			for j := 0; j < len(arcs); j++ {
				c := curves[j].(partialSkewTrim)
				if math.Hypot(c.end.X-uv[(j+1)%len(arcs)].X, c.end.Y-uv[(j+1)%len(arcs)].Y) > tolerance {
					return fail(geom.ErrUnrepresentable)
				}
				c.end = uv[(j+1)%len(arcs)]
				curves[j] = c
			}
			hole, e := wire(surface, p, uv, curves)
			if e != nil {
				return fail(e)
			}
			builder.faces[len(builder.faces)-1].Holes = append(builder.faces[len(builder.faces)-1].Holes, hole)
		}
	}
	solid, e := NewSolid(builder.vertices, builder.edges, builder.faces, tolerance)
	if e != nil {
		return fail(e)
	}
	mass, e := solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-10, MaxSubdivisions: 128})
	if e != nil {
		return fail(e)
	}
	volume, area := x*y*z, 2*(x*y+x*z+y*z)
	if !finite(volume) || !finite(area) {
		return fail(geom.ErrInvalid)
	}
	return BoxCrossedBoresResult{Solid: solid, RemovedVolume: volume - mass.Volume, SurfaceAreaDelta: mass.Area - area}, nil
}

// TessellatePartialSkewBores retains shared analytic edge ownership while
// applying the existing conforming directional refinement and caller budgets.
func (s Solid) TessellatePartialSkewBores(ctx context.Context, o parametric.TessellationOptions) (mesh.Mesh, error) {
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
	if len(s.faces) != 10 {
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
				case partialSkewTrim:
					found = true
				case nil, EllipseUV, unionArcUV:
				default:
					return mesh.Mesh{}, geom.ErrDomain
				}
			}
		}
	}
	if !found {
		return mesh.Mesh{}, geom.ErrDomain
	}
	return s.tessellateAnalyticAdaptive(ctx, o)
}
