package geom

import (
	"context"
	"math/big"
)

// IntersectWrappedAnisotropicHorizontalLineRational proves isolated contacts
// against the original weighted profile at a strictly interior height. Exact
// diagonal normalization preserves t. Source proof, normalization and all four
// conic root stages share one operation ledger; subdivisions are bounded per
// quadrant. Nonhorizontal and endpoint-plane lines are deliberately unsupported.
func IntersectWrappedAnisotropicHorizontalLineRational(ctx context.Context, w *WrappedSurfacePatch, origin, direction [3]*big.Rat, minT, maxT *big.Rat, o ToroidLineOptions) (CylinderLineIntersections, error) {
	empty := CylinderLineIntersections{}
	options, e := validateRationalCircularLine(ctx, origin[:], direction[:], minT, maxT, o)
	if e != nil {
		return empty, e
	}
	if direction[2].Sign() != 0 {
		return empty, ErrInvalid
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 1024
	}
	if o.MaxSubdivisions < 1 || o.MaxSubdivisions > 65536 {
		return empty, ErrInvalid
	}
	proof, e := CertifyWrappedAnisotropicEmbedding(ctx, w, options)
	if e != nil {
		return empty, e
	}
	if origin[2].Cmp(proof.LowerZ) <= 0 || origin[2].Cmp(proof.UpperZ) >= 0 {
		return empty, ErrInvalid
	}
	b := &wrappedAnisotropicBudget{ctx: ctx, work: polynomialBudget{operations: proof.ExactOperations, maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}}
	t := b.div(b.sub(origin[2], proof.LowerZ), b.sub(proof.UpperZ, proof.LowerZ))
	var xy [2]rationalPolynomial
	for axis := 0; axis < 2; axis++ {
		scale := b.add(proof.LowerScale[axis], b.mul(t, b.sub(proof.UpperScale[axis], proof.LowerScale[axis])))
		xy[axis] = rationalPolynomial{b.div(b.sub(origin[axis], proof.Center[axis]), scale), b.div(direction[axis], scale)}
	}
	if b.err != nil {
		return empty, b.err
	}
	result, e := intersectWeightedProfileConicLine(ctx, w, xy[0], xy[1], rationalPolynomial{big.NewRat(1, 1), new(big.Rat)}, minT, maxT, 0, &b.work, o)
	if e != nil {
		return empty, e
	}
	if len(result.Overlaps) > 0 {
		return empty, ErrConvergence
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	return result, nil
}
