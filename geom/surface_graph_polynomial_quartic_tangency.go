package geom

import (
	"context"
	"math/big"
)

// SplineGraphPolynomialQuarticTangency proves two distinct original graph
// branches share an order-four contact with unchanged branch order. In
// original chart units Power[1]-Power[0]=SeparationScale*(x-r)^4, where r is
// the free coordinate at the contact and SeparationScale is positive.
// The first three derivatives agree there. Vertices 0..3 are the branch
// terminals and vertex 4 is the contact. Edges 0..1 and 2..3 follow the two
// branches. Regions are below, between left, between right, and above.
type SplineGraphPolynomialQuarticTangency struct {
	Proven              bool
	Domain              ParameterRectangle
	FreeAxis            int
	Power               [2][]*big.Rat
	SeparationScale     *big.Rat
	SecondDerivative    *big.Rat // common original dependent second derivative
	ThirdDerivative     *big.Rat // common original dependent third derivative
	FourthDerivativeGap *big.Rat // original fourth derivative of Power[1]-Power[0]
	TerminalMasks       [2]uint8
	Vertices            [5]SplineGraphPolynomialVertex
	Edges               [4]SplineGraphPolynomialPiece
	Node                SplineGraphPolynomialJunction
	Regions             [4]SplineGraphPolynomialRegion
	CertifiedCells      []ParameterRectangle
	Graph               SurfaceInjectivity
	ExactOperations     int
}

// DiscoverSplineGraphPolynomialQuarticTangency requires exactly one rational
// order-four contact strictly inside the chart. It independently verifies the
// complete dependent-quadratic identity over every original union-knot cell,
// with cubic or quartic branch powers. Coincident branches and other contact
// patterns fail atomically.
func DiscoverSplineGraphPolynomialQuarticTangency(ctx context.Context, a, b *SurfacePatch, axis int, o GraphPointContactOptions) (SplineGraphPolynomialQuarticTangency, error) {
	var result SplineGraphPolynomialQuarticTangency
	mode := polynomialGraphContactMode{quarticTangencies: 1, certify: result.certifyArrangement}
	pair, e := discoverSplineGraphTwoPolynomials(ctx, a, b, axis, o, mode)
	if e != nil {
		return SplineGraphPolynomialQuarticTangency{ExactOperations: pair.ExactOperations}, e
	}
	result.Proven, result.ExactOperations = true, pair.ExactOperations
	return result, nil
}

// RepresentedVertices checks native UV and XYZ coordinates separately from
// the exact arrangement. A failure returns empty coordinates.
func (r SplineGraphPolynomialQuarticTangency) RepresentedVertices() ([5]Vec2, [5]Vec3, error) {
	var uv [5]Vec2
	var xyz [5]Vec3
	if !r.Proven {
		return uv, xyz, ErrInvalid
	}
	if e := representPolynomialArrangementVertices(r.Vertices[:], uv[:], xyz[:]); e != nil {
		return [5]Vec2{}, [5]Vec3{}, e
	}
	return uv, xyz, nil
}

func (r *SplineGraphPolynomialQuarticTangency) certifyArrangement(ctx context.Context, a *SurfacePatch, pair SplineGraphTwoPolynomials, roots []*big.Rat, d ParameterRectangle, b *polynomialBudget) error {
	r.Domain, r.FreeAxis, r.Power = pair.Domain, pair.FreeAxis, pair.Power
	r.TerminalMasks, r.CertifiedCells, r.Graph = pair.TerminalMasks, pair.CertifiedCells, pair.Graph
	free := roots[0]
	if e := r.certifyTangency(free, b); e != nil {
		return e
	}
	stations := [3]*big.Rat{new(big.Rat).SetFloat64(d.UMin), free, new(big.Rat).SetFloat64(d.UMax)}
	if e := buildPolynomialGraphPieces(ctx, a, pair, stations[:], r.Vertices[:], r.Edges[:], b); e != nil {
		return e
	}
	nodes := [1]SplineGraphPolynomialJunction{r.Node}
	orderPolynomialGraphRegions(r.FreeAxis, r.Graph.ProjectionOrientation, pair.StripSigns[0], []int{0, 0}, nodes[:], r.Regions[:])
	r.Node = nodes[0]
	if e := graphPolynomialRegionWitnesses(pair, d, stations[:], r.Vertices[4].Parameters[1-r.FreeAxis], r.Regions[:], b); e != nil {
		return e
	}
	return ctx.Err()
}

func (r *SplineGraphPolynomialQuarticTangency) certifyTangency(free *big.Rat, b *polynomialBudget) error {
	quotient, e := graphPolynomialSum(r.Power[1], r.Power[0], -1, b)
	if e != nil {
		return e
	}
	factor := rationalPolynomial{new(big.Rat).Neg(free), big.NewRat(1, 1)}
	for range 4 {
		var remainder rationalPolynomial
		quotient, remainder, e = polynomialDivide(quotient, factor, b)
		if e != nil {
			return e
		}
		if len(remainder) != 0 {
			return ErrConvergence
		}
	}
	if len(quotient) != 1 || quotient[0].Sign() <= 0 {
		return ErrConvergence
	}
	r.SeparationScale = quotient[0]
	r.FourthDerivativeGap = new(big.Rat).Mul(big.NewRat(24, 1), r.SeparationScale)
	if e = b.check(r.FourthDerivativeGap); e != nil {
		return e
	}

	// The complete fourth power proves equality through the third derivative.
	// Evaluate the common original chart derivatives on the lower branch.
	first := polynomialDerivative(r.Power[0])
	second := polynomialDerivative(first)
	slope, e := polynomialValue(first, free, b)
	if e != nil {
		return e
	}
	r.SecondDerivative, e = polynomialValue(second, free, b)
	if e != nil {
		return e
	}
	r.ThirdDerivative, e = polynomialValue(polynomialDerivative(second), free, b)
	if e != nil {
		return e
	}
	r.Node.Vertex = 4
	for i := range r.Node.Tangents {
		r.Node.Tangents[i] = graphPolynomialUV(r.FreeAxis, big.NewRat(1, 1), new(big.Rat).Set(slope))
	}
	return nil
}
