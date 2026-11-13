package geom

import (
	"context"
	"math/big"
)

type TubeAlgebraicGraphEndpointOptions struct {
	SurfaceCurveAgreementOptions
	Tolerance float64
}

// TubeAlgebraicGraphEndpointCertificate binds an exact edge/graph contact to
// the unique endpoint of an immutable uniform tube. Parameters use original
// ordered A_U,A_V,B_U,B_V coordinates. SourceIsoTransverse concerns crossing the
// target graph by the fixed-free-coordinate source iso, not tangent orientation
// of the whole surface intersection. All roots are immutable; boxes are outward.
type TubeAlgebraicGraphEndpointCertificate struct {
	Proven, SourceIsoTransverse        bool
	SourceIsoMultiplicity              int
	Parameters                         [4]*ExactAlgebraicRoot
	ModelEvent                         *ExactComposedCurveEvent
	ParameterBoundsA, ParameterBoundsB UVBox
	PointBounds                        Box
	ExactOperations                    int
}

// CertifyAlgebraicGraphEndpoint uses the chart containing the tube's free axis
// as source and requires the other chart to prove an invertible affine XY projection. The proposal is the
// source iso's original varying coordinate. Both graph contact and strict
// containment of all dependent coordinates are independently rechecked, so tube
// uniqueness identifies the selected endpoint. Source root degree <=4 is
// supported. Dependent boundary equality is accepted only at the exact immutable anchor.
func (t VerifiedSurfaceTube) CertifyAlgebraicGraphEndpoint(ctx context.Context, upper bool, proposal *ExactAlgebraicRoot, o TubeAlgebraicGraphEndpointOptions) (TubeAlgebraicGraphEndpointCertificate, error) {
	if proposal == nil || proposal.root == nil {
		return TubeAlgebraicGraphEndpointCertificate{}, ErrInvalid
	}
	return t.algebraicGraphEndpoint(ctx, upper, proposal, nil, o)
}

// DiscoverAlgebraicGraphEndpoint autonomously isolates the source endpoint iso
// against the identity-XY target chart. Exactly one isolated contact must be
// recovered, independently proved transverse and strictly inside all dependent
// tube coordinates. Coincidence, multiple distinct candidates and unsupported
// algebraic degrees remain unresolved; no numerical hint selects a root.
func (t VerifiedSurfaceTube) DiscoverAlgebraicGraphEndpoint(ctx context.Context, upper bool, o TubeAlgebraicGraphEndpointOptions) (TubeAlgebraicGraphEndpointCertificate, error) {
	return t.algebraicGraphEndpoint(ctx, upper, nil, nil, o)
}

func (t VerifiedSurfaceTube) algebraicGraphEndpoint(ctx context.Context, upper bool, proposal *ExactAlgebraicRoot, fixedOverride *big.Rat, o TubeAlgebraicGraphEndpointOptions) (r TubeAlgebraicGraphEndpointCertificate, err error) {
	if ctx == nil || t.a == nil || t.b == nil || t.axis < 0 || t.axis > 3 || t.tangentSign == 0 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	options, e := patchOptions(o.SurfacePatchOptions)
	if e != nil {
		return r, e
	}
	o.SurfacePatchOptions = options
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 8192
	}
	if o.Tolerance == 0 {
		o.Tolerance = 1e-9
	}
	if o.MaxSubdivisions < 1 || !finite(o.Tolerance) || o.Tolerance <= 0 {
		return r, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	defer func() { r.ExactOperations = budget.operations }()
	source, target := t.a, t.b
	sourceOwner := 0
	if t.axis >= 2 {
		source, target = t.b, t.a
		sourceOwner = 1
	}
	fixedAxis := t.axis % 2
	d := source.Domain()
	ranges := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	side := 0
	if upper {
		side = 1
	}
	fixed := new(big.Rat).SetFloat64(ranges[fixedAxis][side])
	if fixedOverride != nil {
		if e := budget.check(fixedOverride); e != nil {
			return r, e
		}
		if fixedOverride.Cmp(new(big.Rat).SetFloat64(ranges[fixedAxis][0])) < 0 || fixedOverride.Cmp(new(big.Rat).SetFloat64(ranges[fixedAxis][1])) > 0 {
			return r, ErrDomain
		}
		fixed = new(big.Rat).Set(fixedOverride)
	}

	rationalRoot := func(q *big.Rat) *ExactAlgebraicRoot {
		return &ExactAlgebraicRoot{root: &ExactGraphLineRoot{polynomial: rationalPolynomial{new(big.Rat).Neg(q), big.NewRat(1, 1)}, bounds: [2]*big.Rat{new(big.Rat).Set(q), new(big.Rat).Set(q)}}}
	}
	origin, direction := [2]*big.Rat{new(big.Rat), new(big.Rat)}, [2]*big.Rat{new(big.Rat), new(big.Rat)}
	origin[fixedAxis] = fixed
	direction[1-fixedAxis] = big.NewRat(1, 1)
	lo := rationalRoot(new(big.Rat).SetFloat64(ranges[1-fixedAxis][0]))
	hi := rationalRoot(new(big.Rat).SetFloat64(ranges[1-fixedAxis][1]))
	section, e := source.BuildAffineSection(ctx, origin, direction, lo, hi, o.SurfaceCurveAgreementOptions)
	budget.operations += section.ExactOperations
	if e != nil {
		return r, e
	}
	var selected *ExactComposedCurveEvent
	var selectedInverse [2][3]*big.Rat
	for _, cell := range section.Cells {
		child := o.SurfaceCurveAgreementOptions
		child.MaxOperations = options.MaxOperations - budget.operations
		if child.MaxOperations < 1 {
			return r, ErrConvergence
		}
		contacts, e := IntersectExactComposedCurveAffineGraph(ctx, cell, target, child)
		budget.operations += contacts.ExactOperations
		if e != nil {
			return r, e
		}
		for _, piece := range contacts.Cells {
			if piece.Intersection.Coincident {
				return r, ErrConvergence
			}
			for _, event := range piece.Intersection.Events {

				match := proposal == nil
				if proposal != nil {
					comparison, e := compareRootBudget(ctx, event.root, proposal.root, budget, o.MaxSubdivisions)
					if e != nil {
						return r, e
					}
					if !comparison.Proven {
						return r, ErrConvergence
					}
					match = comparison.Order == 0
				}
				if match {
					if selected != nil {
						comparison, e := compareRootBudget(ctx, event.root, selected.root, budget, o.MaxSubdivisions)
						if e != nil {
							return r, e
						}
						if !comparison.Proven || comparison.Order != 0 || event.Multiplicity() != selected.Multiplicity() {
							return r, ErrConvergence
						}
					} else {
						selected = event
						selectedInverse = contacts.InverseXY
					}
				}

			}
		}
	}
	if selected == nil {
		return r, ErrDomain
	}
	r.SourceIsoMultiplicity = selected.Multiplicity()
	r.SourceIsoTransverse = r.SourceIsoMultiplicity == 1
	// Strict uniform tube contraction proves a nonsingular fixed-section root.
	// A multiple iso root cannot be accepted as that regular endpoint.
	if !r.SourceIsoTransverse {
		return r, ErrConvergence
	}
	r.Parameters[2*sourceOwner+fixedAxis] = rationalRoot(fixed)
	r.Parameters[2*sourceOwner+1-fixedAxis] = selected.Root()
	otherOwner := 1 - sourceOwner
	projected, e := transformComposedXY(selected.cell, selectedInverse, budget)
	if e != nil {
		return r, e
	}

	for axis := 0; axis < 2; axis++ {
		remaining := options.MaxOperations - budget.operations
		if remaining < 1 {
			return r, ErrConvergence
		}
		image, ops, e := projected.CoordinateRoot(ctx, selected.Root(), axis, ExactCurvePlaneEventOptions{MaxOperations: remaining, MaxCoefficientBits: options.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions})
		budget.operations += ops
		if e != nil {
			return r, e
		}
		r.Parameters[2*otherOwner+axis] = image
	}
	atAnchor, e := t.matchesAnchorRoots(ctx, r.Parameters, budget, o.MaxSubdivisions)
	if e != nil {
		return r, e
	}
	da, db := t.a.Domain(), t.b.Domain()
	boxes := [4][2]float64{{da.UMin, da.UMax}, {da.VMin, da.VMax}, {db.UMin, db.UMax}, {db.VMin, db.VMax}}
	var bounds [4][2]float64
	for axis, root := range r.Parameters {
		if axis != t.axis {
			for side := 0; side < 2; side++ {
				bound := rationalRoot(new(big.Rat).SetFloat64(boxes[axis][side]))
				comparison, e := compareRootBudget(ctx, root.root, bound.root, budget, o.MaxSubdivisions)
				if e != nil {
					return r, e
				}
				if !comparison.Proven {
					return r, ErrConvergence
				}
				if side == 0 && comparison.Order < 0 || side == 1 && comparison.Order > 0 || comparison.Order == 0 && !atAnchor {
					return r, ErrDomain
				}
			}
		}
		// Intersect the display enclosure with already-proved box membership.
		bounds[axis][0] = max(boxes[axis][0], rationalFloatBound(root.root.bounds[0], true))
		bounds[axis][1] = min(boxes[axis][1], rationalFloatBound(root.root.bounds[1], false))
	}
	remaining := options.MaxOperations - budget.operations
	if remaining < 1 {
		return r, ErrConvergence
	}
	point, e := selected.Enclose(ctx, ComposedCurveEndpointOptions{Tolerance: o.Tolerance, ExactCurvePlaneEventOptions: ExactCurvePlaneEventOptions{MaxOperations: remaining, MaxCoefficientBits: options.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions}})
	budget.operations += point.ExactOperations
	if e != nil {
		return r, e
	}
	r.ParameterBoundsA = UVBox{Min: Vec2{X: bounds[0][0], Y: bounds[1][0]}, Max: Vec2{X: bounds[0][1], Y: bounds[1][1]}}
	r.ParameterBoundsB = UVBox{Min: Vec2{X: bounds[2][0], Y: bounds[3][0]}, Max: Vec2{X: bounds[2][1], Y: bounds[3][1]}}
	r.PointBounds = point.Bounds
	r.ModelEvent = selected
	r.Proven = true
	return r, nil
}
