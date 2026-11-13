package fillet

import (
	"context"
	"math"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// RectangleBore retains the complete pre-bore fillet history. Existing vertices,
// edges and faces keep their IDs; only the two cap hole lists change. BoreFaces
// lists the four appended cylindrical walls, and BoreRims the lower/upper arcs.
// Nominal measures describe the ideal circular construction, as in PrismCorners.
type RectangleBore struct {
	Solid                                        trimmed.Solid
	Stock                                        SelectiveResult
	Center                                       geom.Vec2
	Radius                                       float64
	BoreFaces                                    [4]int
	BoreRims                                     [2][4]int
	NominalArea, NominalPerimeter, NominalVolume float64
}

// PrismRectangleCornersBored adds a concentric Z-directed circular through bore
// to CCW rectangular stock (corner zero is min). It preserves analytic fillets.
// This deliberately conservative subset requires the bore disk strictly inside
// the rectangle inset by twice the largest fillet radius plus eight tolerances.
// Every removed corner lies outside that central rectangle. Clearance uses exact
// represented arithmetic; native fillet chart sewing retains Options.Tolerance.
// Center and height must be losslessly representable. Failure returns no solid.
func PrismRectangleCornersBored(ctx context.Context, min, max geom.Vec2, zMin, zMax, radius float64, selection []CornerRadius, o Options) (RectangleBore, error) {
	fail := func(e error) (RectangleBore, error) { return RectangleBore{}, e }
	if ctx == nil || !finite(min.X) || !finite(min.Y) || !finite(max.X) || !finite(max.Y) || min.X >= max.X || min.Y >= max.Y || !finite(radius) || radius <= 0 {
		return fail(ErrInvalid)
	}
	p := []geom.Vec2{min, {X: max.X, Y: min.Y}, max, {X: min.X, Y: max.Y}}
	stock, e := PrismCorners(ctx, p, zMin, zMax, selection, o)
	if e != nil {
		return fail(e)
	}
	tol := o.Tolerance
	if tol == 0 {
		tol = 1e-9
	}
	largest := 0.0
	for _, s := range selection {
		largest = math.Max(largest, s.Radius)
	}
	margin := qa(qm(num(2), num(largest)), qm(num(8), num(tol)))
	need := qa(num(radius), margin)
	for _, width := range []q{qs(num(max.X), num(min.X)), qs(num(max.Y), num(min.Y))} {
		if width.Cmp(qm(num(2), need)) <= 0 {
			return fail(ErrRadius)
		}
	}
	cx, e := geom.RepresentedExactScalar(new(big.Rat).Quo(qa(num(min.X), num(max.X)), num(2)))
	if e != nil {
		return fail(e)
	}
	cy, e := geom.RepresentedExactScalar(new(big.Rat).Quo(qa(num(min.Y), num(max.Y)), num(2)))
	if e != nil {
		return fail(e)
	}
	h, e := geom.RepresentedExactScalar(qs(num(zMax), num(zMin)))
	if e != nil {
		return fail(e)
	}
	frame, e := geom.NewFrame(geom.Vec3{Z: zMin}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		return fail(e)
	}
	center := geom.Vec2{X: cx, Y: cy}
	donor, e := trimmed.ExtrudePolygonWithCircularHoles(ctx, frame, p, []trimmed.ExtrusionCircle{{Center: center, Radius: radius}}, h)
	if e != nil {
		return fail(e)
	}
	dv, de, df := donor.Vertices(), donor.Edges(), donor.Faces()
	if len(df) != 10 || len(df[0].Holes) != 1 || len(df[1].Holes) != 1 {
		return fail(ErrInvalid)
	}
	vertices, edges, faces := stock.Solid.Vertices(), stock.Solid.Edges(), stock.Solid.Faces()
	usedE := make([]bool, len(de))
	usedV := make([]bool, len(dv))
	for _, f := range df[6:] {
		for _, u := range f.Outer {
			usedE[u.Edge] = true
			usedV[de[u.Edge].Start] = true
			usedV[de[u.Edge].End] = true
		}
	}
	vm, em := make([]int, len(dv)), make([]int, len(de))
	for i, yes := range usedV {
		if yes {
			vm[i] = len(vertices)
			vertices = append(vertices, dv[i])
		}
	}
	for i, yes := range usedE {
		if yes {
			x := de[i]
			x.Start = vm[x.Start]
			x.End = vm[x.End]
			em[i] = len(edges)
			edges = append(edges, x)
		}
	}
	remap := func(loop []trimmed.Use) []trimmed.Use {
		out := append([]trimmed.Use(nil), loop...)
		for i := range out {
			out[i].Edge = em[out[i].Edge]
		}
		return out
	}
	r := RectangleBore{Stock: stock, Center: center, Radius: radius}
	for cap, id := range stock.Caps {
		loop := remap(df[cap].Holes[0])
		faces[id].Holes = append(faces[id].Holes, loop)
		for j, u := range loop {
			r.BoreRims[cap][j] = u.Edge
		}
	}
	for i, f := range df[6:] {
		f.Outer = remap(f.Outer)
		r.BoreFaces[i] = len(faces)
		faces = append(faces, f)
	}
	r.Solid, e = trimmed.NewSolidContext(ctx, vertices, edges, faces, tol)
	if e != nil {
		return fail(e)
	}
	r.NominalArea = stock.NominalArea - math.Pi*radius*radius
	r.NominalPerimeter = stock.NominalPerimeter + 2*math.Pi*radius
	r.NominalVolume = r.NominalArea * h
	return r, nil
}
