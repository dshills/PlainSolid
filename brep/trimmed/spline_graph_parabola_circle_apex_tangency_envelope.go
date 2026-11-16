package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// ParabolaCircleApexEnvelopeHistory identifies the native tangent vertex,
// four original rays and one interior bridge on the enclosing roof.
// Rays index parabola and circle arcs in negative, positive free order.
// The enclosing source region uses its original index and index 3.
type ParabolaCircleApexEnvelopeHistory struct {
	Base       int
	RoofFaces  [4]int
	RoofOwners [4]int
	Circle     [4]int
	Bridge     int
	Vertex     int
	Rays       [2][2]int
	WallFaces  [7]int
}

// ParabolaCircleApexTangencyEnvelope is a closed single shell supported by
// the original roofs and a source-proved partition at the tangent vertex.
type ParabolaCircleApexTangencyEnvelope struct {
	solid     Solid
	history   ParabolaCircleApexEnvelopeHistory
	sources   [3]*geom.BSplineSurface
	axis      int
	op        parametric.BooleanOp
	tolerance float64
}

func (r *ParabolaCircleApexTangencyEnvelope) Boundary() Solid {
	if r == nil {
		return Solid{}
	}
	return r.solid
}

func (r *ParabolaCircleApexTangencyEnvelope) History() ParabolaCircleApexEnvelopeHistory {
	if r == nil {
		return ParabolaCircleApexEnvelopeHistory{}
	}
	return r.history
}

// CombineParabolaCircleApexTangencySplineGraphs builds Common or Fuse on
// the original shared-base stock. A seam from the far circle extremum to
// the stock wall splits the enclosing roof into two simple cusp-ended
// loops. This leaves an embedded graph with one cyclic link at the apex.
func CombineParabolaCircleApexTangencySplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface, axis int,
	op parametric.BooleanOp, tolerance float64, o ParabolaCircleGraphOptions) (*ParabolaCircleApexTangencyEnvelope, error) {
	if op != parametric.Intersection && op != parametric.Union {
		return nil, geom.ErrInvalid
	}
	src := [3]*geom.BSplineSurface{base, a, b}
	partition, err := proveParabolaCircleApexPartition(ctx, src, axis, tolerance, o)
	if err != nil {
		return nil, err
	}
	regions, bridge, err := parabolaCircleApexEnvelopeRegions(partition)
	if err != nil {
		return nil, err
	}
	s, assembly, err := combineOpenGraphRegions(ctx, base, a, b, regions[:], op, tolerance, exactGraphVertexEvaluator(ctx, o.Proof))
	if err != nil {
		return nil, err
	}
	h, err := bindParabolaCircleTangentEnvelope(ctx, s, assembly, partition.tangentSection(), regions, bridge, src, op, o.Proof)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &ParabolaCircleApexTangencyEnvelope{solid: s, history: h, sources: src, axis: axis, op: op, tolerance: tolerance}, nil
}

func parabolaCircleApexEnvelopeRegions(p parabolaCircleApexPartition) (regions [4]openGraphRegion, bridge openSectionPiece, err error) {
	copy(regions[:], p.regions[:])
	side, axis := p.proof.DiskRegion, p.proof.FreeAxis
	apex, err := exactGraphPartitionUV(p.proof.Parameters)
	if err != nil {
		return regions, bridge, err
	}
	farUV := p.proof.CircleCenter
	farUV[1-axis] = new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(2, 1), farUV[1-axis]), p.proof.Parameters[1-axis])
	far, err := exactGraphPartitionUV(farUV)
	if err != nil {
		return regions, bridge, err
	}
	stockUV := farUV
	stockUV[1-axis] = big.NewRat(int64(side), 1)
	stock, err := exactGraphPartitionUV(stockUV)
	if err != nil {
		return regions, bridge, err
	}
	return splitParabolaCircleTangentEnvelopeRegions(p.regions, axis, side, apex, far, stock)
}

func splitParabolaCircleTangentEnvelopeRegions(original [3]openGraphRegion, axis, side int, apex, far, stock geom.Vec2) (
	regions [4]openGraphRegion, bridge openSectionPiece, err error) {
	copy(regions[:], original[:])
	bridge = openSectionPiece{a: far, b: stock, side: -1}
	rev := reverseParabolaCircleCrossingPiece

	// The source proves the bridge lies on the outward normal beyond
	// the far extremum and stays strictly in the enclosing region.
	// Its two ends are the only bridge contacts with the boundaries.
	var split []openSectionPiece
	splits := 0
	for _, piece := range original[side].loop {
		a, b := [2]float64{piece.a.X, piece.a.Y}, [2]float64{piece.b.X, piece.b.Y}
		v := [2]float64{stock.X, stock.Y}
		if piece.side >= 0 && a[1-axis] == v[1-axis] && b[1-axis] == v[1-axis] &&
			min(a[axis], b[axis]) < v[axis] && v[axis] < max(a[axis], b[axis]) {
			split = append(split, openSectionPiece{a: piece.a, b: stock, side: piece.side},
				openSectionPiece{a: stock, b: piece.b, side: piece.side})
			splits++
		} else {
			split = append(split, piece)
		}
	}
	if splits != 1 {
		return regions, bridge, geom.ErrConvergence
	}
	outside, err := splitParabolaCircleApexLoop(split, apex, stock)
	if err != nil {
		return regions, bridge, err
	}
	inner := make([]openSectionPiece, len(original[2].loop))
	for i, arc := range original[2].loop {
		inner[len(inner)-1-i] = rev(arc)
	}
	circle, err := splitParabolaCircleApexLoop(inner, apex, far)
	if err != nil {
		return regions, bridge, err
	}
	for i, loop := range [2][]openSectionPiece{
		slices.Concat(outside[0], []openSectionPiece{rev(bridge)}, circle[1]),
		slices.Concat(outside[1], circle[0], []openSectionPiece{bridge}),
	} {
		region := [2]int{side, 3}[i]
		starts := make(map[geom.Vec2]bool, len(loop))
		for j, piece := range loop {
			if starts[piece.a] || piece.b != loop[(j+1)%len(loop)].a {
				return regions, bridge, geom.ErrConvergence
			}
			starts[piece.a] = true
		}
		regions[region] = openGraphRegion{loop: loop, sign: original[side].sign}
	}
	return regions, bridge, nil
}

func splitParabolaCircleApexLoop(loop []openSectionPiece, from, to geom.Vec2) ([2][]openSectionPiece, error) {
	start := slices.IndexFunc(loop, func(piece openSectionPiece) bool { return piece.a == from })
	if start < 0 {
		return [2][]openSectionPiece{}, geom.ErrConvergence
	}
	rotated := slices.Concat(loop[start:], loop[:start])
	end := slices.IndexFunc(rotated, func(piece openSectionPiece) bool { return piece.a == to })
	if end < 1 {
		return [2][]openSectionPiece{}, geom.ErrConvergence
	}
	return [2][]openSectionPiece{rotated[:end], rotated[end:]}, nil
}

func bindParabolaCircleTangentEnvelope(ctx context.Context, s Solid, assembly SplineGraphBooleanHistory, sec parabolaCircleTangentSection,
	regions [4]openGraphRegion, bridge openSectionPiece, src [3]*geom.BSplineSurface, op parametric.BooleanOp,
	opts geom.GraphPointContactOptions) (h ParabolaCircleApexEnvelopeHistory, err error) {
	topo := s.Topology()
	if ([3]int{topo.Faces, topo.Edges, topo.Vertices}) != ([3]int{12, 28, 18}) || len(topo.Shells) != 1 ||
		topo.EulerCharacteristic != 2 || topo.InnerWires != 0 || topo.CollapsedEdges != 0 ||
		assembly.Base != 4 || len(assembly.WallFaces) != 7 || len(assembly.ContourEdges) != 7 {
		return h, geom.ErrConvergence
	}
	h.Base = assembly.Base
	base := s.faces[h.Base]
	if base.Surface != src[0] || !base.Reversed {
		return h, geom.ErrConvergence
	}
	copy(h.WallFaces[:], assembly.WallFaces)
	side := sec.diskRegion
	for i, r := range regions {
		owner := 0
		if (r.sign > 0) != (op == parametric.Union) {
			owner = 1
		}
		h.RoofFaces[i], h.RoofOwners[i] = i, owner
		roof, face := src[1+owner], s.faces[i]
		if face.Surface != roof || face.Reversed || len(face.Holes) != 0 || len(face.Outer) != len(r.loop) {
			return h, geom.ErrConvergence
		}
		for j, piece := range r.loop {
			use := face.Outer[j]
			if use.UVStart != piece.a || use.UVEnd != piece.b || !sameParabolaCircleCrossingTrim(use.Curve, piece.c) {
				return h, geom.ErrConvergence
			}
		}
	}
	// Stock and standalone disk loops use the ordinary trim gate. The
	// two cusp-ended loops use the bounded source proof above. The
	// complete factors, bridge and oriented coedge identities prove
	// non-overlap and a simple boundary for each of those two disks.
	if err := validateGraphEnvelopeSplineTrims(ctx, []Face{s.faces[1-side], s.faces[2], base}); err != nil {
		return h, err
	}
	for _, id := range h.WallFaces {
		wall := s.faces[id]
		if len(wall.Outer) != 4 {
			return h, geom.ErrConvergence
		}
		bottom, hasBase := graphWallSourceTrim(wall.Outer[0].Curve, false, false)
		top, hasRoof := graphWallSourceTrim(wall.Outer[2].Curve, true, false)
		if !hasBase || !hasRoof || bottom.surface != src[0] || bottom.a != top.a || bottom.b != top.b {
			return h, geom.ErrConvergence
		}
		matched := 0
		for region, r := range regions {
			for _, piece := range r.loop {
				if piece.side >= 0 && top.a == piece.a && top.b == piece.b && top.surface == src[1+h.RoofOwners[region]] {
					matched++
				}
			}
		}
		if matched != 1 {
			return h, geom.ErrConvergence
		}
	}
	ids, err := bindParabolaCircleCrossingEdges(s, assembly.ContourEdges, append(slices.Clone(sec.regions[2].loop), bridge))
	if err != nil {
		return h, err
	}
	copy(h.Circle[:], ids[:4])
	h.Bridge = ids[4]
	if h.RoofOwners[side] != h.RoofOwners[3] ||
		!opposedGraphContourUses(s.faces[side].Outer, s.faces[3].Outer, h.Bridge) ||
		s.edges[h.Bridge].Curve.(surfaceTrimCurve).surface != src[1+h.RoofOwners[side]] {
		return h, geom.ErrConvergence
	}
	h.Vertex, err = exactParabolaCircleCrossingVertex(s, sec.point)
	if err != nil {
		return h, err
	}
	adjacent, rays := [2][2]int{{-1, -1}, {-1, -1}}, [2][2]int{{-1, -1}, {-1, -1}}
	incident := 0
	for id, edge := range s.edges {
		if edge.Start != h.Vertex && edge.End != h.Vertex {
			continue
		}
		incident++
		branch, regular, second := 0, 1-side, sec.second[0]
		if slices.Contains(h.Circle[:], id) {
			branch, regular, second = 1, 2, sec.second[1]
		}
		direction, err := parabolaCircleTangentRay(s, id, h.Vertex, sec.axis, sec.slope, second)
		if err != nil {
			return h, err
		}
		neighbors := nodalCubicIncidentFaces(s, id)
		if len(neighbors) != 2 || !slices.Contains(neighbors, regular) {
			return h, geom.ErrConvergence
		}
		other := neighbors[0] + neighbors[1] - regular
		j := (direction + 1) / 2
		if rays[branch][j] >= 0 || (other != side && other != 3) ||
			!opposedGraphContourUses(s.faces[regular].Outer, s.faces[other].Outer, id) {
			return h, geom.ErrConvergence
		}
		model := s.edges[id].Curve.(surfaceTrimCurve)
		bound := false
		for _, face := range [2]Face{s.faces[regular], s.faces[other]} {
			bound = bound || (face.Surface == model.surface && slices.ContainsFunc(face.Outer, func(use Use) bool {
				return use.Edge == id && !use.Reversed && use.UVStart == model.a && use.UVEnd == model.b &&
					sameParabolaCircleCrossingTrim(use.Curve, model.curve)
			}))
		}
		if !bound {
			return h, geom.ErrConvergence
		}
		adjacent[branch][j], rays[branch][j] = other, id
	}
	if incident != 4 || adjacent[0] != adjacent[1] || adjacent[0][0] == adjacent[0][1] {
		return h, fmt.Errorf("trimmed: parabola-circle apex has no single four-sector link: %w", geom.ErrConvergence)
	}
	h.Rays = rays
	if err := affineMaterialVertexLinks(ctx, s); err != nil {
		return h, err
	}
	return h, validateExactGraphPartitionEdges(ctx, s, opts)
}
