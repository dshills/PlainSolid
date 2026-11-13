package trimmed

import (
	"context"
	"math"
	"plainsolid/geom"
)

// TopFilletedRoundedRectangle rounds the entire upper cap perimeter of a
// uniformly rounded rectangle. Straight portions retain quarter cylinders and
// rounded corners retain quarter tori. The lower cap remains unchanged. The
// current domain requires corner radius > twice the fillet radius, so each
// toroidal support is a regular ring torus, and positive extrusion height.
func TopFilletedRoundedRectangle(ctx context.Context, frame geom.Frame, profile RoundedRectangle, height, radius, tolerance float64) (Solid, error) {
	if ctx == nil || !finite(height) || !finite(radius) || !finite(tolerance) || tolerance <= 0 {
		return Solid{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, e
	}
	pad := 8 * tolerance
	corner := profile.Radii[0]
	if !finite(pad) || radius <= pad || height-radius <= pad || !finite(corner) || corner-2*radius <= pad {
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
	upper := make([]geom.Vec2, 8)
	upperArcs := append([]extrusionQuarterArc(nil), arcs...)
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
	b := cornerCylinderBuilder{ctx: ctx, frame: frame, edgesByEnds: map[[2]int]int{}, verticesByPoint: map[geom.Vec3]int{}}
	for _, cap := range []struct {
		p       []geom.Vec2
		arcs    []extrusionQuarterArc
		z       float64
		reverse bool
	}{{lower, arcs, 0, true}, {upper, upperArcs, height, false}} {
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
		points := []geom.Vec3{{X: p.X, Y: p.Y}, {X: q.X, Y: q.Y}, {X: q.X, Y: q.Y, Z: height - radius}, {X: p.X, Y: p.Y, Z: height - radius}}
		if a.circle.Radius > 0 {
			surface, uv, e := a.wall(frame, 0, height-radius)
			if e != nil {
				return Solid{}, e
			}
			if e = b.face(surface, points, uv[:], nil, false); e != nil {
				return Solid{}, e
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
			if e = b.wall(p, q, 0, height-radius); e != nil {
				return Solid{}, e
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
	return NewSolidContext(ctx, b.vertices, b.edges, b.faces, tolerance)
}
