package geom

import (
	"context"
	"math/big"
)

// SplineGraphFourPolynomialNodes proves the complete original UV contact
// arrangement of two cubic or quartic branches with four transverse nodes.
// Branch zero is lower at the low free endpoint. Power and SeparationScale
// use original chart units, with Power[1]-Power[0] equal to SeparationScale
// times the product of the four monic contact factors. Vertices 0..3 are the
// branch terminals and vertices 4..7 are the contacts in increasing free
// order. Edges 0..4 and 5..9 follow the original branches. Regions 0 and 6
// lie below and above the branches, and regions 1..5 lie between them in
// free order. The proof retains both original owners on every knot cell.
type SplineGraphFourPolynomialNodes struct {
	Proven          bool
	Domain          ParameterRectangle
	FreeAxis        int
	Power           [2][]*big.Rat
	SeparationScale *big.Rat
	TerminalMasks   [2]uint8
	Vertices        [8]SplineGraphPolynomialVertex
	Edges           [10]SplineGraphPolynomialPiece
	Nodes           [4]SplineGraphPolynomialJunction
	Regions         [7]SplineGraphPolynomialRegion
	CertifiedCells  []ParameterRectangle
	Graph           SurfaceInjectivity
	ExactOperations int
}

// DiscoverSplineGraphFourPolynomialNodes verifies the complete original
// two-factor identity over every union-knot cell. It requires four distinct
// simple rational crossings strictly inside the free axis. Unsupported
// branches, repeated roots, endpoints and irrational roots fail atomically.
func DiscoverSplineGraphFourPolynomialNodes(ctx context.Context, a, b *SurfacePatch, axis int, o GraphPointContactOptions) (SplineGraphFourPolynomialNodes, error) {
	var result SplineGraphFourPolynomialNodes
	mode := polynomialGraphContactMode{crossings: 4, certify: result.certifyArrangement}
	pair, e := discoverSplineGraphTwoPolynomials(ctx, a, b, axis, o, mode)
	if e != nil {
		return SplineGraphFourPolynomialNodes{ExactOperations: pair.ExactOperations}, e
	}
	result.Proven = true
	result.ExactOperations = pair.ExactOperations
	return result, nil
}

// RepresentedVertices checks lossless native UV and XYZ conversion separately
// from the exact geometry. A failure returns empty coordinates.
func (r SplineGraphFourPolynomialNodes) RepresentedVertices() ([8]Vec2, [8]Vec3, error) {
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

func (r *SplineGraphFourPolynomialNodes) certifyArrangement(ctx context.Context, a *SurfacePatch, pair SplineGraphTwoPolynomials, roots []*big.Rat, domain ParameterRectangle, b *polynomialBudget) error {
	r.Domain, r.FreeAxis, r.Power = pair.Domain, pair.FreeAxis, pair.Power
	r.TerminalMasks, r.CertifiedCells, r.Graph = pair.TerminalMasks, pair.CertifiedCells, pair.Graph
	difference, e := graphPolynomialSum(r.Power[1], r.Power[0], -1, b)
	if e != nil {
		return e
	}
	for i, x := range roots {
		node := &r.Nodes[i]
		node.Vertex = i + 4
		for j, power := range r.Power {
			slope, e := polynomialValue(polynomialDerivative(power), x, b)
			if e != nil {
				return e
			}
			node.Tangents[j] = graphPolynomialUV(r.FreeAxis, big.NewRat(1, 1), slope)
		}
		if node.Tangents[0][1-r.FreeAxis].Cmp(node.Tangents[1][1-r.FreeAxis]) == 0 {
			return ErrDegenerate
		}
		factor := rationalPolynomial{new(big.Rat).Neg(x), big.NewRat(1, 1)}
		var remainder rationalPolynomial
		difference, remainder, e = polynomialDivide(difference, factor, b)
		if e != nil {
			return e
		}
		if len(remainder) != 0 {
			return ErrConvergence
		}
	}
	// Four simple factors account for the entire original quartic gap.
	if len(difference) != 1 || difference[0].Sign() <= 0 {
		return ErrConvergence
	}
	r.SeparationScale = difference[0]
	stations := [6]*big.Rat{
		new(big.Rat).SetFloat64(domain.UMin), roots[0], roots[1], roots[2], roots[3], new(big.Rat).SetFloat64(domain.UMax),
	}
	if e = buildPolynomialGraphPieces(ctx, a, pair, stations[:], r.Vertices[:], r.Edges[:], b); e != nil {
		return e
	}
	// A simple root switches the lower branch on each consecutive slab.
	orderPolynomialGraphRegions(r.FreeAxis, r.Graph.ProjectionOrientation, pair.StripSigns[0], []int{0, 1, 0, 1, 0}, r.Nodes[:], r.Regions[:])
	if e = graphPolynomialRegionWitnesses(pair, domain, stations[:], r.Vertices[4].Parameters[1-r.FreeAxis], r.Regions[:], b); e != nil {
		return e
	}
	return ctx.Err()
}
