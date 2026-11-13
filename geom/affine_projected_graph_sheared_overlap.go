package geom

import (
	"context"
	"math/big"
)

// ShearedProjectedGraphOverlap retains paired original chart polygons and
// exact original XYZ at their vertices. Vertex order is counterclockwise in A,
// not necessarily in the selected physical projection. ModelCorners describe
// the projected footprint; they are not a bound on the residual coordinate.
// They are populated only when Coincident is proved. ProjectionOrientation
// refers to each original chart's Jacobian in the chosen ordered physical axes.
type ShearedProjectedGraphOverlap struct {
	ShearedGraphOverlap
	ProjectionAxes [2]int
	ResidualAxis   int
	ModelCorners   [][3]*big.Rat
}

// CertifyShearedProjectedGraphOverlap proves partial graph coincidence on any
// ordered pair of distinct model coordinate axes, without rounding or changing
// the input charts. It retains CertifyShearedAffineXYGraphOverlap's supported domain
// and its distinction between no area and unresolved mixed contact.
func CertifyShearedProjectedGraphOverlap(ctx context.Context, a, c *SurfacePatch, axes [2]int, o SurfacePatchOptions) (out ShearedProjectedGraphOverlap, err error) {
	if ctx == nil || a == nil || c == nil {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	order, e := modelProjectionPermutation(axes)
	if e != nil {
		return out, e
	}
	options, e := patchOptions(o)
	if e != nil {
		return out, e
	}
	out.ProjectionAxes = axes
	out.ResidualAxis = order[2]
	budget := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	defer func() {
		out.ExactOperations = budget.operations
		if err != nil {
			out.Coincident = false
		}
	}()
	pa, e := permuteGraphPatch(a, order, budget)
	if e != nil {
		return out, e
	}
	pc, e := permuteGraphPatch(c, order, budget)
	if e != nil {
		return out, e
	}
	remaining := options.MaxOperations - budget.operations
	if remaining < 1 {
		return out, ErrConvergence
	}
	child := options
	child.MaxOperations = remaining
	out.ShearedGraphOverlap, e = CertifyShearedAffineXYGraphOverlap(ctx, pa, pc, child)
	budget.operations += out.ExactOperations
	if e != nil {
		return out, e
	}
	if !out.Coincident {
		return out, nil
	}
	out.ModelCorners = make([][3]*big.Rat, len(out.A))
	for i, uv := range out.A {
		remaining = options.MaxOperations - budget.operations
		if remaining < 1 {
			return out, ErrConvergence
		}
		jet, e := a.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: options.MaxCoefficientBits})
		budget.operations += jet.ExactOperations
		if e != nil {
			return out, e
		}
		out.ModelCorners[i] = jet.Point
	}
	return out, nil
}
