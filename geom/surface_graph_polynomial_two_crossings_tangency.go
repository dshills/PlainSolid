package geom

import (
	"context"
	"math/big"
)

// SplineGraphPolynomialTwoCrossingsTangency proves the complete original
// graph arrangement of two transverse crossings and one quadratic tangency.
// Branch zero is lower at the low free endpoint. Power and SeparationScale
// use the original chart. The difference of branch powers equals that scale
// times both monic crossing factors and the squared tangency factor.
// Vertices 0..3 are branch terminals, and 4..6 are contacts in free order.
// Edges 0..3 and 4..7 follow the original branches. Regions 0 and 5 lie
// below and above the branches, and regions 1..4 are the intervening slabs.
type SplineGraphPolynomialTwoCrossingsTangency struct {
	Proven          bool
	Domain          ParameterRectangle
	FreeAxis        int
	Power           [2][]*big.Rat
	SeparationScale *big.Rat
	TerminalMasks   [2]uint8
	Vertices        [7]SplineGraphPolynomialVertex
	Edges           [8]SplineGraphPolynomialPiece
	Nodes           [3]SplineGraphPolynomialJunction
	ContactOrder    [3]int      // root multiplicities in increasing free order
	DerivativeGap   [3]*big.Rat // derivative of order ContactOrder of Power[1]-Power[0]
	Regions         [6]SplineGraphPolynomialRegion
	CertifiedCells  []ParameterRectangle
	Graph           SurfaceInjectivity
	ExactOperations int
}

// DiscoverSplineGraphPolynomialTwoCrossingsTangency verifies the complete
// original factor identity on every union-knot cell. It requires two simple
// and one double interior rational root, in any station order. Endpoint,
// irrational, coincident or unsupported contacts fail atomically.
func DiscoverSplineGraphPolynomialTwoCrossingsTangency(ctx context.Context, a, b *SurfacePatch, axis int, o GraphPointContactOptions) (SplineGraphPolynomialTwoCrossingsTangency, error) {
	var result SplineGraphPolynomialTwoCrossingsTangency
	mode := polynomialGraphContactMode{crossings: 2, tangencies: 1, certify: result.certifyArrangement}
	pair, e := discoverSplineGraphTwoPolynomials(ctx, a, b, axis, o, mode)
	if e != nil {
		return SplineGraphPolynomialTwoCrossingsTangency{ExactOperations: pair.ExactOperations}, e
	}
	result.Proven = true
	result.ExactOperations = pair.ExactOperations
	return result, nil
}

// RepresentedVertices separately checks lossless native UV and XYZ
// conversion. A failure returns empty coordinates.
func (r SplineGraphPolynomialTwoCrossingsTangency) RepresentedVertices() ([7]Vec2, [7]Vec3, error) {
	var uv [7]Vec2
	var xyz [7]Vec3
	if !r.Proven {
		return uv, xyz, ErrInvalid
	}
	if e := representPolynomialArrangementVertices(r.Vertices[:], uv[:], xyz[:]); e != nil {
		return [7]Vec2{}, [7]Vec3{}, e
	}
	return uv, xyz, nil
}

func (r *SplineGraphPolynomialTwoCrossingsTangency) certifyArrangement(ctx context.Context, a *SurfacePatch, pair SplineGraphTwoPolynomials, roots []*big.Rat, domain ParameterRectangle, b *polynomialBudget) error {
	r.Domain, r.FreeAxis, r.Power = pair.Domain, pair.FreeAxis, pair.Power
	r.TerminalMasks, r.CertifiedCells, r.Graph = pair.TerminalMasks, pair.CertifiedCells, pair.Graph
	stations := [5]*big.Rat{new(big.Rat).SetFloat64(domain.UMin), roots[0], roots[1], roots[2], new(big.Rat).SetFloat64(domain.UMax)}
	orders, e := certifyPolynomialGraphMixedOrders(r.FreeAxis, r.Power, stations[:], r.Nodes[:], b)
	if e != nil {
		return e
	}
	r.ContactOrder, r.DerivativeGap, r.SeparationScale = [3]int(orders.orders), [3]*big.Rat(orders.gaps), orders.cofactor[0]
	if e = buildPolynomialGraphPieces(ctx, a, pair, stations[:], r.Vertices[:], r.Edges[:], b); e != nil {
		return e
	}
	orderPolynomialGraphRegions(r.FreeAxis, r.Graph.ProjectionOrientation, pair.StripSigns[0], orders.lower, r.Nodes[:], r.Regions[:])
	if e = graphPolynomialRegionWitnesses(pair, domain, stations[:], r.Vertices[4].Parameters[1-r.FreeAxis], r.Regions[:], b); e != nil {
		return e
	}
	return ctx.Err()
}
