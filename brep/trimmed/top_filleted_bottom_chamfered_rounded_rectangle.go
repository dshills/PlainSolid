package trimmed

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// TopFilletedBottomChamferedRoundedRectangle retains a top perimeter fillet
// and an independent equal-distance lower perimeter chamfer. Caps remain
// faces 0 (lower) and 1 (upper), with exact planar circular trim loops.
func TopFilletedBottomChamferedRoundedRectangle(ctx context.Context, frame geom.Frame, profile RoundedRectangle, height, radius, chamfer, tolerance float64) (Solid, error) {
	return buildBottomChamferedRoundedRectangle(ctx, frame, profile, height, radius, chamfer, tolerance, true)
}

// BottomChamferedRoundedRectangle bevels the lower cap perimeter of a
// uniformly rounded rectangular extrusion. It retains the original upper
// cap, the cylindrical corner walls, and the conical corner transitions.
func BottomChamferedRoundedRectangle(ctx context.Context, frame geom.Frame, profile RoundedRectangle, height, chamfer, tolerance float64) (Solid, error) {
	return buildBottomChamferedRoundedRectangle(ctx, frame, profile, height, 0, chamfer, tolerance, false)
}

func buildBottomChamferedRoundedRectangle(ctx context.Context, frame geom.Frame, profile RoundedRectangle, height, radius, chamfer, tolerance float64, withFillet bool) (Solid, error) {
	if ctx == nil || !finite(height) || !finite(radius) || !finite(chamfer) || !finite(tolerance) || tolerance <= 0 {
		return Solid{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, e
	}
	pad := 8 * tolerance
	corner := profile.Radii[0]
	if !finite(pad) || chamfer <= pad || corner-chamfer <= pad || height-radius-chamfer <= pad || !finite(corner) {
		return Solid{}, geom.ErrDomain
	}
	if withFillet && (radius <= pad || corner-2*radius <= pad) {
		return Solid{}, geom.ErrDomain
	}
	for _, r := range profile.Radii {
		if r != corner {
			return Solid{}, geom.ErrDomain
		}
	}
	lower, arcs, e := roundedExtrusionProfile(profile)
	if e != nil {
		return Solid{}, e
	}
	if len(lower) != 8 {
		return Solid{}, geom.ErrDomain
	}
	// The torus and inset quarter arcs have exactly the original corner
	// centers. Derive their tangent vertices from those same supports.
	// Decimal fillets may round a coordinate addition, which final
	// shared-edge validation checks against the specified tolerance.
	upper, upperArcs := lower, arcs
	if withFillet {
		upper = make([]geom.Vec2, 8)
		upperArcs = append([]extrusionQuarterArc(nil), arcs...)
		for i, arc := range upperArcs {
			if arc.circle.Radius > 0 {
				upperArcs[i].circle.Radius = corner - radius
				points := [4]geom.Vec2{{X: arc.circle.Center.X + (corner - radius), Y: arc.circle.Center.Y},
					{X: arc.circle.Center.X, Y: arc.circle.Center.Y + (corner - radius)},
					{X: arc.circle.Center.X - (corner - radius), Y: arc.circle.Center.Y},
					{X: arc.circle.Center.X, Y: arc.circle.Center.Y - (corner - radius)}}
				upper[i], upper[(i+1)%8] = points[arc.quadrant], points[(arc.quadrant+1)%4]
			}
		}
	}
	lowerCap := make([]geom.Vec2, 8)
	lowerArcs := append([]extrusionQuarterArc(nil), arcs...)
	for i, arc := range lowerArcs {
		if arc.circle.Radius > 0 {
			lowerArcs[i].circle.Radius = corner - chamfer
			r := corner - chamfer
			points := [4]geom.Vec2{{X: arc.circle.Center.X + r, Y: arc.circle.Center.Y}, {X: arc.circle.Center.X, Y: arc.circle.Center.Y + r}, {X: arc.circle.Center.X - r, Y: arc.circle.Center.Y}, {X: arc.circle.Center.X, Y: arc.circle.Center.Y - r}}
			lowerCap[i], lowerCap[(i+1)%8] = points[arc.quadrant], points[(arc.quadrant+1)%4]
		}
	}
	b := cornerCylinderBuilder{ctx: ctx, frame: frame, edgesByEnds: map[[2]int]int{}, verticesByPoint: map[geom.Vec3]int{}}
	for _, cap := range []struct {
		p       []geom.Vec2
		arcs    []extrusionQuarterArc
		z       float64
		reverse bool
	}{{lowerCap, lowerArcs, 0, true}, {upper, upperArcs, height, false}} {
		f, e := geom.NewFrame(frame.ToWorld(geom.Vec3{Z: cap.z}), frame.XAxis(), frame.ZAxis())
		if e != nil {
			return Solid{}, e
		}
		surface, e := geom.NewPlaneSurface(f)
		if e != nil {
			return Solid{}, e
		}
		points := make([]geom.Vec3, 8)
		curves := map[int]PCurve{}
		for i, p := range cap.p {
			points[i] = geom.Vec3{X: p.X, Y: p.Y, Z: cap.z}
			a := cap.arcs[i]
			if a.circle.Radius > 0 {
				c := EllipseUV{Center: a.circle.Center, Cos: geom.Vec2{X: a.circle.Radius}, Sin: geom.Vec2{Y: a.circle.Radius}, From: float64(a.quadrant) * math.Pi / 2, To: float64(a.quadrant+1) * math.Pi / 2}
				curve, e := newAnchoredCircularUV(c, p, cap.p[(i+1)%8])
				if e != nil {
					return Solid{}, e
				}
				curves[i] = curve
			}
		}
		if e = b.face(surface, points, cap.p, curves, cap.reverse); e != nil {
			return Solid{}, e
		}
	}
	for i, p := range lower {
		j := (i + 1) % 8
		q := lower[j]
		a := arcs[i]
		points := []geom.Vec3{{X: p.X, Y: p.Y, Z: chamfer}, {X: q.X, Y: q.Y, Z: chamfer}, {X: q.X, Y: q.Y, Z: height - radius}, {X: p.X, Y: p.Y, Z: height - radius}}
		if a.circle.Radius > 0 {
			surface, uv, e := a.wall(frame, chamfer, height-radius)
			if e != nil {
				return Solid{}, e
			}
			if e = b.face(surface, points, uv[:], nil, false); e != nil {
				return Solid{}, e
			}
			if !withFillet {
				continue
			}
			f, e := geom.NewFrame(frame.ToWorld(geom.Vec3{X: a.circle.Center.X, Y: a.circle.Center.Y, Z: height - radius}), frame.XAxis(), frame.ZAxis())
			if e != nil {
				return Solid{}, e
			}
			torus, e := geom.NewTorusSurface(f, corner-radius, radius)
			if e != nil {
				return Solid{}, e
			}
			lo, hi := float64(a.quadrant)*math.Pi/2, float64(a.quadrant+1)*math.Pi/2
			points = []geom.Vec3{{X: p.X, Y: p.Y, Z: height - radius}, {X: q.X, Y: q.Y, Z: height - radius}, {X: upper[j].X, Y: upper[j].Y, Z: height}, {X: upper[i].X, Y: upper[i].Y, Z: height}}
			torusUV := []geom.Vec2{{X: lo}, {X: hi}, {X: hi, Y: math.Pi / 2}, {X: lo, Y: math.Pi / 2}}
			if e = b.face(torus, points, torusUV, nil, false); e != nil {
				return Solid{}, e
			}
		} else {
			if e = b.wall(p, q, chamfer, height-radius); e != nil {
				return Solid{}, e
			}
			if !withFillet {
				continue
			}
			edge := geom.Vec3{X: q.X - p.X, Y: q.Y - p.Y}
			axis, e := edge.Unit()
			if e != nil {
				return Solid{}, e
			}
			length := edge.Length()
			inward := geom.Vec3{X: -axis.Y, Y: axis.X}
			origin := geom.Vec3{X: p.X + radius*inward.X, Y: p.Y + radius*inward.Y, Z: height - radius}
			f, e := geom.NewFrame(frame.ToWorld(origin), frame.ZAxis(), frame.XAxis().Scale(axis.X).Add(frame.YAxis().Scale(axis.Y)))
			if e != nil {
				return Solid{}, e
			}
			surface, e := geom.NewCylinderSurface(f, radius)
			if e != nil {
				return Solid{}, e
			}
			points = []geom.Vec3{{X: upper[i].X, Y: upper[i].Y, Z: height}, {X: p.X, Y: p.Y, Z: height - radius}, {X: q.X, Y: q.Y, Z: height - radius}, {X: upper[j].X, Y: upper[j].Y, Z: height}}
			uv := []geom.Vec2{{}, {X: math.Pi / 2}, {X: math.Pi / 2, Y: length}, {Y: length}}
			if e = b.face(surface, points, uv, nil, false); e != nil {
				return Solid{}, e
			}
		}
	}
	for i, p := range lowerCap {
		j := (i + 1) % 8
		q := lowerCap[j]
		points := []geom.Vec3{{X: p.X, Y: p.Y}, {X: q.X, Y: q.Y}, {X: lower[j].X, Y: lower[j].Y, Z: chamfer}, {X: lower[i].X, Y: lower[i].Y, Z: chamfer}}
		a := arcs[i]
		if a.circle.Radius > 0 {
			f, e := geom.NewFrame(frame.ToWorld(geom.Vec3{X: a.circle.Center.X, Y: a.circle.Center.Y}), frame.XAxis(), frame.ZAxis())
			if e != nil {
				return Solid{}, e
			}
			surface, e := parametric.RevolutionSide(f, geom.Vec2{X: corner - chamfer}, geom.Vec2{X: corner, Y: chamfer})
			if e != nil {
				return Solid{}, e
			}
			lo, hi := float64(a.quadrant)*math.Pi/2, float64(a.quadrant+1)*math.Pi/2
			if e = b.face(surface, points, []geom.Vec2{{X: lo}, {X: hi}, {X: hi, Y: 1}, {X: lo, Y: 1}}, nil, false); e != nil {
				return Solid{}, e
			}
		} else {
			along := points[1].Sub(points[0])
			up := points[3].Sub(points[0])
			f, e := geom.NewFrame(frame.ToWorld(points[0]), frame.XAxis().Scale(along.X).Add(frame.YAxis().Scale(along.Y)), frame.XAxis().Scale(along.Cross(up).X).Add(frame.YAxis().Scale(along.Cross(up).Y)).Add(frame.ZAxis().Scale(along.Cross(up).Z)))
			if e != nil {
				return Solid{}, e
			}
			surface, e := geom.NewPlaneSurface(f)
			if e != nil {
				return Solid{}, e
			}
			uv := make([]geom.Vec2, 4)
			for k, p := range points {
				q := f.ToLocal(frame.ToWorld(p))
				uv[k] = geom.Vec2{X: q.X, Y: q.Y}
			}
			if e = b.face(surface, points, uv, nil, false); e != nil {
				return Solid{}, e
			}
		}
	}
	return NewSolidContext(ctx, b.vertices, b.edges, b.faces, tolerance)
}
