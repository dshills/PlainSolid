package trimmed

import (
	"context"
	"fmt"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// ParabolaTwoCirclesGraphHistory identifies the two open-side original
// roofs, two disk roofs, their enclosing roofs, and both contact wires.
type ParabolaTwoCirclesGraphHistory struct {
	Base                 int
	RoofFaces            [4]int // below, above, then interior of each ordered circle
	RoofOwners           [4]int
	EnclosingRoof        int // Shared enclosing roof, or -1 if each circle has its own.
	CircleEnclosingRoofs [2]int
	ParabolaEdge         int
	CircleEdges          [2][4]int
	WallFaces            [6]int
}

// ParabolaTwoCirclesGraphEnvelope is one closed Common or Fuse over the
// original positive graph stock with two strictly separated circular trims.
type ParabolaTwoCirclesGraphEnvelope struct {
	solid     Solid
	history   ParabolaTwoCirclesGraphHistory
	sources   [3]*geom.BSplineSurface
	axis      int
	op        parametric.BooleanOp
	tolerance float64
}

func (r *ParabolaTwoCirclesGraphEnvelope) Boundary() Solid {
	if r == nil {
		return Solid{}
	}
	return r.solid
}

func (r *ParabolaTwoCirclesGraphEnvelope) History() ParabolaTwoCirclesGraphHistory {
	if r == nil {
		return ParabolaTwoCirclesGraphHistory{}
	}
	return r.history
}

// CombineParabolaTwoCirclesSplineGraphs builds one original Common or Fuse
// shell after exact discovery of a wall-to-wall parabola and two disjoint
// circles. The original graph stock shares the identity
// unit XY chart. The native arcs and vertices must be exactly representable.
func CombineParabolaTwoCirclesSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface,
	freeAxis int, op parametric.BooleanOp, tolerance float64, o ParabolaCircleGraphOptions) (*ParabolaTwoCirclesGraphEnvelope, error) {
	if op != parametric.Intersection && op != parametric.Union {
		return nil, geom.ErrInvalid
	}
	stock := [3]*geom.BSplineSurface{base, a, b}
	regions, enclosing, err := prepareParabolaTwoCirclesGraphRegions(ctx, stock, freeAxis, tolerance, o)
	if err != nil {
		return nil, err
	}
	s, assembly, err := combineOpenGraphRegions(ctx, base, a, b, regions[:], op, tolerance, exactGraphVertexEvaluator(ctx, o.Proof))
	if err != nil {
		return nil, err
	}
	if err := validateExactGraphPartitionEdges(ctx, s, o.Proof); err != nil {
		return nil, err
	}
	h, err := bindParabolaTwoCirclesGraphEnvelope(ctx, s, assembly, regions, enclosing, stock, op)
	if err != nil {
		return nil, err
	}
	return &ParabolaTwoCirclesGraphEnvelope{solid: s, history: h, sources: stock, axis: freeAxis, op: op, tolerance: tolerance}, nil
}

func prepareParabolaTwoCirclesGraphRegions(ctx context.Context, stock [3]*geom.BSplineSurface,
	freeAxis int, tolerance float64, o ParabolaCircleGraphOptions) (regions [4]openGraphRegion, enclosing [2]int, err error) {
	if ctx == nil || stock[0] == nil || stock[1] == nil || stock[2] == nil || freeAxis < 0 || freeAxis > 1 ||
		tolerance <= 0 || !finite(tolerance) || o.Stock.Domain != (geom.ParameterRectangle{}) {
		return regions, enclosing, geom.ErrInvalid
	}
	patches, err := prepareSharedBaseGraphContact(ctx, stock[0], stock[1], stock[2], o.Stock)
	if err != nil {
		return regions, enclosing, err
	}
	p, err := geom.DiscoverSplineGraphParabolaTwoCircles(ctx, patches[0], patches[1], freeAxis, o.Proof)
	if err != nil {
		return regions, enclosing, err
	}
	open, err := parabolaOpenGraphRegions(p.ParabolaControls, freeAxis)
	if err != nil {
		return regions, enclosing, err
	}
	copy(regions[:2], open[:])
	enclosing = p.CircleRegions
	for i, c := range p.Circles {
		disk, hole, err := exactGraphCircleRings(c.Center, c.RadiusSquared)
		if err != nil {
			return regions, enclosing, fmt.Errorf("trimmed: circle %d: %w", i, err)
		}
		roof := &regions[enclosing[i]]
		roof.holes = append(roof.holes, hole)
		regions[2+i].loop = disk
	}
	for i, sign := range p.RegionSides {
		regions[i].sign = sign
	}
	return regions, enclosing, nil
}

func bindParabolaTwoCirclesGraphEnvelope(ctx context.Context, s Solid, assembly SplineGraphBooleanHistory,
	regions [4]openGraphRegion, enclosing [2]int, stock [3]*geom.BSplineSurface, op parametric.BooleanOp) (ParabolaTwoCirclesGraphHistory, error) {
	var h ParabolaTwoCirclesGraphHistory
	topo := s.Topology()
	if len(topo.Shells) != 1 || topo.CollapsedEdges != 0 || topo.EulerCharacteristic != 2 || topo.InnerWires != 2 ||
		topo.Faces != 11 || topo.Edges != 27 || topo.Vertices != 20 || assembly.Base != 4 ||
		len(assembly.ContourEdges) != 9 || len(assembly.WallFaces) != 6 {
		return h, fmt.Errorf("trimmed: parabola and two circles envelope topology: %w", geom.ErrConvergence)
	}
	h.Base = assembly.Base
	h.RoofFaces = [4]int{0, 1, 2, 3}
	h.EnclosingRoof = -1
	if enclosing[0] == enclosing[1] {
		h.EnclosingRoof = h.RoofFaces[enclosing[0]]
	}
	h.ParabolaEdge = assembly.ContourEdges[0]
	h.WallFaces = [6]int(assembly.WallFaces)
	if s.faces[h.Base].Surface != stock[0] || !s.faces[h.Base].Reversed {
		return h, geom.ErrConvergence
	}
	for i, face := range h.RoofFaces {
		if (regions[i].sign > 0) != (op == parametric.Union) {
			h.RoofOwners[i] = 1
		}
		if s.faces[face].Surface != stock[1+h.RoofOwners[i]] || len(s.faces[face].Holes) != len(regions[i].holes) {
			return h, geom.ErrConvergence
		}
	}
	if !opposedGraphContourUses(s.faces[0].Outer, s.faces[1].Outer, h.ParabolaEdge) {
		return h, geom.ErrConvergence
	}
	// Native edges are allocated in face order, which can differ from
	// circle order. Bind each exact disk to its own source roof hole.
	var nextHole [2]int
	for i, region := range enclosing {
		roof := h.RoofFaces[region]
		h.CircleEnclosingRoofs[i] = roof
		hole := s.faces[roof].Holes[nextHole[region]]
		nextHole[region]++
		if len(hole) != len(h.CircleEdges[i]) {
			return h, geom.ErrConvergence
		}
		for j, use := range hole {
			if !opposedGraphContourUses(hole, s.faces[h.RoofFaces[2+i]].Outer, use.Edge) {
				return h, geom.ErrConvergence
			}
			h.CircleEdges[i][j] = use.Edge
		}
	}
	if err := validateGraphEnvelopeSplineTrims(ctx, s.faces[:h.Base+1]); err != nil {
		return h, err
	}
	return h, nil
}
