package geom

import (
	"context"
	"math/big"
)

// ComposedAffineEdgeEvent proves a paired parameter identity at one model
// contact. CurveEvent carries exact model evaluation; LineRoot is the independent
// affine edge's original normalized parameter, including reversed orientation.
type ComposedAffineEdgeEvent struct {
	CurveEvent   *ExactComposedPlaneEvent
	LineRoot     *ExactAlgebraicRoot
	BoundaryMask uint8
}
type ComposedAffineEdgePortion struct{ Start, End ComposedAffineEdgeEvent }
type ComposedAffineEdgeIntersection struct {
	Complete, Coincident bool
	Hits                 []ComposedAffineEdgeEvent
	Portions             []ComposedAffineEdgePortion
	// LineParameter=Offset+Scale*curveParameter, proved exactly by projection.
	Offset, Scale            *big.Rat
	Operations, Subdivisions int
}

// IntersectExactComposedCurveAffineEdge certifies a restricted spatial curve
// against a single-span affine edge on [0,1]. At least one nonconstant line
// coordinate must be affine in the curve parameter. This sufficient projection
// identity gives both exact root identities without rounded model matching.
// Nonlinear curve coordinates orthogonal to that projection are unrestricted.
// Coincident portions have the same model image with a strictly monotone paired
// parameter map; Portions stay ordered in curve parameter, even when reversed.
func IntersectExactComposedCurveAffineEdge(ctx context.Context, cell *ExactComposedCurveCell, line *ExactSplineCurve, o ComposedCurveLineOptions) (r ComposedAffineEdgeIntersection, err error) {
	if ctx == nil || cell == nil || line == nil || line.patch == nil || len(cell.power[3]) == 0 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	options, e := graphRootOptions(o.ExactCurvePlaneEventOptions)
	if e != nil {
		return r, e
	}
	b := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	defer func() { r.Operations = b.operations }()
	net, ops, e := line.BezierControlNet(ctx, SurfacePatchOptions{MaxOperations: options.MaxOperations, MaxCoefficientBits: options.MaxCoefficientBits})
	b.operations += ops
	if e != nil {
		return r, e
	}
	if net.Start != 0 || net.End != 1 || len(net.Control) > 65 {
		return r, ErrConvergence
	}
	var power [4]rationalPolynomial
	for axis := 0; axis < 4; axis++ {
		values := make([]*big.Rat, len(net.Control))
		for i := range values {
			values[i] = net.Control[i][axis]
		}
		power[axis], e = bernsteinToPower(values, b)
		if e != nil {
			return r, e
		}
	}
	var origins, directions [3]*big.Rat
	for axis := 0; axis < 3; axis++ {
		quotient, remainder, e := polynomialDivide(power[axis], power[3], b)
		if e != nil {
			return r, e
		}
		if len(remainder) != 0 || len(quotient) > 2 {
			return r, ErrConvergence
		}
		origins[axis], directions[axis] = new(big.Rat), new(big.Rat)
		if len(quotient) > 0 {
			origins[axis].Set(quotient[0])
		}
		if len(quotient) > 1 {
			directions[axis].Set(quotient[1])
		}
	}
	for axis := 0; axis < 3; axis++ {
		if directions[axis].Sign() == 0 {
			continue
		}
		quotient, remainder, e := polynomialDivide(cell.power[axis], cell.power[3], b)
		if e != nil {
			return r, e
		}
		if len(remainder) != 0 || len(quotient) != 2 || quotient[1].Sign() == 0 {
			continue
		}
		r.Offset = new(big.Rat).Quo(new(big.Rat).Sub(quotient[0], origins[axis]), directions[axis])
		r.Scale = new(big.Rat).Quo(quotient[1], directions[axis])
		break
	}
	if r.Scale == nil {
		return r, ErrConvergence
	}
	for _, x := range []*big.Rat{r.Offset, r.Scale} {
		if e := b.check(x); e != nil {
			return r, e
		}
	}
	o.ExactCurvePlaneEventOptions = options
	o.MaxOperations -= b.operations
	if o.MaxOperations < 1 {
		return r, ErrConvergence
	}
	intersections, e := cell.IntersectRationalFiniteLine(ctx, origins, directions, [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}, o)
	b.operations += intersections.Operations
	r.Subdivisions = intersections.Subdivisions
	if e != nil {
		return r, e
	}
	inverse := new(big.Rat).Inv(r.Scale)
	origin := new(big.Rat).Neg(new(big.Rat).Mul(r.Offset, inverse))
	paired := func(event *ExactComposedPlaneEvent, mask uint8) (ComposedAffineEdgeEvent, error) {
		root, e := mapComposedGraphRoot(event.root, origin, inverse, r.Offset, r.Scale, b)
		if e != nil {
			return ComposedAffineEdgeEvent{}, e
		}
		if r.Scale.Sign() < 0 {
			root.bounds[0], root.bounds[1] = root.bounds[1], root.bounds[0]
		}
		return ComposedAffineEdgeEvent{CurveEvent: event, LineRoot: &ExactAlgebraicRoot{root: root}, BoundaryMask: mask}, nil
	}
	for _, hit := range intersections.Hits {
		p, e := paired(hit.Event, hit.BoundaryMask)
		if e != nil {
			return r, e
		}
		r.Hits = append(r.Hits, p)
	}
	for _, portion := range intersections.Portions {
		start, e := paired(portion.Start, 0)
		if e != nil {
			return r, e
		}
		end, e := paired(portion.End, 0)
		if e != nil {
			return r, e
		}
		r.Portions = append(r.Portions, ComposedAffineEdgePortion{start, end})
	}
	r.Coincident = intersections.Coincident
	r.Complete = intersections.Complete
	return r, nil
}
