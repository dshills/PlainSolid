package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/geom"
)

func mappedSplineJoinedSupportKnotCuts(ctx context.Context, chains [2]MappedSplineChain, patches [2]*geom.SurfacePatch, cuts []*big.Rat, limits geom.SurfacePatchOptions, events geom.ExactCurvePlaneEventOptions) ([]*big.Rat, []bool, error) {
	cells := len(cuts) - 1
	if cells < 2 || cells > 3 {
		return nil, nil, geom.ErrConvergence
	}
	var interior [3]*big.Rat
	var ownerKnots [2]int
	for owner, chain := range chains {
		if len(chain.parts) != cells {
			return nil, nil, fmt.Errorf("the original roofs require the same model cells: %w", geom.ErrConvergence)
		}
		for i, station := range chain.stations {
			if station.Cmp(cuts[i+1]) != 0 {
				return nil, nil, fmt.Errorf("the original roofs require the same model joints: %w", geom.ErrConvergence)
			}
		}
		planes, err := mappedSplineInteriorSupportKnotPlanes(patches[owner], 2*cells-2)
		if err != nil {
			return nil, nil, err
		}
		ownerKnots[owner] = len(planes)
		var shared [2]bool
		for _, plane := range planes {
			var hitCells [2]int
			var hits [2]*geom.ExactComposedPlaneEvent
			count := 0
			for i, part := range chain.parts {
				h, err := mappedSplineSupportKnotPlaneHits(ctx, part.curve, plane, limits, events)
				if err != nil {
					return nil, nil, fmt.Errorf("roof %d original cell %d: %w", owner, i, err)
				}
				if len(h.Events) == 0 {
					continue
				}
				if len(h.Events) != 1 || count == len(hits) || h.Events[0].Multiplicity() != 1 {
					return nil, nil, fmt.Errorf("roof %d original support knot must cross once transversely: %w", owner, geom.ErrConvergence)
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
				if shared[i] || incoming*outgoing != -1 {
					return nil, nil, fmt.Errorf("roof %d requires one transverse support line at original joint %d: %w", owner, i, geom.ErrConvergence)
				}
				shared[i] = true
			case count == 1 && hits[0].EndpointMask() == 0 && interior[i] == nil:
				root, err := recoverMappedSplineSupportKnotRoot(ctx, hits[0], events)
				if err != nil {
					return nil, nil, err
				}
				width := new(big.Rat).Sub(cuts[i+1], cuts[i])
				interior[i] = new(big.Rat).Add(cuts[i], new(big.Rat).Mul(width, root))
			default:
				return nil, nil, fmt.Errorf("roof %d support knot must belong to one original cell or joint: %w", owner, geom.ErrConvergence)
			}
		}
		for i, proved := range shared[:cells-1] {
			if !proved {
				return nil, nil, fmt.Errorf("roof %d did not cross support at original joint %d: %w", owner, i, geom.ErrConvergence)
			}
		}
	}
	if min(ownerKnots[0], ownerKnots[1]) != cells || max(ownerKnots[0], ownerKnots[1]) != 2*cells-2 {
		return nil, nil, fmt.Errorf("each roof must contribute its own interior event and cross every original joint: %w", geom.ErrConvergence)
	}
	out, repaired := make([]*big.Rat, 2*cells+1), make([]bool, 2*cells+1)
	for i, station := range interior[:cells] {
		if station == nil {
			return nil, nil, fmt.Errorf("original cell %d is missing its owner-side interior crossing: %w", i, geom.ErrConvergence)
		}
		out[2*i], out[2*i+1], repaired[2*i+1] = cuts[i], station, true
	}
	out[2*cells] = cuts[cells]
	return out, repaired, nil
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
