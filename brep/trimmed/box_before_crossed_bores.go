package trimmed

import (
	"context"
	"fmt"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// BeforeCrossedBoreFloorFaces identifies the two surviving planar floor segments.
const (
	BeforeCrossedBoreFloorFirst  = 24
	BeforeCrossedBoreFloorSecond = 25
)

// BoxBeforeCrossedBores removes an X through bore and a lower-entry Z blind
// bore from rectangular stock. Equal radii and signed cardinal frames are
// required. The blind floor lies strictly between the near end of the intersection and
// the crossing center; zero-offset and tangent limits reject.
func BoxBeforeCrossedBores(ctx context.Context, frame geom.Frame, dimensions, crossing geom.Vec3, radius, depth, tolerance float64) (BoxCrossedBoresResult, error) {
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
	if !finite(depth) || depth <= 0 || depth >= dimensions.Z-8*tolerance {
		return fail(geom.ErrDomain)
	}
	floorPoint := geom.Vec3{X: crossing.X, Y: crossing.Y, Z: depth}
	represented := frame.ToLocal(frame.ToWorld(floorPoint))
	if represented.Distance(floorPoint) > tolerance/8 || frame.ToLocal(frame.ToWorld(represented)) != represented {
		return fail(geom.ErrUnrepresentable)
	}
	depth = represented.Z
	h := depth - crossing.Z
	pad := max(8*tolerance, 1024*0x1p-52*max(radius, depth))
	if h >= -pad || h <= -radius+pad {
		return fail(geom.ErrDomain)
	}
	ah := -h
	q := math.Sqrt((radius - h) * (radius + h))
	theta := math.Acos(ah / radius)
	b, e := crossedStockBuilder(ctx, frame, dimensions)
	if e != nil {
		return fail(e)
	}
	x, y, z := dimensions.X, dimensions.Y, dimensions.Z
	angles := []float64{0, theta, math.Pi / 2, math.Pi - theta, math.Pi, math.Pi + theta, 3 * math.Pi / 2, 2*math.Pi - theta, 2 * math.Pi}
	rx := []float64{radius, ah, 0, -ah, -radius, -ah, 0, ah, radius}
	ry := []float64{0, q, radius, q, 0, -q, -radius, -q, 0}
	normalize := func(p geom.Vec3) (geom.Vec3, error) {
		v := frame.ToLocal(frame.ToWorld(p))
		if v.Distance(p) > tolerance/8 || frame.ToLocal(frame.ToWorld(v)) != v {
			return geom.Vec3{}, geom.ErrUnrepresentable
		}
		return v, nil
	}
	face := func(surface geom.Surface, points []geom.Vec3, uv []geom.Vec2, curves map[int]PCurve, reverse bool) error {
		for i, p := range points {
			v, e := normalize(p)
			if e != nil {
				return e
			}
			points[i] = v
		}
		if points[0] == points[len(points)-1] {
			points = points[:len(points)-1]
			uv = uv[:len(uv)-1]
		}
		for i, p := range curves {
			raw := p.(EllipseUV)
			end := (i + 1) % len(uv)
			for k, want := range []geom.Vec2{uv[i], uv[end]} {
				got, e := raw.EvalUV(float64(k))
				if e != nil || math.Hypot(got.X-want.X, got.Y-want.Y) > tolerance {
					return geom.ErrUnrepresentable
				}
			}
			curves[i] = unionArcUV{raw, uv[i], uv[end]}
		}
		return b.face(surface, points, uv, curves, reverse)
	}
	vector := func(v geom.Vec3) geom.Vec3 {
		return frame.XAxis().Scale(v.X).Add(frame.YAxis().Scale(v.Y)).Add(frame.ZAxis().Scale(v.Z))
	}
	for axis := 0; axis < 2; axis++ {
		radialX, radialY, direction := geom.Vec3{X: 1}, geom.Vec3{Y: 1}, geom.Vec3{Z: 1}
		low, high := -crossing.Z, h
		caps := []int{0}
		if axis == 1 {
			radialX, radialY, direction = geom.Vec3{Z: 1}, geom.Vec3{Y: -1}, geom.Vec3{X: 1}
			low, high = -crossing.X, x-crossing.X
			caps = []int{5, 3}
		}
		cf, e := geom.NewFrame(frame.ToWorld(crossing), vector(radialX), vector(direction))
		if e != nil {
			return fail(e)
		}
		surface, e := geom.NewCylinderSurface(cf, radius)
		if e != nil {
			return fail(e)
		}
		point := func(i int, height float64) geom.Vec3 {
			return crossing.Add(radialX.Scale(rx[i])).Add(radialY.Scale(ry[i])).Add(direction.Scale(height))
		}
		for k, capID := range caps {
			level := low
			if k == 1 {
				level = high
			}
			if e = b.crossedCapWire(frame, frame.ToWorld(crossing.Add(direction.Scale(level))), vector(radialX), vector(radialY), radius, capID, k == 1, angles, func(i int) geom.Vec3 { return point(i, level) }, normalize); e != nil {
				return fail(e)
			}
		}
		for i := 0; i < 8; i++ {
			a, c := angles[i], angles[i+1]
			sign := 1.
			if i >= 2 && i < 6 {
				sign = -1
			}
			if axis == 1 && i != 3 && i != 4 {
				points := []geom.Vec3{point(i, low), point(i+1, low), point(i+1, high), point(i, high)}
				uv := []geom.Vec2{{X: a, Y: low}, {X: c, Y: low}, {X: c, Y: high}, {X: a, Y: high}}
				// Split the floor-height seam into three axial segments so its central
				// segment belongs to a floor chord and its outer segments to neighboring walls.
				if i == 2 {
					points = []geom.Vec3{point(i, low), point(i+1, low), point(i+1, -ah), point(i+1, ah), point(i+1, high), point(i, high)}
					uv = []geom.Vec2{{X: a, Y: low}, {X: c, Y: low}, {X: c, Y: -ah}, {X: c, Y: ah}, {X: c, Y: high}, {X: a, Y: high}}
				} else if i == 5 {
					points = []geom.Vec3{point(i, low), point(i+1, low), point(i+1, high), point(i, high), point(i, ah), point(i, -ah)}
					uv = []geom.Vec2{{X: a, Y: low}, {X: c, Y: low}, {X: c, Y: high}, {X: a, Y: high}, {X: a, Y: ah}, {X: a, Y: -ah}}
				}
				if e = face(surface, points, uv, nil, true); e != nil {
					return fail(e)
				}
				continue
			}
			if axis == 0 && (i == 1 || i == 2 || i == 5 || i == 6) {
				points := []geom.Vec3{point(i, low), point(i+1, low), point(i+1, h), point(i, h)}
				uv := []geom.Vec2{{X: a, Y: low}, {X: c, Y: low}, {X: c, Y: h}, {X: a, Y: h}}
				if e = face(surface, points, uv, nil, true); e != nil {
					return fail(e)
				}
				continue
			}
			for _, upper := range []bool{false, true} {
				if axis == 0 && upper {
					continue
				}
				u0, u1 := math.Abs(rx[i]), math.Abs(rx[i+1])
				lo0, lo1, hi0, hi1 := low, low, -u0, -u1
				coeff := -radius * sign
				edge := 2
				from, to := c, a
				if upper {
					lo0, lo1, hi0, hi1 = u0, u1, high, high
					coeff = radius * sign
					edge = 0
					from, to = a, c
				}
				points := []geom.Vec3{point(i, lo0), point(i+1, lo1), point(i+1, hi1), point(i, hi0)}
				uv := []geom.Vec2{{X: a, Y: lo0}, {X: c, Y: lo1}, {X: c, Y: hi1}, {X: a, Y: hi0}}
				// If the second endpoint collapses, remove that zero side and move the
				// reversed analytic upper edge to the surviving index.
				if points[1] == points[2] {
					points = append(points[:2], points[3])
					uv = append(uv[:2], uv[3])
					if edge == 2 {
						edge = 1
					}
				}
				if e = face(surface, points, uv, map[int]PCurve{edge: EllipseUV{Linear: geom.Vec2{X: 1}, Cos: geom.Vec2{Y: coeff}, From: from, To: to}}, true); e != nil {
					return fail(e)
				}
			}
		}
	}
	// Two circular-segment floor patches, normal opposite the incoming Z tool.
	ff, e := geom.NewFrame(frame.ToWorld(crossing.Add(geom.Vec3{Z: h})), frame.XAxis(), frame.ZAxis().Scale(-1))
	if e != nil {
		return fail(e)
	}
	floorSurface, e := geom.NewPlaneSurface(ff)
	if e != nil {
		return fail(e)
	}
	for _, indices := range [][]int{{1, 3, 2}, {5, 7, 6}} {
		points := make([]geom.Vec3, 3)
		uv := make([]geom.Vec2, 3)
		for j, i := range indices {
			points[j] = crossing.Add(geom.Vec3{X: rx[i], Y: ry[i], Z: h})
			uv[j] = geom.Vec2{X: rx[i], Y: -ry[i]}
		}
		curves := map[int]PCurve{}
		for j := 1; j < 3; j++ {
			from, to := angles[indices[j]], angles[indices[(j+1)%3]]
			curves[j] = EllipseUV{Cos: geom.Vec2{X: radius}, Sin: geom.Vec2{Y: -radius}, From: from, To: to}
		}
		if e = face(floorSurface, points, uv, curves, false); e != nil {
			return fail(e)
		}
	}
	solid, e := NewSolid(b.vertices, b.edges, b.faces, tolerance)
	if e != nil {
		return fail(fmt.Errorf("before-axis crossed topology: %w", e))
	}
	mass, e := solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-10, MaxSubdivisions: 128})
	if e != nil {
		return fail(e)
	}
	return BoxCrossedBoresResult{Solid: solid, RemovedVolume: x*y*z - mass.Volume, SurfaceAreaDelta: mass.Area - 2*(x*y+x*z+y*z)}, nil
}

// TessellateBeforeCrossedBores uses bounded analytic adaptive refinement.
func (s Solid) TessellateBeforeCrossedBores(ctx context.Context, o parametric.TessellationOptions) (mesh.Mesh, error) {
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
	if len(s.faces) != 26 {
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
