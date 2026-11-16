package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// ParabolaCircleCrossingsJunction binds one exact source node to the native
// vertex, with the four edge and face IDs in original counterclockwise order.
type ParabolaCircleCrossingsJunction struct {
	Vertex  int
	Rays    [4]int
	Sectors [4]int
}

// ParabolaCircleCrossingsHistory orders roof faces by the four signed source
// regions, parabola edges by increasing free parameter, and circle edges from
// the larger free extremum via the upper arc in the free/dependent chart.
type ParabolaCircleCrossingsHistory struct {
	Base          int
	RoofFaces     [4]int
	RoofOwners    [4]int
	ParabolaEdges [3]int
	CircleEdges   [4]int
	WallFaces     [6]int
	Junctions     [2]ParabolaCircleCrossingsJunction
}

// ParabolaCircleCrossingsEnvelope is an exact Common or Fuse with the four
// original signed sectors retained on their original roof supports.
type ParabolaCircleCrossingsEnvelope struct {
	solid     Solid
	history   ParabolaCircleCrossingsHistory
	sources   [3]*geom.BSplineSurface
	axis      int
	op        parametric.BooleanOp
	tolerance float64
}

func (r *ParabolaCircleCrossingsEnvelope) Boundary() Solid {
	if r == nil {
		return Solid{}
	}
	return r.solid
}

func (r *ParabolaCircleCrossingsEnvelope) History() ParabolaCircleCrossingsHistory {
	if r == nil {
		return ParabolaCircleCrossingsHistory{}
	}
	return r.history
}

// CombineParabolaCircleCrossingsSplineGraphs builds one original-source graph
// shell for Common or Fuse with two transverse parabola/circle junctions.
func CombineParabolaCircleCrossingsSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface,
	axis int, op parametric.BooleanOp, tolerance float64, o ParabolaCircleGraphOptions) (*ParabolaCircleCrossingsEnvelope, error) {
	if op != parametric.Intersection && op != parametric.Union {
		return nil, geom.ErrInvalid
	}
	sources := [3]*geom.BSplineSurface{base, a, b}
	source, err := proveParabolaCircleCrossingsPartition(ctx, sources, axis, tolerance, o)
	if err != nil {
		return nil, err
	}
	solid, assembly, err := combineOpenGraphRegions(ctx, base, a, b, source.regions[:], op, tolerance, exactGraphVertexEvaluator(ctx, o.Proof))
	if err != nil {
		return nil, err
	}
	history, err := bindParabolaCircleCrossingsEnvelope(ctx, solid, assembly, source, sources, op, o.Proof)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &ParabolaCircleCrossingsEnvelope{solid: solid, history: history, sources: sources, axis: axis, op: op, tolerance: tolerance}, nil
}

type parabolaCircleCrossingsPartition struct {
	proof   *geom.SplineGraphParabolaCircleCrossings
	pieces  [7]openSectionPiece
	regions [4]openGraphRegion
}

func proveParabolaCircleCrossingsPartition(ctx context.Context, sources [3]*geom.BSplineSurface, axis int, tolerance float64,
	o ParabolaCircleGraphOptions) (p parabolaCircleCrossingsPartition, err error) {
	if ctx == nil || sources[0] == nil || sources[1] == nil || sources[2] == nil || axis < 0 || axis > 1 ||
		tolerance <= 0 || !finite(tolerance) || o.Stock.Domain != (geom.ParameterRectangle{}) {
		return p, geom.ErrInvalid
	}
	patches, err := prepareSharedBaseGraphContact(ctx, sources[0], sources[1], sources[2], o.Stock)
	if err != nil {
		return p, err
	}
	proof, err := geom.DiscoverSplineGraphParabolaCircleCrossings(ctx, patches[0], patches[1], axis, o.Proof)
	if err != nil {
		return p, err
	}
	return buildParabolaCircleCrossingsPartition(proof)
}

func buildParabolaCircleCrossingsPartition(proof *geom.SplineGraphParabolaCircleCrossings) (p parabolaCircleCrossingsPartition, err error) {
	p.proof = proof
	axis := proof.FreeAxis
	var control [3][3]*big.Rat
	for i, c := range p.proof.ParabolaControls {
		control[i] = [3]*big.Rat{c[0], c[1], big.NewRat(1, 1)}
	}
	curve, err := newExactQuadraticUV(control, false)
	if err != nil {
		return p, err
	}
	stations := [4]*big.Rat{new(big.Rat), p.proof.Nodes[0].Parameters[axis], p.proof.Nodes[1].Parameters[axis], big.NewRat(1, 1)}
	for i := range 3 {
		part, err := curve.Restrict(stations[i], stations[i+1])
		if err != nil {
			return p, err
		}
		p.pieces[i] = openSectionPiece{a: part.ends[0], b: part.ends[1], c: part, side: -1}
	}
	outer, inner, err := exactGraphCircleRings(p.proof.CircleCenter, p.proof.RadiusSquared)
	if err != nil {
		return p, err
	}
	if axis == 0 {
		copy(p.pieces[3:], outer)
	} else {
		// Physical UV is the reversed free/dependent chart. Start at
		// physical +Y and follow the free/dependent counterclockwise arc.
		copy(p.pieces[3:], []openSectionPiece{inner[3], inner[0], inner[1], inner[2]})
	}
	p0, p1, p2 := p.pieces[0], p.pieces[1], p.pieces[2]
	c0, c1, c2, c3 := p.pieces[3], p.pieces[4], p.pieces[5], p.pieces[6]
	if p0.b != c2.a || p2.a != c0.a {
		return p, geom.ErrConvergence
	}
	reverse := reverseParabolaCircleCrossingPiece
	chains := [4][]openSectionPiece{
		{reverse(p2), reverse(c3), reverse(c2), reverse(p0)},
		{p0, reverse(c1), reverse(c0), p2},
		{c2, c3, reverse(p1)},
		{p1, c0, c1},
	}
	for region, loop := range chains {
		if axis == 1 {
			slices.Reverse(loop)
			for i := range loop {
				loop[i] = reverse(loop[i])
			}
		}
		if region < 2 {
			walls, err := saddleRectangleArc(loop[len(loop)-1].b, loop[0].a, geom.ParameterRectangle{UMax: 1, VMax: 1})
			if err != nil {
				return p, err
			}
			if len(walls) != 3 {
				return p, geom.ErrConvergence
			}
			loop = append(loop, walls...)
		}
		for i, piece := range loop {
			if piece.b != loop[(i+1)%len(loop)].a {
				return p, geom.ErrConvergence
			}
		}
		p.regions[region] = openGraphRegion{loop: loop, sign: p.proof.RegionSides[region]}
	}
	return p, nil
}

func reverseParabolaCircleCrossingPiece(piece openSectionPiece) openSectionPiece {
	return openSectionPiece{a: piece.b, b: piece.a, c: reversePCurve(piece.c), side: piece.side}
}

func sameParabolaCircleCrossingTrim(actual, expected PCurve) bool {
	if expected == nil {
		return actual == nil
	}
	a, oka := actual.(ExactQuadraticUV)
	b, okb := expected.(ExactQuadraticUV)
	if !oka || !okb || a.rationalEndpoints || b.rationalEndpoints {
		return false
	}
	for i, row := range a.control {
		for j, x := range row {
			if x.Cmp(b.control[i][j]) != 0 {
				return false
			}
		}
	}
	return true
}

func bindParabolaCircleCrossingFace(ctx context.Context, face Face, roof *geom.BSplineSurface,
	reversed bool, loop []openSectionPiece, witness [2]*big.Rat) error {
	return bindParabolaCirclesRegionFace(ctx, face, roof, reversed, openGraphRegion{loop: loop}, witness)
}

func bindParabolaCirclesRegionFace(ctx context.Context, face Face, roof *geom.BSplineSurface,
	reversed bool, region openGraphRegion, witness [2]*big.Rat) error {
	if face.Surface != roof || face.Reversed != reversed || len(face.Holes) != len(region.holes) {
		return geom.ErrConvergence
	}
	bindsLoop := func(uses []Use, loop []openSectionPiece) bool {
		if len(uses) != len(loop) {
			return false
		}
		for i, piece := range loop {
			use := uses[i]
			if use.UVStart != piece.a || use.UVEnd != piece.b || !sameParabolaCircleCrossingTrim(use.Curve, piece.c) {
				return false
			}
		}
		return true
	}
	if !bindsLoop(face.Outer, region.loop) {
		return geom.ErrConvergence
	}
	for i, hole := range face.Holes {
		if !bindsLoop(hole, region.holes[i]) {
			return geom.ErrConvergence
		}
	}
	location, proof, err := LocateRationalInFaceTrim(ctx, face, witness, trimvalidate.Options{})
	if err != nil {
		return err
	}
	if proof.Status != trimvalidate.Certified || location != trimvalidate.Inside {
		return fmt.Errorf("trimmed: parabola-circle crossing face %s: %w", proof.Reason, geom.ErrConvergence)
	}
	return nil
}

func bindParabolaCircleCrossingsEnvelope(ctx context.Context, solid Solid, assembly SplineGraphBooleanHistory,
	p parabolaCircleCrossingsPartition, sources [3]*geom.BSplineSurface, op parametric.BooleanOp,
	opts geom.GraphPointContactOptions) (h ParabolaCircleCrossingsHistory, err error) {
	topo := solid.Topology()
	if len(topo.Shells) != 1 || topo.EulerCharacteristic != 2 || topo.CollapsedEdges != 0 ||
		topo.Faces != 11 || topo.Edges != 25 || topo.Vertices != 16 || topo.InnerWires != 0 ||
		assembly.Base != 4 || len(assembly.WallFaces) != 6 || len(assembly.ContourEdges) != 7 {
		return h, geom.ErrConvergence
	}
	return bindParabolaCrossingEnvelopeRegions(ctx, solid, assembly, p, sources, op, opts, false)
}

func bindParabolaCrossingEnvelopeRegions(ctx context.Context, solid Solid, assembly SplineGraphBooleanHistory,
	p parabolaCircleCrossingsPartition, sources [3]*geom.BSplineSurface, op parametric.BooleanOp,
	opts geom.GraphPointContactOptions, includeInnerWires bool) (h ParabolaCircleCrossingsHistory, err error) {
	h.Base = assembly.Base
	base := solid.faces[h.Base]
	if base.Surface != sources[0] || !base.Reversed {
		return h, geom.ErrConvergence
	}
	copy(h.WallFaces[:], assembly.WallFaces)
	for i, region := range p.regions {
		h.RoofFaces[i] = i
		if (region.sign > 0) != (op == parametric.Union) {
			h.RoofOwners[i] = 1
		}
		if err := bindParabolaCirclesRegionFace(ctx, solid.faces[i], sources[1+h.RoofOwners[i]], false, region, p.proof.RegionWitnesses[i]); err != nil {
			return h, fmt.Errorf("trimmed: parabola-circle crossing roof %d: %w", i, err)
		}
	}
	if err := validateGraphEnvelopeSplineTrims(ctx, solid.faces[h.Base:h.Base+1]); err != nil {
		return h, err
	}
	for _, id := range h.WallFaces {
		wall := solid.faces[id]
		if len(wall.Outer) != 4 {
			return h, geom.ErrConvergence
		}
		bottom, hasBase := graphWallSourceTrim(wall.Outer[0].Curve, false, false)
		top, hasRoof := graphWallSourceTrim(wall.Outer[2].Curve, true, false)
		if !hasBase || !hasRoof || bottom.surface != sources[0] || bottom.a != top.a || bottom.b != top.b {
			return h, geom.ErrConvergence
		}
		ownerFound := false
		for region := range 2 {
			for _, piece := range p.regions[region].loop {
				if piece.side >= 0 && top.a == piece.a && top.b == piece.b && top.surface == sources[1+h.RoofOwners[region]] {
					ownerFound = true
				}
			}
		}
		if !ownerFound {
			return h, geom.ErrConvergence
		}
	}
	edges, err := bindParabolaCircleCrossingEdges(solid, assembly.ContourEdges, p.pieces[:])
	if err != nil {
		return h, err
	}
	copy(h.ParabolaEdges[:], edges[:3])
	copy(h.CircleEdges[:], edges[3:])
	for i, node := range p.proof.Nodes {
		vertex, err := exactParabolaCircleCrossingVertex(solid, node.Point)
		if err != nil {
			return h, err
		}
		junction := ParabolaCircleCrossingsJunction{Vertex: vertex, Sectors: node.SectorRegions}
		for j, sourceEdge := range p.rayPieces(i) {
			id := edges[sourceEdge]
			junction.Rays[j] = id
			before, after := node.SectorRegions[(j+3)%4], node.SectorRegions[j]
			if !slices.Equal(nodalCubicIncidentFaces(solid, id), []int{min(before, after), max(before, after)}) ||
				!opposedGraphContourUses(solid.faces[before].Outer, solid.faces[after].Outer, id) ||
				!matchesParabolaCircleCrossingRay(solid, id, vertex, node.Rays[j]) {
				return h, geom.ErrConvergence
			}
			model := solid.edges[id].Curve.(surfaceTrimCurve)
			boundOwner := false
			for _, face := range []Face{solid.faces[before], solid.faces[after]} {
				boundOwner = boundOwner || (face.Surface == model.surface && slices.ContainsFunc(face.Outer, func(use Use) bool {
					return use.Edge == id && !use.Reversed && use.UVStart == model.a && use.UVEnd == model.b &&
						sameParabolaCircleCrossingTrim(use.Curve, model.curve)
				}))
			}
			if !boundOwner {
				return h, geom.ErrConvergence
			}
		}
		for id, edge := range solid.edges {
			if (edge.Start == vertex || edge.End == vertex) && !slices.Contains(junction.Rays[:], id) {
				return h, geom.ErrConvergence
			}
		}
		h.Junctions[i] = junction
	}
	if err := materialVertexLinks(ctx, solid, includeInnerWires); err != nil {
		return h, err
	}
	return h, validateExactGraphPartitionEdges(ctx, solid, opts)
}

func bindParabolaCircleCrossingEdges(s Solid, ids []int, pieces []openSectionPiece) ([]int, error) {
	out := make([]int, len(pieces))
	for i, piece := range pieces {
		out[i] = -1
		reversed := reverseParabolaCircleCrossingPiece(piece)
		for _, id := range ids {
			trim, ok := s.edges[id].Curve.(surfaceTrimCurve)
			if !ok {
				return nil, geom.ErrConvergence
			}
			if (trim.a == piece.a && trim.b == piece.b && sameParabolaCircleCrossingTrim(trim.curve, piece.c)) ||
				(trim.a == reversed.a && trim.b == reversed.b && sameParabolaCircleCrossingTrim(trim.curve, reversed.c)) {
				if out[i] >= 0 {
					return nil, geom.ErrConvergence
				}
				out[i] = id
			}
		}
		if out[i] < 0 {
			return nil, geom.ErrConvergence
		}
	}
	return out, nil
}

func (p parabolaCircleCrossingsPartition) rayPieces(node int) [4]int {
	ids := [2][4]int{{1, 4, 0, 5}, {2, 3, 1, 6}}[node]
	if p.proof.FreeAxis == 1 {
		ids[1], ids[3] = ids[3], ids[1]
	}
	return ids
}

func exactParabolaCircleCrossingVertex(s Solid, p [3]*big.Rat) (int, error) {
	var xyz [3]float64
	for i, q := range p {
		var err error
		xyz[i], err = geom.RepresentedExactScalar(q)
		if err != nil {
			return -1, err
		}
	}
	id := -1
	for i, point := range s.vertices {
		if point == (geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]}) {
			if id >= 0 {
				return -1, geom.ErrConvergence
			}
			id = i
		}
	}
	if id < 0 {
		return -1, geom.ErrConvergence
	}
	return id, nil
}

func matchesParabolaCircleCrossingRay(s Solid, id, vertex int, direction [3]*big.Rat) bool {
	edge := s.edges[id]
	if (edge.Start == vertex) == (edge.End == vertex) {
		return false
	}
	trim, ok := edge.Curve.(surfaceTrimCurve)
	if !ok {
		return false
	}
	curve, ok := trim.curve.(ExactQuadraticUV)
	if !ok {
		return false
	}
	if edge.End == vertex {
		curve = reversePCurve(curve).(ExactQuadraticUV)
	}
	_, xy, err := curve.EvaluateRational(new(big.Rat))
	if err != nil {
		return false
	}
	// The original geometry proof lifts both owner curves through the
	// same exact surface jet. Identity XY determines the directed model ray.
	return new(big.Rat).Mul(xy[0], direction[1]).Cmp(new(big.Rat).Mul(xy[1], direction[0])) == 0 &&
		new(big.Rat).Add(new(big.Rat).Mul(xy[0], direction[0]), new(big.Rat).Mul(xy[1], direction[1])).Sign() > 0
}
