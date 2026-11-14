package geom

import (
	"context"
	"math/big"
)

// ProjectedQuarticGramContact is a re-proved isolated point contact over the
// complete matching projected rectangles. Parameters and Tangent use A's
// original UV; TargetParameters and TargetDomain retain B's original chart.
// Point and Tangent are in original XYZ. Side multiplies A-B in ResidualAxis.
// This is geometric contact evidence, not a trimmed Boolean or topology result.
type ProjectedQuarticGramContact struct {
	QuarticGramGraphContact
	TargetParameters [2]*big.Rat
	TargetDomain     ParameterRectangle
	ProjectionAxes   [2]int
	ResidualAxis     int
}

// CertifyProjectedQuarticGramGraphPointContact accepts an explicit exact A UV
// witness and Gram split, never a trusted proposal flag. Both original affine
// projections, exact chart alignment and the complete quartic identity are
// re-proved. Signed-axis swaps/scales are supported; partial projected overlap,
// shear, nonrepresentable remapped knots and boundary witnesses fail closed.
// The embedded coefficients remain in normalized A UV, not physical XYZ.
func CertifyProjectedQuarticGramGraphPointContact(ctx context.Context, a, c *SurfacePatch, axes [2]int, uv [2]*big.Rat, cross *big.Rat, o GraphPointContactOptions) (r ProjectedQuarticGramContact, err error) {
	if ctx == nil || a == nil || c == nil || uv[0] == nil || uv[1] == nil || cross == nil {
		return r, ErrInvalid
	}
	if err = ctx.Err(); err != nil {
		return r, err
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxOperations > 128000000 || o.MaxCoefficientBits < 64 || o.MaxCoefficientBits > 16384 || o.MaxCells < 0 || o.MaxCells > 65536 || o.MaxDepth < 0 || o.MaxDepth > 128 {
		return r, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() {
		if err != nil {
			r = ProjectedQuarticGramContact{}
		}
		r.ExactOperations = b.operations
	}()
	for _, q := range []*big.Rat{uv[0], uv[1], cross} {
		if err = b.check(q); err != nil {
			return r, err
		}
	}
	order, e := modelProjectionPermutation(axes)
	if e != nil {
		return r, e
	}
	pa, e := permuteGraphPatch(a, order, b)
	if e != nil {
		return r, e
	}
	pc, e := permuteGraphPatch(c, order, b)
	if e != nil {
		return r, e
	}
	remaining := o.MaxOperations - b.operations
	if remaining < 1 {
		return r, ErrConvergence
	}
	alignment, work, e := AlignAffineXYChartsWithWork(ctx, pa, pc, SurfacePatchOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
	b.operations += work
	if e != nil {
		return r, e
	}
	remaining = o.MaxOperations - b.operations
	if remaining < 1 {
		return r, ErrConvergence
	}
	local := o
	local.MaxOperations = remaining
	proof, e := CertifyQuarticGramGraphPointContact(ctx, pa, alignment.target, uv, cross, local)
	b.operations += proof.ExactOperations
	if e != nil {
		return r, e
	}
	r.QuarticGramGraphContact = proof
	var point [3]*big.Rat
	var tangent [2][3]*big.Rat
	for i, j := range order {
		point[j] = proof.Point[i]
		for k := range tangent {
			tangent[k][j] = proof.Tangent[k][i]
		}
	}
	r.Point = point
	r.Tangent = tangent
	r.ProjectionAxes = axes
	r.ResidualAxis = order[2]
	r.Graph.ProjectionAxes = axes
	r.TargetDomain = c.Domain()
	for i := range r.TargetParameters {
		value := new(big.Rat).Set(alignment.mapping[i][2])
		for j, q := range uv {
			term := new(big.Rat).Mul(alignment.mapping[i][j], q)
			if e = b.check(term); e != nil {
				return r, e
			}
			value.Add(value, term)
			if e = b.check(value); e != nil {
				return r, e
			}
		}
		r.TargetParameters[i] = value
	}
	return r, nil
}
