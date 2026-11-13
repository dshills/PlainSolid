package trimvalidate

import "context"

// RingPlacement identifies a directed ring's immediate enclosing ring by
// input index (-1 for an exterior ring). Depth counts strict ancestors.
// Orientation is +1 for counterclockwise and -1 for clockwise.
type RingPlacement struct {
	Parent, Depth, Orientation int
}

// ClassifyRingForest proves each exact ring simple and every ring pair
// disjoint. Winding then determines the complete nesting forest, independently
// of the input order or orientations. It does not prescribe material: callers
// can require alternating orientation or use their own winding rule.
// Default Limits apply globally across rings, and failed proofs return no
// placements. An empty input is a certified empty forest.
func ClassifyRingForest(ctx context.Context, rings []Loop, o Options) ([]RingPlacement, Report, error) {
	if len(rings) == 0 {
		r := emptyLoopReport()
		if _, e := o.WithDefaults(); e != nil || ctx == nil {
			return nil, r, ErrOptions
		}
		if e := ctx.Err(); e != nil {
			return nil, r, e
		}
		r.Status = Certified
		return nil, r, nil
	}
	var out []RingPlacement
	r, _, e := validateLoops(ctx, rings[0], rings[1:], nil, o, nil, &out)
	if e != nil || r.Status != Certified {
		return nil, r, e
	}
	return out, r, nil
}

func classifyRingForest(loops [][]curve, signs []int, r *Report, b *budget) ([]RingPlacement, Status, string) {
	parents := make([][]int, len(loops))
	for i, loop := range loops {
		r.LoopA, r.EdgeA = i, 0
		for j, other := range loops {
			if i == j {
				continue
			}
			r.LoopB, r.EdgeB = j, -1
			w, ok := winding(loop[0].p[0], other, b)
			if !ok {
				return nil, Unresolved, "containment proof budget exhausted"
			}
			if w == signs[j] {
				parents[i] = append(parents[i], j)
			} else if w != 0 {
				return nil, Unresolved, "unexpected winding for a simple ring"
			}
		}
	}
	out := make([]RingPlacement, len(loops))
	for i, ancestors := range parents {
		out[i] = RingPlacement{Parent: -1, Depth: len(ancestors), Orientation: signs[i]}
		// Since the ring boundaries are disjoint Jordan curves, all
		// containing rings form a strict chain ordered by depth.
		for _, j := range ancestors {
			if !b.tick() {
				return nil, Unresolved, "containment proof budget exhausted"
			}
			if len(parents[j]) == len(ancestors)-1 {
				if out[i].Parent >= 0 {
					return nil, Unresolved, "ambiguous immediate parent"
				}
				out[i].Parent = j
			}
		}
		if len(ancestors) > 0 && out[i].Parent < 0 {
			return nil, Unresolved, "missing immediate parent"
		}
	}
	return out, Certified, ""
}
