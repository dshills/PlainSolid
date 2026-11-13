package trimmed

import (
	"context"
	"math"
	"plainsolid/geom"
)

// BoxCornerCylinderThrough cuts a quarter circular notch through the complete
// axial extent of a rectangular prism. Opposite radial sides remain untouched.
func BoxCornerCylinderThrough(ctx context.Context, frame geom.Frame, width, height, length, radius, tolerance float64) (Solid, error) {
	if ctx == nil || !finite(width) || !finite(height) || !finite(length) || !finite(radius) || !finite(tolerance) || tolerance <= 0 {
		return Solid{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, e
	}
	pad := 8 * tolerance
	if !finite(pad) || radius <= pad || length <= pad || width-radius <= pad || height-radius <= pad {
		return Solid{}, geom.ErrDomain
	}
	b := cornerCylinderBuilder{ctx: ctx, frame: frame, edgesByEnds: map[[2]int]int{}, verticesByPoint: map[geom.Vec3]int{}}
	p := []geom.Vec2{{X: radius}, {X: width}, {X: width, Y: height}, {Y: height}, {Y: radius}}
	arcs := map[int][2]float64{4: {math.Pi / 2, 0}}
	if e := b.horizontal(p, arcs, 0, true, radius); e != nil {
		return Solid{}, e
	}
	if e := b.horizontal(p, arcs, length, false, radius); e != nil {
		return Solid{}, e
	}
	for i, a := range p {
		q := p[(i+1)%len(p)]
		var e error
		if i == 4 {
			e = b.cylinder(0, math.Pi/2, length, radius, true, a, q)
		} else {
			e = b.wall(a, q, 0, length)
		}
		if e != nil {
			return Solid{}, e
		}
	}
	return NewSolidContext(ctx, b.vertices, b.edges, b.faces, tolerance)
}
