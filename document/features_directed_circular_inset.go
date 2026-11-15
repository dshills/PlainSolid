package document

import (
	"fmt"

	"plainsolid/geom"
)

// DirectedCircularInsetJoinFeature joins two authored cap-abutting circular
// sweeps. Its Input is the host and its Tool is the strictly contained guest.
const DirectedCircularInsetJoinFeature FeatureKind = "directed-circular-inset-join"

// CircularInsetOwner selects the original sweep of a joined face or edge.
type CircularInsetOwner string

const (
	CircularInsetHost  CircularInsetOwner = "host"
	CircularInsetGuest CircularInsetOwner = "guest"
)

// CircularInsetFace names an original cap or a wall by its authored segment.
type CircularInsetFace struct {
	Feature string
	Owner   CircularInsetOwner
	Role    string
}

// CircularInsetEdge names an original rim or vertical edge.
type CircularInsetEdge struct {
	Feature, Segment string
	Owner            CircularInsetOwner
	Role             CircularExtrusionEdgeRole
}

type circularInsetRoleLayout struct {
	input, tool           string
	hostCount, guestCount int
	hostCap, guestCap     int
	floors                int
}

func (s State) circularInsetRoles(id string) (circularInsetRoleLayout, error) {
	var zero circularInsetRoleLayout
	seen := map[string]bool{}
	for id != "" {
		if seen[id] {
			return zero, fmt.Errorf("document: feature cycle at %q", id)
		}
		seen[id] = true
		f, ok := s.Features[id]
		if !ok {
			return zero, fmt.Errorf("document: missing feature %q", id)
		}
		if f.Kind == TransformFeature {
			if _, err := f.Transform.Inverse(); err != nil {
				return zero, err
			}
			id = f.Input
			continue
		}
		if f.Kind != DirectedCircularInsetJoinFeature {
			return zero, fmt.Errorf("document: feature kind %q has no circular inset roles", f.Kind)
		}
		desc := func(id string) (circularSweepNames, error) {
			k := s.Features[id].Kind
			if !localCircularSweepKind(k) {
				return circularSweepNames{}, fmt.Errorf("document: circular inset %q is not an authored unplaced sweep", id)
			}
			return s.directedCircularSource(id)
		}
		h, err := desc(f.Input)
		if err != nil {
			return zero, err
		}
		g, err := desc(f.Tool)
		if err != nil {
			return zero, err
		}
		r := circularInsetRoleLayout{input: f.Input, tool: f.Tool, hostCount: h.count(), guestCount: g.count(), floors: len(g.rings) - 1}
		switch {
		case h.top == g.bottom:
			r.hostCap = 1
		case h.bottom == g.top:
			r.guestCap = 1
		default:
			return zero, geom.ErrDomain
		}
		return r, nil
	}
	return zero, fmt.Errorf("document: empty circular inset feature reference")
}

// ResolveCircularInsetFaces resolves surviving faces from the original named
// member in the evaluated joined asset. The consumed guest cap returns no
// faces. The modified host cap returns its exterior fragment followed by one
// exposed floor for each original guest opening.
func (s State) ResolveCircularInsetFaces(ref CircularInsetFace) ([]int, error) {
	r, err := s.circularInsetRoles(ref.Feature)
	if err != nil {
		return nil, err
	}
	switch ref.Owner {
	case CircularInsetHost:
		i, err := s.ResolveDirectedCircularExtrusionFace(FeatureFace{Feature: r.input, Role: ref.Role})
		if err != nil {
			return nil, err
		}
		out := []int{i}
		if i == r.hostCap {
			start := r.hostCount + r.guestCount + 3
			for j := 0; j < r.floors; j++ {
				out = append(out, start+j)
			}
		}
		return out, nil
	case CircularInsetGuest:
		i, err := s.ResolveDirectedCircularExtrusionFace(FeatureFace{Feature: r.tool, Role: ref.Role})
		if err != nil {
			return nil, err
		}
		if i == r.guestCap {
			return nil, nil
		}
		if i > r.guestCap {
			i--
		}
		return []int{r.hostCount + 2 + i}, nil
	default:
		return nil, geom.ErrInvalid
	}
}

// ResolveCircularInsetEdge locates the surviving original model edge without
// welding the guest rim to an unrelated host edge. Names also resolve through
// a placement applied after the join.
func (s State) ResolveCircularInsetEdge(ref CircularInsetEdge) (int, error) {
	r, err := s.circularInsetRoles(ref.Feature)
	if err != nil {
		return 0, err
	}
	f, offset := r.input, 0
	switch ref.Owner {
	case CircularInsetHost:
	case CircularInsetGuest:
		f, offset = r.tool, 3*r.hostCount
	default:
		return 0, geom.ErrInvalid
	}
	i, err := s.ResolveDirectedCircularExtrusionEdge(CircularExtrusionEdge{Feature: f, Segment: ref.Segment, Role: ref.Role})
	if err != nil {
		return 0, err
	}
	return offset + i, nil
}
