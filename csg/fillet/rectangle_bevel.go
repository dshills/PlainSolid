package fillet

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// RectangleBevel retains the unchanged lower stock (ending at ShoulderZ).
// Lower face IDs survive except its top cap, replaced by the smaller top cap.
// BevelFaces follow lower cap boundary order, alternating cone and plane.
// Measures refer to the ideal uniform circular construction; native analytic
// charts retain Options.Tolerance endpoint/tangency agreement.
type RectangleBevel struct {
	Solid                             trimmed.Solid
	Lower                             SelectiveResult
	BevelFaces                        [8]int
	TopEdges, Connectors              [8]int
	ShoulderZ, Radius, Inset, Rise    float64
	NominalVolume, NominalSurfaceArea float64
}

// PrismRectangleTopBeveled constructs a uniform perimeter bevel above a
// rectangular prism with all four vertical corners rounded to radius. At the
// top, the rectangle is inset by inset and corner radius becomes radius-inset.
// Both straight-wall and cylindrical-wall ancestry remain native analytic.
// Dimensions, support slope, and cone apex must be losslessly representable;
// apex/pole cases and exhausted corner radii are excluded. No partial result.
func PrismRectangleTopBeveled(ctx context.Context, min, max geom.Vec2, zMin, zMax, radius, inset, rise float64, o Options) (RectangleBevel, error) {
	fail := func(e error) (RectangleBevel, error) { return RectangleBevel{}, e }
	if !finite(min.X) || !finite(min.Y) || !finite(max.X) || !finite(max.Y) || min.X >= max.X || min.Y >= max.Y {
		return fail(ErrInvalid)
	}
	if ctx == nil || !finite(radius) || !finite(inset) || !finite(rise) || radius <= 0 || inset <= 0 || inset >= radius || rise <= 0 || !finite(zMin) || !finite(zMax) {
		return fail(ErrInvalid)
	}
	exact := geom.RepresentedExactScalar
	shoulder, e := exact(qs(num(zMax), num(rise)))
	if e != nil {
		return fail(e)
	}
	topR, e := exact(qs(num(radius), num(inset)))
	if e != nil {
		return fail(e)
	}
	slope, e := exact(newRatQuo(num(inset), num(rise)))
	if e != nil {
		return fail(e)
	}
	bottomV, e := exact(newRatQuo(num(radius), num(slope)))
	if e != nil {
		return fail(e)
	}
	topV, e := exact(newRatQuo(num(topR), num(slope)))
	if e != nil {
		return fail(e)
	}
	apex, e := exact(qa(num(shoulder), num(bottomV)))
	if e != nil {
		return fail(e)
	}
	topMin, topMax := geom.Vec2{}, geom.Vec2{}
	topMin.X, e = exact(qa(num(min.X), num(inset)))
	if e != nil {
		return fail(e)
	}
	topMin.Y, e = exact(qa(num(min.Y), num(inset)))
	if e != nil {
		return fail(e)
	}
	topMax.X, e = exact(qs(num(max.X), num(inset)))
	if e != nil {
		return fail(e)
	}
	topMax.Y, e = exact(qs(num(max.Y), num(inset)))
	if e != nil {
		return fail(e)
	}
	selection := func(r float64) []CornerRadius {
		return []CornerRadius{{Corner: 0, Radius: r}, {Corner: 1, Radius: r}, {Corner: 2, Radius: r}, {Corner: 3, Radius: r}}
	}
	polygon := func(a, b geom.Vec2) []geom.Vec2 { return []geom.Vec2{a, {X: b.X, Y: a.Y}, b, {X: a.X, Y: b.Y}} }
	lower, e := PrismCorners(ctx, polygon(min, max), zMin, shoulder, selection(radius), o)
	if e != nil {
		return fail(e)
	}
	upper, e := PrismCorners(ctx, polygon(topMin, topMax), shoulder, zMax, selection(topR), o)
	if e != nil {
		return fail(e)
	}
	vertices, edges, faces := lower.Solid.Vertices(), lower.Solid.Edges(), lower.Solid.Faces()
	uv, ue, uf := upper.Solid.Vertices(), upper.Solid.Edges(), upper.Solid.Faces()
	if len(vertices) != 16 || len(edges) != 24 || len(faces) != 10 {
		return fail(ErrInvalid)
	}
	r := RectangleBevel{Lower: lower, ShoulderZ: shoulder, Radius: radius, Inset: inset, Rise: rise}
	vertices = append(vertices, uv[8:16]...)
	for i := 0; i < 8; i++ {
		x := ue[8+i]
		x.Start += 8
		x.End += 8
		r.TopEdges[i] = len(edges)
		edges = append(edges, x)
	}
	for i := 0; i < 8; i++ {
		a, b := vertices[8+i], vertices[16+i]
		d := b.Sub(a)
		line, e := geom.NewLine(a, d)
		if e != nil {
			return fail(e)
		}
		r.Connectors[i] = len(edges)
		edges = append(edges, parametric.Edge{Start: 8 + i, End: 16 + i, Curve: line, Range: parametric.Interval{Max: d.Length()}})
	}
	cap := uf[1]
	for i := range cap.Outer {
		cap.Outer[i].Edge = r.TopEdges[i]
	}
	faces[1] = cap
	for i := 0; i < 8; i++ {
		if e := ctx.Err(); e != nil {
			return fail(e)
		}
		var surface geom.Surface
		var chart [4]geom.Vec2
		if i%2 == 0 {
			corner := lower.Corners[i/2]
			a := vertices[8+i]
			frame, e := geom.NewFrame(geom.Vec3{X: corner.Center.X, Y: corner.Center.Y, Z: apex}, geom.Vec3{X: a.X - corner.Center.X, Y: a.Y - corner.Center.Y}, geom.Vec3{Z: -1})
			if e != nil {
				return fail(e)
			}
			cone, e := geom.NewConeSurfaceSlope(frame, slope)
			if e != nil {
				return fail(e)
			}
			surface = cone
			chart = [4]geom.Vec2{{Y: bottomV}, {X: -corner.Sweep, Y: bottomV}, {X: -corner.Sweep, Y: topV}, {Y: topV}}
		} else {
			a, b := vertices[8+i], vertices[8+(i+1)%8]
			direction := b.Sub(a)
			delta := vertices[16+i].Sub(a)
			frame, e := geom.NewFrame(a, direction, direction.Cross(delta))
			if e != nil {
				return fail(e)
			}
			plane, e := geom.NewPlaneSurface(frame)
			if e != nil {
				return fail(e)
			}
			surface = plane
			chart = [4]geom.Vec2{{}, {X: direction.Length()}, {X: direction.Length(), Y: delta.Length()}, {Y: delta.Length()}}
		}
		ids := [4]int{8 + i, r.Connectors[(i+1)%8], r.TopEdges[i], r.Connectors[i]}
		f := trimmed.Face{Surface: surface}
		for j, id := range ids {
			f.Outer = append(f.Outer, trimmed.Use{Edge: id, Reversed: j >= 2, UVStart: chart[j], UVEnd: chart[(j+1)%4]})
		}
		r.BevelFaces[i] = len(faces)
		faces = append(faces, f)
	}
	tol := o.Tolerance
	if tol == 0 {
		tol = 1e-9
	}
	r.Solid, e = trimmed.NewSolidContext(ctx, vertices, edges, faces, tol)
	if e != nil {
		return fail(e)
	}
	w, h := max.X-min.X, max.Y-min.Y
	area := w*h - (4-math.Pi)*radius*radius
	perimeter := 2*(w+h) - 8*radius + 2*math.Pi*radius
	r.NominalVolume = area*(shoulder-zMin) + rise*(area-perimeter*inset/2+math.Pi*inset*inset/3)
	topArea := area - perimeter*inset + math.Pi*inset*inset
	r.NominalSurfaceArea = area + topArea + perimeter*(shoulder-zMin) + (perimeter-math.Pi*inset)*math.Hypot(inset, rise)
	return r, nil
}

func newRatQuo(a, b q) q { return new(big.Rat).Quo(a, b) }
