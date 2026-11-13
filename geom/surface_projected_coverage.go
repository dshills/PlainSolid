package geom

import "context"

// CertifyProjectedAffineSurfacePairCellInTubes transfers coverage through an
// exact model-coordinate permutation and represented affine chart alignment.
// Candidate/tube chart coordinates remain original. All tubes are re-proved by
// the underlying certificate, and mutable supplied regions never imply proof.
// Diagnostic boxes use the first chart's aligned coordinates, as in the XY API.
func CertifyProjectedAffineSurfacePairCellInTubes(ctx context.Context, a, b *SurfacePatch, cell SurfacePairParameterBox, tubes []SurfacePairTubeRegion, axes [2]int, o SurfaceTubeUnionCoverageOptions) (r SurfaceTubeUnionCoverage, err error) {
	if ctx == nil || a == nil || b == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	order, e := modelProjectionPermutation(axes)
	if e != nil {
		return r, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations += budget.operations }()
	pa, e := permuteGraphPatch(a, order, budget)
	if e != nil {
		return r, e
	}
	pb, e := permuteGraphPatch(b, order, budget)
	if e != nil {
		return r, e
	}
	o.MaxOperations -= budget.operations
	if o.MaxOperations < 1 {
		return r, ErrConvergence
	}
	return CertifyAffineXYSurfacePairCellInTubes(ctx, pa, pb, cell, tubes, o)
}
