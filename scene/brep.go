package scene

import (
	"context"
	"fmt"
	"strconv"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// CertifiedTrimmedEdges draws original spline model edges using a certified
// chord error budget. All edge IDs refer to the shared source topology.
// An unsupported edge causes an error instead of a sampled fallback.
func CertifiedTrimmedEdges(ctx context.Context, s trimmed.Solid, options geom.CertifiedTessellationOptions, maxSegments int) ([]DrawingEdge, error) {
	chords, e := s.CertifiedEdgeChords(ctx, options, maxSegments)
	if e != nil {
		return nil, e
	}
	out := make([]DrawingEdge, len(chords))
	for i, x := range chords {
		out[i] = DrawingEdge{ID: strconv.Itoa(x.Edge), Start: x.Start, End: x.End}
	}
	return out, nil
}

// PlanarEdges returns the topological edges of a planar B-rep. Tessellation
// diagonals are excluded. IDs are the decimal source edge indices.
func PlanarEdges(s brep.Solid) []DrawingEdge {
	v, e := s.Vertices(), s.Edges()
	out := make([]DrawingEdge, len(e))
	for i, x := range e {
		out[i] = DrawingEdge{ID: strconv.Itoa(i), Start: v[x.Start], End: v[x.End]}
	}
	return out
}

// ParseEdgeID resolves an ID emitted by PlanarEdges.
func ParseEdgeID(id string, s brep.Solid) (int, error) {
	i, e := strconv.Atoi(id)
	if e != nil || i < 0 || i >= len(s.Edges()) || id != strconv.Itoa(i) {
		return 0, fmt.Errorf("scene: invalid planar edge ID %q", id)
	}
	return i, nil
}

// ParametricEdges discretizes the shared model curves with the requested
// sampled chord tolerance. Every chord uses the ID of its original edge.
func ParametricEdges(s parametric.Solid, options geom.CurveTessellationOptions, maxSegments int) ([]DrawingEdge, error) {
	x, e := s.EdgeChords(options, maxSegments)
	return edgeChords(x, e)
}

// TrimmedEdges discretizes the original shared model curves of a trimmed
// solid. Each chord retains its source edge ID and excludes mesh diagonals.
func TrimmedEdges(s trimmed.Solid, options geom.CurveTessellationOptions, maxSegments int) ([]DrawingEdge, error) {
	x, e := parametric.TessellateEdges(s.Vertices(), s.Edges(), options, maxSegments)
	return edgeChords(x, e)
}

func edgeChords(x []parametric.EdgeChord, e error) ([]DrawingEdge, error) {
	if e != nil {
		return nil, e
	}
	r := make([]DrawingEdge, len(x))
	for i, v := range x {
		r[i] = DrawingEdge{ID: strconv.Itoa(v.Edge), Start: v.Start, End: v.End}
	}
	return r, nil
}
