package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
)

type mappedSplineJoinedSupportPartition struct {
	cuts     []*big.Rat
	inserted []bool
	events   []MappedSplineChainSupportEvent
}

func mappedSplineJoinedSupportKnotCuts(ctx context.Context, chains [2]MappedSplineChain, patches [2]*geom.SurfacePatch, cuts []*big.Rat, repair mappedChainKnotRepair, o edgecheck.Options) (mappedSplineJoinedSupportPartition, error) {
	var zero mappedSplineJoinedSupportPartition
	asymmetric := repair == mappedChainAsymmetricJoinedKnotsRepair
	ordered := repair == mappedChainOrderedJoinedKnotsRepair
	cells := len(cuts) - 1
	if ordered {
		if cells < 4 || cells > MaxJoinedMappedSplineCells {
			return zero, geom.ErrConvergence
		}
	} else if cells < 2 || cells > 3 || asymmetric && cells != 3 {
		return zero, geom.ErrConvergence
	}
	limits := geom.SurfacePatchOptions{MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits}
	events := geom.ExactCurvePlaneEventOptions{MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits, MaxSubdivisions: o.MaxDepth}
	interior := make([]MappedSplineChainSupportEvent, cells)
	var interiorOwners [2]bool
	var ownerKnots [2]int
	jointChanges := make([][2]bool, cells-1)
	for owner, chain := range chains {
		if len(chain.parts) != cells {
			return zero, fmt.Errorf("the original roofs require the same model cells: %w", geom.ErrConvergence)
		}
		for i, station := range chain.stations {
			if station.Cmp(cuts[i+1]) != 0 {
				return zero, fmt.Errorf("the original roofs require the same model joints: %w", geom.ErrConvergence)
			}
		}
		planes, err := mappedSplineInteriorSupportKnotPlanes(patches[owner], 2*cells-2)
		if err != nil {
			return zero, err
		}
		ownerKnots[owner] = len(planes)
		for _, plane := range planes {
			var hitCells [2]int
			var hits [2]*geom.ExactComposedPlaneEvent
			count := 0
			for i, part := range chain.parts {
				h, err := mappedSplineSupportKnotPlaneHits(ctx, part.curve, plane, limits, events)
				if err != nil {
					return zero, fmt.Errorf("roof %d original cell %d: %w", owner, i, err)
				}
				if len(h.Events) == 0 {
					continue
				}
				if len(h.Events) != 1 || count == len(hits) || h.Events[0].Multiplicity() != 1 {
					return zero, fmt.Errorf("roof %d original support knot must cross once transversely: %w", owner, geom.ErrConvergence)
				}
				hitCells[count], hits[count] = i, h.Events[0]
				count++
			}
			i := hitCells[0]
			switch {
			case count == 2 && hitCells[1] == i+1 && hits[0].EndpointMask() == 2 && hits[1].EndpointMask() == 1:
				// Completeness gives a constant sign on each open adjacent
				// cell. Simple endpoint events of opposite signs prove
				// the crossing without adding a new model vertex.
				incoming := mappedSplineRawPlaneSign(chain.parts[i].curve.ends[0], plane)
				outgoing := mappedSplineRawPlaneSign(chain.parts[i+1].curve.ends[1], plane)
				if jointChanges[i][owner] || incoming*outgoing != -1 {
					return zero, fmt.Errorf("roof %d requires one transverse support line at original joint %d: %w", owner, i, geom.ErrConvergence)
				}
				jointChanges[i][owner] = true
			case count == 1 && hits[0].EndpointMask() == 0 && interior[i].ModelStation == nil:
				root, err := recoverMappedSplineSupportKnotRoot(ctx, hits[0], events)
				if err != nil {
					return zero, err
				}
				width := new(big.Rat).Sub(cuts[i+1], cuts[i])
				interior[i].ModelStation = new(big.Rat).Add(cuts[i], new(big.Rat).Mul(width, root))
				interior[i].OwnersChangingCells[owner] = true
				interiorOwners[owner] = true
			default:
				return zero, fmt.Errorf("roof %d support knot must belong to one original cell or joint: %w", owner, geom.ErrConvergence)
			}
		}
		if !asymmetric && !ordered {
			for i, changed := range jointChanges {
				if !changed[owner] {
					return zero, fmt.Errorf("roof %d did not cross support at original joint %d: %w", owner, i, geom.ErrConvergence)
				}
			}
		}
	}
	minKnots, maxKnots := min(ownerKnots[0], ownerKnots[1]), max(ownerKnots[0], ownerKnots[1])
	if ordered {
		if interiorOwners != [2]bool{true, true} {
			return zero, fmt.Errorf("ordered joined cells require interior events from both roofs: %w", geom.ErrConvergence)
		}
		for i, changes := range jointChanges {
			if changes == [2]bool{} {
				return zero, fmt.Errorf("original joint %d has no transverse support change: %w", i, geom.ErrConvergence)
			}
		}
	} else if asymmetric {
		shared := [2]bool{true, true}
		first, second := jointChanges[0], jointChanges[1]
		if minKnots != 2 || maxKnots != 4 || interiorOwners != shared || !(first == shared && second[0] != second[1] || second == shared && first[0] != first[1]) {
			return zero, fmt.Errorf("the original source requires one shared joint, one unilateral joint and interior events from both roofs: %w", geom.ErrConvergence)
		}
	} else if minKnots != cells || maxKnots != 2*cells-2 {
		return zero, fmt.Errorf("each roof must contribute its own interior event and cross every original joint: %w", geom.ErrConvergence)
	}
	out, repaired := make([]*big.Rat, 2*cells+1), make([]bool, 2*cells+1)
	eventsInOrder := make([]MappedSplineChainSupportEvent, 2*cells-1)
	for i, event := range interior {
		if event.ModelStation == nil {
			return zero, fmt.Errorf("original cell %d is missing its owner-side interior crossing: %w", i, geom.ErrConvergence)
		}
		out[2*i], out[2*i+1], repaired[2*i+1] = cuts[i], event.ModelStation, true
		eventsInOrder[2*i] = event
		if i < len(jointChanges) {
			eventsInOrder[2*i+1] = MappedSplineChainSupportEvent{
				ModelStation: cuts[i+1], OwnersChangingCells: jointChanges[i], OriginalJoint: true,
			}
		}
	}
	out[2*cells] = cuts[cells]
	return mappedSplineJoinedSupportPartition{cuts: out, inserted: repaired, events: eventsInOrder}, nil
}

func mappedSplineRawPlaneSign(p geom.Vec2, plane [4]*big.Rat) int {
	value := new(big.Rat).Set(plane[3])
	for axis, x := range [2]float64{p.X, p.Y} {
		if plane[axis] != nil {
			value.Add(value, new(big.Rat).Mul(plane[axis], ratInset(x)))
		}
	}
	return value.Sign()
}
