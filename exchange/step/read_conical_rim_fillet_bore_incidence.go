package step

import (
	"context"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/internal/borefillet"
	"plainsolid/geom"
)

func bindConicalRimFilletBoreFaces(source conicalRimFilletBoreFaces,
	result trimmed.WrappedFilletedFrustumBoreSet) (borefillet.Graph, []boreRimSTEPFace, error) {
	fail := func() (borefillet.Graph, []boreRimSTEPFace, error) {
		return borefillet.Graph{}, nil, errConicalRimFilletBoreSTEP
	}
	s, h := result.Solid, result.Fillets
	vertices, edges, faces := s.Vertices(), s.Edges(), s.Faces()
	if len(faces) != 6 || len(edges) != 28 || len(vertices) != 24 || len(result.Cuts) != 1 {
		return fail()
	}
	g := borefillet.Graph{Vertices: make([][3]*big.Rat, len(vertices)),
		Edges: make([]borefillet.Edge, len(edges)), Faces: make([]borefillet.Face, len(faces))}
	for i, v := range vertices {
		g.Vertices[i] = borefillet.Vec(v.X, v.Y, v.Z)
	}
	for i, e := range edges {
		g.Edges[i] = borefillet.Edge{Start: e.Start, End: e.End, Forward: true}
	}
	for i, f := range faces {
		g.Faces[i] = borefillet.Face{Reversed: f.Reversed, Outer: f.Outer, Holes: f.Holes}
	}
	owners := make([]boreRimSTEPFace, len(faces))
	owners[h.Lateral], owners[result.Cuts[0].BoreFace] = source.cone.boreRimSTEPFace, source.bore
	stock := borefillet.Vec(source.cone.center.X, source.cone.center.Y, 0)
	bore := borefillet.Vec(source.bore.center.X, source.bore.center.Y, 0)
	ring := func(ids []int, center [3]*big.Rat, r *big.Rat) bool {
		if len(ids) != 4 {
			return false
		}
		for _, id := range ids {
			g.Edges[id].Circle = borefillet.Circle{Center: center, Radius: r}
			if !conicalRimFilletBoreEdgeSense(&g, id, true) {
				return false
			}
		}
		return true
	}
	for end, c := range h.Circles {
		t := source.toroid[end]
		cap, finite := source.caps[end], source.cone.station[end]
		if t.center.X != source.cone.center.X || t.center.Y != source.cone.center.Y ||
			c.Center[0].Cmp(sideRat(t.major)) != 0 || c.Center[1].Cmp(sideRat(t.center.Z)) != 0 ||
			c.Radius.Cmp(sideRat(t.minor)) != 0 || c.Tangency[0].Cmp(sideRat(finite.Radius)) != 0 ||
			c.Tangency[1].Cmp(sideRat(finite.Z)) != 0 {
			return fail()
		}
		owners[h.Caps[end]], owners[h.Blends[end]] = cap, t
		capZ, tanZ := sideRat(cap.center.Z), c.Tangency[1]
		if !ring(h.Rims[end][0][:], [3]*big.Rat{stock[0], stock[1], capZ}, c.Center[0]) ||
			!ring(h.Rims[end][1][:], [3]*big.Rat{stock[0], stock[1], tanZ}, c.Tangency[0]) ||
			!ring(result.Cuts[0].BoreRims[end], [3]*big.Rat{bore[0], bore[1], capZ}, sideRat(source.bore.major)) {
			return fail()
		}
		// The checked native rim constructor owns exactly one directed
		// meridian edge on both sides of each toroidal seam. Its span
		// is less than a semicircle, even at the obtuse upper fillet.
		seam := -1
		for _, u := range faces[h.Blends[end]].Outer {
			if g.Edges[u.Edge].Circle.Radius != nil {
				continue
			}
			if seam >= 0 && seam != u.Edge {
				return fail()
			}
			seam = u.Edge
		}
		if seam < 0 {
			return fail()
		}
		centerX := new(big.Rat).Add(stock[0], c.Center[0])
		g.Edges[seam].Circle = borefillet.Circle{
			Center: [3]*big.Rat{centerX, stock[1], c.Center[1]}, Radius: c.Radius, Meridian: true,
		}
		if !conicalRimFilletBoreEdgeSense(&g, seam, false) {
			return fail()
		}
	}
	return g, owners, nil
}

func conicalRimFilletBoreEdgeSense(g *borefillet.Graph, edge int, quarter bool) bool {
	e := &g.Edges[edge]
	dim, fixed := 1, 2
	if e.Circle.Meridian {
		dim, fixed = 2, 1
	}
	var xy [2][2]*big.Rat
	for i, p := range [2][3]*big.Rat{g.Vertices[e.Start], g.Vertices[e.End]} {
		if p[fixed].Cmp(e.Circle.Center[fixed]) != 0 {
			return false
		}
		x := new(big.Rat).Sub(p[0], e.Circle.Center[0])
		y := new(big.Rat).Sub(p[dim], e.Circle.Center[dim])
		r2 := new(big.Rat).Mul(e.Circle.Radius, e.Circle.Radius)
		if new(big.Rat).Add(new(big.Rat).Mul(x, x), new(big.Rat).Mul(y, y)).Cmp(r2) != 0 ||
			e.Circle.Meridian && x.Sign() < 0 {
			return false
		}
		xy[i] = [2]*big.Rat{x, y}
	}
	a, b := xy[0], xy[1]
	if quarter && new(big.Rat).Add(new(big.Rat).Mul(a[0], b[0]), new(big.Rat).Mul(a[1], b[1])).Sign() != 0 {
		return false
	}
	cross := new(big.Rat).Mul(a[0], b[1]).Cmp(new(big.Rat).Mul(a[1], b[0]))
	e.Forward = cross > 0
	return cross != 0
}

func proveConicalRimFilletBoreSTEP(ctx context.Context, f *File, g borefillet.Graph, owners []boreRimSTEPFace) ([]int64, []int64, []int64, error) {
	fail := func(err error) ([]int64, []int64, []int64, error) { return nil, nil, nil, err }
	p := boreRimSTEPIncidence{
		graph: g, nativeByPoint: make(map[geom.Vec3]int, len(g.Vertices)), nativeByEnds: make(map[[2]int]int, len(g.Edges)),
		edgeIDs: make([]int64, len(g.Edges)), pointIDs: make([]int64, len(g.Vertices)),
	}
	for i, v := range g.Vertices {
		point, err := geom.RepresentedExactPoint(v)
		if err != nil {
			return fail(err)
		}
		if _, used := p.nativeByPoint[point]; used {
			return fail(errConicalRimFilletBoreSTEP)
		}
		p.nativeByPoint[point] = i
	}
	for i, e := range g.Edges {
		p.nativeByEnds[boreRimEdgePair(e.Start, e.End)] = i
	}
	directions := map[int64][]bool{}
	for _, owner := range owners {
		for _, loop := range owner.loops {
			for _, use := range loop {
				directions[use.id] = append(directions[use.id], use.sense)
			}
		}
	}
	if len(directions) != 10 {
		return fail(errConicalRimFilletBoreSTEP)
	}
	known := make(map[int64][]analyticUse, len(directions))
	for id, senses := range directions {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		if len(senses) != 2 || senses[0] == senses[1] {
			return fail(errConicalRimFilletBoreSTEP)
		}
		r, err := simple(f, id, "EDGE_CURVE", 5)
		if err != nil {
			return fail(err)
		}
		shapeID, err := ref(r.Args[3])
		if err != nil || len(f.Entities[shapeID].Records) != 1 {
			return fail(errConicalRimFilletBoreSTEP)
		}
		if f.Entities[shapeID].Records[0].Type == "B_SPLINE_CURVE_WITH_KNOTS" {
			known[id], err = p.readAnnularChamferEdge(f, id)
		} else {
			known[id], err = p.readEdge(f, id)
		}
		if err != nil {
			return fail(err)
		}
	}
	for _, id := range p.edgeIDs {
		if id == 0 {
			return fail(errConicalRimFilletBoreSTEP)
		}
	}
	ids := make([]int64, len(owners))
	for i, owner := range owners {
		if !matchBoreRimSTEPWires(owner, g.Faces[i], known) {
			return fail(errConicalRimFilletBoreSTEP)
		}
		ids[i] = owner.id
	}
	return ids, p.edgeIDs, p.pointIDs, nil
}
