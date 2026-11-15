package document

import (
	"context"
	"fmt"
	"math"

	"plainsolid/brep/trimmed"
	"plainsolid/csg/fillet"
	"plainsolid/geom"
)

// DirectedCircularCornerFilletFeature rounds named corners of an authored
// model-Z sweep before placement.
const DirectedCircularCornerFilletFeature FeatureKind = "directed-circular-corner-fillet"

// NamedCircularCornerRound identifies the original directed segment whose
// start is rounded. ArcID gives the inserted circular span a separate name.
type NamedCircularCornerRound struct {
	Segment, ArcID string
	Radius         float64
}

type circularSweepNames struct {
	rings       [][]string
	bottom, top float64
}

func (n circularSweepNames) count() int {
	sum := 0
	for _, r := range n.rings {
		sum += len(r)
	}
	return sum
}
func (n circularSweepNames) index(id string) (int, bool) {
	offset := 0
	for _, r := range n.rings {
		for i, k := range r {
			if k == id {
				return offset + i, true
			}
		}
		offset += len(r)
	}
	return 0, false
}

func localCircularSweepKind(k FeatureKind) bool {
	switch k {
	case DirectedCircularExtrusionFeature, DirectedCircularExtrusionOffsetFeature, DirectedCircularCornerFilletFeature:
		return true
	}
	return false
}

func namedCircularFilletLayout(in circularSweepNames, specs []NamedCircularCornerRound) (circularSweepNames, []fillet.CircularSweepCorner, error) {
	var zero circularSweepNames
	if len(specs) == 0 || len(specs) > 64-in.count() {
		return zero, nil, geom.ErrInvalid
	}
	positions := map[string][2]int{}
	for i, ring := range in.rings {
		for j, id := range ring {
			positions[id] = [2]int{i, j}
		}
	}
	added := map[string]string{}
	used := map[string]bool{}
	var selected []fillet.CircularSweepCorner
	for _, s := range specs {
		p, ok := positions[s.Segment]
		_, collides := positions[s.ArcID]
		if !ok || collides || used[s.ArcID] || !validCircularSegmentID(s.ArcID) || added[s.Segment] != "" ||
			s.Radius <= 0 || math.IsNaN(s.Radius) || math.IsInf(s.Radius, 0) {
			return zero, nil, fmt.Errorf("%w: invalid or duplicate circular round on %q", geom.ErrInvalid, s.Segment)
		}
		selected = append(selected, fillet.CircularSweepCorner{Ring: p[0], Corner: p[1], Radius: s.Radius})
		added[s.Segment], used[s.ArcID] = s.ArcID, true
	}
	out := circularSweepNames{rings: make([][]string, len(in.rings)), bottom: in.bottom, top: in.top}
	for i, ring := range in.rings {
		for _, id := range ring {
			if q := added[id]; q != "" {
				out.rings[i] = append(out.rings[i], q)
			}
			out.rings[i] = append(out.rings[i], id)
		}
	}
	return out, selected, nil
}

func (s State) applyNamedCircularFillets(ctx context.Context, f Feature, src trimmed.ExactCircularSweep) (fillet.CircularSweepCornerResult, error) {
	names, err := s.directedCircularSource(f.Input)
	if err != nil {
		return fillet.CircularSweepCornerResult{}, err
	}
	_, selected, err := namedCircularFilletLayout(names, f.CircularCornerRounds)
	if err != nil {
		return fillet.CircularSweepCornerResult{}, err
	}
	o := fillet.OrthogonalPrismOptions{Circular: trimmed.ExactCircularLoopOptions{DisplayTolerance: f.Tolerance}}
	return fillet.CircularSweepCorners(ctx, src, selected, o)
}
