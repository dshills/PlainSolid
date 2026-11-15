package document

import (
	"context"
	"fmt"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// DirectedCircularCapOverlayFeature joins two authored sweeps at an entire
// cap. Their original line and arc partitions may need refinement to match.
const DirectedCircularCapOverlayFeature FeatureKind = "directed-circular-cap-overlay"

// NamedCircularCapOverlaySegment identifies the output walls, rims, and
// vertical edge descended from a directed profile segment. Rims and start
// vertices are indexed lower then upper, in the original profile frame.
type NamedCircularCapOverlaySegment struct {
	Walls         []int
	Rims          [2][]trimmed.CircularSweepEdgeFragment
	Vertical      trimmed.CircularSweepEdgeFragment
	StartVertices [2]int
}

// NamedCircularCapOverlayMember resolves roles against one original feature.
// Caps are indexed lower then upper. A consumed cap has no surviving faces.
type NamedCircularCapOverlayMember struct {
	Feature  string
	Caps     [2][]int
	Segments map[string]NamedCircularCapOverlaySegment
}

// NamedCircularCapOverlay contains the evaluated solid and both source maps.
// Members are ordered Input, Tool. SharedRimEdges lists each seam edge once.
type NamedCircularCapOverlay struct {
	Solid          trimmed.Solid
	Members        [2]NamedCircularCapOverlayMember
	SharedRimEdges []int
}

// EvaluateCircularCapOverlay resolves named roles against the same exact
// replay that generates the joined solid. A placement after the join moves
// the result without changing edge or face indices or source parameters.
func (s State) EvaluateCircularCapOverlay(ctx context.Context, id string) (NamedCircularCapOverlay, error) {
	var zero NamedCircularCapOverlay
	if ctx == nil {
		return zero, geom.ErrInvalid
	}
	feature := id
	seen := map[string]bool{}
	for {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		if seen[feature] || len(seen) >= 10000 {
			return zero, fmt.Errorf("document: circular cap overlay feature cycle or chain too long at %q", feature)
		}
		seen[feature] = true
		f, ok := s.Features[feature]
		if !ok {
			return zero, fmt.Errorf("document: missing feature %q", feature)
		}
		if f.Kind == DirectedCircularCapOverlayFeature {
			break
		}
		if f.Kind != TransformFeature {
			return zero, fmt.Errorf("document: feature %q is not a directed circular cap overlay", feature)
		}
		feature = f.Input
	}
	var proof trimmed.CircularSweepCapOverlay
	evaluated, err := s.evaluateFeatures(ctx, &featureFaceObserver{recordCircularOverlay: func(name string, q trimmed.CircularSweepCapOverlay) {
		if name == feature {
			proof = q
		}
	}})
	if err != nil {
		return zero, err
	}
	f := s.Features[feature]
	out := NamedCircularCapOverlay{Solid: evaluated.Trimmed[id], SharedRimEdges: proof.SharedRimEdges}
	for operand, src := range [2]string{f.Input, f.Tool} {
		names, err := s.directedCircularSource(src)
		if err != nil {
			return zero, err
		}
		n := names.count()
		if len(proof.Faces[operand]) != n+2 || len(proof.Edges[operand]) != 3*n || len(proof.Vertices[operand]) != 2*n {
			return zero, fmt.Errorf("document: circular cap overlay source map size mismatch: %w", geom.ErrInvalid)
		}
		m := NamedCircularCapOverlayMember{Feature: src, Caps: [2][]int{proof.Faces[operand][0], proof.Faces[operand][1]},
			Segments: make(map[string]NamedCircularCapOverlaySegment, n)}
		i := 0
		for _, ring := range names.rings {
			for _, name := range ring {
				if len(proof.Edges[operand][2*n+i]) != 1 {
					return zero, fmt.Errorf("document: circular cap overlay lost original vertical edge: %w", geom.ErrInvalid)
				}
				m.Segments[name] = NamedCircularCapOverlaySegment{
					Walls:         proof.Faces[operand][2+i],
					Rims:          [2][]trimmed.CircularSweepEdgeFragment{proof.Edges[operand][i], proof.Edges[operand][n+i]},
					Vertical:      proof.Edges[operand][2*n+i][0],
					StartVertices: [2]int{proof.Vertices[operand][i], proof.Vertices[operand][n+i]},
				}
				i++
			}
		}
		out.Members[operand] = m
	}
	return out, nil
}
