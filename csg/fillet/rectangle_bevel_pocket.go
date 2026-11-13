package fillet

import (
	"context"
	"math"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// BoredBeveledRectanglePocket retains the complete beveled/bored predecessor.
// Only its entry cap gains an inner wire; existing IDs and supports survive.
type BoredBeveledRectanglePocket struct {
	Solid                             trimmed.Solid
	Stock                             BoredRectangleBevel
	Pocket                            RectanglePocket
	EntryFace, FloorFace              int
	WallFaces                         [4]int
	EntryEdges, FloorEdges            [4]int
	NominalVolume, NominalSurfaceArea float64
}

// PrismRectangleTopBeveledBoredPocket adds one offset blind pocket from either
// cap. Its exact conservative footprint is inside the rectangle inset by 2r
// plus eight tolerances and is disjoint from the bore. At bevel offset t the
// sufficient central inset is t+2(r-t)=2r-t, so this single worst-section proof
// covers the whole height, including floors crossing the shoulder. Depth leaves
// eight tolerances of bottom ligament. Existing native chart sewing tolerance
// and ideal nominal-mass conventions are unchanged. Failure is atomic.
func PrismRectangleTopBeveledBoredPocket(ctx context.Context, min, max geom.Vec2, zMin, zMax, radius, inset, rise, boreRadius float64, pocket RectanglePocket, o Options) (BoredBeveledRectanglePocket, error) {
	fail := func(e error) (BoredBeveledRectanglePocket, error) { return BoredBeveledRectanglePocket{}, e }
	stock, e := PrismRectangleTopBeveledBored(ctx, min, max, zMin, zMax, radius, inset, rise, boreRadius, o)
	if e != nil {
		return fail(e)
	}
	selection := []CornerRadius{{Corner: 0, Radius: radius}, {Corner: 1, Radius: radius}, {Corner: 2, Radius: radius}, {Corner: 3, Radius: radius}}
	donor, e := PrismRectangleCornersBoredPocket(ctx, min, max, zMin, zMax, boreRadius, selection, pocket, o)
	if e != nil {
		return fail(e)
	}
	dv, de, df := donor.Solid.Vertices(), donor.Solid.Edges(), donor.Solid.Faces()
	vertices, edges, faces := stock.Solid.Vertices(), stock.Solid.Edges(), stock.Solid.Faces()
	usedE, usedV := make([]bool, len(de)), make([]bool, len(dv))
	generated := append(append([]int(nil), donor.WallFaces[:]...), donor.FloorFace)
	for _, id := range generated {
		for _, u := range df[id].Outer {
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
	r := BoredBeveledRectanglePocket{Stock: stock, Pocket: pocket, EntryFace: stock.Stock.Lower.Caps[pocket.EntryCap]}
	holes := df[donor.EntryFace].Holes
	if len(holes) != 2 {
		return fail(ErrInvalid)
	}
	loop := remap(holes[1])
	faces[r.EntryFace].Holes = append(faces[r.EntryFace].Holes, loop)
	for i, u := range loop {
		r.EntryEdges[i] = u.Edge
	}
	for i, id := range generated {
		f := df[id]
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
	tol := o.Tolerance
	if tol == 0 {
		tol = 1e-9
	}
	r.Solid, e = trimmed.NewSolidContext(ctx, vertices, edges, faces, tol)
	if e != nil {
		return fail(e)
	}
	r.NominalVolume = stock.NominalVolume - math.Pi*pocket.Radius*pocket.Radius*pocket.Depth
	r.NominalSurfaceArea = stock.NominalSurfaceArea + 2*math.Pi*pocket.Radius*pocket.Depth
	return r, nil
}
