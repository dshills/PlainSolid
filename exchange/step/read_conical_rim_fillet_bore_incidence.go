package step

import (
	"context"

	"plainsolid/exchange/internal/borefillet"
	"plainsolid/geom"
)

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
