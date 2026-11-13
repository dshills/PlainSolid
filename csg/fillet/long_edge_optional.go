package fillet

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// TreatLongCapEdgesOptional builds bounded incremental edge treatments and optional
// disjoint through bore, through rectangular pocket, and upper rectangular boss.
// A zero radius disables the fillet; paired zero setbacks disable the chamfer.
// bottomFrontChamfer selects the lower front edge instead of the upper back edge.
// Missing generated face IDs are -1. Existing stock faces retain their IDs.
func TreatLongCapEdgesOptional(ctx context.Context, length, width, height, radius, topSetback, sideSetback float64, bottomFrontChamfer bool, bore *trimmed.ExtrusionCircle, pocket *trimmed.ExtrusionRectangle, boss *trimmed.ExtrusionBoss, o Options) (LongEdgeChain, error) {
	fail := func(e error) (LongEdgeChain, error) { return LongEdgeChain{}, e }
	stock, e := treatOptionalStock(ctx, length, width, height, radius, topSetback, sideSetback, bottomFrontChamfer, o)
	if e != nil {
		return fail(e)
	}
	low, high := radius, width-topSetback
	if bottomFrontChamfer {
		low = max(radius, topSetback)
		high = width
	}
	return appendLongEdgeFeatures(ctx, stock, low, high, bore, pocket, boss, o)
}

func appendLongEdgeFeatures(ctx context.Context, stock LongEdgeTreatment, low, high float64, bore *trimmed.ExtrusionCircle, pocket *trimmed.ExtrusionRectangle, boss *trimmed.ExtrusionBoss, o Options) (LongEdgeChain, error) {
	fail := func(e error) (LongEdgeChain, error) { return LongEdgeChain{}, e }
	length, width, height := stock.Length, stock.Width, stock.Height
	if bore == nil && pocket == nil && boss == nil {
		return LongEdgeChain{Solid: stock.Solid, Stock: stock, PocketFaces: [4]int{-1, -1, -1, -1}, BoreFaces: [4]int{-1, -1, -1, -1}, BossFaces: [5]int{-1, -1, -1, -1, -1}, NominalVolume: stock.NominalVolume, NominalSurfaceArea: stock.NominalSurfaceArea}, nil
	}
	var e error
	if (boss != nil && !boss.Upper) || (bore != nil && (!finite(bore.Center.X) || !finite(bore.Center.Y) || !finite(bore.Radius) || bore.Radius <= 0)) {
		return fail(ErrInvalid)
	}
	tol := o.Tolerance
	if tol == 0 {
		tol = 1e-9
	}
	pad := qm(num(8), num(tol))
	lowY, highY := qa(num(low), pad), qs(num(high), pad)
	check := func(x0, y0, x1, y1 q) bool {
		return x0.Cmp(pad) > 0 && x1.Cmp(qs(num(length), pad)) < 0 && y0.Cmp(lowY) > 0 && y1.Cmp(highY) < 0
	}
	if bore != nil && !check(qs(num(bore.Center.X), num(bore.Radius)), qs(num(bore.Center.Y), num(bore.Radius)), qa(num(bore.Center.X), num(bore.Radius)), qa(num(bore.Center.Y), num(bore.Radius))) {
		return fail(ErrRadius)
	}
	var rects []trimmed.ExtrusionRectangle
	if pocket != nil {
		rects = append(rects, *pocket)
	}
	if boss != nil {
		rects = append(rects, boss.Footprint)
	}
	for _, r := range rects {
		if !finite(r.Min.X) || !finite(r.Min.Y) || !finite(r.Max.X) || !finite(r.Max.Y) || !check(num(r.Min.X), num(r.Min.Y), num(r.Max.X), num(r.Max.Y)) {
			return fail(ErrRadius)
		}
	}
	frame, e := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		return fail(e)
	}
	var pockets []trimmed.ExtrusionRectangle
	var bores []trimmed.ExtrusionCircle
	if pocket != nil {
		pockets = append(pockets, *pocket)
	}
	if bore != nil {
		bores = append(bores, *bore)
	}
	var donor trimmed.Solid
	profile := trimmed.RoundedRectangle{Max: geom.Vec2{X: length, Y: width}}
	if boss != nil {
		donor, e = trimmed.ExtrudeMixedRectangleWithBoss(ctx, frame, profile, pockets, bores, nil, height, *boss)
	} else {
		donor, e = trimmed.ExtrudeMixedRectangle(ctx, frame, profile, pockets, bores, nil, height)
	}
	if e != nil {
		return fail(e)
	}
	dv, de, df := donor.Vertices(), donor.Edges(), donor.Faces()
	expected := 6 + 4*len(pockets) + 4*len(bores)
	upperHoles := len(pockets) + len(bores)
	if boss != nil {
		expected += 5
		upperHoles++
	}
	if len(df) != expected || len(df[0].Holes) != len(pockets)+len(bores) || len(df[1].Holes) != upperHoles {
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
			base, _ := parametric.CurveMapping(edges[u.Edge].Curve)
			if _, ok := base.(geom.Line); !ok {
				return fail(ErrInvalid)
			}
			if math.Abs(p.Z-z) > tol || math.Abs(q.Z-z) > tol {
				return fail(ErrInvalid)
			}
			u.UVStart = geom.Vec2{X: p.X, Y: p.Y}
			u.UVEnd = geom.Vec2{X: q.X, Y: q.Y}
			u.Curve = nil
			loop = append(loop, u)
		}
		area := num(0)
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
	r := LongEdgeChain{Stock: stock, PocketFaces: [4]int{-1, -1, -1, -1}, BoreFaces: [4]int{-1, -1, -1, -1}, BossFaces: [5]int{-1, -1, -1, -1, -1}}
	if bore != nil {
		r.Bore = *bore
	}
	if pocket != nil {
		r.Pocket = *pocket
	}
	if boss != nil {
		r.Boss = *boss
	}
	np, nb := 0, 0
	if pocket != nil {
		np = 4
	}
	if bore != nil {
		nb = 4
	}
	for i, f := range df[6:] {
		f.Outer = remap(f.Outer)
		id := len(faces)
		switch {
		case i < np:
			r.PocketFaces[i] = id
		case i < np+nb:
			r.BoreFaces[i-np] = id
		default:
			r.BossFaces[i-np-nb] = id
		}
		faces = append(faces, f)
	}
	r.Solid, e = trimmed.NewSolidContext(ctx, vertices, edges, faces, tol)
	if e != nil {
		return fail(e)
	}
	r.NominalVolume = stock.NominalVolume
	r.NominalSurfaceArea = stock.NominalSurfaceArea
	if bore != nil {
		area := math.Pi * bore.Radius * bore.Radius
		r.NominalVolume -= area * height
		r.NominalSurfaceArea += 2*math.Pi*bore.Radius*height - 2*area
	}
	if pocket != nil {
		w, h := pocket.Max.X-pocket.Min.X, pocket.Max.Y-pocket.Min.Y
		r.NominalVolume -= w * h * height
		r.NominalSurfaceArea += 2*(w+h)*height - 2*w*h
	}
	if boss != nil {
		w, h := boss.Footprint.Max.X-boss.Footprint.Min.X, boss.Footprint.Max.Y-boss.Footprint.Min.Y
		r.NominalVolume += w * h * boss.Height
		r.NominalSurfaceArea += 2 * (w + h) * boss.Height
	}
	if !finite(r.NominalVolume) || !finite(r.NominalSurfaceArea) || r.NominalVolume <= 0 || r.NominalSurfaceArea <= 0 {
		return fail(ErrInvalid)
	}
	return r, nil
}

// treatOptionalStock retains a convex YZ profile swept along X. Zero radius
// disables the top/front fillet; paired zero setbacks disable the chamfer.
func treatOptionalStock(ctx context.Context, length, width, height, radius, topSetback, sideSetback float64, bottomFront bool, o Options) (LongEdgeTreatment, error) {
	fail := func(e error) (LongEdgeTreatment, error) { return LongEdgeTreatment{}, e }
	if ctx == nil || !finite(length) || !finite(width) || !finite(height) || !finite(radius) || !finite(topSetback) || !finite(sideSetback) || length <= 0 || width <= 0 || height <= 0 || radius < 0 || topSetback < 0 || sideSetback < 0 || (topSetback == 0) != (sideSetback == 0) {
		return fail(ErrInvalid)
	}
	tol := o.Tolerance
	if tol == 0 {
		tol = 1e-9
	}
	if !finite(tol) || tol <= 0 {
		return fail(ErrInvalid)
	}
	pad := qm(num(8), num(tol))
	rem := []q{num(length), qs(num(width), num(radius)), qs(num(height), num(radius))}
	if topSetback > 0 {
		rem = append(rem, num(topSetback), num(sideSetback), qs(num(width), num(topSetback)), qs(num(height), num(sideSetback)))
		if bottomFront {
			rem = append(rem, qs(qs(num(height), num(radius)), num(sideSetback)))
		} else {
			rem = append(rem, qs(qs(num(width), num(radius)), num(topSetback)))
		}
	}
	for _, v := range rem {
		if v.Cmp(pad) <= 0 {
			return fail(ErrRadius)
		}
	}
	var p []geom.Vec2
	bottom, back, top, front, chamfer, corner := 0, 1, 2, 3, -1, 3
	if topSetback == 0 {
		p = []geom.Vec2{{}, {X: width}, {X: width, Y: height}, {Y: height}}
	} else if bottomFront {
		p = []geom.Vec2{{X: topSetback}, {X: width}, {X: width, Y: height}, {Y: height}, {Y: sideSetback}}
		chamfer = 4
	} else {
		y, e := geom.RepresentedExactScalar(qs(num(width), num(topSetback)))
		if e != nil {
			return fail(e)
		}
		z, e := geom.RepresentedExactScalar(qs(num(height), num(sideSetback)))
		if e != nil {
			return fail(e)
		}
		p = []geom.Vec2{{}, {X: width}, {X: width, Y: z}, {X: y, Y: height}, {Y: height}}
		top, front, chamfer, corner = 3, 4, 2, 4
	}
	var selection []CornerRadius
	if radius > 0 {
		if _, e := geom.RepresentedExactScalar(qs(num(height), num(radius))); e != nil {
			return fail(e)
		}
		selection = []CornerRadius{{Corner: corner, Radius: radius}}
	}
	local, e := prismCorners(ctx, p, 0, length, selection, o)
	if e != nil {
		return fail(e)
	}
	world, e := local.Solid.Transformed(geom.Affine{{0, 0, 1, 0}, {1, 0, 0, 0}, {0, 1, 0, 0}})
	if e != nil {
		return fail(e)
	}
	r := LongEdgeTreatment{Solid: world, Profile: local, Length: length, Width: width, Height: height, Radius: radius, TopSetback: topSetback, SideSetback: sideSetback, EndFaces: local.Caps, BottomFace: local.StraightFaces[bottom], BackFace: local.StraightFaces[back], TopFace: local.StraightFaces[top], FrontFace: local.StraightFaces[front], FilletFace: -1, ChamferFace: -1}
	r.CornerFilletFaces = [4]int{-1, -1, -1, -1}
	r.CornerChamferFaces = [4]int{-1, -1, -1, -1}
	r.TopMinY, r.TopMaxY, r.BottomMinY, r.BottomMaxY = radius, width, 0, width
	if bottomFront {
		r.BottomMinY = topSetback
	} else {
		r.TopMaxY = width - topSetback
	}
	if radius > 0 {
		r.FilletFace = local.Corners[corner].CylinderFace
		r.CornerFilletFaces[3] = r.FilletFace
	}
	if chamfer >= 0 {
		r.ChamferFace = local.StraightFaces[chamfer]
		if bottomFront {
			r.CornerChamferFaces[0] = r.ChamferFace
		} else {
			r.CornerChamferFaces[2] = r.ChamferFace
		}
	}
	r.NominalVolume = length * (width*height - (1-math.Pi/4)*radius*radius - topSetback*sideSetback/2)
	per := 2*(width+height) - 2*radius + math.Pi*radius/2 - topSetback - sideSetback + math.Hypot(topSetback, sideSetback)
	r.NominalSurfaceArea = 2*r.NominalVolume/length + length*per
	if !finite(r.NominalVolume) || !finite(r.NominalSurfaceArea) || r.NominalVolume <= 0 || r.NominalSurfaceArea <= 0 {
		return fail(ErrInvalid)
	}
	return r, nil
}
