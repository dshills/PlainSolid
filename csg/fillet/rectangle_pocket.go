package fillet

import (
	"context"
	"math"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// RectanglePocket specifies an offset blind circular pocket. EntryCap is zero
// for the lower cap and one for the upper cap; Depth is positive into stock.
type RectanglePocket struct {
	Center        geom.Vec2
	Radius, Depth float64
	EntryCap      int
}

// BoredRectanglePocket retains the complete pre-pocket bore history and IDs.
// Only EntryFace's hole list changes; four walls and one floor are appended.
// NominalSurfaceArea and NominalVolume refer to the ideal analytic construction.
type BoredRectanglePocket struct {
	Solid                             trimmed.Solid
	Bore                              RectangleBore
	Pocket                            RectanglePocket
	EntryFace, FloorFace              int
	WallFaces                         [4]int
	EntryEdges, FloorEdges            [4]int
	NominalSurfaceArea, NominalVolume float64
}

// PrismRectangleCornersBoredPocket adds one disjoint blind pocket to the bored
// selectively rounded rectangle. Both disks must lie in the conservative central
// rectangle used by PrismRectangleCornersBored. Exact represented arithmetic
// proves footprint separation and a positive bottom ligament of eight tolerances.
// The native analytic charts retain the original fillet sewing tolerance contract.
// No result is returned on failure. All prior topology IDs are preserved.
func PrismRectangleCornersBoredPocket(ctx context.Context, min, max geom.Vec2, zMin, zMax, boreRadius float64, selection []CornerRadius, pocket RectanglePocket, o Options) (BoredRectanglePocket, error) {
	fail := func(e error) (BoredRectanglePocket, error) { return BoredRectanglePocket{}, e }
	if !finite(pocket.Center.X) || !finite(pocket.Center.Y) || !finite(pocket.Radius) || !finite(pocket.Depth) || pocket.Radius <= 0 || pocket.Depth <= 0 || pocket.EntryCap < 0 || pocket.EntryCap > 1 {
		return fail(ErrInvalid)
	}
	bore, e := PrismRectangleCornersBored(ctx, min, max, zMin, zMax, boreRadius, selection, o)
	if e != nil {
		return fail(e)
	}
	tol := o.Tolerance
	if tol == 0 {
		tol = 1e-9
	}
	pad := qm(num(8), num(tol))
	largest := 0.0
	for _, s := range selection {
		largest = math.Max(largest, s.Radius)
	}
	need := qa(qa(qm(num(2), num(largest)), pad), num(pocket.Radius))
	for _, distance := range []q{qs(num(pocket.Center.X), num(min.X)), qs(num(max.X), num(pocket.Center.X)), qs(num(pocket.Center.Y), num(min.Y)), qs(num(max.Y), num(pocket.Center.Y))} {
		if distance.Cmp(need) <= 0 {
			return fail(ErrRadius)
		}
	}
	separation := qa(qa(num(boreRadius), num(pocket.Radius)), pad)
	if squaredDistance(bore.Center, pocket.Center).Cmp(qm(separation, separation)) <= 0 {
		return fail(ErrRadius)
	}
	height := qs(num(zMax), num(zMin))
	if num(pocket.Depth).Cmp(pad) <= 0 || qa(num(pocket.Depth), pad).Cmp(height) >= 0 {
		return fail(ErrRadius)
	}
	origin, sign := zMin, 1.0
	if pocket.EntryCap == 1 {
		origin = zMax
		sign = -1
	}
	if _, e = geom.RepresentedExactScalar(qa(num(origin), qm(num(sign), num(pocket.Depth)))); e != nil {
		return fail(e)
	}
	h, e := geom.RepresentedExactScalar(qm(num(sign), height))
	if e != nil {
		return fail(e)
	}
	frame, e := geom.NewFrame(geom.Vec3{Z: origin}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		return fail(e)
	}
	p := []geom.Vec2{min, {X: max.X, Y: min.Y}, max, {X: min.X, Y: max.Y}}
	donor, e := trimmed.ExtrudePolygonWithHoles(ctx, frame, p, nil, []trimmed.ExtrusionBlindHole{{ExtrusionCircle: trimmed.ExtrusionCircle{Center: pocket.Center, Radius: pocket.Radius}, Depth: pocket.Depth}}, h)
	if e != nil {
		return fail(e)
	}
	dv, de, df := donor.Vertices(), donor.Edges(), donor.Faces()
	if len(df) != 11 || len(df[pocket.EntryCap].Holes) != 1 {
		return fail(ErrInvalid)
	}
	vertices, edges, faces := bore.Solid.Vertices(), bore.Solid.Edges(), bore.Solid.Faces()
	usedE, usedV := make([]bool, len(de)), make([]bool, len(dv))
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
		r := append([]trimmed.Use(nil), loop...)
		for i := range r {
			r[i].Edge = em[r[i].Edge]
		}
		return r
	}
	r := BoredRectanglePocket{Bore: bore, Pocket: pocket, EntryFace: bore.Stock.Caps[pocket.EntryCap]}
	loop := remap(df[pocket.EntryCap].Holes[0])
	faces[r.EntryFace].Holes = append(faces[r.EntryFace].Holes, loop)
	for i, u := range loop {
		r.EntryEdges[i] = u.Edge
	}
	for i, f := range df[6:] {
		f.Outer = remap(f.Outer)
		if i < 4 {
			r.WallFaces[i] = len(faces)
		} else {
			r.FloorFace = len(faces)
			for j, u := range f.Outer {
				r.FloorEdges[j] = u.Edge
			}
		}
		faces = append(faces, f)
	}
	r.Solid, e = trimmed.NewSolidContext(ctx, vertices, edges, faces, tol)
	if e != nil {
		return fail(e)
	}
	r.NominalVolume = bore.NominalVolume - math.Pi*pocket.Radius*pocket.Radius*pocket.Depth
	hAbs, _ := height.Float64()
	r.NominalSurfaceArea = 2*bore.NominalArea + bore.NominalPerimeter*hAbs + 2*math.Pi*pocket.Radius*pocket.Depth
	return r, nil
}
