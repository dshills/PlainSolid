package geom

import (
	"context"
	"math/big"
)

// ExactComposedCurveEvent is a model event with immutable algebraic parameter
// provenance. A bound arbitrary root has multiplicity zero and carries no
// assertion of an external plane contact.
type ExactComposedCurveEvent = ExactComposedPlaneEvent

func (e *ExactComposedPlaneEvent) Root() *ExactAlgebraicRoot {
	if e == nil || e.root == nil {
		return nil
	}
	return &ExactAlgebraicRoot{root: e.root}
}
func (c *ExactComposedCurveCell) EndpointRoot(endpoint int) (*ExactAlgebraicRoot, error) {
	if c == nil || endpoint < 0 || endpoint > 1 || c.endpoints[endpoint] == nil {
		return nil, ErrInvalid
	}
	return &ExactAlgebraicRoot{root: c.endpoints[endpoint]}, nil
}

// HasSurfaceCell distinguishes actual tensor-chart cells from direct Bézier
// descriptors, whose Cell() is the zero rectangle.
func (c *ExactComposedCurveCell) HasSurfaceCell() bool {
	return c != nil && c.cell.UMin < c.cell.UMax && c.cell.VMin < c.cell.VMax
}

type ExactComposedCellRestriction struct {
	Proven                   bool
	Cell                     *ExactComposedCurveCell
	Operations, Subdivisions int
}

func copyExactRoot(root *ExactGraphLineRoot) *ExactGraphLineRoot {
	out := &ExactGraphLineRoot{bounds: [2]*big.Rat{new(big.Rat).Set(root.bounds[0]), new(big.Rat).Set(root.bounds[1])}, polynomial: make(rationalPolynomial, len(root.polynomial))}
	for i, x := range root.polynomial {
		out.polynomial[i] = new(big.Rat).Set(x)
	}
	return out
}
func compareRootBudget(ctx context.Context, a, c *ExactGraphLineRoot, b *polynomialBudget, maxSubdivisions int) (ExactCurveEventOrder, error) {
	if maxSubdivisions < 1 {
		return ExactCurveEventOrder{}, ErrConvergence
	}
	for _, root := range []*ExactGraphLineRoot{a, c} {
		for _, x := range root.polynomial {
			if e := b.check(x); e != nil {
				return ExactCurveEventOrder{}, e
			}
		}
		for _, x := range root.bounds {
			if e := b.check(x); e != nil {
				return ExactCurveEventOrder{}, e
			}
		}
	}
	return compareExactPolynomialEvents(ctx, [2]rationalPolynomial{a.polynomial, c.polynomial}, [2][2]*big.Rat{a.bounds, c.bounds}, b, maxSubdivisions)
}

// RestrictAlgebraic proves strictly ordered cuts inside the original true domain
// and retains the same global-parameter continuation. It introduces no rounded
// knots or local reparametrization. Imported roots must first be validated by
// ImportExactRoot; source events may supply Root() directly.
func (c *ExactComposedCurveCell) RestrictAlgebraic(ctx context.Context, lower, upper *ExactAlgebraicRoot, o ExactCurvePlaneEventOptions) (r ExactComposedCellRestriction, err error) {
	if ctx == nil || c == nil || c.endpoints[0] == nil || c.endpoints[1] == nil || lower == nil || upper == nil || lower.root == nil || upper.root == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	o, e := graphRootOptions(o)
	if e != nil {
		return r, e
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.Operations = b.operations }()
	cuts := [2]*ExactGraphLineRoot{copyExactRoot(lower.root), copyExactRoot(upper.root)}
	comparison, e := compareRootBudget(ctx, cuts[0], cuts[1], b, o.MaxSubdivisions)
	r.Subdivisions += comparison.Subdivisions
	if e != nil {
		return r, e
	}
	if !comparison.Proven || comparison.Order != -1 {
		return r, ErrDomain
	}
	cuts[0].bounds = comparison.Intervals[0]
	cuts[1].bounds = comparison.Intervals[1]
	for side := 0; side < 2; side++ {
		comparison, e := compareRootBudget(ctx, cuts[side], c.endpoints[side], b, o.MaxSubdivisions-r.Subdivisions)
		r.Subdivisions += comparison.Subdivisions
		if e != nil {
			return r, e
		}
		if !comparison.Proven {
			return r, ErrConvergence
		}
		if side == 0 && comparison.Order < 0 || side == 1 && comparison.Order > 0 {
			return r, ErrDomain
		}
		cuts[side].bounds = comparison.Intervals[0]
	}
	copy := *c
	copy.endpoints = cuts
	r.Cell = &copy
	r.Proven = true
	return r, nil
}

// BindRoot proves membership of an arbitrary validated root and attaches the
// current immutable model continuation. The returned event can Enclose its
// actual model point; numeric root validity alone never bypasses domain checks.
func (c *ExactComposedCurveCell) BindRoot(ctx context.Context, root *ExactAlgebraicRoot, o ExactCurvePlaneEventOptions) (*ExactComposedCurveEvent, error) {
	if ctx == nil || c == nil || c.endpoints[0] == nil || c.endpoints[1] == nil || root == nil || root.root == nil {
		return nil, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	o, e := graphRootOptions(o)
	if e != nil {
		return nil, e
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	subdivisions := 0
	owned := copyExactRoot(root.root)
	mask := uint8(0)
	for side, endpoint := range c.endpoints {
		comparison, e := compareRootBudget(ctx, owned, endpoint, b, o.MaxSubdivisions-subdivisions)
		subdivisions += comparison.Subdivisions
		if e != nil {
			return nil, e
		}
		if !comparison.Proven {
			return nil, ErrConvergence
		}
		if side == 0 && comparison.Order < 0 || side == 1 && comparison.Order > 0 {
			return nil, ErrDomain
		}
		if comparison.Order == 0 {
			mask |= 1 << uint(side)
		}
		owned.bounds = comparison.Intervals[0]
	}
	return &ExactComposedCurveEvent{root: owned, cell: c, endpointMask: mask}, nil
}

// NewExactComposedBezierCell creates a normalized [0,1] direct exact model
// descriptor from one positive rational Bézier. It also supports native exact UV
// quadratics (X/Y with Z=0) without manufacturing a tensor support. Positive
// Bernstein weights prove the denominator positive throughout the full domain.
func NewExactComposedBezierCell(ctx context.Context, curve *ExactSplineCurve, o SurfacePatchOptions) (*ExactComposedCurveCell, error) {
	result, _, e := NewExactComposedBezierCellWithWork(ctx, curve, o)
	return result, e
}

// NewExactComposedBezierCellWithWork is NewExactComposedBezierCell with
// the exact Bernstein-to-power operation count, including on failure.
func NewExactComposedBezierCellWithWork(ctx context.Context, curve *ExactSplineCurve, o SurfacePatchOptions) (_ *ExactComposedCurveCell, work int, err error) {
	if ctx == nil || curve == nil || curve.patch == nil {
		return nil, 0, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, 0, e
	}
	o, e := patchOptions(o)
	if e != nil {
		return nil, 0, e
	}
	p := curve.patch
	lo, hi := curve.Domain()
	if lo != 0 || hi != 1 || p.countU != p.degreeU+1 || p.degreeU > 64 || len(p.net) > o.MaxPoles {
		return nil, 0, ErrConvergence
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { work = b.operations }()
	result := &ExactComposedCurveCell{}
	for axis := 0; axis < 4; axis++ {
		values := make([]*big.Rat, p.countU)
		for i := range values {
			h := p.net[i*p.countV]
			if h[3].Sign() <= 0 {
				return nil, 0, ErrInvalid
			}
			values[i] = h[axis]
		}
		result.power[axis], e = bernsteinToPower(values, b)
		if e != nil {
			return nil, 0, e
		}
	}
	result.endpoints = [2]*ExactGraphLineRoot{{polynomial: rationalPolynomial{new(big.Rat), big.NewRat(1, 1)}, bounds: [2]*big.Rat{new(big.Rat), new(big.Rat)}}, {polynomial: rationalPolynomial{big.NewRat(-1, 1), big.NewRat(1, 1)}, bounds: [2]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 1)}}}
	return result, 0, nil
}
