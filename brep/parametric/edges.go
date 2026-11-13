package parametric

import (
	"fmt"

	"plainsolid/geom"
)

// EdgeChords approximates each noncollapsed model edge independently. The
// original edge index is retained and adjacent faces reference the same
// chords. A zero options value uses geom.TessellateCurve's sampled defaults.
type EdgeChord struct {
	Edge int
	geom.CurveSegment
}

func (s Solid) EdgeChords(options geom.CurveTessellationOptions, maxSegments int) ([]EdgeChord, error) {
	return TessellateEdges(s.vertices, s.edges, options, maxSegments)
}

// TessellateEdges approximates shared model curves without consulting face
// triangulation. The caller supplies the topology's vertex and edge arrays.
func TessellateEdges(vertices []geom.Vec3, edges []Edge, options geom.CurveTessellationOptions, maxSegments int) ([]EdgeChord, error) {
	if maxSegments <= 0 {
		return nil, fmt.Errorf("parametric: positive edge chord limit required")
	}
	var out []EdgeChord
	for i, e := range edges {
		if e.Start < 0 || e.End < 0 || e.Start >= len(vertices) || e.End >= len(vertices) || !vertices[e.Start].IsFinite() || !vertices[e.End].IsFinite() {
			return nil, fmt.Errorf("parametric: invalid edge %d", i)
		}
		if e.Curve == nil {
			continue
		}
		left := maxSegments - len(out)
		if left == 0 {
			return nil, ErrBudget
		}
		o := options
		// Limit temporary subdivision work as well as the returned output.
		if left < 100000 && (o.MaxEvaluations == 0 || o.MaxEvaluations > left*8+64) {
			o.MaxEvaluations = min(100000, left*8+64)
		}
		chords, err := geom.TessellateCurve(e.Curve, e.Range.Min, e.Range.Max, o)
		if err != nil {
			return nil, fmt.Errorf("parametric: edge %d: %w", i, err)
		}
		if len(chords) > left {
			return nil, ErrBudget
		}
		chords[0].Start = vertices[e.Start]
		chords[len(chords)-1].End = vertices[e.End]
		for _, c := range chords {
			out = append(out, EdgeChord{i, c})
		}
	}
	return out, nil
}
