package geom

import (
	"context"
	"math/big"
)

// AffineProjectedGraphOverlap preserves the original input chart coordinates.
// ModelCorners are exact original XYZ values at overlap corners in source-UV
// counterclockwise order (lower/lower,upper/lower,upper/upper,lower/upper).
// They describe the exact projected footprint, not an XYZ bounding box: the
// graph's residual coordinate may have interior extrema. Corners are populated
// only for Coincident. ProjectionOrientation refers to the selected ordered
// physical axes; Orientation still maps the original A chart to the B chart.
type AffineProjectedGraphOverlap struct {
	AffineXYGraphOverlap
	ProjectionAxes [2]int
	ResidualAxis   int
	ModelCorners   [4][3]*big.Rat
}

// CertifyAffineProjectedGraphOverlap proves partial graph coincidence on any
// ordered pair of distinct model coordinate axes, without rounding or changing
// the input charts. It retains CertifyAffineXYGraphOverlap's supported domain
// and its distinction between no area and unresolved mixed contact.
func CertifyAffineProjectedGraphOverlap(ctx context.Context, a, c *SurfacePatch, axes [2]int, o SurfacePatchOptions) (out AffineProjectedGraphOverlap, err error) {
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
	out.AffineXYGraphOverlap, e = CertifyAffineXYGraphOverlap(ctx, pa, pc, child)
	budget.operations += out.ExactOperations
	if e != nil {
		return out, e
	}
	if !out.Coincident {
		return out, nil
	}
	indices := [4][2]int{{0, 0}, {1, 0}, {1, 1}, {0, 1}}
	for i, ij := range indices {
		remaining = options.MaxOperations - budget.operations
		if remaining < 1 {
			return out, ErrConvergence
		}
		jet, e := a.EvaluateRational(ctx, out.A[0][ij[0]], out.A[1][ij[1]], SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: options.MaxCoefficientBits})
		budget.operations += jet.ExactOperations
		if e != nil {
			return out, e
		}
		out.ModelCorners[i] = jet.Point
	}
	return out, nil
}
