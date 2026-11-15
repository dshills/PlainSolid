package geom

import (
	"context"
	"math/big"
)

// SplineGraphPolynomialCubicCrossing proves a sign-changing order-three
// contact on the two original graph branches. If x is the free coordinate
// and r is the interior station, Power[1]-Power[0]=(x-r)^3*Separation(x).
// Separation is strictly negative on the original domain. The lower branch
// is zero before the crossing and one after it. The two branches have equal
// first and second derivatives in the original UV chart at the crossing.
type SplineGraphPolynomialCubicCrossing struct {
	Proven             bool
	Domain             ParameterRectangle
	FreeAxis           int
	Power              [2][]*big.Rat
	Separation         []*big.Rat
	SecondDerivative   *big.Rat // common second derivative of the original dependent coordinate
	ThirdDerivativeGap *big.Rat // Power[1] third derivative minus Power[0]'s
	TerminalMasks      [2]uint8
	Vertices           [5]SplineGraphPolynomialVertex // four branch terminals, then the crossing
	Edges              [4]SplineGraphPolynomialPiece  // two per branch in free order
	Node               SplineGraphPolynomialJunction
	Regions            [4]SplineGraphPolynomialRegion // below, between left, between right, above
	CertifiedCells     []ParameterRectangle
	Graph              SurfaceInjectivity
	ExactOperations    int
}

// DiscoverSplineGraphPolynomialCubicCrossing requires exactly one interior
// rational triple root. Every original union-knot cell must reproduce the
// full dependent-quadratic identity for the two cubic or quartic branches.
// The proof rejects coincident branches, higher multiplicity, free endpoint
// contacts and any additional interior contact.
func DiscoverSplineGraphPolynomialCubicCrossing(ctx context.Context, a, b *SurfacePatch, axis int, o GraphPointContactOptions) (SplineGraphPolynomialCubicCrossing, error) {
	var result SplineGraphPolynomialCubicCrossing
	mode := polynomialGraphContactMode{cubicCrossings: 1, certify: result.certifyArrangement}
	pair, e := discoverSplineGraphTwoPolynomials(ctx, a, b, axis, o, mode)
	if e != nil {
		return SplineGraphPolynomialCubicCrossing{ExactOperations: pair.ExactOperations}, e
	}
	result.Proven, result.ExactOperations = true, pair.ExactOperations
	return result, nil
}

// RepresentedVertices checks lossless native UV and XYZ conversion separately
// from the exact contact arrangement. A failure returns empty coordinates.
func (r SplineGraphPolynomialCubicCrossing) RepresentedVertices() ([5]Vec2, [5]Vec3, error) {
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

func (r *SplineGraphPolynomialCubicCrossing) certifyArrangement(ctx context.Context, a *SurfacePatch, pair SplineGraphTwoPolynomials, roots []*big.Rat, domain ParameterRectangle, b *polynomialBudget) error {
	r.Domain, r.FreeAxis, r.Power = pair.Domain, pair.FreeAxis, pair.Power
	r.TerminalMasks, r.CertifiedCells, r.Graph = pair.TerminalMasks, pair.CertifiedCells, pair.Graph
	stations := [3]*big.Rat{new(big.Rat).SetFloat64(domain.UMin), roots[0], new(big.Rat).SetFloat64(domain.UMax)}
	if e := r.certifyCrossing(stations, b); e != nil {
		return e
	}
	if e := buildPolynomialGraphPieces(ctx, a, pair, stations[:], r.Vertices[:], r.Edges[:], b); e != nil {
		return e
	}
	nodes := [1]SplineGraphPolynomialJunction{r.Node}
	orderPolynomialGraphRegions(r.FreeAxis, r.Graph.ProjectionOrientation, pair.StripSigns[0], []int{0, 1}, nodes[:], r.Regions[:])
	r.Node = nodes[0]
	if e := graphPolynomialRegionWitnesses(pair, domain, stations[:], r.Vertices[4].Parameters[1-r.FreeAxis], r.Regions[:], b); e != nil {
		return e
	}
	return ctx.Err()
}

func (r *SplineGraphPolynomialCubicCrossing) certifyCrossing(stations [3]*big.Rat, b *polynomialBudget) error {
	x := stations[1]
	quotient, e := graphPolynomialSum(r.Power[1], r.Power[0], -1, b)
	if e != nil {
		return e
	}
	factor := rationalPolynomial{new(big.Rat).Neg(x), big.NewRat(1, 1)}
	for range 3 {
		var remainder rationalPolynomial
		quotient, remainder, e = polynomialDivide(quotient, factor, b)
		if e != nil {
			return e
		}
		if len(remainder) != 0 {
			return ErrConvergence
		}
	}
	if len(quotient) == 0 || len(quotient) > 2 {
		return ErrConvergence
	}
	// The constant or affine quotient has a strict sign on the full span
	// if both endpoint signs are negative. The triple root then switches
	// the branch order and no other contact is possible.
	for _, endpoint := range []*big.Rat{stations[0], stations[2]} {
		y, e := polynomialValue(quotient, endpoint, b)
		if e != nil {
			return e
		}
		if y.Sign() >= 0 {
			return ErrConvergence
		}
	}
	r.Separation = quotient
	gap, e := polynomialValue(quotient, x, b)
	if e != nil {
		return e
	}
	r.ThirdDerivativeGap = gap.Mul(gap, big.NewRat(6, 1))
	if e = b.check(r.ThirdDerivativeGap); e != nil {
		return e
	}

	// The proved cubic factor makes the two first and second derivatives
	// equal. Evaluate the common chart derivatives on one original branch.
	first := polynomialDerivative(r.Power[0])
	slope, e := polynomialValue(first, x, b)
	if e != nil {
		return e
	}
	r.SecondDerivative, e = polynomialValue(polynomialDerivative(first), x, b)
	if e != nil {
		return e
	}
	r.Node.Vertex = 4
	for i := range r.Node.Tangents {
		r.Node.Tangents[i] = graphPolynomialUV(r.FreeAxis, big.NewRat(1, 1), new(big.Rat).Set(slope))
	}
	return nil
}
