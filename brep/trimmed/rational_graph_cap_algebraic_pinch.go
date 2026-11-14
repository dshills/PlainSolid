package trimmed

import (
	"math/big"
	"slices"

	"plainsolid/geom"
)

// exactVerticalPinchOwners matches both cap trims to the same exact spatial
// point. Since X and Y determine a unique point on each graph, and both cap
// equations are known to hold there, the simultaneous coordinate constraints
// prove full spatial identity. A seam and all original wall occurrences must
// be found on both independently sewed caps.
func (m AlgebraicGraphVerticalMaterial) exactVerticalPinchOwners(p [3]*big.Rat, b *capClipBudget) ([2]int, error) {
	empty := [2]int{-1, -1}
	q := big.NewRat
	planes := [2][4]*big.Rat{
		{q(1, 1), nil, nil, new(big.Rat).Neg(p[0])},
		{nil, q(1, 1), nil, new(big.Rat).Neg(p[1])},
	}
	var faces [2][]int
	seams := [2]AlgebraicGraphCapSeamOwner{{-1, -1}, {-1, -1}}
	for i, cap := range m.caps {
		for wi, wire := range cap.wires {
			for ui, use := range wire {
				h, err := use.Cell.IntersectRawPlanePair(b.ctx, planes, b.limits())
				if err != nil {
					return empty, err
				}
				if err = b.charge(h.Operations, h.Subdivisions); err != nil {
					return empty, err
				}
				if !h.Complete || h.Coincident || len(h.Events) > 1 {
					return empty, geom.ErrConvergence
				}
				if len(h.Events) == 0 {
					continue
				}
				if use.Seam >= 0 {
					if seams[i].Wire >= 0 || h.Events[0].EndpointMask() > 2 {
						return empty, geom.ErrConvergence
					}
					seams[i] = AlgebraicGraphCapSeamOwner{wi, ui}
					continue
				}
				if use.Source.Face < 0 || h.Events[0].EndpointMask() == 0 || h.Events[0].EndpointMask() > 2 {
					return empty, geom.ErrConvergence
				}
				if !slices.Contains(faces[i], use.Source.Face) {
					faces[i] = append(faces[i], use.Source.Face)
				}
			}
		}
		slices.Sort(faces[i])
	}
	if seams[0].Wire < 0 || seams[1].Wire < 0 ||
		len(faces[0]) > len(empty) || !slices.Equal(faces[0], faces[1]) {
		return empty, geom.ErrConvergence
	}
	paired := false
	for _, seam := range m.seams {
		if err := b.charge(1, 0); err != nil {
			return empty, err
		}
		if seam.Owners == seams {
			paired = true
			break
		}
	}
	if !paired {
		return empty, geom.ErrConvergence
	}
	copy(empty[:], faces[0])
	return empty, nil
}
