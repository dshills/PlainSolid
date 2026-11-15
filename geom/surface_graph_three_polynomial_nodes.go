package geom

import (
	"context"
	"math/big"
	"slices"
)

// SplineGraphThreePolynomialNodes proves the complete original UV contact
// arrangement of two cubic or quartic branches with three transverse nodes.
// Branch zero is lower at the low free endpoint. Power uses original chart
// units. Vertices 0..3 are the start and end of each branch, and vertices 4..6
// are the nodes in increasing free order. Edges 0..3 and 4..7 follow the two
// original branches. Regions 0 and 5 lie below and above both branches.
// Regions 1..4 lie between them in increasing free order. Original-cell
// verification retains both owners without assuming C1 at source knots.
type SplineGraphThreePolynomialNodes struct {
	Proven          bool
	Domain          ParameterRectangle
	FreeAxis        int
	Power           [2][]*big.Rat
	TerminalMasks   [2]uint8
	Vertices        [7]SplineGraphPolynomialVertex
	Edges           [8]SplineGraphPolynomialPiece
	Nodes           [3]SplineGraphPolynomialJunction
	Regions         [6]SplineGraphPolynomialRegion
	CertifiedCells  []ParameterRectangle
	Graph           SurfaceInjectivity
	ExactOperations int
}

// DiscoverSplineGraphThreePolynomialNodes verifies the full original
// two-factor identity on every union-knot cell. Exactly three interior
// rational roots of multiplicity one prove the signed arrangement. Endpoint,
// tangent, coincident, irrational and additional contacts fail atomically.
func DiscoverSplineGraphThreePolynomialNodes(ctx context.Context, a, b *SurfacePatch, axis int, o GraphPointContactOptions) (SplineGraphThreePolynomialNodes, error) {
	var result SplineGraphThreePolynomialNodes
	pair, e := discoverSplineGraphTwoPolynomials(ctx, a, b, axis, o, polynomialGraphContactMode{count: 3, multiplicity: 1, certify: result.certifyArrangement})
	if e != nil {
		return SplineGraphThreePolynomialNodes{ExactOperations: pair.ExactOperations}, e
	}
	result.Proven = true
	result.ExactOperations = pair.ExactOperations
	return result, nil
}

// RepresentedVertices checks lossless native UV and XYZ admission separately
// from the exact rational arrangement. Any failure returns empty coordinates.
func (r SplineGraphThreePolynomialNodes) RepresentedVertices() ([7]Vec2, [7]Vec3, error) {
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

func (r *SplineGraphThreePolynomialNodes) certifyArrangement(ctx context.Context, a *SurfacePatch, pair SplineGraphTwoPolynomials, free []*big.Rat, d ParameterRectangle, b *polynomialBudget) error {
	r.Domain, r.FreeAxis, r.Power = pair.Domain, pair.FreeAxis, pair.Power
	r.TerminalMasks, r.CertifiedCells, r.Graph = pair.TerminalMasks, pair.CertifiedCells, pair.Graph
	stations := [5]*big.Rat{new(big.Rat).SetFloat64(d.UMin), free[0], free[1], free[2], new(big.Rat).SetFloat64(d.UMax)}
	if e := buildPolynomialGraphPieces(ctx, a, pair, stations[:], r.Vertices[:], r.Edges[:], b); e != nil {
		return e
	}
	orientation := r.Graph.ProjectionOrientation
	if r.FreeAxis == 1 {
		orientation = -orientation
	}
	r.Nodes = [3]SplineGraphPolynomialJunction{
		{Vertex: 4, RayCycle: [4]int{5, 1, 4, 0}, Regions: [4]int{2, 5, 1, 0}},
		{Vertex: 5, RayCycle: [4]int{2, 6, 1, 5}, Regions: [4]int{3, 5, 2, 0}},
		{Vertex: 6, RayCycle: [4]int{7, 3, 6, 2}, Regions: [4]int{4, 5, 3, 0}},
	}
	for i := range r.Nodes {
		n := &r.Nodes[i]
		for j, power := range r.Power {
			slope, e := polynomialValue(polynomialDerivative(power), free[i], b)
			if e != nil {
				return e
			}
			n.Tangents[j] = graphPolynomialUV(r.FreeAxis, big.NewRat(1, 1), slope)
		}
		if orientation < 0 {
			slices.Reverse(n.RayCycle[1:])
			slices.Reverse(n.Regions[:])
		}
	}
	perimeter := func(start, end int) *SplineGraphPolynomialPerimeter {
		return &SplineGraphPolynomialPerimeter{start, end, r.Graph.ProjectionOrientation}
	}
	// The lower branch on the four slabs is 0,1,0,1. The two interior
	// lenses each use distinct original edges sharing their two endpoints.
	sign := pair.StripSigns[0]
	r.Regions = [6]SplineGraphPolynomialRegion{
		{Edges: []SplineGraphPolynomialUse{{7, true}, {2, true}, {5, true}, {0, true}}, Perimeter: perimeter(0, 3), HeightSide: sign},
		{Edges: []SplineGraphPolynomialUse{{0, false}, {4, true}}, Perimeter: perimeter(2, 0), HeightSide: -sign},
		{Edges: []SplineGraphPolynomialUse{{5, false}, {1, true}}, HeightSide: -sign},
		{Edges: []SplineGraphPolynomialUse{{2, false}, {6, true}}, HeightSide: -sign},
		{Edges: []SplineGraphPolynomialUse{{3, true}, {7, false}}, Perimeter: perimeter(3, 1), HeightSide: -sign},
		{Edges: []SplineGraphPolynomialUse{{4, false}, {1, false}, {6, false}, {3, false}}, Perimeter: perimeter(1, 2), HeightSide: sign},
	}
	if orientation < 0 {
		reversePolynomialGraphRegionBoundaries(r.Regions[:])
	}
	if e := graphPolynomialRegionWitnesses(pair, d, stations[:], r.Vertices[4].Parameters[1-r.FreeAxis], r.Regions[:], b); e != nil {
		return e
	}
	return ctx.Err()
}
