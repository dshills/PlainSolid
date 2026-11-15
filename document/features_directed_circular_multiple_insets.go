package document

import (
	"context"
	"fmt"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// DirectedCircularMultipleInsetJoinFeature joins ordered authored guests to
// either cap of an authored host sweep. It uses Input and Tools.
const DirectedCircularMultipleInsetJoinFeature FeatureKind = "directed-circular-multiple-inset-join"

// NamedCircularInsetGroupMember resolves the original roles of one member
// of a multi-inset join. Caps are lower then upper. A consumed guest cap
// has no faces; host cap fragments include independent exposed floors.
type NamedCircularInsetGroupMember struct {
	Feature string
	Caps    [2][]int
	Walls   map[string]int
	// Edges are lower rim, upper rim, and the original start generator.
	Edges map[string][3]int
}

// NamedCircularInsetGroup is the evaluated solid and source roles keyed by
// original member and segment names, including after an output placement.
type NamedCircularInsetGroup struct {
	Solid  trimmed.Solid
	Host   NamedCircularInsetGroupMember
	Guests map[string]NamedCircularInsetGroupMember
}

// EvaluateCircularMultipleInsets evaluates an authored join and resolves
// each original member. Reordering Tools changes native indices but not
// which authored identities own the resulting faces and edges.
func (s State) EvaluateCircularMultipleInsets(ctx context.Context, id string) (NamedCircularInsetGroup, error) {
	var zero NamedCircularInsetGroup
	if ctx == nil {
		return zero, geom.ErrInvalid
	}
	source := id
	seen := map[string]bool{}
	var join Feature
	for {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		f, ok := s.Features[source]
		if !ok || seen[source] || len(seen) >= 10000 {
			return zero, fmt.Errorf("document: invalid circular multi-inset feature chain at %q", source)
		}
		seen[source] = true
		if f.Kind == DirectedCircularMultipleInsetJoinFeature {
			join = f
			break
		}
		if f.Kind != TransformFeature {
			return zero, fmt.Errorf("document: feature %q is not a circular multi-inset join", source)
		}
		source = f.Input
	}
	var proof trimmed.CircularSweepInsets
	materialized, err := s.evaluateFeatures(ctx, &featureFaceObserver{recordCircularInsets: func(name string, p trimmed.CircularSweepInsets) {
		if name == source {
			proof = p
		}
	}})
	if err != nil {
		return zero, err
	}
	if len(proof.Guests) != len(join.Tools) {
		return zero, geom.ErrInvalid
	}
	member := func(id string, caps [2][]int, walls []int, edgeOffset int) (NamedCircularInsetGroupMember, error) {
		n, err := s.directedCircularSource(id)
		if err != nil {
			return NamedCircularInsetGroupMember{}, err
		}
		count := n.count()
		if count != len(walls) {
			return NamedCircularInsetGroupMember{}, geom.ErrInvalid
		}
		m := NamedCircularInsetGroupMember{
			Feature: id, Caps: caps, Walls: make(map[string]int, count), Edges: make(map[string][3]int, count),
		}
		i := 0
		for _, ring := range n.rings {
			for _, segment := range ring {
				m.Walls[segment] = walls[i]
				m.Edges[segment] = [3]int{edgeOffset + i, edgeOffset + i + count, edgeOffset + i + 2*count}
				i++
			}
		}
		return m, nil
	}
	if len(proof.HostFaces) < 2 {
		return zero, geom.ErrInvalid
	}
	hostWalls := make([]int, len(proof.HostFaces)-2)
	for i, f := range proof.HostFaces[2:] {
		if len(f) != 1 {
			return zero, geom.ErrInvalid
		}
		hostWalls[i] = f[0]
	}
	h, err := member(join.Input, [2][]int{proof.HostFaces[0], proof.HostFaces[1]}, hostWalls, 0)
	if err != nil {
		return zero, err
	}
	out := NamedCircularInsetGroup{Solid: materialized.Trimmed[id], Host: h, Guests: make(map[string]NamedCircularInsetGroupMember, len(proof.Guests))}
	for i, g := range proof.Guests {
		var caps [2][]int
		for j, face := range g.Faces[:2] {
			if face >= 0 {
				caps[j] = []int{face}
			}
		}
		m, err := member(join.Tools[i], caps, g.Faces[2:], g.EdgeOffset)
		if err != nil {
			return zero, err
		}
		out.Guests[join.Tools[i]] = m
	}
	return out, nil
}
