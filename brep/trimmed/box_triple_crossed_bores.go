package trimmed

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// BoxTripleCrossedBores removes three concurrent equal-radius cardinal through
// bores from rectangular stock. Stock faces0..5 stay fixed; the next three
// groups of16cylinder patches follow local Z,X,Y order.
func BoxTripleCrossedBores(ctx context.Context, frame geom.Frame, dimensions, crossing geom.Vec3, radius, tolerance float64) (BoxCrossedBoresResult, error) {
	return BoxFiniteTripleCrossedBores(ctx, frame, dimensions, crossing, radius, [3]float64{}, [3]bool{}, tolerance)
}

// BoxFiniteTripleCrossedBores retains any subset of the three blind floors
// once their stops clear the complete intersection. Depths and opposite
// entries use local X/Y/Z order. A zero depth denotes a through bore.
func BoxFiniteTripleCrossedBores(ctx context.Context, frame geom.Frame, dimensions, crossing geom.Vec3, radius float64, depths [3]float64, opposite [3]bool, tolerance float64) (BoxCrossedBoresResult, error) {
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
	var floors [3]float64
	span := 0.
	floorCount := 0
	for axis, dimension := range [3]float64{dimensions.X, dimensions.Y, dimensions.Z} {
		unit := [3]geom.Vec3{{X: 1}, {Y: 1}, {Z: 1}}[axis]
		center := crossing.Dot(unit)
		if center-radius <= 8*tolerance || dimension-center-radius <= 8*tolerance {
			return fail(geom.ErrDomain)
		}
		depth := depths[axis]
		if depth == 0 && !opposite[axis] {
			span += dimension
			continue
		}
		if !finite(depth) || depth <= 0 || depth >= dimension-8*tolerance {
			return fail(geom.ErrDomain)
		}
		floor := depth
		if opposite[axis] {
			floor = dimension - depth
		}
		p := crossing
		// Set the coordinate directly so non-dyadic depths retain the same
		// rounding as the through/single-floor native representation.
		switch axis {
		case 0:
			p.X = floor
		case 1:
			p.Y = floor
		case 2:
			p.Z = floor
		}
		represented := frame.ToLocal(frame.ToWorld(p))
		if !represented.IsFinite() || represented.Distance(p) > tolerance/8 || frame.ToLocal(frame.ToWorld(represented)) != represented {
			return fail(geom.ErrUnrepresentable)
		}
		floor = represented.Dot(unit)
		if (!opposite[axis] && floor <= center+radius+8*tolerance) || (opposite[axis] && floor >= center-radius-8*tolerance) {
			return fail(geom.ErrDomain)
		}
		depths[axis] = floor
		if opposite[axis] {
			depths[axis] = dimension - floor
		}
		floors[axis] = floor
		span += depths[axis]
		floorCount++
	}
	b, e := crossedStockBuilder(ctx, frame, dimensions)
	if e != nil {
		return fail(e)
	}
	q := radius / math.Sqrt2
	angles := make([]float64, 9)
	for i := range angles {
		angles[i] = float64(i) * math.Pi / 4
	}
	rx := []float64{radius, q, 0, -q, -radius, -q, 0, q, radius}
	ry := []float64{0, q, radius, q, 0, -q, -radius, -q, 0}
	normalize := func(p geom.Vec3) (geom.Vec3, error) {
		v := frame.ToLocal(frame.ToWorld(p))
		if !v.IsFinite() || v.Distance(p) > tolerance/8 || frame.ToLocal(frame.ToWorld(v)) != v {
			return geom.Vec3{}, geom.ErrUnrepresentable
		}
		return v, nil
	}
	vector := func(v geom.Vec3) geom.Vec3 {
		return frame.XAxis().Scale(v.X).Add(frame.YAxis().Scale(v.Y)).Add(frame.ZAxis().Scale(v.Z))
	}
	// Reserve all floor IDs before creating any cylindrical wall group.
	var floorIDs [3]int
	for _, axis := range [3]int{2, 0, 1} {
		if depths[axis] != 0 {
			floorIDs[axis] = len(b.faces)
			b.faces = append(b.faces, Face{})
		}
	}
	for axis := 0; axis < 3; axis++ {
		radialX, radialY, direction := geom.Vec3{X: 1}, geom.Vec3{Y: 1}, geom.Vec3{Z: 1}
		low, high := -crossing.Z, dimensions.Z-crossing.Z
		caps := [2]int{0, 1}
		if axis == 1 {
			radialX, radialY, direction = geom.Vec3{Z: 1}, geom.Vec3{Y: -1}, geom.Vec3{X: 1}
			low, high = -crossing.X, dimensions.X-crossing.X
			caps = [2]int{5, 3}
		}
		if axis == 2 {
			radialX, radialY, direction = geom.Vec3{X: 1}, geom.Vec3{Z: -1}, geom.Vec3{Y: 1}
			low, high = -crossing.Y, dimensions.Y-crossing.Y
			caps = [2]int{2, 4}
		}
		localAxis := [3]int{2, 0, 1}[axis]
		if floorIDs[localAxis] != 0 {
			floor := floors[localAxis] - crossing.Dot(direction)
			if opposite[localAxis] {
				low = floor
			} else {
				high = floor
			}
		}
		cf, e := geom.NewFrame(frame.ToWorld(crossing), vector(radialX), vector(direction))
		if e != nil {
			return fail(e)
		}
		surface, e := geom.NewCylinderSurface(cf, radius)
		if e != nil {
			return fail(e)
		}
		point := func(i int, h float64) geom.Vec3 {
			return crossing.Add(radialX.Scale(rx[i])).Add(radialY.Scale(ry[i])).Add(direction.Scale(h))
		}
		for k, id := range caps {
			level := low
			if k == 1 {
				level = high
			}
			isFloor := floorIDs[localAxis] != 0 && opposite[localAxis] == (k == 0)
			if isFloor {
				normal := vector(direction).Scale(-1)
				if opposite[localAxis] {
					normal = vector(direction)
				}
				ff, err := geom.NewFrame(frame.ToWorld(crossing.Add(direction.Scale(level))), vector(radialX), normal)
				if err != nil {
					return fail(err)
				}
				plane, err := geom.NewPlaneSurface(ff)
				if err != nil {
					return fail(err)
				}
				id = floorIDs[localAxis]
				b.faces[id] = Face{Surface: plane}
			}
			if e = b.crossedCapWire(frame, frame.ToWorld(crossing.Add(direction.Scale(level))), vector(radialX), vector(radialY), radius, id, k == 1, angles, func(i int) geom.Vec3 { return point(i, level) }, normalize); e != nil {
				return fail(e)
			}
			if isFloor {
				b.faces[id].Outer = b.faces[id].Holes[0]
				b.faces[id].Holes = nil
			}

		}
		for i := 0; i < 8; i++ {
			a, c := angles[i], angles[i+1]
			cosine := i == 0 || i == 3 || i == 4 || i == 7
			sign := 1.
			if cosine && (i == 3 || i == 4) || !cosine && (i == 5 || i == 6) {
				sign = -1
			}
			chartSurface, shift := surface, 0.
			if !cosine {
				rotated, err := geom.NewFrame(cf.Origin(), cf.YAxis(), cf.ZAxis())
				if err != nil {
					return fail(err)
				}
				chartSurface, err = geom.NewCylinderSurface(rotated, radius)
				if err != nil {
					return fail(err)
				}
				shift = math.Pi / 2
			}
			for _, upper := range []bool{false, true} {
				u0, u1 := max(math.Abs(rx[i]), math.Abs(ry[i])), max(math.Abs(rx[i+1]), math.Abs(ry[i+1]))
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
				for j, p := range points {
					points[j], e = normalize(p)
					if e != nil {
						return fail(e)
					}
				}
				uv := []geom.Vec2{{X: a - shift, Y: lo0}, {X: c - shift, Y: lo1}, {X: c - shift, Y: hi1}, {X: a - shift, Y: hi0}}
				raw := EllipseUV{Linear: geom.Vec2{X: 1}, Cos: geom.Vec2{Y: coeff}, From: from - shift, To: to - shift}
				for k, want := range []geom.Vec2{uv[edge], uv[(edge+1)%4]} {
					got, e := raw.EvalUV(float64(k))
					if e != nil || math.Hypot(got.X-want.X, got.Y-want.Y) > tolerance {
						return fail(geom.ErrUnrepresentable)
					}
				}
				if e = b.face(chartSurface, points, uv, map[int]PCurve{edge: unionArcUV{raw, uv[edge], uv[(edge+1)%4]}}, true); e != nil {
					return fail(e)
				}
			}
		}
	}
	solid, e := NewSolid(b.vertices, b.edges, b.faces, tolerance)
	if e != nil {
		return fail(e)
	}
	removed := math.Pi*radius*radius*span - 8*math.Sqrt2*radius*radius*radius
	delta := 2*math.Pi*radius*span - 24*math.Sqrt2*radius*radius - 6*math.Pi*radius*radius
	delta += float64(floorCount) * 2 * math.Pi * radius * radius
	if !finite(removed) || !finite(delta) {
		return fail(geom.ErrInvalid)
	}
	return BoxCrossedBoresResult{Solid: solid, RemovedVolume: removed, SurfaceAreaDelta: delta}, nil
}

func (s Solid) TessellateTripleCrossedBores(ctx context.Context, o parametric.TessellationOptions) (mesh.Mesh, error) {
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
	if len(s.faces) < 54 || len(s.faces) > 57 {
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
