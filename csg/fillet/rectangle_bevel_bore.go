package fillet

import (
	"context"
	"math"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// BoredRectangleBevel retains every native bevel vertex, edge and face ID.
// The two caps acquire inner wires; four cylindrical bore walls are appended.
// Nominal measures retain the ideal analytic convention of RectangleBevel.
type BoredRectangleBevel struct {
	Solid                             trimmed.Solid
	Stock                             RectangleBevel
	Center                            geom.Vec2
	Radius                            float64
	BoreFaces                         [4]int
	BoreRims                          [2][4]int
	NominalVolume, NominalSurfaceArea float64
}

// PrismRectangleTopBeveledBored adds a concentric through bore to the uniformly
// rounded, top-beveled rectangle. Exact conservative disk containment is checked
// in both endpoint sections: rectangle inset 2r at the bottom and d+2(r-d) at
// the top, with eight sewing tolerances of clearance. Along the bevel this
// sufficient central-box clearance varies affinely, so endpoint proofs cover
// every section. Existing rounded-chart sewing remains tolerance controlled.
// No existing topology IDs change, and failures return no partial solid.
func PrismRectangleTopBeveledBored(ctx context.Context, min, max geom.Vec2, zMin, zMax, radius, inset, rise, boreRadius float64, o Options) (BoredRectangleBevel, error) {
	fail := func(e error) (BoredRectangleBevel, error) { return BoredRectangleBevel{}, e }
	stock, e := PrismRectangleTopBeveled(ctx, min, max, zMin, zMax, radius, inset, rise, o)
	if e != nil {
		return fail(e)
	}
	if !finite(boreRadius) || boreRadius <= 0 {
		return fail(ErrInvalid)
	}
	tol := o.Tolerance
	if tol == 0 {
		tol = 1e-9
	}
	for _, margin := range []q{qm(num(2), num(radius)), qa(num(inset), qm(num(2), qs(num(radius), num(inset))))} {
		need := qa(qa(margin, num(boreRadius)), qm(num(8), num(tol)))
		for _, width := range []q{qs(num(max.X), num(min.X)), qs(num(max.Y), num(min.Y))} {
			if width.Cmp(qm(num(2), need)) <= 0 {
				return fail(ErrRadius)
			}
		}
	}
	selection := []CornerRadius{{Corner: 0, Radius: radius}, {Corner: 1, Radius: radius}, {Corner: 2, Radius: radius}, {Corner: 3, Radius: radius}}
	donor, e := PrismRectangleCornersBored(ctx, min, max, zMin, zMax, boreRadius, selection, o)
	if e != nil {
		return fail(e)
	}
	dv, de, df := donor.Solid.Vertices(), donor.Solid.Edges(), donor.Solid.Faces()
	vertices, edges, faces := stock.Solid.Vertices(), stock.Solid.Edges(), stock.Solid.Faces()
	usedE, usedV := make([]bool, len(de)), make([]bool, len(dv))
	for _, id := range donor.BoreFaces {
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
	r := BoredRectangleBevel{Stock: stock, Center: donor.Center, Radius: boreRadius}
	for cap, id := range stock.Lower.Caps {
		loop := remap(df[donor.Stock.Caps[cap]].Holes[0])
		faces[id].Holes = append(faces[id].Holes, loop)
		for j, u := range loop {
			r.BoreRims[cap][j] = u.Edge
		}
	}
	for i, id := range donor.BoreFaces {
		f := df[id]
		f.Outer = remap(f.Outer)
		r.BoreFaces[i] = len(faces)
		faces = append(faces, f)
	}
	r.Solid, e = trimmed.NewSolidContext(ctx, vertices, edges, faces, tol)
	if e != nil {
		return fail(e)
	}
	height := zMax - zMin
	r.NominalVolume = stock.NominalVolume - math.Pi*boreRadius*boreRadius*height
	r.NominalSurfaceArea = stock.NominalSurfaceArea + 2*math.Pi*boreRadius*height - 2*math.Pi*boreRadius*boreRadius
	return r, nil
}
