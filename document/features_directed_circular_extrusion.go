package document

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

const (
	DirectedCircularExtrusionFeature       FeatureKind = "directed-circular-extrusion"
	DirectedCircularExtrusionOffsetFeature FeatureKind = "directed-circular-extrusion-side-offset"
)

// NamedCircularProfileSegment identifies one authored line or directed minor
// circular arc. IDs are unique across the exterior and all hole wires and
// remain stable when the wire's start or hole order changes.
type NamedCircularProfileSegment struct {
	ID    string
	Curve trimmed.ExactCircularLoopSegment
}

// DirectedCircularExtrusionRecipe stores one CCW exterior wire, disjoint
// clockwise hole wires, and the two exact model-Z cap elevations.
type DirectedCircularExtrusionRecipe struct {
	Outer       []NamedCircularProfileSegment
	Holes       [][]NamedCircularProfileSegment `json:",omitempty"`
	Bottom, Top float64
}

func (r DirectedCircularExtrusionRecipe) clone() DirectedCircularExtrusionRecipe {
	r.Outer = append([]NamedCircularProfileSegment(nil), r.Outer...)
	if len(r.Holes) > 0 {
		holes := make([][]NamedCircularProfileSegment, len(r.Holes))
		for i, ring := range r.Holes {
			holes[i] = append([]NamedCircularProfileSegment(nil), ring...)
		}
		r.Holes = holes
	}
	return r
}

func (r *DirectedCircularExtrusionRecipe) profile() ([][]trimmed.ExactCircularLoopSegment, map[string]int, error) {
	if r == nil || len(r.Outer) < 3 || len(r.Outer) > 64 || len(r.Holes) > 20 {
		return nil, nil, geom.ErrInvalid
	}
	rings := append([][]NamedCircularProfileSegment{r.Outer}, r.Holes...)
	curves := make([][]trimmed.ExactCircularLoopSegment, len(rings))
	index := make(map[string]int, len(r.Outer))
	for i, ring := range rings {
		if len(ring) < 3 || len(ring) > 64-len(index) {
			return nil, nil, geom.ErrInvalid
		}
		curves[i] = make([]trimmed.ExactCircularLoopSegment, len(ring))
		for j, a := range ring {
			_, duplicate := index[a.ID]
			if !validCircularSegmentID(a.ID) || duplicate {
				return nil, nil, fmt.Errorf("%w: empty, reserved or repeated profile segment ID %q", geom.ErrInvalid, a.ID)
			}
			index[a.ID] = len(index)
			curves[i][j] = a.Curve
		}
	}
	return curves, index, nil
}

func evaluateDirectedCircularExtrusion(ctx context.Context, f Feature) (trimmed.ExactCircularSweep, trimmed.Solid, error) {
	r := f.DirectedCircularExtrusion
	curves, _, err := r.profile()
	if err != nil {
		return trimmed.ExactCircularSweep{}, trimmed.Solid{}, err
	}
	solid, err := trimmed.ExtrudeExactDirectedCircularLoops(ctx, curves[0], curves[1:], r.Bottom, r.Top, trimmed.ExactCircularLoopOptions{DisplayTolerance: f.Tolerance})
	if err != nil {
		return trimmed.ExactCircularSweep{}, trimmed.Solid{}, err
	}
	return trimmed.ExactCircularSweep{Outer: curves[0], Holes: curves[1:], Bottom: r.Bottom, Top: r.Top}, solid, nil
}

func validCircularSegmentID(id string) bool {
	return len(id) > 0 && len(id) <= 128 && utf8.ValidString(id) && !strings.Contains(id, ":")
}

func (s State) directedCircularSource(id string) (circularSweepNames, error) {
	var zero circularSweepNames
	seen := map[string]bool{}
	var rounds [][]NamedCircularCornerRound
	for id != "" {
		if seen[id] || len(seen) >= 10000 {
			return zero, fmt.Errorf("document: circular feature cycle or chain too long at %q", id)
		}
		seen[id] = true
		f, ok := s.Features[id]
		if !ok {
			return zero, fmt.Errorf("document: missing feature %q", id)
		}
		switch f.Kind {
		case TransformFeature:
			if _, err := f.Transform.Inverse(); err != nil {
				return zero, err
			}
			id = f.Input
		case DirectedCircularExtrusionFeature:
			r := f.DirectedCircularExtrusion
			if _, _, err := r.profile(); err != nil {
				return zero, err
			}
			n := circularSweepNames{bottom: r.Bottom, top: r.Top}
			for _, ring := range append([][]NamedCircularProfileSegment{r.Outer}, r.Holes...) {
				ids := make([]string, len(ring))
				for i, a := range ring {
					ids[i] = a.ID
				}
				n.rings = append(n.rings, ids)
			}
			for i := len(rounds) - 1; i >= 0; i-- {
				var err error
				n, _, err = namedCircularFilletLayout(n, rounds[i])
				if err != nil {
					return zero, err
				}
			}
			return n, nil
		case DirectedCircularExtrusionOffsetFeature, DirectedCircularCornerFilletFeature:
			if !localCircularSweepKind(s.Features[f.Input].Kind) {
				return zero, fmt.Errorf("document: feature %q has no authored circular sweep input", id)
			}
			if f.Kind == DirectedCircularCornerFilletFeature {
				rounds = append(rounds, f.CircularCornerRounds)
			}
			id = f.Input
		default:
			return zero, fmt.Errorf("document: feature kind %q has no directed circular extrusion roles", f.Kind)
		}
	}
	return zero, fmt.Errorf("document: empty feature reference")
}

// ResolveDirectedCircularExtrusionFace resolves lower-cap, upper-cap or
// wall:<segment ID> in the evaluated asset. A transform preserves the source
// cap names and segment ownership, including when its world axes change.
// Lateral offsets retain the original role IDs. Named fillets retain original
// span names and expose their new arcs by their own IDs.
func (s State) ResolveDirectedCircularExtrusionFace(ref FeatureFace) (int, error) {
	r, err := s.directedCircularSource(ref.Feature)
	if err != nil {
		return 0, err
	}
	switch ref.Role {
	case "lower-cap":
		return 0, nil
	case "upper-cap":
		return 1, nil
	}
	if id, ok := strings.CutPrefix(ref.Role, "wall:"); ok {
		if i, exists := r.index(id); exists {
			return i + 2, nil
		}
	}
	return 0, fmt.Errorf("document: unknown directed circular extrusion face role %q", ref.Role)
}

// CircularExtrusionEdge is one original rim or vertical edge. For a vertical
// role, Segment names the surviving profile segment whose directed start
// is the edge's lower endpoint.
type CircularExtrusionEdge struct {
	Feature, Segment string
	Role             CircularExtrusionEdgeRole
}

type CircularExtrusionEdgeRole string

const (
	CircularExtrusionLowerRim CircularExtrusionEdgeRole = "lower-rim"
	CircularExtrusionUpperRim CircularExtrusionEdgeRole = "upper-rim"
	CircularExtrusionVertical CircularExtrusionEdgeRole = "vertical"
)

// ResolveDirectedCircularExtrusionEdge finds the native edge originating
// from the named authored segment in the evaluated asset.
func (s State) ResolveDirectedCircularExtrusionEdge(ref CircularExtrusionEdge) (int, error) {
	r, err := s.directedCircularSource(ref.Feature)
	if err != nil {
		return 0, err
	}
	i, ok := r.index(ref.Segment)
	if !ok {
		return 0, fmt.Errorf("document: unknown directed circular extrusion segment %q", ref.Segment)
	}
	switch ref.Role {
	case CircularExtrusionLowerRim:
		return i, nil
	case CircularExtrusionUpperRim:
		return i + r.count(), nil
	case CircularExtrusionVertical:
		return i + 2*r.count(), nil
	default:
		return 0, fmt.Errorf("document: unknown directed circular extrusion edge role %q", ref.Role)
	}
}
