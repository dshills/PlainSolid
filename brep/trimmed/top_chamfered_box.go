package trimmed

import (
	"context"
	"math"
	"plainsolid/geom"
)

// TopChamferedBox constructs rectangular stock with equal setbacks on all four
// top perimeter edges. An optional axial through bore must stay strictly inside
// the inset top rectangle. The result retains planar bevels and circular trims;
// it does not approximate a chamfer by tessellation. A zero bore radius omits it.
func TopChamferedBox(ctx context.Context, frame geom.Frame, width, depth, height, setback float64, boreCenter geom.Vec2, boreRadius, tolerance float64) (Solid, error) {
	if ctx == nil || !finite(width) || !finite(depth) || !finite(height) || !finite(setback) || !finite(boreRadius) || !finite(tolerance) || tolerance <= 0 || boreRadius < 0 || !finite(boreCenter.X) || !finite(boreCenter.Y) {
		return Solid{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, e
	}
	pad := 8 * tolerance
	if !finite(pad) || setback <= pad || height-setback <= pad || width-2*setback <= pad || depth-2*setback <= pad {
		return Solid{}, geom.ErrDomain
	}
	if boreRadius > 0 && (boreRadius <= pad || boreCenter.X-boreRadius-setback <= pad || width-setback-boreCenter.X-boreRadius <= pad || boreCenter.Y-boreRadius-setback <= pad || depth-setback-boreCenter.Y-boreRadius <= pad) {
		return Solid{}, geom.ErrDomain
	}
	if frame.XAxis().Length() == 0 || frame.ZAxis().Length() == 0 {
		return Solid{}, geom.ErrInvalid
	}
	b := cornerCylinderBuilder{ctx: ctx, frame: frame, edgesByEnds: map[[2]int]int{}, verticesByPoint: map[geom.Vec3]int{}}
	outer := []geom.Vec2{{}, {X: width}, {X: width, Y: depth}, {Y: depth}}
	inner := []geom.Vec2{{X: setback, Y: setback}, {X: width - setback, Y: setback}, {X: width - setback, Y: depth - setback}, {X: setback, Y: depth - setback}}
	for _, cap := range []struct {
		p       []geom.Vec2
		z       float64
		reverse bool
	}{{outer, 0, true}, {inner, height, false}} {
		if e := b.horizontal(cap.p, nil, cap.z, cap.reverse, 0); e != nil {
			return Solid{}, e
		}
		if boreRadius > 0 {
			face := &b.faces[len(b.faces)-1]
			points := make([]geom.Vec3, 4)
			uv := make([]geom.Vec2, 4)
			curves := map[int]PCurve{}
			ring := []geom.Vec2{{X: boreCenter.X + boreRadius, Y: boreCenter.Y}, {X: boreCenter.X, Y: boreCenter.Y - boreRadius}, {X: boreCenter.X - boreRadius, Y: boreCenter.Y}, {X: boreCenter.X, Y: boreCenter.Y + boreRadius}}
			for i, p := range ring {
				points[i] = geom.Vec3{X: p.X, Y: p.Y, Z: cap.z}
				uv[i] = p
				c, e := newAnchoredCircularUV(EllipseUV{Center: boreCenter, Cos: geom.Vec2{X: boreRadius}, Sin: geom.Vec2{Y: -boreRadius}, From: float64(i) * math.Pi / 2, To: float64(i+1) * math.Pi / 2}, p, ring[(i+1)%4])
				if e != nil {
					return Solid{}, e
				}
				curves[i] = c
			}
			hole, e := b.wire(face.Surface, points, uv, curves)
			if e != nil {
				return Solid{}, e
			}
			face.Holes = [][]Use{hole}
		}
	}
	for i, p := range outer {
		j := (i + 1) % 4
		q := outer[j]
		if e := b.wall(p, q, 0, height-setback); e != nil {
			return Solid{}, e
		}
		points := []geom.Vec3{{X: p.X, Y: p.Y, Z: height - setback}, {X: q.X, Y: q.Y, Z: height - setback}, {X: inner[j].X, Y: inner[j].Y, Z: height}, {X: inner[i].X, Y: inner[i].Y, Z: height}}
		origin := frame.ToWorld(points[0])
		x := frame.ToWorld(points[1]).Sub(origin)
		normal := x.Cross(frame.ToWorld(points[3]).Sub(origin))
		f, e := geom.NewFrame(origin, x, normal)
		if e != nil {
			return Solid{}, e
		}
		surface, e := geom.NewPlaneSurface(f)
		if e != nil {
			return Solid{}, e
		}
		uv := make([]geom.Vec2, 4)
		for k, p := range points {
			local := f.ToLocal(frame.ToWorld(p))
			uv[k] = geom.Vec2{X: local.X, Y: local.Y}
		}
		if e = b.face(surface, points, uv, nil, false); e != nil {
			return Solid{}, e
		}
	}
	if boreRadius > 0 {
		f, e := geom.NewFrame(frame.ToWorld(geom.Vec3{X: boreCenter.X, Y: boreCenter.Y}), frame.XAxis(), frame.ZAxis())
		if e != nil {
			return Solid{}, e
		}
		surface, e := geom.NewCylinderSurface(f, boreRadius)
		if e != nil {
			return Solid{}, e
		}
		ring := []geom.Vec2{{X: boreCenter.X + boreRadius, Y: boreCenter.Y}, {X: boreCenter.X, Y: boreCenter.Y + boreRadius}, {X: boreCenter.X - boreRadius, Y: boreCenter.Y}, {X: boreCenter.X, Y: boreCenter.Y - boreRadius}}
		for i, p := range ring {
			q := ring[(i+1)%4]
			lo, hi := float64(i)*math.Pi/2, float64(i+1)*math.Pi/2
			points := []geom.Vec3{{X: p.X, Y: p.Y}, {X: q.X, Y: q.Y}, {X: q.X, Y: q.Y, Z: height}, {X: p.X, Y: p.Y, Z: height}}
			uv := []geom.Vec2{{X: lo}, {X: hi}, {X: hi, Y: height}, {X: lo, Y: height}}
			if e = b.face(surface, points, uv, nil, true); e != nil {
				return Solid{}, e
			}
		}
	}
	return NewSolidContext(ctx, b.vertices, b.edges, b.faces, tolerance)
}
