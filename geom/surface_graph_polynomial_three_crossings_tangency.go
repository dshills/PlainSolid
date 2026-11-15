package geom

import (
	"context"
	"math/big"
)

// SplineGraphPolynomialThreeCrossingsTangency proves the complete arrangement
// of two original graph branches with three transverse crossings and one
// quadratic tangency. Their difference in original chart units equals
// SeparationScale times the monic factors at all four stations with their
// respective orders. Branch zero is lower at the start, so SeparationScale
// is negative. This requires a quintic branch. Vertices 0..3 are branch
// terminals and 4..7 are contacts in free order. Edges 0..4 and 5..9 follow the
// two original branches. Regions 0 and 6 are exterior to both branches, with
// the five intervening regions in free order.
type SplineGraphPolynomialThreeCrossingsTangency struct {
	Proven          bool
	Domain          ParameterRectangle
	FreeAxis        int
	Power           [2][]*big.Rat
	SeparationScale *big.Rat
	TerminalMasks   [2]uint8
	Vertices        [8]SplineGraphPolynomialVertex
	Edges           [10]SplineGraphPolynomialPiece
	Nodes           [4]SplineGraphPolynomialJunction
	ContactOrder    [4]int      // root multiplicities in increasing free order
	DerivativeGap   [4]*big.Rat // derivative of order ContactOrder of Power[1]-Power[0]
	Regions         [7]SplineGraphPolynomialRegion
	CertifiedCells  []ParameterRectangle
	Graph           SurfaceInjectivity
	ExactOperations int
}

// DiscoverSplineGraphPolynomialThreeCrossingsTangency requires exactly three
// simple and one double rational roots strictly inside the chart, in any
// station order. It re-expands the full dependent-quadratic identity on every
// original union-knot cell. Branch degrees are three through five, and the
// contact factors exhaust the fifth-degree branch difference.
func DiscoverSplineGraphPolynomialThreeCrossingsTangency(ctx context.Context, a, b *SurfacePatch, axis int, o GraphPointContactOptions) (SplineGraphPolynomialThreeCrossingsTangency, error) {
	var result SplineGraphPolynomialThreeCrossingsTangency
	mode := polynomialGraphContactMode{crossings: 3, tangencies: 1, allowQuintic: true, certify: result.certifyArrangement}
	pair, e := discoverSplineGraphTwoPolynomials(ctx, a, b, axis, o, mode)
	if e != nil {
		return SplineGraphPolynomialThreeCrossingsTangency{ExactOperations: pair.ExactOperations}, e
	}
	result.Proven, result.ExactOperations = true, pair.ExactOperations
	return result, nil
}

// RepresentedVertices separately checks exact native UV and XYZ coordinates.
// It returns empty coordinates on failure. Trim admission is separate.
func (r SplineGraphPolynomialThreeCrossingsTangency) RepresentedVertices() ([8]Vec2, [8]Vec3, error) {
	var uv [8]Vec2
	var xyz [8]Vec3
	if !r.Proven {
		return uv, xyz, ErrInvalid
	}
	if e := representPolynomialArrangementVertices(r.Vertices[:], uv[:], xyz[:]); e != nil {
		return [8]Vec2{}, [8]Vec3{}, e
	}
	return uv, xyz, nil
}

func (r *SplineGraphPolynomialThreeCrossingsTangency) certifyArrangement(ctx context.Context, a *SurfacePatch, pair SplineGraphTwoPolynomials, roots []*big.Rat, d ParameterRectangle, b *polynomialBudget) error {
	r.Domain, r.FreeAxis, r.Power = pair.Domain, pair.FreeAxis, pair.Power
	r.TerminalMasks, r.CertifiedCells, r.Graph = pair.TerminalMasks, pair.CertifiedCells, pair.Graph
	stations := [6]*big.Rat{new(big.Rat).SetFloat64(d.UMin), roots[0], roots[1], roots[2], roots[3], new(big.Rat).SetFloat64(d.UMax)}
	orders, e := certifyPolynomialGraphContactOrders(r.FreeAxis, r.Power, stations[:], r.Nodes[:], 1, 0, b)
	if e != nil {
		return e
	}
	if len(orders.cofactor) != 1 || orders.cofactor[0].Sign() >= 0 {
		return ErrConvergence
	}
	r.ContactOrder, r.DerivativeGap, r.SeparationScale = [4]int(orders.orders), [4]*big.Rat(orders.gaps), orders.cofactor[0]
	if e = buildPolynomialGraphPieces(ctx, a, pair, stations[:], r.Vertices[:], r.Edges[:], b); e != nil {
		return e
	}
	orderPolynomialGraphRegions(r.FreeAxis, r.Graph.ProjectionOrientation, pair.StripSigns[0], orders.lower, r.Nodes[:], r.Regions[:])
	if e = graphPolynomialRegionWitnesses(pair, d, stations[:], r.Vertices[4].Parameters[1-r.FreeAxis], r.Regions[:], b); e != nil {
		return e
	}
	return ctx.Err()
}
