package geom

import (
	"context"
	"math/big"
)

// SplineGraphPolynomialMixedContact proves the complete original UV contact
// arrangement of two cubic or quartic graph branches with one transverse
// crossing and one quadratic tangency. Power uses original chart units, and
// branch zero is lower at the low free endpoint. Vertices 0..3 are the branch
// terminals, and vertices 4 and 5 are the contacts in increasing free order.
// Edges 0..2 and 3..5 follow the two original branches. Regions 0 and 4 are
// below and above both branches, and regions 1..3 lie between them in order.
// The proof retains the original roof supports cellwise. Native trim and
// shell admission require separate proofs.
type SplineGraphPolynomialMixedContact struct {
	Proven          bool
	Domain          ParameterRectangle
	FreeAxis        int
	Power           [2][]*big.Rat
	TerminalMasks   [2]uint8
	Vertices        [6]SplineGraphPolynomialVertex
	Edges           [6]SplineGraphPolynomialPiece
	Nodes           [2]SplineGraphPolynomialJunction
	ContactOrder    [2]int      // root multiplicity in increasing original free-coordinate order
	DerivativeGap   [2]*big.Rat // derivative of order ContactOrder of Power[1]-Power[0] at each contact
	Cofactor        []*big.Rat  // original Power[1]-Power[0] divided by the simple and squared contact factors
	Regions         [5]SplineGraphPolynomialRegion
	CertifiedCells  []ParameterRectangle
	Graph           SurfaceInjectivity
	ExactOperations int
}

// DiscoverSplineGraphPolynomialMixedContact certifies the original two-factor
// identity over every union-knot cell. It requires exactly one simple root and
// one double root at distinct, interior rational stations, in either order.
// Coincident factors and higher multiplicities return ErrDegenerate, endpoints
// return ErrDomain, and irrational contacts return ErrUnrepresentable.
func DiscoverSplineGraphPolynomialMixedContact(ctx context.Context, a, b *SurfacePatch, axis int, o GraphPointContactOptions) (SplineGraphPolynomialMixedContact, error) {
	var result SplineGraphPolynomialMixedContact
	mode := polynomialGraphContactMode{crossings: 1, tangencies: 1, certify: result.certifyArrangement}
	pair, e := discoverSplineGraphTwoPolynomials(ctx, a, b, axis, o, mode)
	if e != nil {
		return SplineGraphPolynomialMixedContact{ExactOperations: pair.ExactOperations}, e
	}
	result.Proven = true
	result.ExactOperations = pair.ExactOperations
	return result, nil
}

// RepresentedVertices checks lossless native UV and XYZ conversion separately
// from the exact arrangement and returns empty coordinates on failure.
func (r SplineGraphPolynomialMixedContact) RepresentedVertices() ([6]Vec2, [6]Vec3, error) {
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

func (r *SplineGraphPolynomialMixedContact) certifyArrangement(ctx context.Context, a *SurfacePatch, pair SplineGraphTwoPolynomials, roots []*big.Rat, d ParameterRectangle, b *polynomialBudget) error {
	r.Domain, r.FreeAxis, r.Power = pair.Domain, pair.FreeAxis, pair.Power
	r.TerminalMasks, r.CertifiedCells, r.Graph = pair.TerminalMasks, pair.CertifiedCells, pair.Graph
	stations := [4]*big.Rat{new(big.Rat).SetFloat64(d.UMin), roots[0], roots[1], new(big.Rat).SetFloat64(d.UMax)}
	lower, e := r.certifyContactOrders(stations, b)
	if e != nil {
		return e
	}
	if e = buildPolynomialGraphPieces(ctx, a, pair, stations[:], r.Vertices[:], r.Edges[:], b); e != nil {
		return e
	}
	orderPolynomialGraphRegions(r.FreeAxis, r.Graph.ProjectionOrientation, pair.StripSigns[0], lower[:], r.Nodes[:], r.Regions[:])
	if e = graphPolynomialRegionWitnesses(pair, d, stations[:], r.Vertices[4].Parameters[1-r.FreeAxis], r.Regions[:], b); e != nil {
		return e
	}
	return ctx.Err()
}

func (r *SplineGraphPolynomialMixedContact) certifyContactOrders(stations [4]*big.Rat, b *polynomialBudget) ([3]int, error) {
	var lower [3]int
	delta, e := graphPolynomialSum(r.Power[1], r.Power[0], -1, b)
	if e != nil {
		return lower, e
	}
	cofactor := delta
	for i, x := range stations[1:3] {
		n := &r.Nodes[i]
		n.Vertex = 4 + i
		for j, power := range r.Power {
			slope, e := polynomialValue(polynomialDerivative(power), x, b)
			if e != nil {
				return lower, e
			}
			n.Tangents[j] = graphPolynomialUV(r.FreeAxis, big.NewRat(1, 1), slope)
		}
		dep := 1 - r.FreeAxis
		gap := new(big.Rat).Sub(n.Tangents[1][dep], n.Tangents[0][dep])
		if e = b.check(gap); e != nil {
			return lower, e
		}
		order := 1
		lower[i+1] = 1 - lower[i]
		if gap.Sign() == 0 {
			order = 2
			lower[i+1] = lower[i]
			gap, e = polynomialValue(polynomialDerivative(polynomialDerivative(delta)), x, b)
			if e != nil {
				return lower, e
			}
			if gap.Sign() == 0 {
				return lower, ErrDegenerate
			}
		}
		r.ContactOrder[i], r.DerivativeGap[i] = order, gap
		factor := rationalPolynomial{new(big.Rat).Neg(x), big.NewRat(1, 1)}
		for range order {
			var remainder rationalPolynomial
			cofactor, remainder, e = polynomialDivide(cofactor, factor, b)
			if e != nil {
				return lower, e
			}
			if len(remainder) != 0 {
				return lower, ErrConvergence
			}
		}
	}
	if r.ContactOrder[0]+r.ContactOrder[1] != 3 || len(cofactor) == 0 || len(cofactor) > 2 {
		return lower, ErrConvergence
	}
	// The gap is positive at the low endpoint by branch labeling. The simple
	// factor there is negative and the squared factor is positive. A constant
	// or affine cofactor is strictly negative throughout if both ends are.
	for _, x := range []*big.Rat{stations[0], stations[3]} {
		y, e := polynomialValue(cofactor, x, b)
		if e != nil {
			return lower, e
		}
		if y.Sign() >= 0 {
			return lower, ErrConvergence
		}
	}
	r.Cofactor = cofactor
	return lower, nil
}
