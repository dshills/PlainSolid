package trimmed

import (
	"context"
	"math"
	"plainsolid/geom"
)

// BoxSideCylinderThrough removes a finite axial circular notch crossing just
// the lower Y side. Frame's axis passes through the circle center; stock spans
// X=[-left,right], Y=[-inset,back], Z=[0,length].
func BoxSideCylinderThrough(ctx context.Context, frame geom.Frame, left, right, back, inset, length, radius, tolerance float64) (Solid, error) {
	if ctx == nil {
		return Solid{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, e
	}
	for _, v := range []float64{left, right, back, inset, length, radius, tolerance} {
		if !finite(v) {
			return Solid{}, geom.ErrInvalid
		}
	}
	pad := 8 * tolerance
	if tolerance <= 0 || !finite(pad) || radius <= pad || length <= pad || radius-math.Abs(inset) <= pad || left-radius <= pad || right-radius <= pad || back-radius <= pad {
		return Solid{}, geom.ErrDomain
	}
	root := math.Sqrt((radius - inset) * (radius + inset))
	start, end := math.Pi+math.Asin(inset/radius), -math.Asin(inset/radius)
	n := int(math.Ceil((start - end) / (math.Pi / 2)))
	p := []geom.Vec2{{X: root, Y: -inset}, {X: right, Y: -inset}, {X: right, Y: back}, {X: -left, Y: back}, {X: -left, Y: -inset}, {X: -root, Y: -inset}}
	arcs := map[int][2]float64{}
	for i := 0; i < n; i++ {
		a := start + (end-start)*float64(i)/float64(n)
		z := start + (end-start)*float64(i+1)/float64(n)
		arcs[5+i] = [2]float64{a, z}
		if i+1 < n {
			p = append(p, geom.Vec2{X: radius * math.Cos(z), Y: radius * math.Sin(z)})
		}
	}
	// Shared world representatives must survive cardinal placement. Only the
	// already-qualified model tolerance covers trig/intersection roundoff.
	for i, v := range p {
		q := frame.ToLocal(frame.ToWorld(geom.Vec3{X: v.X, Y: v.Y}))
		if math.Hypot(q.X-v.X, q.Y-v.Y) > tolerance/8 || q.Z != 0 {
			return Solid{}, geom.ErrUnrepresentable
		}
		p[i] = geom.Vec2{X: q.X, Y: q.Y}
	}
	b := cornerCylinderBuilder{ctx: ctx, frame: frame, edgesByEnds: map[[2]int]int{}, verticesByPoint: map[geom.Vec3]int{}}

	for _, cap := range []struct {
		z       float64
		reverse bool
	}{{0, true}, {length, false}} {
		cf, e := geom.NewFrame(frame.ToWorld(geom.Vec3{Z: cap.z}), frame.XAxis(), frame.ZAxis())
		if e != nil {
			return Solid{}, e
		}
		surface, e := geom.NewPlaneSurface(cf)
		if e != nil {
			return Solid{}, e
		}
		points := make([]geom.Vec3, len(p))
		curves := map[int]PCurve{}
		for i, v := range p {
			points[i] = geom.Vec3{X: v.X, Y: v.Y, Z: cap.z}
			if a, ok := arcs[i]; ok {
				curve := EllipseUV{Cos: geom.Vec2{X: radius}, Sin: geom.Vec2{Y: radius}, From: a[0], To: a[1]}
				lo, e0 := curve.EvalUV(0)
				hi, e1 := curve.EvalUV(1)
				q := p[(i+1)%len(p)]
				if e0 != nil || e1 != nil || math.Hypot(lo.X-v.X, lo.Y-v.Y) > tolerance || math.Hypot(hi.X-q.X, hi.Y-q.Y) > tolerance {
					return Solid{}, geom.ErrUnrepresentable
				}
				curves[i] = unionArcUV{EllipseUV: curve, Start: v, End: q}
			}
		}
		if e = b.face(surface, points, p, curves, cap.reverse); e != nil {
			return Solid{}, e
		}
	}

	for i, v := range p {
		q := p[(i+1)%len(p)]
		var e error
		if a, ok := arcs[i]; ok {
			e = b.cylinder(a[1], a[0], length, radius, true, v, q)
		} else {
			e = b.wall(v, q, 0, length)
		}
		if e != nil {
			return Solid{}, e
		}
	}
	return NewSolidContext(ctx, b.vertices, b.edges, b.faces, tolerance)
}
