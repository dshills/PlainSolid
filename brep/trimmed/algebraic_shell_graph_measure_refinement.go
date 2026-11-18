//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// AlgebraicGraphMeasureRefinement retains independent proof partitions, never
// a fabricated merged classification. RunningVolume and RunningOwnerAreas
// intersect all certificates through each pass, so enclosure widths cannot
// increase. Passes may have different unresolved cells/areas. The immutable
// source preparation (including optional refined tube boxes) is shared; spatial
// classification is repeated and charged separately for each authored budget.
type AlgebraicGraphMeasureRefinement struct {
	Passes                                                           []AlgebraicGraphWholeMeasures
	RunningVolume                                                    [][2]*big.Rat
	RunningOwnerAreas                                                [][2][2]*big.Rat
	Volume, BoundaryArea                                             [2]*big.Rat
	OwnerAreas                                                       [2][2]*big.Rat
	CellAttempts, SurfaceCells, ExactOperations, SurfaceQuotaCharged int
}

// CompareWholeMeasureRefinements executes one to four explicit resolution
// budgets. Total work is bounded by their sum plus64 interval operations, not the final budget alone.
// Cancellation, unsupported geometry, exhaustion or inconsistent certificates
// fail atomically. No convergence rate or eventual zero unresolved area is
// promised. No polynomial moments are requested implicitly.
func (q *AlgebraicGraphMaterialQuery) CompareWholeMeasureRefinements(ctx context.Context, schedule []AlgebraicGraphWindowAreaOptions) (out AlgebraicGraphMeasureRefinement, err error) {
	defer func() {
		if err != nil {
			out = AlgebraicGraphMeasureRefinement{CellAttempts: out.CellAttempts, SurfaceCells: out.SurfaceCells, ExactOperations: out.ExactOperations, SurfaceQuotaCharged: out.SurfaceQuotaCharged}
		}
	}()
	if ctx == nil || len(schedule) < 1 || len(schedule) > 4 {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	out.ExactOperations = 64
	bitLimit := 0
	for _, o := range schedule {
		limits, e := graphMaterialLimits(o.Refinement.CellOptions.Exact)
		if e != nil {
			return out, e
		}
		if limits.MaxCoefficientBits > bitLimit {
			bitLimit = limits.MaxCoefficientBits
		}
	}
	add := func(total *int, n int) error {
		if n < 0 || n > int(^uint(0)>>1)-*total {
			return geom.ErrConvergence
		}
		*total += n
		return nil
	}
	intersect := func(a, b [2]*big.Rat) (r [2]*big.Rat, e error) {
		r = [2]*big.Rat{new(big.Rat).Set(b[0]), new(big.Rat).Set(b[1])}
		if a[0] != nil {
			if a[0].Cmp(r[0]) > 0 {
				r[0].Set(a[0])
			}
			if a[1].Cmp(r[1]) < 0 {
				r[1].Set(a[1])
			}
		}
		if r[0].Cmp(r[1]) > 0 {
			return r, geom.ErrConvergence
		}
		return r, nil
	}
	for _, options := range schedule {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		pass, e := q.BoundWholeMeasures(ctx, options)
		if e != nil {
			return out, e
		}
		if e := add(&out.CellAttempts, pass.Boundary.Partition.CellAttempts); e != nil {
			return out, e
		}
		if e := add(&out.SurfaceCells, pass.Boundary.SurfaceCells); e != nil {
			return out, e
		}
		if e := add(&out.ExactOperations, pass.DomainOperations); e != nil {
			return out, e
		}
		if e := add(&out.ExactOperations, pass.Boundary.Partition.ExactOperations); e != nil {
			return out, e
		}
		if e := add(&out.SurfaceQuotaCharged, pass.Boundary.SurfaceQuotaCharged); e != nil {
			return out, e
		}
		out.Volume, err = intersect(out.Volume, pass.Volume)
		if err != nil {
			return out, err
		}
		for owner := 0; owner < 2; owner++ {
			out.OwnerAreas[owner], err = intersect(out.OwnerAreas[owner], pass.Boundary.Owners[owner].Area)
			if err != nil {
				return out, err
			}
		}
		out.Passes = append(out.Passes, pass)
		out.RunningVolume = append(out.RunningVolume, [2]*big.Rat{new(big.Rat).Set(out.Volume[0]), new(big.Rat).Set(out.Volume[1])})
		var owners [2][2]*big.Rat
		for owner := 0; owner < 2; owner++ {
			for end := 0; end < 2; end++ {
				owners[owner][end] = new(big.Rat).Set(out.OwnerAreas[owner][end])
			}
		}
		out.RunningOwnerAreas = append(out.RunningOwnerAreas, owners)
	}
	for end := 0; end < 2; end++ {
		out.BoundaryArea[end] = new(big.Rat).Add(out.OwnerAreas[0][end], out.OwnerAreas[1][end])
		if !graphPolynomialBits(out.BoundaryArea[end], bitLimit) {
			return out, geom.ErrConvergence
		}
	}
	return out, nil
}
