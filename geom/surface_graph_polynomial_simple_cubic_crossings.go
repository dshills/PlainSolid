package geom

import (
	"context"
	"math/big"
)

// SplineGraphPolynomialSimpleCubicCrossings proves two original graph branches
// cross at two distinct interior stations, once with order one and once with
// order three. Power[1]-Power[0] is SeparationScale times the monic factors
// with the stated orders. The scale is positive in original chart units.
// Branch zero is lower on the first and last slabs. Vertices 0..3 are branch
// terminals, followed by the two crossings in increasing free order. Edges
// 0..2 and 3..5 follow the two branches. Regions 0 and 4 are exterior to
// both branches, with three intervening regions in free order.
type SplineGraphPolynomialSimpleCubicCrossings struct {
	Proven                bool
	Domain                ParameterRectangle
	FreeAxis              int
	Power                 [2][]*big.Rat
	SeparationScale       *big.Rat
	ContactOrder          [2]int      // multiplicities in increasing free order
	DerivativeGap         [2]*big.Rat // difference of the branch derivatives of the corresponding order
	CubicSecondDerivative *big.Rat    // common original second derivative at the order-three station
	TerminalMasks         [2]uint8
	Vertices              [6]SplineGraphPolynomialVertex
	Edges                 [6]SplineGraphPolynomialPiece
	Nodes                 [2]SplineGraphPolynomialJunction
	Regions               [5]SplineGraphPolynomialRegion
	CertifiedCells        []ParameterRectangle
	Graph                 SurfaceInjectivity
	ExactOperations       int
}

// DiscoverSplineGraphPolynomialSimpleCubicCrossings requires one interior
// simple root and one interior triple root in either order. It verifies the
// complete dependent-quadratic identity on every original union-knot cell.
// The two roots exhaust the allowed quartic branch difference. Coincident
// branches, unsupported multiplicities and endpoint contacts fail atomically.
func DiscoverSplineGraphPolynomialSimpleCubicCrossings(ctx context.Context, a, b *SurfacePatch, axis int, o GraphPointContactOptions) (SplineGraphPolynomialSimpleCubicCrossings, error) {
	var result SplineGraphPolynomialSimpleCubicCrossings
	mode := polynomialGraphContactMode{crossings: 1, cubicCrossings: 1, certify: result.certifyArrangement}
	pair, e := discoverSplineGraphTwoPolynomials(ctx, a, b, axis, o, mode)
	if e != nil {
		return SplineGraphPolynomialSimpleCubicCrossings{ExactOperations: pair.ExactOperations}, e
	}
	result.Proven, result.ExactOperations = true, pair.ExactOperations
	return result, nil
}

// RepresentedVertices checks native UV and XYZ coordinates separately from
// the exact geometry. A failure returns empty coordinates.
func (r SplineGraphPolynomialSimpleCubicCrossings) RepresentedVertices() ([6]Vec2, [6]Vec3, error) {
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

func (r *SplineGraphPolynomialSimpleCubicCrossings) certifyArrangement(ctx context.Context, a *SurfacePatch, pair SplineGraphTwoPolynomials, roots []*big.Rat, d ParameterRectangle, b *polynomialBudget) error {
	r.Domain, r.FreeAxis, r.Power = pair.Domain, pair.FreeAxis, pair.Power
	r.TerminalMasks, r.CertifiedCells, r.Graph = pair.TerminalMasks, pair.CertifiedCells, pair.Graph
	stations := [4]*big.Rat{new(big.Rat).SetFloat64(d.UMin), roots[0], roots[1], new(big.Rat).SetFloat64(d.UMax)}
	orders, e := certifyPolynomialGraphContactOrders(r.FreeAxis, r.Power, stations[:], r.Nodes[:], 0, 1, b)
	if e != nil {
		return e
	}
	r.ContactOrder, r.DerivativeGap, r.SeparationScale = [2]int(orders.orders), [2]*big.Rat(orders.gaps), orders.cofactor[0]
	cubic := roots[0]
	if r.ContactOrder[1] == 3 {
		cubic = roots[1]
	}
	r.CubicSecondDerivative, e = polynomialValue(polynomialDerivative(polynomialDerivative(r.Power[0])), cubic, b)
	if e != nil {
		return e
	}
	if e = buildPolynomialGraphPieces(ctx, a, pair, stations[:], r.Vertices[:], r.Edges[:], b); e != nil {
		return e
	}
	orderPolynomialGraphRegions(r.FreeAxis, r.Graph.ProjectionOrientation, pair.StripSigns[0], orders.lower, r.Nodes[:], r.Regions[:])
	if e = graphPolynomialRegionWitnesses(pair, d, stations[:], r.Vertices[4].Parameters[1-r.FreeAxis], r.Regions[:], b); e != nil {
		return e
	}
	return ctx.Err()
}
