package geom

import (
	"context"
	"math/big"
	"slices"
)

// SplineGraphPolynomialTangency certifies two distinct cubic or quartic
// contact branches meeting once at an interior rational tangency. Branches
// are ordered lower then upper on both sides. Power and Separation use the
// original free coordinate x, with Power[1]-Power[0]=(x-r)^2*Separation(x)
// for the tangent station r. Separation is strictly positive on the domain.
// Vertex 4 is the tangency, and vertices 0..3 are the start and end of each
// branch in order. Edges 0..1 and 2..3 follow branches zero and one. Regions are
// below, between on the left, between on the right, and above. Signed sectors
// retain original support ownership across knot cells without assuming C1
// continuity of the lifted model curves at a source knot.
type SplineGraphPolynomialTangency struct {
	Proven              bool
	Domain              ParameterRectangle
	FreeAxis            int
	Power               [2][]*big.Rat
	Separation          []*big.Rat
	SecondDerivativeGap *big.Rat    // difference of the original branch second derivatives at the contact
	Tangent             [2]*big.Rat // common original UV tangent toward increasing free coordinate
	TerminalMasks       [2]uint8
	Vertices            [5]SplineGraphPolynomialVertex
	Edges               [4]SplineGraphPolynomialPiece
	RayCycle            [4]int // CCW in model XY using one-sided branch ordering
	SectorRegions       [4]int // region between each ray and the next in RayCycle
	Regions             [4]SplineGraphPolynomialRegion
	CertifiedCells      []ParameterRectangle
	Graph               SurfaceInjectivity
	ExactOperations     int
}

// DiscoverSplineGraphPolynomialTangency verifies the full dependent-quadratic
// coefficient identity on every original union-knot cell. A single root of
// multiplicity two proves the contact and preserves branch order. Coincident
// branches and higher multiplicities return ErrDegenerate, endpoint contacts
// return ErrDomain, and an irrational tangency returns ErrUnrepresentable.
// Additional or unresolved contacts fail without publishing partial geometry.
func DiscoverSplineGraphPolynomialTangency(ctx context.Context, a, b *SurfacePatch, axis int, o GraphPointContactOptions) (SplineGraphPolynomialTangency, error) {
	var result SplineGraphPolynomialTangency
	pair, e := discoverSplineGraphTwoPolynomials(ctx, a, b, axis, o, polynomialGraphContactMode{count: 1, multiplicity: 2, certify: result.certifyArrangement})
	if e != nil {
		return SplineGraphPolynomialTangency{ExactOperations: pair.ExactOperations}, e
	}
	result.Proven = true
	result.ExactOperations = pair.ExactOperations
	return result, nil
}

func (r *SplineGraphPolynomialTangency) certifyArrangement(ctx context.Context, a *SurfacePatch, pair SplineGraphTwoPolynomials, roots []*big.Rat, d ParameterRectangle, b *polynomialBudget) error {
	free := roots[0]
	r.Domain, r.FreeAxis, r.Power = pair.Domain, pair.FreeAxis, pair.Power
	r.TerminalMasks, r.CertifiedCells, r.Graph = pair.TerminalMasks, pair.CertifiedCells, pair.Graph
	delta, e := graphPolynomialSum(pair.Power[1], pair.Power[0], -1, b)
	if e != nil {
		return e
	}
	factor := rationalPolynomial{new(big.Rat).Neg(free), big.NewRat(1, 1)}
	square, e := polynomialProduct(factor, factor, b)
	if e != nil {
		return e
	}
	quotient, rem, e := polynomialDivide(delta, square, b)
	if e != nil {
		return e
	}
	if len(rem) != 0 {
		return ErrConvergence
	}
	atContact, e := polynomialValue(quotient, free, b)
	if e != nil {
		return e
	}
	if atContact.Sign() <= 0 {
		return ErrConvergence
	}
	// Root isolation already excludes any other zero of the difference on
	// this interval. The quotient is continuous and positive at the removed
	// double root, so it is strictly positive everywhere on the same interval.
	r.Separation = quotient
	r.SecondDerivativeGap = new(big.Rat).Mul(big.NewRat(2, 1), atContact)
	if e = b.check(r.SecondDerivativeGap); e != nil {
		return e
	}
	slope, e := polynomialValue(polynomialDerivative(r.Power[0]), free, b)
	if e != nil {
		return e
	}
	r.Tangent = graphPolynomialUV(r.FreeAxis, big.NewRat(1, 1), slope)
	stations := [3]*big.Rat{new(big.Rat).SetFloat64(d.UMin), free, new(big.Rat).SetFloat64(d.UMax)}
	if e = buildPolynomialGraphPieces(ctx, a, pair, stations[:], r.Vertices[:], r.Edges[:], b); e != nil {
		return e
	}
	perimeter := func(start, end int) *SplineGraphPolynomialPerimeter {
		return &SplineGraphPolynomialPerimeter{start, end, r.Graph.ProjectionOrientation}
	}
	sign := pair.StripSigns[0]
	r.Regions = [4]SplineGraphPolynomialRegion{
		{Edges: []SplineGraphPolynomialUse{{1, true}, {0, true}}, Perimeter: perimeter(0, 1), HeightSide: sign},
		{Edges: []SplineGraphPolynomialUse{{0, false}, {2, true}}, Perimeter: perimeter(2, 0), HeightSide: -sign},
		{Edges: []SplineGraphPolynomialUse{{3, true}, {1, false}}, Perimeter: perimeter(1, 3), HeightSide: -sign},
		{Edges: []SplineGraphPolynomialUse{{2, false}, {3, false}}, Perimeter: perimeter(3, 2), HeightSide: sign},
	}
	// The right-going rays are lower then upper, and the left-going rays
	// are upper then lower. This uses branch ordering in nearby slices,
	// since the two rays in either direction have exactly equal tangents.
	r.RayCycle = [4]int{1, 3, 2, 0}
	r.SectorRegions = [4]int{2, 3, 1, 0}
	orientation := r.Graph.ProjectionOrientation
	if r.FreeAxis == 1 {
		orientation = -orientation
	}
	if orientation < 0 {
		slices.Reverse(r.RayCycle[1:])
		slices.Reverse(r.SectorRegions[:])
		reversePolynomialGraphRegionBoundaries(r.Regions[:])
	}
	if e = graphPolynomialRegionWitnesses(pair, d, stations[:], r.Vertices[4].Parameters[1-r.FreeAxis], r.Regions[:], b); e != nil {
		return e
	}
	return ctx.Err()
}
