package geom

import (
	"context"
	"math/big"
)

// ModelCurveCells constructs a piece's exact original source iso in its original
// varying-coordinate units. The immutable chain already proves this whole iso
// lies on the target support throughout the retained domain. Both model endpoint
// identities are additionally compared independently after composition. It does
// not round the free-coordinate restriction or introduce legacy tube semantics.
func (c *RationalIsoTubeChain) ModelCurveCells(ctx context.Context, index int, o SurfaceCurveAgreementOptions) (SurfaceCurveCompositionCells, error) {
	var empty SurfaceCurveCompositionCells
	if ctx == nil || c == nil || index < 0 || index >= len(c.pieces) {
		return empty, ErrInvalid
	}
	opts, err := patchOptions(o.SurfacePatchOptions)
	if err != nil {
		return empty, err
	}
	o.SurfacePatchOptions = opts
	p := c.pieces[index]
	origin := [2]*big.Rat{new(big.Rat), new(big.Rat)}
	direction := [2]*big.Rat{new(big.Rat), new(big.Rat)}
	origin[c.fixedAxis].Set(c.fixed)
	direction[1-c.fixedAxis].SetInt64(1)
	r, err := c.a.BuildAffineSection(ctx, origin, direction, p.endpoints[0].Parameters[1-c.fixedAxis], p.endpoints[1].Parameters[1-c.fixedAxis], o)
	if err != nil {
		return empty, err
	}
	if !r.Complete || len(r.Cells) == 0 {
		return empty, ErrConvergence
	}
	used := r.ExactOperations
	for end, cell := range [2]*ExactComposedCurveCell{r.Cells[0], r.Cells[len(r.Cells)-1]} {
		remaining := opts.MaxOperations - used
		if remaining < 1 {
			return empty, ErrConvergence
		}
		event := &ExactComposedPlaneEvent{cell: cell, root: cell.endpoints[end], endpointMask: 1 << end}
		same, n, err := event.CompareModelPoint(ctx, p.endpoints[end].ModelEvent, ExactCurvePlaneEventOptions{MaxOperations: remaining, MaxCoefficientBits: opts.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions})
		used += n
		if err != nil {
			return empty, err
		}
		if !same {
			return empty, ErrDomain
		}
	}
	r.ExactOperations = used
	return r, nil
}
