package trimmed

import (
	"context"
	"fmt"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// ParabolaTwoCirclesGraphHistory identifies the two open-side original
// roofs, two disk roofs, their enclosing roof, and both contact wires.
type ParabolaTwoCirclesGraphHistory struct {
	Base          int
	RoofFaces     [4]int // below, above, then interior of each ordered circle
	RoofOwners    [4]int
	EnclosingRoof int
	ParabolaEdge  int
	CircleEdges   [2][4]int
	WallFaces     [6]int
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
// circles on the same side. The original graph stock shares the identity
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
	freeAxis int, tolerance float64, o ParabolaCircleGraphOptions) (regions [4]openGraphRegion, enclosing int, err error) {
	if ctx == nil || stock[0] == nil || stock[1] == nil || stock[2] == nil || freeAxis < 0 || freeAxis > 1 ||
		tolerance <= 0 || !finite(tolerance) || o.Stock.Domain != (geom.ParameterRectangle{}) {
		return regions, 0, geom.ErrInvalid
	}
	patches, err := prepareSharedBaseGraphContact(ctx, stock[0], stock[1], stock[2], o.Stock)
	if err != nil {
		return regions, 0, err
	}
	p, err := geom.DiscoverSplineGraphParabolaTwoCircles(ctx, patches[0], patches[1], freeAxis, o.Proof)
	if err != nil {
		return regions, 0, err
	}
	open, err := parabolaOpenGraphRegions(p.ParabolaControls, freeAxis)
	if err != nil {
		return regions, 0, err
	}
	copy(regions[:2], open[:])
	if p.CirclesAbove {
		enclosing = 1
	}
	for i, c := range p.Circles {
		disk, hole, err := exactGraphCircleRings(c.Center, c.RadiusSquared)
		if err != nil {
			return regions, 0, fmt.Errorf("trimmed: circle %d: %w", i, err)
		}
		regions[enclosing].holes = append(regions[enclosing].holes, hole)
		regions[2+i].loop = disk
	}
	for i, sign := range p.RegionSides {
		regions[i].sign = sign
	}
	return regions, enclosing, nil
}

func bindParabolaTwoCirclesGraphEnvelope(ctx context.Context, s Solid, assembly SplineGraphBooleanHistory,
	regions [4]openGraphRegion, enclosing int, stock [3]*geom.BSplineSurface, op parametric.BooleanOp) (ParabolaTwoCirclesGraphHistory, error) {
	var h ParabolaTwoCirclesGraphHistory
	topo := s.Topology()
	if len(topo.Shells) != 1 || topo.CollapsedEdges != 0 || topo.EulerCharacteristic != 2 || topo.InnerWires != 2 ||
		topo.Faces != 11 || topo.Edges != 27 || topo.Vertices != 20 || assembly.Base != 4 ||
		len(assembly.ContourEdges) != 9 || len(assembly.WallFaces) != 6 {
		return h, fmt.Errorf("trimmed: parabola and two circles envelope topology: %w", geom.ErrConvergence)
	}
	h.Base, h.EnclosingRoof = assembly.Base, enclosing
	h.RoofFaces = [4]int{0, 1, 2, 3}
	h.ParabolaEdge = assembly.ContourEdges[0]
	h.CircleEdges = [2][4]int{[4]int(assembly.ContourEdges[1:5]), [4]int(assembly.ContourEdges[5:9])}
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
	for i, ring := range h.CircleEdges {
		for _, edge := range ring {
			if !opposedGraphContourUses(s.faces[enclosing].Holes[i], s.faces[2+i].Outer, edge) {
				return h, geom.ErrConvergence
			}
		}
	}
	if err := validateGraphEnvelopeSplineTrims(ctx, s.faces[:h.Base+1]); err != nil {
		return h, err
	}
	return h, nil
}
