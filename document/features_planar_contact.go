package document

import (
	"context"
	"math/big"

	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/csg"
	"plainsolid/csg/poly"
	"plainsolid/geom"
)

func evaluatePlanarContactFeature(ctx context.Context, kind FeatureKind, a, b brep.Solid, wantFaces, wantEdges bool) (brep.Solid, poly.TopologyHistory, error) {
	var s brep.Solid
	var topology poly.TopologyHistory
	var faces [2][]int
	var edges [2][][]csg.PlanarRegionResultEdge
	switch kind {
	case PlanarSeamJoinFeature:
		p, h, err := csg.JoinRefinedCoincidentPlanarRegions(ctx, a, b, csg.PlanarRegionRefinementOptions{})
		if err != nil {
			return brep.Solid{}, topology, err
		}
		s = p
		for i, o := range h.Owners {
			faces[i] = o.Faces
			edges[i] = o.Edges
		}
	case PlanarInsetJoinFeature:
		p, h, err := csg.JoinCoincidentPlanarInsets(ctx, a, b, collision.PlanarFaceContactOptions{})
		if err != nil {
			return brep.Solid{}, topology, err
		}
		s = p
		for i, o := range h.Owners {
			faces[i] = o.Faces
			if wantEdges {
				edges[i] = make([][]csg.PlanarRegionResultEdge, len(o.Edges))
				for j, e := range o.Edges {
					edges[i][j] = []csg.PlanarRegionResultEdge{{Result: e, Range: [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}}}
				}
			}
		}
	default:
		return brep.Solid{}, topology, geom.ErrInvalid
	}
	if !wantFaces && !wantEdges {
		return s, topology, nil
	}
	sf := s.Faces()
	topology.Faces = make([]poly.Source, len(sf))
	seen := make([]bool, len(sf))
	for i, m := range faces {
		for j, k := range m {
			if k < 0 {
				continue
			}
			if k >= len(sf) || seen[k] {
				return brep.Solid{}, poly.TopologyHistory{}, geom.ErrUnrepresentable
			}
			seen[k] = true
			// The complete original face owns this polygon, not an individual
			// source tessellation triangle.
			topology.Faces[k] = poly.Source{Operand: poly.Operand(i), Face: j, Triangle: -1}
		}
	}
	for _, ok := range seen {
		if !ok {
			return brep.Solid{}, poly.TopologyHistory{}, geom.ErrUnrepresentable
		}
	}
	if !wantEdges {
		return s, topology, nil
	}
	topology.Edges = make([]poly.EdgeAncestry, len(s.Edges()))
	for i, source := range edges {
		for j, pieces := range source {
			if err := ctx.Err(); err != nil {
				return brep.Solid{}, poly.TopologyHistory{}, err
			}
			for _, k := range pieces {
				if k.Result.Edge < 0 {
					continue
				}
				a, b := k.Range[0], k.Range[1]
				if k.Result.Reversed {
					a, b = b, a
				}
				e := &topology.Edges[k.Result.Edge]
				e.Overlaps = append(e.Overlaps, poly.EdgeOverlap{Operand: poly.Operand(i), Edge: j,
					OutputFrom: new(big.Rat), OutputTo: big.NewRat(1, 1),
					InputFrom: new(big.Rat).Set(a), InputTo: new(big.Rat).Set(b)})
			}
		}
	}
	for i, f := range sf {
		for _, wire := range append([][]brep.EdgeUse{f.Boundary}, f.Holes...) {
			for _, u := range wire {
				e := &topology.Edges[u.Edge]
				e.AdjacentFaces = append(e.AdjacentFaces, topology.Faces[i])
			}
		}
	}
	for _, e := range topology.Edges {
		if len(e.Overlaps) == 0 {
			return brep.Solid{}, poly.TopologyHistory{}, geom.ErrUnrepresentable
		}
	}
	return s, topology, nil
}
