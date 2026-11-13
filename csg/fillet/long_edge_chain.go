package fillet

import (
	"context"
	"math"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// LongEdgeChain retains the actual X-long horizontal-edge-treated stock.
// Original IDs remain stable. Only top/bottom cap charts are replaced by exact
// XY charts on the same planes, with unchanged outer model edges and added
// feature wires. Generated face arrays follow donor footprint traversal order.
type LongEdgeChain struct {
	Solid                             trimmed.Solid
	Stock                             LongEdgeTreatment
	Bore                              trimmed.ExtrusionCircle
	Pocket                            trimmed.ExtrusionRectangle
	Boss                              trimmed.ExtrusionBoss
	PocketFaces, BoreFaces            [4]int
	BossFaces                         [5]int
	NominalVolume, NominalSurfaceArea float64
}

// TreatLongCapEdgesWithFeatures composes one Z-through circular bore, one
// rectangular Z-through pocket and one upper rectangular boss with the actual
// horizontal fillet/chamfer stock. Every footprint must lie strictly in the
// retained flat top strip, with eight tolerances of clearance. Exact represented
// footprint separation is re-proved by the donor constructor. No curved stock
// face is replaced, and no bounding-box substitute is used. Failure is atomic.
func TreatLongCapEdgesWithFeatures(ctx context.Context, length, width, height, radius, topSetback, sideSetback float64, bore trimmed.ExtrusionCircle, pocket trimmed.ExtrusionRectangle, boss trimmed.ExtrusionBoss, o Options) (LongEdgeChain, error) {
	fail := func(e error) (LongEdgeChain, error) { return LongEdgeChain{}, e }
	stock, e := TreatOppositeLongCapEdges(ctx, length, width, height, radius, topSetback, sideSetback, o)
	if e != nil {
		return fail(e)
	}
	if !boss.Upper || !finite(bore.Center.X) || !finite(bore.Center.Y) || !finite(bore.Radius) || bore.Radius <= 0 {
		return fail(ErrInvalid)
	}
	tol := o.Tolerance
	if tol == 0 {
		tol = 1e-9
	}
	pad := qm(num(8), num(tol))
	lowY, highY := qa(num(radius), pad), qs(qs(num(width), num(topSetback)), pad)
	check := func(x0, y0, x1, y1 q) bool {
		return x0.Cmp(pad) > 0 && x1.Cmp(qs(num(length), pad)) < 0 && y0.Cmp(lowY) > 0 && y1.Cmp(highY) < 0
	}
	if !check(qs(num(bore.Center.X), num(bore.Radius)), qs(num(bore.Center.Y), num(bore.Radius)), qa(num(bore.Center.X), num(bore.Radius)), qa(num(bore.Center.Y), num(bore.Radius))) {
		return fail(ErrRadius)
	}
	for _, r := range []trimmed.ExtrusionRectangle{pocket, boss.Footprint} {
		if !finite(r.Min.X) || !finite(r.Min.Y) || !finite(r.Max.X) || !finite(r.Max.Y) || !check(num(r.Min.X), num(r.Min.Y), num(r.Max.X), num(r.Max.Y)) {
			return fail(ErrRadius)
		}
	}
	frame, e := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		return fail(e)
	}
	donor, e := trimmed.ExtrudeMixedRectangleWithBoss(ctx, frame, trimmed.RoundedRectangle{Max: geom.Vec2{X: length, Y: width}}, []trimmed.ExtrusionRectangle{pocket}, []trimmed.ExtrusionCircle{bore}, nil, height, boss)
	if e != nil {
		return fail(e)
	}
	dv, de, df := donor.Vertices(), donor.Edges(), donor.Faces()
	if len(df) != 19 || len(df[0].Holes) != 2 || len(df[1].Holes) != 3 {
		return fail(ErrInvalid)
	}
	vertices, edges, faces := stock.Solid.Vertices(), stock.Solid.Edges(), stock.Solid.Faces()
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
	for cap, id := range []int{stock.BottomFace, stock.TopFace} {
		z := 0.0
		if cap == 1 {
			z = height
		}
		f := faces[id]
		var loop []trimmed.Use
		for _, u := range f.Outer {
			a, b := edges[u.Edge].Start, edges[u.Edge].End
			if u.Reversed {
				a, b = b, a
			}
			p, q := vertices[a], vertices[b]
			if p.Z != z || q.Z != z {
				return fail(ErrInvalid)
			}
			u.UVStart = geom.Vec2{X: p.X, Y: p.Y}
			u.UVEnd = geom.Vec2{X: q.X, Y: q.Y}
			u.Curve = nil
			loop = append(loop, u)
		}
		area := newRatQuo(num(0), num(1))
		for _, u := range loop {
			area.Add(area, qs(qm(num(u.UVStart.X), num(u.UVEnd.Y)), qm(num(u.UVStart.Y), num(u.UVEnd.X))))
		}
		if area.Sign() == 0 {
			return fail(ErrInvalid)
		}
		if area.Sign() < 0 {
			for i, j := 0, len(loop)-1; i < j; i, j = i+1, j-1 {
				loop[i], loop[j] = loop[j], loop[i]
			}
			for i := range loop {
				loop[i].Reversed = !loop[i].Reversed
				loop[i].UVStart, loop[i].UVEnd = loop[i].UVEnd, loop[i].UVStart
			}
		}
		chart, e := geom.NewFrame(geom.Vec3{Z: z}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
		if e != nil {
			return fail(e)
		}
		plane, e := geom.NewPlaneSurface(chart)
		if e != nil {
			return fail(e)
		}
		f.Surface = plane
		f.Reversed = cap == 0
		f.Outer = loop
		for _, hole := range df[cap].Holes {
			f.Holes = append(f.Holes, remap(hole))
		}
		faces[id] = f
	}
	r := LongEdgeChain{Stock: stock, Bore: bore, Pocket: pocket, Boss: boss}
	for i, f := range df[6:] {
		f.Outer = remap(f.Outer)
		id := len(faces)
		switch {
		case i < 4:
			r.PocketFaces[i] = id
		case i < 8:
			r.BoreFaces[i-4] = id
		default:
			r.BossFaces[i-8] = id
		}
		faces = append(faces, f)
	}
	r.Solid, e = trimmed.NewSolidContext(ctx, vertices, edges, faces, tol)
	if e != nil {
		return fail(e)
	}
	pw, ph := pocket.Max.X-pocket.Min.X, pocket.Max.Y-pocket.Min.Y
	bw, bh := boss.Footprint.Max.X-boss.Footprint.Min.X, boss.Footprint.Max.Y-boss.Footprint.Min.Y
	r.NominalVolume = stock.NominalVolume - math.Pi*bore.Radius*bore.Radius*height - pw*ph*height + bw*bh*boss.Height
	r.NominalSurfaceArea = stock.NominalSurfaceArea - 2*math.Pi*bore.Radius*bore.Radius + 2*math.Pi*bore.Radius*height - 2*pw*ph + 2*(pw+ph)*height + 2*(bw+bh)*boss.Height
	return r, nil
}
