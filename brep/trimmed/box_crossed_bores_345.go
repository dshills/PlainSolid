package trimmed

import (
	"context"
	"math"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// BoxCrossedBores345 removes equal-radius concurrent Z and(4/5,0,3/5)
// through bores in local stock coordinates. The oblique tool opens on the
// two X faces, with strictly interior ellipses and a fully retained crossing.
// Non-cardinal frame coefficients are qualified against the common analytic
// conic coefficients within the existing sewing tolerance.
func BoxCrossedBores345(ctx context.Context, frame geom.Frame, dimensions, crossing geom.Vec3, radius, tolerance float64) (BoxCrossedBoresResult, error) {
	return BoxCrossedObliqueBores(ctx, frame, dimensions, crossing, geom.Vec2{X: 4, Y: 3}, radius, tolerance)
}

// BoxCrossedObliqueBores removes equal-radius concurrent Z and oblique through
// cylinders in signed stock coordinates. AxisRatio contains the positive X and
// Z ratios. The accepted ratios are 4:3, 3:4, 12:5 and 5:12. Both oblique X-cap ellipses
// must be fully inside the stock faces.
func BoxCrossedObliqueBores(ctx context.Context, frame geom.Frame, dimensions, crossing geom.Vec3, axisRatio geom.Vec2, radius, tolerance float64) (BoxCrossedBoresResult, error) {
	out, _, err := boxCrossedObliqueBores(ctx, frame, dimensions, crossing, axisRatio, radius, 0, false, tolerance)
	return out, err
}

// BoxObliqueFloorCrossedBores retains a planar stop at station measured
// along the positive oblique axis from the intersection. The entry is the
// lower X cap unless opposite is set. The cardinal bore remains through.
func BoxObliqueFloorCrossedBores(ctx context.Context, frame geom.Frame, dimensions, crossing geom.Vec3, ratio geom.Vec2, radius, station float64, opposite bool, tolerance float64) (BoxCrossedBoresResult, int, error) {
	if !finite(station) || (!opposite && station <= 0) || (opposite && station >= 0) {
		return BoxCrossedBoresResult{}, -1, geom.ErrDomain
	}
	return boxCrossedObliqueBores(ctx, frame, dimensions, crossing, ratio, radius, station, opposite, tolerance)
}

func boxCrossedObliqueBores(ctx context.Context, frame geom.Frame, dimensions, crossing geom.Vec3, axisRatio geom.Vec2, radius, station float64, opposite bool, tolerance float64) (BoxCrossedBoresResult, int, error) {
	fail := func(e error) (BoxCrossedBoresResult, int, error) { return BoxCrossedBoresResult{}, -1, e }
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
	coefficients, ok := CrossedObliqueCoefficients(axisRatio)
	if !ok {
		return fail(geom.ErrDomain)
	}
	sin, cos, slope, stretch := coefficients.Sin, coefficients.Cos, coefficients.Slope, coefficients.Stretch
	positive, negative := coefficients.Positive, coefficients.Negative

	// Reserve arithmetic error when comparing represented conic coefficients.
	// Very large translated coordinates cannot consume the sewing allowance.
	origin := frame.Origin()
	arithmeticReserve := 128 * 0x1p-52 * max(math.Abs(origin.X), math.Abs(origin.Y), math.Abs(origin.Z), dimensions.X, dimensions.Y, dimensions.Z, radius, 1)
	if arithmeticReserve > tolerance/8 {
		return fail(geom.ErrUnrepresentable)
	}
	pad := max(8*tolerance, 1024*0x1p-52*max(dimensions.X, dimensions.Y, dimensions.Z, radius))
	if crossing.X-radius <= pad || dimensions.X-crossing.X-radius <= pad || crossing.Y-radius <= pad || dimensions.Y-crossing.Y-radius <= pad || crossing.Z-positive*radius <= pad || dimensions.Z-crossing.Z-positive*radius <= pad {
		return fail(geom.ErrDomain)
	}
	for _, x := range []float64{0, dimensions.X} {
		z := crossing.Z + slope*(x-crossing.X)
		if z-stretch*radius <= pad || dimensions.Z-z-stretch*radius <= pad {
			return fail(geom.ErrDomain)
		}
	}
	remaining := 0.
	if station != 0 {
		advance, exit := station, stretch*(dimensions.X-crossing.X)
		if opposite {
			advance, exit = -station, stretch*crossing.X
		}
		remaining = exit - advance
		if advance <= positive*radius+pad || remaining <= slope*radius+pad {
			return fail(geom.ErrDomain)
		}
	}
	b, e := crossedStockBuilder(ctx, frame, dimensions)
	if e != nil {
		return fail(e)
	}
	angles := []float64{0, math.Pi / 2, math.Pi, 3 * math.Pi / 2, 2 * math.Pi}
	rx := []float64{radius, 0, -radius, 0, radius}
	ry := []float64{0, radius, 0, -radius, 0}
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
	var floor *Face
	for axis := 0; axis < 2; axis++ {
		radialX, radialY, direction := geom.Vec3{X: 1}, geom.Vec3{Y: 1}, geom.Vec3{Z: 1}
		caps := [2]int{0, 1}
		low, high := -crossing.Z, dimensions.Z-crossing.Z
		capCos := 0.
		if axis == 1 {
			radialX, radialY, direction = geom.Vec3{X: -cos, Z: sin}, geom.Vec3{Y: -1}, geom.Vec3{X: sin, Z: cos}
			caps = [2]int{5, 3}
			low, high = -stretch*crossing.X, stretch*(dimensions.X-crossing.X)
			capCos = slope * radius
		}
		cf, e := geom.NewFrame(frame.ToWorld(crossing), vector(radialX), vector(direction))
		if e != nil {
			return fail(e)
		}
		if cf.XAxis() != vector(radialX) || cf.YAxis() != vector(radialY) || cf.ZAxis() != vector(direction) {
			return fail(geom.ErrUnrepresentable)
		}
		surface, e := geom.NewCylinderSurface(cf, radius)
		if e != nil {
			return fail(e)
		}
		point := func(i int, h float64) geom.Vec3 {
			return crossing.Add(radialX.Scale(rx[i])).Add(radialY.Scale(ry[i])).Add(direction.Scale(h))
		}
		rimPoint := func(i int, upper bool) geom.Vec3 {
			coeff := negative
			if upper {
				coeff = positive
			}
			if rx[i] < 0 {
				coeff = positive
				if upper {
					coeff = negative
				}
			}
			if axis == 0 {
				return crossing.Add(geom.Vec3{X: rx[i], Y: ry[i], Z: coeff * rx[i]})
			}
			if coeff == positive {
				return crossing.Add(geom.Vec3{X: rx[i], Y: -ry[i], Z: positive * rx[i]})
			}
			return crossing.Add(geom.Vec3{X: -rx[i], Y: -ry[i], Z: -negative * rx[i]})
		}
		for k, id := range caps {
			atFloor := axis == 1 && station != 0 && (k == 0) == opposite
			if atFloor {
				// Author the common circular rim in a temporary planar face.
				// Append its reversed loop after all walls so through and
				// finite models preserve the same wall identifiers.
				f, e := geom.NewFrame(cf.ToWorld(geom.Vec3{Z: station}), cf.XAxis(), cf.ZAxis())
				if e != nil {
					return fail(e)
				}
				plane, e := geom.NewPlaneSurface(f)
				if e != nil {
					return fail(e)
				}
				id = len(b.faces)
				b.faces = append(b.faces, Face{Surface: plane})
			}
			level := low
			if k == 1 {
				level = high
			}
			center := crossing.Add(direction.Scale(level))
			capRadialX, capRadialY := radialX, radialY
			capPoint := func(i int) geom.Vec3 { return point(i, level) }
			rimCos := capCos
			if atFloor {
				level, center, rimCos = station, crossing.Add(direction.Scale(station)), 0
				capPoint = func(i int) geom.Vec3 { return point(i, station) }
			} else if axis == 1 {
				x := 0.
				if k == 1 {
					x = dimensions.X
				}
				center = geom.Vec3{X: x, Y: crossing.Y, Z: crossing.Z + slope*(x-crossing.X)}
				capRadialX, capRadialY = geom.Vec3{Z: stretch}, geom.Vec3{Y: -1}
				capPoint = func(i int) geom.Vec3 { return center.Add(geom.Vec3{Y: -ry[i], Z: stretch * rx[i]}) }
			}
			// The constant/cosine/sine coefficient residual bounds every point on the
			// common cap conic, not just its sampled endpoints.
			actualCenter := cf.Origin().Add(cf.ZAxis().Scale(level))
			actualCos := cf.XAxis().Scale(radius).Add(cf.ZAxis().Scale(rimCos))
			actualSin := cf.YAxis().Scale(radius)
			residual := actualCenter.Distance(frame.ToWorld(center)) + actualCos.Distance(vector(capRadialX).Scale(radius)) + actualSin.Distance(vector(capRadialY).Scale(radius))
			if residual+arithmeticReserve > tolerance/8 {
				return fail(geom.ErrUnrepresentable)
			}
			if e = b.crossedCapWire(frame, frame.ToWorld(center), vector(capRadialX), vector(capRadialY), radius, id, k == 1, angles, capPoint, normalize); e != nil {
				return fail(e)
			}
			if atFloor {
				f := b.faces[id]
				f.Outer, f.Holes, f.Reversed = f.Holes[0], nil, !opposite
				if !opposite {
					f.Outer = reverseUnionLoop(f.Outer)
				}
				floor = &f
				b.faces = b.faces[:id]
			}
		}
		if axis == 1 {
			for _, coeff := range []float64{positive, negative} {
				actualCos := cf.XAxis().Scale(radius).Add(cf.ZAxis().Scale(coeff * radius))
				want := vector(geom.Vec3{X: radius, Z: positive * radius})
				if coeff == negative {
					want = vector(geom.Vec3{X: -radius, Z: -negative * radius})
				}
				if actualCos.Distance(want)+cf.YAxis().Scale(radius).Distance(vector(geom.Vec3{Y: -radius}))+arithmeticReserve > tolerance/8 {
					return fail(geom.ErrUnrepresentable)
				}
			}
		}
		for i := 0; i < 4; i++ {
			a, c := angles[i], angles[i+1]
			sign := 1.
			if i == 1 || i == 2 {
				sign = -1
			}
			for _, upper := range []bool{false, true} {
				coeff := negative * radius
				if upper == (sign > 0) {
					coeff = positive * radius
				}
				capHeight := low
				if upper {
					capHeight = high
				}
				atFloor := axis == 1 && station != 0 && upper != opposite
				rimCos := capCos
				if atFloor {
					capHeight, rimCos = station, 0
				}
				ca, cc := capHeight+rimCos*[]float64{1, 0, -1, 0, 1}[i], capHeight+rimCos*[]float64{1, 0, -1, 0, 1}[i+1]
				ra, rc := coeff*[]float64{1, 0, -1, 0, 1}[i], coeff*[]float64{1, 0, -1, 0, 1}[i+1]
				lo0, lo1, hi0, hi1 := ca, cc, ra, rc
				if upper {
					lo0, lo1, hi0, hi1 = ra, rc, ca, cc
				}
				points := []geom.Vec3{point(i, lo0), point(i+1, lo1), point(i+1, hi1), point(i, hi0)}
				uv := []geom.Vec2{{X: a, Y: lo0}, {X: c, Y: lo1}, {X: c, Y: hi1}, {X: a, Y: hi0}}
				edge, capEdge := 2, 0
				if upper {
					edge, capEdge = 0, 2
				}
				points[edge], points[(edge+1)%4] = rimPoint(i+1, upper), rimPoint(i, upper)
				if upper {
					points[0], points[1] = rimPoint(i, upper), rimPoint(i+1, upper)
				}
				// Anchor cap vertices to their exact supporting stock plane.
				if axis == 1 && !atFloor {
					x := 0.
					if upper {
						x = dimensions.X
					}
					cp := func(j int) geom.Vec3 {
						return geom.Vec3{X: x, Y: crossing.Y - ry[j], Z: crossing.Z + slope*(x-crossing.X) + stretch*rx[j]}
					}
					if upper {
						points[2], points[3] = cp(i+1), cp(i)
					} else {
						points[0], points[1] = cp(i), cp(i+1)
					}
				}
				curves := map[int]PCurve{}
				rim := EllipseUV{Linear: geom.Vec2{X: 1}, Cos: geom.Vec2{Y: coeff}, From: c, To: a}
				if upper {
					rim.From, rim.To = a, c
				}
				curves[edge] = rim
				if axis == 1 && !atFloor {
					cap := EllipseUV{Center: geom.Vec2{Y: capHeight}, Linear: geom.Vec2{X: 1}, Cos: geom.Vec2{Y: rimCos}, From: a, To: c}
					if upper {
						cap.From, cap.To = c, a
					}
					curves[capEdge] = cap
				}
				for j, p := range points {
					evaluated, err := surface.Eval(uv[j].X, uv[j].Y)
					if err != nil || evaluated.Distance(frame.ToWorld(p)) > tolerance/8 {
						return fail(geom.ErrUnrepresentable)
					}
					points[j], e = normalize(p)
					if e != nil {
						return fail(e)
					}
				}
				for j, p := range curves {
					raw := p.(EllipseUV)
					for k, want := range []geom.Vec2{uv[j], uv[(j+1)%4]} {
						got, e := raw.EvalUV(float64(k))
						if e != nil || math.Hypot(got.X-want.X, got.Y-want.Y) > tolerance {
							return fail(geom.ErrUnrepresentable)
						}
					}
					curves[j] = unionArcUV{raw, uv[j], uv[(j+1)%4]}
				}
				if e = b.face(surface, points, uv, curves, true); e != nil {
					return fail(e)
				}
			}
		}
	}
	floorID := -1
	if floor != nil {
		floorID = len(b.faces)
		b.faces = append(b.faces, *floor)
	}
	solid, e := NewSolid(b.vertices, b.edges, b.faces, tolerance)
	if e != nil {
		return fail(e)
	}
	span := dimensions.Z + stretch*dimensions.X
	intersection := 16 * stretch
	removed := math.Pi*radius*radius*span - intersection*radius*radius*radius/3
	delta := 2*math.Pi*radius*span - intersection*radius*radius - 2*(1+stretch)*math.Pi*radius*radius
	if floor != nil {
		removed -= math.Pi * radius * radius * remaining
		delta += math.Pi*radius*radius*(1+stretch) - 2*math.Pi*radius*remaining
	}
	if !finite(removed) || !finite(delta) {
		return fail(geom.ErrInvalid)
	}
	return BoxCrossedBoresResult{Solid: solid, RemovedVolume: removed, SurfaceAreaDelta: delta}, floorID, nil
}

// BoxBlindCrossedObliqueBores retains a cardinal floor beyond the complete
// crossing. Depth is measured inward from the lower or upper cardinal cap.
// The oblique cylinder still opens through both X faces.
func BoxBlindCrossedObliqueBores(ctx context.Context, frame geom.Frame, dimensions, crossing geom.Vec3, ratio geom.Vec2, radius, depth float64, opposite bool, tolerance float64) (BoxCrossedBoresResult, int, error) {
	out, floors, err := boxCrossedObliqueWithCardinalFloor(ctx, frame, dimensions, crossing, ratio, radius, depth, opposite, 0, tolerance)
	return out, floors[0], err
}

// BoxDoubleBlindCrossedObliqueBores retains both circular floors beyond the
// complete intersection. Floor identifiers are returned in cardinal, oblique
// order. Depth runs inward from its cardinal entry and station is signed
// along the oblique axis measured from the crossing point.
func BoxDoubleBlindCrossedObliqueBores(ctx context.Context, frame geom.Frame, dimensions, crossing geom.Vec3, ratio geom.Vec2, radius, depth float64, opposite bool, station, tolerance float64) (BoxCrossedBoresResult, [2]int, error) {
	if !finite(station) || station == 0 {
		return BoxCrossedBoresResult{}, [2]int{-1, -1}, geom.ErrDomain
	}
	return boxCrossedObliqueWithCardinalFloor(ctx, frame, dimensions, crossing, ratio, radius, depth, opposite, station, tolerance)
}

func boxCrossedObliqueWithCardinalFloor(ctx context.Context, frame geom.Frame, dimensions, crossing geom.Vec3, ratio geom.Vec2, radius, depth float64, opposite bool, station, tolerance float64) (BoxCrossedBoresResult, [2]int, error) {
	fail := func(e error) (BoxCrossedBoresResult, [2]int, error) {
		return BoxCrossedBoresResult{}, [2]int{-1, -1}, e
	}
	k, ok := CrossedObliqueCoefficients(ratio)
	if !ok || ctx == nil || !finite(depth) || depth <= 0 || !finite(tolerance) || tolerance <= 0 {
		return fail(geom.ErrInvalid)
	}
	cap, direction, advance := 1, frame.ZAxis(), depth-crossing.Z
	if opposite {
		cap, direction, advance = 0, frame.ZAxis().Scale(-1), depth-(dimensions.Z-crossing.Z)
	}
	pad := max(8*tolerance, 1024*0x1p-52*max(dimensions.Z, radius))
	if depth >= dimensions.Z-pad || advance <= k.Positive*radius+pad {
		return fail(geom.ErrDomain)
	}
	out, obliqueFloor, err := boxCrossedObliqueBores(ctx, frame, dimensions, crossing, ratio, radius, station, station < 0, tolerance)
	if err != nil {
		return fail(err)
	}
	remaining := dimensions.Z - depth
	out.Solid, cap, err = stopCrossedBoreCap(out.Solid, cap, direction.Scale(-remaining))
	if err != nil {
		return fail(err)
	}
	out.RemovedVolume -= math.Pi * radius * radius * remaining
	out.SurfaceAreaDelta += 2 * math.Pi * radius * (radius - remaining)
	return out, [2]int{cap, obliqueFloor}, nil
}

func (s Solid) TessellateCrossedBores345(ctx context.Context, o parametric.TessellationOptions) (mesh.Mesh, error) {
	return s.TessellateCrossedObliqueBores(ctx, o)
}

// TessellateCrossedObliqueBores tessellates a supported exact crossing.
func (s Solid) TessellateCrossedObliqueBores(ctx context.Context, o parametric.TessellationOptions) (mesh.Mesh, error) {
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
	if len(s.faces) < 22 || len(s.faces) > 24 {
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
