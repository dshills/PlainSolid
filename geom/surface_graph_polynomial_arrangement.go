package geom

import (
	"context"
	"math/big"
)

// SplineGraphPolynomialArrangement is a complete original spline contact
// arrangement for two polynomial graph branches. Power[1]-Power[0] equals
// Cofactor times the monic factors at the contacts with multiplicities in
// ContactOrder. Cofactor has the strict sign CofactorSign on the entire
// original free interval. Branch zero is lower at its beginning.
//
// Vertices 0..3 are the two branch terminals in branch order. Contacts
// follow in increasing free order. Edges are in branch order with one
// exact restriction per open slab. Regions give the two exterior regions
// first and last, with between-branch regions in increasing free order.
// LowerBranches records the lower original branch on each open slab.
type SplineGraphPolynomialArrangement struct {
	Proven          bool
	Domain          ParameterRectangle
	FreeAxis        int
	Power           [2][]*big.Rat
	Cofactor        []*big.Rat
	CofactorSign    int
	ContactOrder    []int
	DerivativeGap   []*big.Rat // derivative of order ContactOrder of Power[1]-Power[0]
	LowerBranches   []int
	TerminalMasks   [2]uint8
	Vertices        []SplineGraphPolynomialVertex
	Edges           []SplineGraphPolynomialPiece
	Nodes           []SplineGraphPolynomialJunction
	Regions         []SplineGraphPolynomialRegion
	CertifiedCells  []ParameterRectangle
	Graph           SurfaceInjectivity
	ExactOperations int
}

// DiscoverSplineGraphPolynomialArrangement discovers all interior contacts
// and their orders independently of a prescribed profile. It accepts cubic
// through quintic branches when they account for every coefficient of the
// original dependent quadratic on every union-knot cell. The branches must
// lie strictly inside the dependent chart and have at least one contact.
// Coincident branches, endpoint or irrational contacts, and incomplete
// proofs fail without publishing a partial arrangement.
func DiscoverSplineGraphPolynomialArrangement(ctx context.Context, a, b *SurfacePatch, axis int, o GraphPointContactOptions) (SplineGraphPolynomialArrangement, error) {
	var result SplineGraphPolynomialArrangement
	mode := polynomialGraphContactMode{allowQuintic: true, discoverArrangement: true, certify: result.certifyArrangement}
	pair, e := discoverSplineGraphTwoPolynomials(ctx, a, b, axis, o, mode)
	if e != nil {
		return SplineGraphPolynomialArrangement{ExactOperations: pair.ExactOperations}, e
	}
	result.Proven, result.ExactOperations = true, pair.ExactOperations
	return result, nil
}

// RepresentedVertices checks lossless UV and XYZ coordinates separately
// from the exact proof. It returns no coordinates on failure.
func (r SplineGraphPolynomialArrangement) RepresentedVertices() ([]Vec2, []Vec3, error) {
	if !r.Proven {
		return nil, nil, ErrInvalid
	}
	uv, xyz := make([]Vec2, len(r.Vertices)), make([]Vec3, len(r.Vertices))
	if e := representPolynomialArrangementVertices(r.Vertices, uv, xyz); e != nil {
		return nil, nil, e
	}
	return uv, xyz, nil
}

func (r *SplineGraphPolynomialArrangement) certifyArrangement(ctx context.Context, a *SurfacePatch, pair SplineGraphTwoPolynomials, roots []*big.Rat, d ParameterRectangle, b *polynomialBudget) error {
	r.Domain, r.FreeAxis, r.Power = pair.Domain, pair.FreeAxis, pair.Power
	r.TerminalMasks, r.CertifiedCells, r.Graph = pair.TerminalMasks, pair.CertifiedCells, pair.Graph
	stations := make([]*big.Rat, len(roots)+2)
	stations[0], stations[len(stations)-1] = new(big.Rat).SetFloat64(d.UMin), new(big.Rat).SetFloat64(d.UMax)
	copy(stations[1:], roots)
	r.Vertices = make([]SplineGraphPolynomialVertex, len(roots)+4)
	r.Edges = make([]SplineGraphPolynomialPiece, 2*(len(roots)+1))
	r.Nodes = make([]SplineGraphPolynomialJunction, len(roots))
	r.Regions = make([]SplineGraphPolynomialRegion, len(roots)+3)
	if e := r.certifyContactFactors(ctx, stations, b); e != nil {
		return e
	}
	if e := buildPolynomialGraphPieces(ctx, a, pair, stations, r.Vertices, r.Edges, b); e != nil {
		return e
	}
	orderPolynomialGraphRegions(r.FreeAxis, r.Graph.ProjectionOrientation, pair.StripSigns[0], r.LowerBranches, r.Nodes, r.Regions)
	if e := graphPolynomialRegionWitnesses(pair, d, stations, r.Vertices[4].Parameters[1-r.FreeAxis], r.Regions, b); e != nil {
		return e
	}
	return ctx.Err()
}

func (r *SplineGraphPolynomialArrangement) certifyContactFactors(ctx context.Context, stations []*big.Rat, b *polynomialBudget) error {
	delta, e := graphPolynomialSum(r.Power[1], r.Power[0], -1, b)
	if e != nil {
		return e
	}
	r.Cofactor = delta
	r.CofactorSign = 1
	r.ContactOrder = make([]int, len(r.Nodes))
	r.DerivativeGap = make([]*big.Rat, len(r.Nodes))
	r.LowerBranches = make([]int, len(r.Nodes)+1)
	firstDerivatives := [2]rationalPolynomial{polynomialDerivative(r.Power[0]), polynomialDerivative(r.Power[1])}
	for i, x := range stations[1 : len(stations)-1] {
		factor := rationalPolynomial{new(big.Rat).Neg(x), big.NewRat(1, 1)}
		order := 0
		for len(r.Cofactor) > 1 {
			quotient, remainder, e := polynomialDivide(r.Cofactor, factor, b)
			if e != nil {
				return e
			}
			if len(remainder) > 0 {
				break
			}
			r.Cofactor = quotient
			order++
		}
		if order == 0 || order > 5 {
			return ErrConvergence
		}
		r.ContactOrder[i] = order
		r.LowerBranches[i+1] = r.LowerBranches[i]
		if order%2 != 0 {
			r.LowerBranches[i+1] = 1 - r.LowerBranches[i]
			r.CofactorSign = -r.CofactorSign
		}

		derivative := delta
		for range order {
			derivative = polynomialDerivative(derivative)
		}
		gap, e := polynomialValue(derivative, x, b)
		if e != nil {
			return e
		}
		if gap.Sign() == 0 {
			return ErrConvergence
		}
		r.DerivativeGap[i] = gap
		r.Nodes[i].Vertex = 4 + i
		for j, first := range firstDerivatives {
			slope, e := polynomialValue(first, x, b)
			if e != nil {
				return e
			}
			r.Nodes[i].Tangents[j] = graphPolynomialUV(r.FreeAxis, big.NewRat(1, 1), slope)
		}
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	// Recheck the quotient independently. It can have roots outside the
	// chart, so even a nonconstant quotient with equal endpoint signs needs
	// an exact proof that it never vanishes anywhere on the closed interval.
	lo, hi := stations[0], stations[len(stations)-1]
	start, e := polynomialValue(r.Cofactor, lo, b)
	if e != nil {
		return e
	}
	if start.Sign() != r.CofactorSign {
		return ErrConvergence
	}
	_, e = isolatePolynomialRoots(r.Cofactor, lo, hi, b, 0, func(_, _ *big.Rat, _ int) (bool, error) {
		return false, ErrConvergence
	})
	return e
}
