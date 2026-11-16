package trimmed

import (
	"math/big"
	"slices"

	"plainsolid/geom"
)

func bindNodalCubicCommon(s Solid, envelope SplineGraphBooleanHistory, roofs [2]*geom.BSplineSurface,
	charts [2]graphEnvelopeChart, p *geom.NodalCubicSection, xyz [4]geom.Vec3, pieces [4][]openSectionPiece,
	regions [4]graphEnvelopeRegion, seam openSectionPiece) (h NodalCubicCommonHistory, err error) {
	t := s.Topology()
	if len(t.Shells) != 1 || t.EulerCharacteristic != 2 || t.CollapsedEdges != 0 || envelope.Base != len(regions) {
		return h, geom.ErrConvergence
	}
	h.Base, h.Seam = envelope.Base, -1
	wallOwners := make(map[int]int, len(envelope.WallFaces))
	for i, region := range regions {
		owner := 1
		if region.first {
			owner = 0
		}
		h.RoofFaces[i], h.RoofOwners[i] = i, owner
		chart := charts[owner]
		loop := make([]openSectionPiece, len(region.loop))
		for j, common := range region.loop {
			loop[j], err = chart.piece(common)
			if err != nil {
				return h, err
			}
		}
		if chart.reversed {
			slices.Reverse(loop)
			for j := range loop {
				use := &loop[j]
				use.a, use.b, use.c = use.b, use.a, reversePCurve(use.c)
			}
		}
		face := s.faces[i]
		if err := bindPolynomialTangencyFace(face, roofs[owner], chart.reversed, loop); err != nil {
			return h, err
		}
		for j, piece := range loop {
			if piece.side >= 0 {
				id := face.Outer[j].Edge
				if _, duplicate := wallOwners[id]; duplicate {
					return h, geom.ErrConvergence
				}
				wallOwners[id] = owner
			}
		}
	}
	if err := bindAlignedPolynomialWallUses(s, envelope.WallFaces, wallOwners, roofs, charts); err != nil {
		return h, err
	}

	var contour []int
	exterior := h.RoofOwners[0]
	for _, id := range envelope.ContourEdges {
		edge := s.edges[id]
		if nodalCubicEdgeXY(s, edge.Start, edge.End) == ([2]geom.Vec2{seam.a, seam.b}) ||
			nodalCubicEdgeXY(s, edge.End, edge.Start) == ([2]geom.Vec2{seam.a, seam.b}) {
			if h.Seam >= 0 || h.RoofOwners[1] != exterior {
				return h, geom.ErrConvergence
			}
			h.Seam = id
			continue
		}
		contour = append(contour, id)
	}
	if h.Seam < 0 {
		return h, geom.ErrConvergence
	}
	if err := bindNodalCubicSeam(s, h.Seam, seam, roofs[exterior], charts[exterior]); err != nil {
		return h, err
	}
	sources := make([]geom.SplineGraphPolynomialPiece, len(p.Edges))
	for i, edge := range p.Edges {
		sources[i].Start, sources[i].End = edge.Start, edge.End
	}
	bind, err := bindPolynomialNodeEdgesOnChart(s, contour, sources, xyz[:], pieces[:], true, nil)
	if err != nil {
		return h, err
	}
	h.Vertices = [4]int(bind.vertices)
	for i, chain := range bind.edges {
		h.Chains[i] = [2]int{chain[0], -1}
		if len(chain) == 2 {
			h.Chains[i][1] = chain[1]
		}
		for j, id := range chain {
			if err := bindNodalCubicModelEdge(s, id, pieces[i][j], roofs, charts); err != nil {
				return h, err
			}
		}
	}
	if err := bindNodalCubicRays(s, p, bind, roofs, &h); err != nil {
		return h, err
	}
	return h, nil
}

func nodalCubicEdgeXY(s Solid, start, end int) [2]geom.Vec2 {
	a, b := s.vertices[start], s.vertices[end]
	return [2]geom.Vec2{{X: a.X, Y: a.Y}, {X: b.X, Y: b.Y}}
}

func bindNodalCubicSeam(s Solid, id int, seam openSectionPiece, owner *geom.BSplineSurface, chart graphEnvelopeChart) error {
	edge := s.edges[id]
	if nodalCubicEdgeXY(s, edge.Start, edge.End) == ([2]geom.Vec2{seam.b, seam.a}) {
		seam.a, seam.b = seam.b, seam.a
	}
	expected, err := chart.piece(seam)
	if err != nil {
		return err
	}
	trim, ok := edge.Curve.(surfaceTrimCurve)
	if !ok || trim.surface != owner || trim.curve != nil || trim.a != expected.a || trim.b != expected.b ||
		!slices.Equal(nodalCubicIncidentFaces(s, id), []int{0, 1}) {
		return geom.ErrConvergence
	}
	return nil
}

func bindNodalCubicModelEdge(s Solid, id int, piece openSectionPiece, roofs [2]*geom.BSplineSurface, charts [2]graphEnvelopeChart) error {
	edge := s.edges[id]
	trim, ok := edge.Curve.(surfaceTrimCurve)
	if !ok {
		return geom.ErrConvergence
	}
	owner := 0
	if trim.surface != roofs[0] {
		if trim.surface != roofs[1] {
			return geom.ErrConvergence
		}
		owner = 1
	}
	if nodalCubicEdgeXY(s, edge.Start, edge.End) == ([2]geom.Vec2{piece.b, piece.a}) {
		piece.a, piece.b, piece.c = piece.b, piece.a, reversePCurve(piece.c)
	}
	mapped, err := charts[owner].piece(piece)
	if err != nil {
		return err
	}
	if trim.a != mapped.a || trim.b != mapped.b || !matchesTangencyTrim(trim.curve, mapped.c) {
		return geom.ErrConvergence
	}
	return nil
}

func bindNodalCubicRays(s Solid, p *geom.NodalCubicSection, bind polynomialNodeBinding, roofs [2]*geom.BSplineSurface,
	h *NodalCubicCommonHistory) error {
	h.NodeSectors = [4]int{0, 2, 1, 3}
	node := h.Vertices[0]
	for i, ray := range p.Rays {
		if ([4]int{0, 0, 1, 2})[h.NodeSectors[i]] != p.Sectors[i] {
			return geom.ErrConvergence
		}
		chain := bind.edges[ray.Edge]
		id := chain[0]
		if ray.Reversed {
			id = chain[len(chain)-1]
		}
		edge := s.edges[id]
		if (edge.Start == node) == (edge.End == node) {
			return geom.ErrConvergence
		}
		h.NodeRays[i] = id
		a, b := h.NodeSectors[(i+3)%4], h.NodeSectors[i]
		if !slices.Equal(nodalCubicIncidentFaces(s, id), []int{min(a, b), max(a, b)}) {
			return geom.ErrConvergence
		}
		trim := edge.Curve.(surfaceTrimCurve)
		owner := 0
		if trim.surface == roofs[1] {
			owner = 1
		}
		controls := trim.curve.(ExactBezierUV).HomogeneousControls()
		near, next := controls[0], controls[1]
		if edge.End == node {
			near, next = controls[len(controls)-1], controls[len(controls)-2]
		}
		// Weights are one on every polynomial restriction. A positive
		// collinear endpoint derivative identifies the exact outgoing germ.
		dx, dy := new(big.Rat).Sub(next[0], near[0]), new(big.Rat).Sub(next[1], near[1])
		u, v := ray.UV[owner][0], ray.UV[owner][1]
		if new(big.Rat).Mul(dx, v).Cmp(new(big.Rat).Mul(dy, u)) != 0 ||
			new(big.Rat).Add(new(big.Rat).Mul(dx, u), new(big.Rat).Mul(dy, v)).Sign() <= 0 {
			return geom.ErrConvergence
		}
	}
	for id, edge := range s.edges {
		if (edge.Start == node || edge.End == node) && !slices.Contains(h.NodeRays[:], id) {
			return geom.ErrConvergence
		}
	}
	return nil
}

func nodalCubicIncidentFaces(s Solid, id int) []int {
	var out []int
	for fi, face := range s.faces {
		for _, use := range face.Outer {
			if use.Edge == id {
				out = append(out, fi)
			}
		}
	}
	return out
}
