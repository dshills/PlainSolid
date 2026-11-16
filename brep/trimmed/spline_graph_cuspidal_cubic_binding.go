package trimmed

import (
	"fmt"
	"math/big"
	"slices"

	"plainsolid/geom"
)

func bindCuspidalCubicEnvelope(s Solid, envelope SplineGraphBooleanHistory, roofs [2]*geom.BSplineSurface, charts [2]graphEnvelopeChart,
	p *geom.CuspidalCubicSection, xyz [3]geom.Vec3, pieces [2]openSectionPiece,
	regions [2]graphEnvelopeRegion) (h CuspidalCubicCommonHistory, err error) {
	topology := s.Topology()
	if len(topology.Shells) != 1 || topology.EulerCharacteristic != 2 || topology.CollapsedEdges != 0 ||
		envelope.Base != len(regions) || len(envelope.ContourEdges) != len(pieces) {
		return h, geom.ErrConvergence
	}
	h.Base = envelope.Base
	wallOwners := make(map[int]int, len(envelope.WallFaces))
	for i, region := range regions {
		owner := 1
		if region.first {
			owner = 0
		}
		h.RoofFaces[i], h.RoofOwners[i] = i, owner
		chart := charts[owner]
		loop := make([]openSectionPiece, len(region.loop))
		for j, piece := range region.loop {
			loop[j], err = chart.piece(piece)
			if err != nil {
				return h, err
			}
		}
		if chart.reversed {
			slices.Reverse(loop)
			for j := range loop {
				p := &loop[j]
				p.a, p.b, p.c = p.b, p.a, reversePCurve(p.c)
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

	for i, point := range xyz {
		id := slices.Index(s.vertices, point)
		if id < 0 || slices.Contains(s.vertices[id+1:], point) {
			return h, geom.ErrConvergence
		}
		h.Vertices[i] = id
	}
	for i, want := range [2][2]int{{h.Vertices[1], h.Vertices[0]}, {h.Vertices[0], h.Vertices[2]}} {
		h.Germs[i] = -1
		for _, id := range envelope.ContourEdges {
			edge := s.edges[id]
			ends := [2]int{edge.Start, edge.End}
			if ends != want && ends != ([2]int{want[1], want[0]}) {
				continue
			}
			if h.Germs[i] >= 0 {
				return h, geom.ErrConvergence
			}
			h.Germs[i] = id
			if err := bindNodalCubicModelEdge(s, id, pieces[i], roofs, charts); err != nil {
				return h, err
			}
		}
		if h.Germs[i] < 0 {
			return h, geom.ErrConvergence
		}
		if err := bindCuspidalCubicGermCoedges(s, h, p, i); err != nil {
			return h, err
		}
	}
	// The cusp link has two arcs, one in each of the two signed roof
	// regions. They are joined at both germs. The stock wall and base
	// cannot meet this interior vertex.
	for id, edge := range s.edges {
		if (edge.Start == h.Vertices[0] || edge.End == h.Vertices[0]) && !slices.Contains(h.Germs[:], id) {
			return h, geom.ErrConvergence
		}
	}
	return h, nil
}

func bindCuspidalCubicGermCoedges(s Solid, h CuspidalCubicCommonHistory, p *geom.CuspidalCubicSection, germ int) error {
	id := h.Germs[germ]
	if !slices.Equal(nodalCubicIncidentFaces(s, id), h.RoofFaces[:]) {
		return geom.ErrConvergence
	}
	edge := s.edges[id]
	q := p.Germs[germ]
	width := new(big.Rat).Sub(q.Range[1], q.Range[0])
	weights := [2]*big.Rat{new(big.Rat).Mul(width, width), new(big.Rat)}
	weights[1].Mul(weights[0], width)
	for i, faceID := range h.RoofFaces {
		owner := h.RoofOwners[i]
		face := s.faces[faceID]
		for _, use := range face.Outer {
			if use.Edge != id {
				continue
			}
			first, last := edge.Start, edge.End
			if use.Reversed {
				first, last = last, first
			}
			outwardReversed := last == h.Vertices[0]
			if !outwardReversed && first != h.Vertices[0] {
				return geom.ErrConvergence
			}
			curve, ok := use.Curve.(ExactBezierUV)
			if !ok {
				return geom.ErrConvergence
			}
			c := curve.HomogeneousControls()
			if len(c) != 4 {
				return geom.ErrConvergence
			}
			if outwardReversed {
				slices.Reverse(c)
			}
			if !cuspidalCubicEndpointJets(c, p.Vertices[0].UV[owner], p.Vertices[germ+1].UV[owner], q.UVSecond[owner], q.UVThird[owner], weights) {
				return fmt.Errorf("trimmed: cusp germ %d on original roof %d changed its exact jets: %w", germ, owner, geom.ErrConvergence)
			}
		}
	}
	return nil
}

func cuspidalCubicEndpointJets(c [][3]*big.Rat, cusp, exit, second, third [2]*big.Rat, scales [2]*big.Rat) bool {
	for _, control := range c {
		if control[2].Cmp(big.NewRat(1, 1)) != 0 {
			return false
		}
	}
	for axis := range 2 {
		p0, p1, p2, p3 := c[0][axis], c[1][axis], c[2][axis], c[3][axis]
		if p0.Cmp(cusp[axis]) != 0 || p3.Cmp(exit[axis]) != 0 || p0.Cmp(p1) != 0 {
			return false
		}
		// This outward segment uses a normalized first parameter with
		// zero first derivative. The exact original second and third
		// jets are scaled by the square and cube of its interval length.
		d2 := new(big.Rat).Mul(big.NewRat(6, 1), new(big.Rat).Sub(p2, p0))
		d3 := new(big.Rat).Add(new(big.Rat).Sub(p3, new(big.Rat).Mul(big.NewRat(3, 1), p2)), new(big.Rat).Mul(big.NewRat(2, 1), p0))
		d3.Mul(d3, big.NewRat(6, 1))
		if d2.Cmp(new(big.Rat).Mul(scales[0], second[axis])) != 0 ||
			d3.Cmp(new(big.Rat).Mul(scales[1], third[axis])) != 0 {
			return false
		}
	}
	return true
}
