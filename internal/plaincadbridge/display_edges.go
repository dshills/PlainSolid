package plaincadbridge

import (
	"context"
	"errors"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

type displayEdgeResult struct {
	Segments []float64 `json:"segments"`
	EdgeUses int       `json:"edgeUses"`
}

func (s stored) displayEdges(maxUses, maxSegments int) (displayEdgeResult, error) {
	if maxUses < 1 || maxUses > 12000 || maxSegments < 1 || maxSegments > 24000 {
		return displayEdgeResult{}, geom.ErrInvalid
	}
	if s.planar() {
		return planarDisplayEdges(s.solid, maxUses, maxSegments)
	}
	if s.tangent != nil {
		return s.tangent.displayEdges(maxUses, maxSegments)
	}
	native, placement, ok := s.retainedTrimmed()
	if !ok {
		var err error
		native, err = trimmed.FromParametric(s.cylinder.solid)
		if err != nil {
			return displayEdgeResult{}, err
		}
	}
	lines, uses, err := native.DisplayEdges(context.Background(), maxUses, maxSegments)
	if errors.Is(err, trimmed.ErrDisplaySegmentBudget) {
		return displayEdgeResult{EdgeUses: uses}, &failureError{"LIMIT", err.Error()}
	}
	if err != nil {
		return displayEdgeResult{EdgeUses: uses}, err
	}
	out := displayEdgeResult{Segments: make([]float64, 0, len(lines)*3), EdgeUses: uses}
	for _, p := range lines {
		v := placement.Apply(p)
		if !v.IsFinite() {
			return displayEdgeResult{EdgeUses: uses}, geom.ErrUnrepresentable
		}
		out.Segments = append(out.Segments, v.X, v.Y, v.Z)
	}
	return out, nil
}

func planarDisplayEdges(s brep.Solid, maxUses, maxSegments int) (displayEdgeResult, error) {
	out := displayEdgeResult{Segments: []float64{}}
	edges, vertices := s.Edges(), s.Vertices()
	first := make([]geom.Vec3, len(edges))
	seen := make([]bool, len(edges))
	for i, f := range s.Faces() {
		plane, err := s.FacePlane(i)
		if err != nil {
			return out, err
		}
		n := plane.Normal()
		for _, loop := range append([][]brep.EdgeUse{f.Boundary}, f.Holes...) {
			for _, use := range loop {
				if out.EdgeUses >= maxUses {
					return out, &failureError{"LIMIT", "planar display edge use limit exceeded"}
				}
				out.EdgeUses++
				e := use.Edge
				if !seen[e] {
					first[e], seen[e] = n, true
					continue
				}
				if first[e].Dot(n) > .99999 {
					continue
				}
				if len(out.Segments)/6 >= maxSegments {
					return out, &failureError{"LIMIT", "native edges exceed the display segment budget"}
				}
				for _, id := range []int{edges[e].Start, edges[e].End} {
					p := vertices[id]
					out.Segments = append(out.Segments, p.X, p.Y, p.Z)
				}
			}
		}
	}
	return out, nil
}
