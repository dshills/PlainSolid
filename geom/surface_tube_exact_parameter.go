package geom

import (
	"context"
	"math/big"
)

// ExactFreeDomain returns the original ordered chart axis and caller-owned
// exact rational domain bounds. It conveys no change of parameter units.
func (t VerifiedSurfaceTube) ExactFreeDomain() (int, ExactParameterInterval, error) {
	if t.a == nil || t.b == nil || t.axis < 0 || t.axis > 3 || t.tangentSign == 0 {
		return 0, ExactParameterInterval{}, ErrInvalid
	}
	p := t.a
	if t.axis >= 2 {
		p = t.b
	}
	d := p.Domain()
	lo, hi := d.UMin, d.UMax
	if t.axis%2 == 1 {
		lo, hi = d.VMin, d.VMax
	}
	return t.axis, ExactParameterInterval{Lower: new(big.Rat).SetFloat64(lo), Upper: new(big.Rat).SetFloat64(hi)}, nil
}

type TubeFreeRootMembership struct {
	Proven, Inside bool
	BoundaryMask   uint8 // 1 lower, 2 upper, zero strictly inside or outside
	Operations     int
}

// ContainsFreeRoot compares an exact algebraic coordinate with the tube's
// closed free domain. Domain membership alone does not certify a model contact.
func (t VerifiedSurfaceTube) ContainsFreeRoot(ctx context.Context, root *ExactAlgebraicRoot, o ExactCurvePlaneEventOptions) (r TubeFreeRootMembership, err error) {
	if ctx == nil || root == nil || root.root == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	_, domain, e := t.ExactFreeDomain()
	if e != nil {
		return r, e
	}
	options, e := graphRootOptions(o)
	if e != nil {
		return r, e
	}
	b := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	defer func() { r.Operations = b.operations }()
	inside := true
	subdivisions := 0
	for side, bound := range []*big.Rat{domain.Lower, domain.Upper} {
		target := &ExactGraphLineRoot{polynomial: rationalPolynomial{new(big.Rat).Neg(bound), big.NewRat(1, 1)}, bounds: [2]*big.Rat{bound, bound}}
		comparison, e := compareRootBudget(ctx, root.root, target, b, options.MaxSubdivisions-subdivisions)
		subdivisions += comparison.Subdivisions
		if e != nil {
			return r, e
		}
		if !comparison.Proven {
			return r, ErrConvergence
		}
		if side == 0 && comparison.Order < 0 || side == 1 && comparison.Order > 0 {
			inside = false
		}
		if comparison.Order == 0 {
			r.BoundaryMask |= 1 << uint(side)
		}
	}
	r.Proven, r.Inside = true, inside
	return r, nil
}

// TubeAlgebraicGraphPointCertificate uses the same exact-coordinate and model
// evidence as an endpoint certificate, but its free coordinate may be interior.
type TubeAlgebraicGraphPointCertificate = TubeAlgebraicGraphEndpointCertificate

// EvaluateRationalGraph evaluates a nonbinary rational free section using exact
// algebraic iso/graph root isolation and immutable tube uniqueness. Other than
// the free coordinate, all three chart coordinates may remain algebraic. The
// opposite chart must prove invertible affine XY; unsupported projections fail closed.
// No conversion of the input value to float64 is used in construction.
func (t VerifiedSurfaceTube) EvaluateRationalGraph(ctx context.Context, free *big.Rat, o TubeAlgebraicGraphEndpointOptions) (TubeAlgebraicGraphPointCertificate, error) {
	if free == nil {
		return TubeAlgebraicGraphPointCertificate{}, ErrInvalid
	}
	return t.algebraicGraphEndpoint(ctx, false, nil, free, o)
}
