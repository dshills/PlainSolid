package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/geom"
)

func mappedSplineJoinedSupportKnotCuts(ctx context.Context, chains [2]MappedSplineChain, patches [2]*geom.SurfacePatch, cuts []*big.Rat, limits geom.SurfacePatchOptions, events geom.ExactCurvePlaneEventOptions) ([]*big.Rat, []bool, error) {
	if len(cuts) != 3 || len(chains[0].parts) != 2 || len(chains[1].parts) != 2 ||
		chains[0].stations[0].Cmp(chains[1].stations[0]) != 0 {
		return nil, nil, fmt.Errorf("joined support repair requires two original cells sharing one joint: %w", geom.ErrConvergence)
	}
	var interior [2]*big.Rat
	for owner, patch := range patches {
		planes, err := mappedSplineInteriorSupportKnotPlanes(patch, 2)
		if err != nil {
			return nil, nil, err
		}
		if len(planes) != 2 {
			return nil, nil, fmt.Errorf("roof %d requires two original support knots: %w", owner, geom.ErrConvergence)
		}
		curves := [2]ExactBezierUV{chains[owner].parts[0].curve, chains[owner].parts[1].curve}
		joint, crossed := false, false
		for _, plane := range planes {
			var hits [2][]*geom.ExactComposedPlaneEvent
			for cell, curve := range curves {
				h, err := mappedSplineSupportKnotPlaneHits(ctx, curve, plane, limits, events)
				if err != nil {
					return nil, nil, fmt.Errorf("roof %d original cell %d: %w", owner, cell, err)
				}
				hits[cell] = h.Events
			}
			before, after := hits[0], hits[1]
			if len(before) == 1 && len(after) == 1 && before[0].EndpointMask() == 2 && after[0].EndpointMask() == 1 {
				// Completeness gives one constant sign on each open cell.
				// Simple endpoint events and opposite signs prove a
				// transverse crossing at the preexisting model joint.
				incoming := mappedSplineRawPlaneSign(curves[0].ends[0], plane)
				outgoing := mappedSplineRawPlaneSign(curves[1].ends[1], plane)
				if joint || before[0].Multiplicity() != 1 || after[0].Multiplicity() != 1 || incoming*outgoing != -1 {
					return nil, nil, fmt.Errorf("roof %d requires exactly one transverse support line at the original joint: %w", owner, geom.ErrConvergence)
				}
				joint = true
				continue
			}
			if crossed || len(before)+len(after) != 1 {
				return nil, nil, fmt.Errorf("roof %d requires one support event at the original joint and one in its own cell: %w", owner, geom.ErrConvergence)
			}
			cell := 0
			if len(before) == 0 {
				cell = 1
			}
			hit := hits[cell][0]
			if hit.EndpointMask() != 0 || hit.Multiplicity() != 1 || interior[cell] != nil {
				return nil, nil, fmt.Errorf("the two original cells require opposite transverse owner events: %w", geom.ErrConvergence)
			}
			root, err := recoverMappedSplineSupportKnotRoot(ctx, hit, events)
			if err != nil {
				return nil, nil, err
			}
			width := new(big.Rat).Sub(cuts[cell+1], cuts[cell])
			interior[cell] = new(big.Rat).Add(cuts[cell], new(big.Rat).Mul(width, root))
			crossed = true
		}
		if !joint || !crossed {
			return nil, nil, fmt.Errorf("roof %d requires both original support events: %w", owner, geom.ErrConvergence)
		}
	}
	return []*big.Rat{cuts[0], interior[0], cuts[1], interior[1], cuts[2]}, []bool{false, true, false, true, false}, nil
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
