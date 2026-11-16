package step

import (
	"context"
	"math/big"

	"plainsolid/exchange/internal/annularchamfer"
	"plainsolid/geom"
)

func proveAnnularChamferSTEPIncidence(ctx context.Context, f *File, faces []annularChamferSTEPFace,
	g annularchamfer.Graph) (faceIDs, edgeIDs, vertexIDs []int64, err error) {
	p := boreRimSTEPIncidence{
		graph: g.Graph, nativeByPoint: make(map[geom.Vec3]int, len(g.Vertices)), nativeByEnds: make(map[[2]int]int, len(g.Edges)),
		edgeIDs: make([]int64, len(g.Edges)), pointIDs: make([]int64, len(g.Vertices)),
	}
	for i, point := range g.Vertices {
		q, err := geom.RepresentedExactPoint(point)
		if err != nil {
			return nil, nil, nil, err
		}
		if _, ok := p.nativeByPoint[q]; ok {
			return nil, nil, nil, errEccentricAnnularChamferSTEP
		}
		p.nativeByPoint[q] = i
	}
	for i, e := range g.Edges {
		p.nativeByEnds[boreRimEdgePair(e.Start, e.End)] = i
	}
	uses := map[int64][]bool{}
	for _, face := range faces {
		for _, loop := range face.loops {
			for _, u := range loop {
				uses[u.id] = append(uses[u.id], u.sense)
			}
		}
	}
	if len(uses) != 14 {
		return nil, nil, nil, errEccentricAnnularChamferSTEP
	}
	known := make(map[int64][]analyticUse, len(uses))
	for id, senses := range uses {
		if len(senses) != 2 || senses[0] == senses[1] {
			return nil, nil, nil, errEccentricAnnularChamferSTEP
		}
		known[id], err = p.readAnnularChamferEdge(f, id)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	for _, id := range p.edgeIDs {
		if id == 0 {
			return nil, nil, nil, errEccentricAnnularChamferSTEP
		}
	}
	faceIDs = make([]int64, len(g.Faces))
	for _, source := range faces {
		if err := ctx.Err(); err != nil {
			return nil, nil, nil, err
		}
		match := -1
		for id, face := range g.Faces {
			b := g.Bands[id]
			if faceIDs[id] != 0 || !matchesAnnularChamferSTEPFace(source, b, face.Origin[2], face.Reversed) ||
				!matchBoreRimSTEPWires(source.boreRimSTEPFace, face, known) {
				continue
			}
			if match != -1 {
				return nil, nil, nil, errEccentricAnnularChamferSTEP
			}
			match = id
		}
		if match == -1 {
			return nil, nil, nil, errEccentricAnnularChamferSTEP
		}
		faceIDs[match] = source.id
	}
	return faceIDs, p.edgeIDs, p.pointIDs, nil
}

func matchesAnnularChamferSTEPFace(source annularChamferSTEPFace, band annularchamfer.Band, capZ *big.Rat, reversed bool) bool {
	if band.Patch == nil {
		return source.station[0].Radius == 0 && capZ.Cmp(sideRat(source.center.Z)) == 0 &&
			(source.sense == (source.axis.Z > 0)) != reversed
	}
	if source.station[0].Radius == 0 || source.sense == reversed || band.X.Cmp(sideRat(source.center.X)) != 0 ||
		band.Y.Cmp(sideRat(source.center.Y)) != 0 {
		return false
	}
	for i, v := range source.station {
		if band.Z[i].Cmp(sideRat(v.Z)) != 0 || band.R[i].Cmp(sideRat(v.Radius)) != 0 {
			return false
		}
	}
	return true
}

func (p *boreRimSTEPIncidence) readAnnularChamferEdge(f *File, id int64) ([]analyticUse, error) {
	r, err := simple(f, id, "EDGE_CURVE", 5)
	if err != nil {
		return nil, err
	}
	refs, err := namedReferences(r, 1, 2, 3)
	if err != nil {
		return nil, err
	}
	direction, err := boolean(r.Args[4])
	if err != nil {
		return nil, err
	}
	a, start, err := p.point(f, refs[0])
	if err != nil {
		return nil, err
	}
	b, end, err := p.point(f, refs[1])
	if err != nil {
		return nil, err
	}
	if a == b {
		return p.fullCircle(f, id, refs[2], a, direction)
	}
	i, exists := p.nativeByEnds[boreRimEdgePair(a, b)]
	if !exists || p.edgeIDs[i] != 0 {
		return nil, errEccentricAnnularChamferSTEP
	}
	e := p.graph.Edges[i]
	if e.Circle.Radius != nil {
		return nil, errEccentricAnnularChamferSTEP
	}
	poles, err := annularChamferSTEPLineSegment(f, refs[2])
	if !direction {
		poles[0], poles[1] = poles[1], poles[0]
	}
	if err != nil || poles != [2]geom.Vec3{start, end} {
		return nil, errEccentricAnnularChamferSTEP
	}
	p.edgeIDs[i] = id
	return []analyticUse{{id: int64(i), sense: e.Start == a}}, nil
}
