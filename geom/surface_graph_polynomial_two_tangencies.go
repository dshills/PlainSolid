package geom

import (
	"context"
	"math/big"
)

// SplineGraphPolynomialTwoTangencies proves the complete original graph
// arrangement of two distinct quadratic tangencies. Branch zero is lower
// on all three slabs. The difference of the original branch powers equals
// SeparationScale times both squared monic factors in original chart units.
// Vertices 0..3 are branch terminals, and 4..5 are the contacts in free order.
// Edges 0..2 and 3..5 follow the two original branches. Regions 0 and 4 lie
// below and above the branches, and regions 1..3 are the intervening slabs.
type SplineGraphPolynomialTwoTangencies struct {
	Proven               bool
	Domain               ParameterRectangle
	FreeAxis             int
	Power                [2][]*big.Rat
	SeparationScale      *big.Rat
	SecondDerivativeGaps [2]*big.Rat // original branch difference in increasing free order
	TerminalMasks        [2]uint8
	Vertices             [6]SplineGraphPolynomialVertex
	Edges                [6]SplineGraphPolynomialPiece
	Nodes                [2]SplineGraphPolynomialJunction
	Regions              [5]SplineGraphPolynomialRegion
	CertifiedCells       []ParameterRectangle
	Graph                SurfaceInjectivity
	ExactOperations      int
}

// DiscoverSplineGraphPolynomialTwoTangencies certifies the original
// dependent-quadratic identity on every union-knot cell. It requires exactly
// two distinct interior rational double roots. Coincident branches and higher
// multiplicities return ErrDegenerate, endpoint contacts return ErrDomain,
// and irrational tangencies return ErrUnrepresentable.
func DiscoverSplineGraphPolynomialTwoTangencies(ctx context.Context, a, b *SurfacePatch, axis int, o GraphPointContactOptions) (SplineGraphPolynomialTwoTangencies, error) {
	var result SplineGraphPolynomialTwoTangencies
	mode := polynomialGraphContactMode{tangencies: 2, certify: result.certifyArrangement}
	pair, e := discoverSplineGraphTwoPolynomials(ctx, a, b, axis, o, mode)
	if e != nil {
		return SplineGraphPolynomialTwoTangencies{ExactOperations: pair.ExactOperations}, e
	}
	result.Proven, result.ExactOperations = true, pair.ExactOperations
	return result, nil
}

// RepresentedVertices checks lossless native UV and XYZ conversion separately
// from the exact arrangement. A failure returns empty coordinates.
func (r SplineGraphPolynomialTwoTangencies) RepresentedVertices() ([6]Vec2, [6]Vec3, error) {
	var uv [6]Vec2
	var xyz [6]Vec3
	if !r.Proven {
		return uv, xyz, ErrInvalid
	}
	if e := representPolynomialArrangementVertices(r.Vertices[:], uv[:], xyz[:]); e != nil {
		return [6]Vec2{}, [6]Vec3{}, e
	}
	return uv, xyz, nil
}

func (r *SplineGraphPolynomialTwoTangencies) certifyArrangement(ctx context.Context, a *SurfacePatch, pair SplineGraphTwoPolynomials, roots []*big.Rat, d ParameterRectangle, b *polynomialBudget) error {
	r.Domain, r.FreeAxis, r.Power = pair.Domain, pair.FreeAxis, pair.Power
	r.TerminalMasks, r.CertifiedCells, r.Graph = pair.TerminalMasks, pair.CertifiedCells, pair.Graph
	stations := [4]*big.Rat{new(big.Rat).SetFloat64(d.UMin), roots[0], roots[1], new(big.Rat).SetFloat64(d.UMax)}
	orders, e := certifyPolynomialGraphContactOrders(r.FreeAxis, r.Power, stations[:], r.Nodes[:], 2, 0, b)
	if e != nil {
		return e
	}
	r.SecondDerivativeGaps, r.SeparationScale = [2]*big.Rat(orders.gaps), orders.cofactor[0]
	if e = buildPolynomialGraphPieces(ctx, a, pair, stations[:], r.Vertices[:], r.Edges[:], b); e != nil {
		return e
	}
	orderPolynomialGraphRegions(r.FreeAxis, r.Graph.ProjectionOrientation, pair.StripSigns[0], orders.lower, r.Nodes[:], r.Regions[:])
	if e = graphPolynomialRegionWitnesses(pair, d, stations[:], r.Vertices[4].Parameters[1-r.FreeAxis], r.Regions[:], b); e != nil {
		return e
	}
	return ctx.Err()
}
