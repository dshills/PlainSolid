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
	orders, e := certifyPolynomialGraphContactOrders(r.FreeAxis, r.Power, stations[:], r.Nodes[:], 1, 0, b)
	if e != nil {
		return e
	}
	r.ContactOrder, r.DerivativeGap, r.Cofactor = [2]int(orders.orders), [2]*big.Rat(orders.gaps), orders.cofactor
	if e = buildPolynomialGraphPieces(ctx, a, pair, stations[:], r.Vertices[:], r.Edges[:], b); e != nil {
		return e
	}
	orderPolynomialGraphRegions(r.FreeAxis, r.Graph.ProjectionOrientation, pair.StripSigns[0], orders.lower, r.Nodes[:], r.Regions[:])
	if e = graphPolynomialRegionWitnesses(pair, d, stations[:], r.Vertices[4].Parameters[1-r.FreeAxis], r.Regions[:], b); e != nil {
		return e
	}
	return ctx.Err()
}

type polynomialGraphContactOrders struct {
	orders   []int
	gaps     []*big.Rat
	lower    []int
	cofactor rationalPolynomial
}

// certifyPolynomialGraphContactOrders rechecks the local derivatives and the
// complete factorization of the prescribed simple, double or triple roots.
// These profiles use at least three degrees, leaving a constant or affine cofactor.
func certifyPolynomialGraphContactOrders(axis int, power [2][]*big.Rat, stations []*big.Rat, nodes []SplineGraphPolynomialJunction,
	expectedTangencies, expectedCubicCrossings int, b *polynomialBudget) (polynomialGraphContactOrders, error) {
	zero := polynomialGraphContactOrders{}
	delta, e := graphPolynomialSum(power[1], power[0], -1, b)
	if e != nil {
		return zero, e
	}
	out := polynomialGraphContactOrders{
		orders: make([]int, len(nodes)), gaps: make([]*big.Rat, len(nodes)),
		lower: make([]int, len(nodes)+1), cofactor: delta,
	}
	dep := 1 - axis
	derivatives := [2]rationalPolynomial{polynomialDerivative(power[0]), polynomialDerivative(power[1])}
	secondDifference := polynomialDerivative(polynomialDerivative(delta))
	orders := [4]int{}
	totalOrder := 0
	for i, x := range stations[1 : len(stations)-1] {
		n := &nodes[i]
		n.Vertex = 4 + i
		for j, derivative := range derivatives {
			slope, e := polynomialValue(derivative, x, b)
			if e != nil {
				return zero, e
			}
			n.Tangents[j] = graphPolynomialUV(axis, big.NewRat(1, 1), slope)
		}
		gap := new(big.Rat).Sub(n.Tangents[1][dep], n.Tangents[0][dep])
		if e = b.check(gap); e != nil {
			return zero, e
		}
		order := 1
		if gap.Sign() == 0 {
			order = 2
			gap, e = polynomialValue(secondDifference, x, b)
			if e != nil {
				return zero, e
			}
			if gap.Sign() == 0 {
				if expectedCubicCrossings == 0 {
					return zero, ErrDegenerate
				}
				order = 3
				gap, e = polynomialValue(polynomialDerivative(secondDifference), x, b)
				if e != nil {
					return zero, e
				}
				if gap.Sign() == 0 {
					return zero, ErrDegenerate
				}
			}
		}
		out.lower[i+1] = out.lower[i]
		if order%2 == 1 {
			out.lower[i+1] = 1 - out.lower[i]
		}
		orders[order]++
		totalOrder += order
		out.orders[i], out.gaps[i] = order, gap
		factor := rationalPolynomial{new(big.Rat).Neg(x), big.NewRat(1, 1)}
		for range order {
			var remainder rationalPolynomial
			out.cofactor, remainder, e = polynomialDivide(out.cofactor, factor, b)
			if e != nil {
				return zero, e
			}
			if len(remainder) != 0 {
				return zero, ErrConvergence
			}
		}
	}
	maxCofactorCoefficients := 5 - totalOrder
	if orders[2] != expectedTangencies || orders[3] != expectedCubicCrossings ||
		len(out.cofactor) == 0 || len(out.cofactor) > maxCofactorCoefficients {
		return zero, ErrConvergence
	}
	// At the low endpoint the gap is positive. Odd powers of the monic
	// factors are negative there and even powers are positive. A constant or
	// affine cofactor has the resulting strict sign throughout if both ends do.
	sign := 1 - 2*((orders[1]+orders[3])%2)
	for _, x := range []*big.Rat{stations[0], stations[len(stations)-1]} {
		y, e := polynomialValue(out.cofactor, x, b)
		if e != nil {
			return zero, e
		}
		if y.Sign() != sign {
			return zero, ErrConvergence
		}
	}
	return out, nil
}
