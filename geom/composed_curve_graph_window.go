package geom

import (
	"context"
	"errors"
	"math/big"
)

// IntersectExactComposedCurveIdentityGraph uses an exact rational outer window
// of the actual algebraic domain. All returned parameters and polynomial root
// identities retain original t units. No continuation outside that window need
// lie in the graph. Strict-interior windows are refined on bounded retries;
// boundary projections or nonpositive Bernstein weights may remain unresolved.
func IntersectExactComposedCurveIdentityGraph(ctx context.Context, cell *ExactComposedCurveCell, surface *SurfacePatch, o SurfaceCurveAgreementOptions) (r IdentityGraphCurveIntersection, err error) {
	if ctx == nil || cell == nil || surface == nil || cell.endpoints[0] == nil || cell.endpoints[1] == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	options, e := patchOptions(o.SurfacePatchOptions)
	if e != nil {
		return r, e
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 8192
	}
	if o.MaxSubdivisions < 1 {
		return r, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	steps := 0
	defer func() { r.ExactOperations = b.operations; r.Subdivisions += steps }()
	roots := [2]*ExactGraphLineRoot{copyExactRoot(cell.endpoints[0]), copyExactRoot(cell.endpoints[1])}
	for attempt := 0; attempt < 16; attempt++ {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		lo, hi := roots[0].bounds[0], roots[1].bounds[1]
		width := new(big.Rat).Sub(hi, lo)
		if width.Sign() <= 0 {
			return r, ErrDomain
		}
		local := &ExactComposedCurveCell{cell: cell.cell}
		for axis := 0; axis < 4; axis++ {
			local.power[axis], e = composeProjectionPolynomial(cell.power[axis], rationalPolynomial{lo, width}, b)
			if e != nil {
				return r, e
			}
		}
		inverseWidth := new(big.Rat).Inv(width)
		inverseOrigin := new(big.Rat).Neg(new(big.Rat).Mul(lo, inverseWidth))
		for endpoint := 0; endpoint < 2; endpoint++ {
			local.endpoints[endpoint], e = mapComposedGraphRoot(roots[endpoint], lo, width, inverseOrigin, inverseWidth, b)
			if e != nil {
				return r, e
			}
		}
		remainingOps := options.MaxOperations - b.operations
		remainingSteps := o.MaxSubdivisions - steps
		if remainingOps < 1 || remainingSteps < 1 {
			return r, ErrConvergence
		}
		child := o
		child.SurfacePatchOptions = options
		child.MaxOperations = remainingOps
		child.MaxSubdivisions = remainingSteps
		candidate, e := intersectNormalizedComposedCurveIdentityGraph(ctx, local, surface, child)
		b.operations += candidate.ExactOperations
		steps += candidate.Subdivisions
		if e == nil && candidate.Complete {
			for i := range candidate.Cells {
				contact := &candidate.Cells[i].Intersection
				mapEvent := func(event *ExactComposedPlaneEvent) (*ExactComposedPlaneEvent, error) {
					if event == nil {
						return nil, nil
					}
					root, e := mapComposedGraphRoot(event.root, inverseOrigin, inverseWidth, lo, width, b)
					if e != nil {
						return nil, e
					}
					model := *cell
					model.cell = event.cell.cell
					for j := 0; j < 2; j++ {
						model.endpoints[j], e = mapComposedGraphRoot(event.cell.endpoints[j], inverseOrigin, inverseWidth, lo, width, b)
						if e != nil {
							return nil, e
						}
					}
					return &ExactComposedPlaneEvent{root: root, cell: &model, multiplicity: event.multiplicity, endpointMask: event.endpointMask}, nil
				}
				for j, event := range contact.Events {
					contact.Events[j], e = mapEvent(event)
					if e != nil {
						return r, e
					}
				}
				for j, event := range contact.Boundary {
					contact.Boundary[j], e = mapEvent(event)
					if e != nil {
						return r, e
					}
				}
				for j := range contact.Domain {
					contact.Domain[j] = ExactParameterInterval{Lower: new(big.Rat).Add(lo, new(big.Rat).Mul(width, contact.Domain[j].Lower)), Upper: new(big.Rat).Add(lo, new(big.Rat).Mul(width, contact.Domain[j].Upper))}
				}
			}
			candidate.Subdivisions = 0
			r = candidate
			return r, nil
		}
		if e != nil && !errors.Is(e, ErrConvergence) && !errors.Is(e, ErrDomain) {
			return r, e
		}
		changed := false
		for _, root := range roots {
			if root.bounds[0].Cmp(root.bounds[1]) == 0 {
				continue
			}
			if steps >= o.MaxSubdivisions {
				return r, ErrConvergence
			}
			target := new(big.Rat).Quo(new(big.Rat).Sub(root.bounds[1], root.bounds[0]), big.NewRat(4, 1))
			var bounds [2]*big.Rat
			n, e := isolatePolynomialRoots(root.polynomial, root.bounds[0], root.bounds[1], b, o.MaxSubdivisions-steps, func(a, c *big.Rat, _ int) (bool, error) {
				if new(big.Rat).Sub(c, a).Cmp(target) > 0 {
					return false, nil
				}
				bounds = [2]*big.Rat{new(big.Rat).Set(a), new(big.Rat).Set(c)}
				return true, nil
			})
			steps += n
			if e != nil {
				return r, e
			}
			if bounds[0] == nil {
				return r, ErrConvergence
			}
			root.bounds = bounds
			changed = true
		}
		if !changed {
			return r, ErrConvergence
		}
	}
	return r, ErrConvergence
}

// The polynomial argument maps new coordinates to old; the bounds map is its
// inverse. Positive slope preserves ordering and root multiplicity exactly.
func mapComposedGraphRoot(root *ExactGraphLineRoot, polyOrigin, polySlope, boundOrigin, boundSlope *big.Rat, b *polynomialBudget) (*ExactGraphLineRoot, error) {
	p, e := composeProjectionPolynomial(root.polynomial, rationalPolynomial{polyOrigin, polySlope}, b)
	if e != nil {
		return nil, e
	}
	out := &ExactGraphLineRoot{polynomial: p}
	for i := range out.bounds {
		out.bounds[i] = new(big.Rat).Add(boundOrigin, new(big.Rat).Mul(boundSlope, root.bounds[i]))
		if e := b.check(out.bounds[i]); e != nil {
			return nil, e
		}
	}
	return out, nil
}
