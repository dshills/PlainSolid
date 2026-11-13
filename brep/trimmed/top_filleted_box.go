package trimmed

import (
	"context"
	"math"
	"plainsolid/geom"
)

// TopFilletedBox rounds the four top perimeter edges of rectangular stock,
// retaining its sharp vertical edges. Adjacent quarter cylinders meet on exact
// diagonal-plane elliptical seams; no spherical corner or faceted substitute
// changes the intended intersection. Radius must leave a nonempty top face.
func TopFilletedBox(ctx context.Context, frame geom.Frame, width, depth, height, radius, tolerance float64) (Solid, error) {
	return topFilletedBox(ctx, frame, width, depth, height, radius, 0, tolerance)
}

func topFilletedBox(ctx context.Context, frame geom.Frame, width, depth, height, radius, bottomChamfer, tolerance float64) (Solid, error) {
	if ctx == nil || !finite(width) || !finite(depth) || !finite(height) || !finite(radius) || !finite(tolerance) || tolerance <= 0 || !finite(bottomChamfer) || bottomChamfer < 0 {
		return Solid{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, e
	}
	pad := 8 * tolerance
	if !finite(pad) || radius <= pad || height-radius-bottomChamfer <= pad || width-2*radius <= pad || depth-2*radius <= pad || width-2*bottomChamfer <= pad || depth-2*bottomChamfer <= pad {
		return Solid{}, geom.ErrDomain
	}
	if frame.XAxis().Length() == 0 || frame.ZAxis().Length() == 0 {
		return Solid{}, geom.ErrInvalid
	}
	b := cornerCylinderBuilder{ctx: ctx, frame: frame, edgesByEnds: map[[2]int]int{}, verticesByPoint: map[geom.Vec3]int{}}
	outer := []geom.Vec2{{}, {X: width}, {X: width, Y: depth}, {Y: depth}}
	inner := []geom.Vec2{{X: radius, Y: radius}, {X: width - radius, Y: radius}, {X: width - radius, Y: depth - radius}, {X: radius, Y: depth - radius}}
	bottom := []geom.Vec2{{X: bottomChamfer, Y: bottomChamfer}, {X: width - bottomChamfer, Y: bottomChamfer}, {X: width - bottomChamfer, Y: depth - bottomChamfer}, {X: bottomChamfer, Y: depth - bottomChamfer}}
	if e := b.horizontal(bottom, nil, 0, true, 0); e != nil {
		return Solid{}, e
	}
	if e := b.horizontal(inner, nil, height, false, 0); e != nil {
		return Solid{}, e
	}
	for i, p := range outer {
		j := (i + 1) % 4
		q := outer[j]
		if bottomChamfer > 0 {
			if e := b.bottomChamferFace(bottom[i], bottom[j], p, q, bottomChamfer); e != nil {
				return Solid{}, e
			}
		}
		if e := b.wall(p, q, bottomChamfer, height-radius); e != nil {
			return Solid{}, e
		}
		edge := geom.Vec3{X: q.X - p.X, Y: q.Y - p.Y}
		length := edge.Length()
		axis, e := edge.Unit()
		if e != nil {
			return Solid{}, e
		}
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
		points := []geom.Vec3{{X: inner[i].X, Y: inner[i].Y, Z: height}, {X: p.X, Y: p.Y, Z: height - radius}, {X: q.X, Y: q.Y, Z: height - radius}, {X: inner[j].X, Y: inner[j].Y, Z: height}}
		uv := []geom.Vec2{{Y: radius}, {X: math.Pi / 2}, {X: math.Pi / 2, Y: length}, {Y: length - radius}}
		curves := map[int]PCurve{}
		for _, c := range []struct {
			index int
			curve EllipseUV
		}{{0, EllipseUV{Center: geom.Vec2{Y: radius}, Sin: geom.Vec2{Y: -radius}, Linear: geom.Vec2{X: 1}, From: 0, To: math.Pi / 2}}, {2, EllipseUV{Center: geom.Vec2{Y: length - radius}, Sin: geom.Vec2{Y: radius}, Linear: geom.Vec2{X: 1}, From: math.Pi / 2, To: 0}}} {
			curves[c.index] = c.curve
		}
		if e = b.face(surface, points, uv, curves, false); e != nil {
			return Solid{}, e
		}
	}
	return NewSolidContext(ctx, b.vertices, b.edges, b.faces, tolerance)
}
