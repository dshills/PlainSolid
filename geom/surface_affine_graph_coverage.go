package geom

import (
	"context"
)

// CertifyAffineXYSurfacePairCellInTubes proves contact coverage after an exact
// rectangular affine chart change. Both candidate and tube boxes must map with
// represented endpoints; no outward tube expansion is accepted. Every original
// and mapped tube is independently re-proved. Diagnostic boxes use aligned
// chart units. Boundary anchors are unsupported.
func CertifyAffineXYSurfacePairCellInTubes(ctx context.Context, a, b *SurfacePatch, cell SurfacePairParameterBox, tubes []SurfacePairTubeRegion, o SurfaceTubeUnionCoverageOptions) (r SurfaceTubeUnionCoverage, err error) {
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	used := 0
	defer func() { r.ExactOperations += used }()
	remaining := func() (int, error) {
		n := o.MaxOperations - used
		if n < 1 {
			return 0, ErrConvergence
		}
		return n, nil
	}
	forward, e := AlignAffineXYCharts(ctx, a, b, SurfacePatchOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return r, e
	}
	used += forward.ExactOperations()
	n, e := remaining()
	if e != nil {
		return r, e
	}
	inverse, e := AlignAffineXYCharts(ctx, b, a, SurfacePatchOptions{MaxOperations: n, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return r, e
	}
	used += inverse.ExactOperations()
	mapBox := func(box ParameterRectangle) (ParameterRectangle, error) {
		n, e := remaining()
		if e != nil {
			return ParameterRectangle{}, e
		}
		mapped, exact, ops, e := inverse.MapSourceBox(ctx, box, SurfacePatchOptions{MaxOperations: n, MaxCoefficientBits: o.MaxCoefficientBits})
		used += ops
		if e != nil {
			return mapped, e
		}
		if !exact {
			return mapped, ErrUnrepresentable
		}
		return mapped, nil
	}
	mappedCell := cell
	mappedCell.B, e = mapBox(cell.B)
	if e != nil {
		return r, e
	}
	mapped := make([]SurfacePairTubeRegion, 0, len(tubes))
	for _, tube := range tubes {
		if tube.Boundary != nil {
			return r, ErrConvergence
		}
		n, e := remaining()
		if e != nil {
			return r, e
		}
		budget := &polynomialBudget{maxOperations: n, maxBits: o.MaxCoefficientBits}
		pa, e := trimPatchBudget(a, tube.Box.A, budget)
		if e != nil {
			return r, e
		}
		pb, e := trimPatchBudget(b, tube.Box.B, budget)
		if e != nil {
			return r, e
		}
		used += budget.operations
		n, e = remaining()
		if e != nil {
			return r, e
		}
		original, e := CertifySurfacePairTube(pa, pb, SurfacePairTubeOptions{FreeAxis: tube.FreeAxis, MaxOperations: n, MaxCoefficientBits: o.MaxCoefficientBits})
		used += original.Test.ExactOperations
		if e != nil {
			return r, e
		}
		if !original.ProvenBranch {
			return r, ErrConvergence
		}
		q := tube
		q.Box.B, e = mapBox(tube.Box.B)
		if e != nil {
			return r, e
		}
		if q.FreeAxis >= 2 {
			axis := q.FreeAxis - 2
			newAxis := 0
			if inverse.mapping[0][axis].Sign() == 0 {
				newAxis = 1
			}
			q.FreeAxis = 2 + newAxis
		}
		mapped = append(mapped, q)
	}
	n, e = remaining()
	if e != nil {
		return r, e
	}
	child := o
	child.MaxOperations = n
	return CertifySurfacePairCellInTubesContext(ctx, a, forward.TargetOnSource(), mappedCell, mapped, child)
}
