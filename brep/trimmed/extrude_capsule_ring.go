package trimmed

import (
	"context"
	"math"

	"plainsolid/geom"
)

// ExtrudeCapsuleRing sweeps the region between two strictly nested capsules
// sharing the same horizontal centerline and centers. The circular and straight
// walls remain analytic; shared top and bottom faces contain native curved holes.
// Height is signed along frame Z. Only one nested capsule hole is supported.
func ExtrudeCapsuleRing(ctx context.Context, frame geom.Frame, left, right geom.Vec2, outerRadius, innerRadius, height float64) (Solid, error) {
	if !finite(innerRadius) || innerRadius <= 0 || !finite(outerRadius) || outerRadius <= innerRadius {
		return Solid{}, geom.ErrDomain
	}
	outer, e := ExtrudeCapsule(ctx, frame, left, right, outerRadius, height)
	if e != nil {
		return Solid{}, e
	}
	inner, e := ExtrudeCapsule(ctx, frame, left, right, innerRadius, height)
	if e != nil {
		return Solid{}, e
	}
	vertices, edges, faces := outer.Vertices(), outer.Edges(), outer.Faces()
	iv, ie, ifs := inner.Vertices(), inner.Edges(), inner.Faces()
	if len(faces) != 8 || len(ifs) != 8 {
		return Solid{}, geom.ErrInvalid
	}
	vo, eo := len(vertices), len(edges)
	vertices = append(vertices, iv...)
	for _, edge := range ie {
		edge.Start += vo
		edge.End += vo
		edges = append(edges, edge)
	}
	for cap := 0; cap < 2; cap++ {
		hole := make([]Use, len(ifs[cap].Outer))
		for i, u := range ifs[cap].Outer {
			u.Edge += eo
			u.Reversed = !u.Reversed
			u.UVStart, u.UVEnd = u.UVEnd, u.UVStart
			if u.Curve != nil {
				arc, ok := u.Curve.(anchoredCircularUV)
				if !ok {
					return Solid{}, geom.ErrInvalid
				}
				x := arc.EllipseUV
				x.Sin = scale2(x.Sin, -1)
				x.From, x.To = 2*math.Pi-x.To, 2*math.Pi-x.From
				c, e := newAnchoredCircularUV(x, u.UVStart, u.UVEnd)
				if e != nil {
					return Solid{}, e
				}
				u.Curve = c
			}
			hole[len(hole)-1-i] = u
		}
		faces[cap].Holes = append(faces[cap].Holes, hole)
	}
	for _, f := range ifs[2:] {
		f.Reversed = !f.Reversed
		for i := range f.Outer {
			f.Outer[i].Edge += eo
		}
		faces = append(faces, f)
	}
	tolerance := math.Max(math.Abs(height), math.Max(right.X-left.X+2*outerRadius, 2*outerRadius)) * 1e-11
	return NewSolidContext(ctx, vertices, edges, faces, tolerance)
}
