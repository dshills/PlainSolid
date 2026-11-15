package geom

import (
	"context"
	"math/big"
	"slices"
)

// SplineGraphPolynomialVertex is an exact original chart and model vertex.
type SplineGraphPolynomialVertex struct {
	Parameters [2]*big.Rat
	Point      [3]*big.Rat
}

// SplineGraphPolynomialPiece is one of the six original contact-branch
// restrictions. Controls are exact original UV Bézier controls on the piece;
// Start and End index the vertices in increasing free-coordinate order.
type SplineGraphPolynomialPiece struct {
	Branch, Start, End int
	Controls           [][2]*big.Rat
}

type SplineGraphPolynomialUse struct {
	Edge     int
	Reversed bool
}

// SplineGraphPolynomialPerimeter closes a region from Start to End on the
// original rectangle perimeter (+1 CCW UV, -1 CW UV).
type SplineGraphPolynomialPerimeter struct {
	Start, End, ParameterDirection int
}

// SplineGraphPolynomialRegion is bounded by a directed chain of contact edges
// followed by its perimeter arc, if any. It is positive in model XY. HeightSide
// gives the sign of A.Z-B.Z throughout the open region.
type SplineGraphPolynomialRegion struct {
	Edges      []SplineGraphPolynomialUse
	Perimeter  *SplineGraphPolynomialPerimeter
	HeightSide int
	Witness    [2]*big.Rat
}

// SplineGraphPolynomialJunction records a transverse contact vertex. Tangents
// direct the two original UV branches toward increasing free parameter.
// RayCycle lists incident edges CCW in model XY. Regions[i] identifies the
// region between RayCycle[i] and the next ray.
type SplineGraphPolynomialJunction struct {
	Vertex   int
	Tangents [2][2]*big.Rat
	RayCycle [4]int
	Regions  [4]int
}

// SplineGraphTwoPolynomialNodes proves the complete original UV contact
// arrangement. Branch powers use the original free coordinate. Branch zero is
// lower at the low free-coordinate endpoint. Vertices 0..3 are the start/end
// of each branch; 4 and 5 are the two nodes in order. Edges 0..2 follow branch
// zero and 3..5 follow branch one. Regions 0 and 4 are below and above both
// branches; 1..3 are the between-branch regions in increasing free order.
// Exact model vertices use the original common XY chart. Geometry across
// original knots is certified cellwise and does not assume C1 model lifts.
type SplineGraphTwoPolynomialNodes struct {
	Proven          bool
	Domain          ParameterRectangle
	FreeAxis        int
	Power           [2][]*big.Rat
	TerminalMasks   [2]uint8
	Vertices        [6]SplineGraphPolynomialVertex
	Edges           [6]SplineGraphPolynomialPiece
	Nodes           [2]SplineGraphPolynomialJunction
	Regions         [5]SplineGraphPolynomialRegion
	CertifiedCells  []ParameterRectangle
	Graph           SurfaceInjectivity
	ExactOperations int
}

// DiscoverSplineGraphTwoPolynomialNodes proves exactly two simple rational
// intersections of cubic/quartic branches strictly inside the dependent chart
// bounds. The complete two-factor identity is checked on every original
// union-knot cell. Coincident or multiple roots are ErrDegenerate, a root at an
// original free endpoint is ErrDomain, and an irrational node is
// ErrUnrepresentable. Other unresolved proofs return an error without geometry.
func DiscoverSplineGraphTwoPolynomialNodes(ctx context.Context, a, b *SurfacePatch, axis int, o GraphPointContactOptions) (SplineGraphTwoPolynomialNodes, error) {
	var result SplineGraphTwoPolynomialNodes
	pair, e := discoverSplineGraphTwoPolynomials(ctx, a, b, axis, o, nil, &result)
	if e != nil {
		return SplineGraphTwoPolynomialNodes{ExactOperations: pair.ExactOperations}, e
	}
	result.ExactOperations = pair.ExactOperations
	result.Proven = true
	return result, nil
}

// RepresentedVertices checks native UV and XYZ coordinate admission separately
// from exact arrangement discovery. A rational node may have no lossless
// float64 representation. No rounded coordinates are returned on error.
func (r SplineGraphTwoPolynomialNodes) RepresentedVertices() ([6]Vec2, [6]Vec3, error) {
	var uv [6]Vec2
	var xyz [6]Vec3
	if !r.Proven {
		return uv, xyz, ErrInvalid
	}
	for i, v := range r.Vertices {
		if v.Parameters[0] == nil || v.Parameters[1] == nil {
			return [6]Vec2{}, [6]Vec3{}, ErrInvalid
		}
		u, e := RepresentedExactScalar(v.Parameters[0])
		if e != nil {
			return [6]Vec2{}, [6]Vec3{}, e
		}
		w, e := RepresentedExactScalar(v.Parameters[1])
		if e != nil {
			return [6]Vec2{}, [6]Vec3{}, e
		}
		uv[i] = Vec2{u, w}
		xyz[i], e = RepresentedExactPoint(v.Point)
		if e != nil {
			return [6]Vec2{}, [6]Vec3{}, e
		}
	}
	return uv, xyz, nil
}

func (r *SplineGraphTwoPolynomialNodes) certifyArrangement(ctx context.Context, a *SurfacePatch, pair SplineGraphTwoPolynomials, free [2]*big.Rat, d ParameterRectangle, b *polynomialBudget) error {
	r.Domain, r.FreeAxis, r.Power = pair.Domain, pair.FreeAxis, pair.Power
	r.TerminalMasks, r.CertifiedCells, r.Graph = pair.TerminalMasks, pair.CertifiedCells, pair.Graph
	chart := func(x, y *big.Rat) [2]*big.Rat {
		if r.FreeAxis == 1 {
			return [2]*big.Rat{y, x}
		}
		return [2]*big.Rat{x, y}
	}
	stations := [4]*big.Rat{new(big.Rat).SetFloat64(d.UMin), free[0], free[1], new(big.Rat).SetFloat64(d.UMax)}
	vertexID := [2][4]int{{0, 4, 5, 1}, {2, 4, 5, 3}}
	for k, power := range r.Power {
		for i := 0; i < 3; i++ {
			width := new(big.Rat).Sub(stations[i+1], stations[i])
			if e := b.check(width); e != nil {
				return e
			}
			local, e := composeProjectionPolynomial(power, rationalPolynomial{stations[i], width}, b)
			if e != nil {
				return e
			}
			bern, e := graphFactorBernstein(local, b)
			if e != nil {
				return e
			}
			edge := SplineGraphPolynomialPiece{Branch: k, Start: vertexID[k][i], End: vertexID[k][i+1], Controls: make([][2]*big.Rat, len(bern))}
			for j, y := range bern {
				x := new(big.Rat).Add(stations[i], new(big.Rat).Mul(width, big.NewRat(int64(j), int64(len(bern)-1))))
				if e := b.check(x); e != nil {
					return e
				}
				edge.Controls[j] = chart(x, y)
			}
			r.Edges[3*k+i] = edge
			for j, id := range [2]int{edge.Start, edge.End} {
				uv := edge.Controls[j*(len(bern)-1)]
				v := &r.Vertices[id]
				if v.Parameters[0] != nil {
					if v.Parameters[0].Cmp(uv[0]) != 0 || v.Parameters[1].Cmp(uv[1]) != 0 {
						return ErrConvergence
					}
					continue
				}
				v.Parameters = [2]*big.Rat{new(big.Rat).Set(uv[0]), new(big.Rat).Set(uv[1])}
			}
		}
	}
	for i := range r.Vertices {
		v := &r.Vertices[i]
		remaining := b.maxOperations - b.operations
		if remaining < 1 {
			return ErrConvergence
		}
		p, e := a.EvaluateRational(ctx, v.Parameters[0], v.Parameters[1], SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: b.maxBits})
		b.operations += p.ExactOperations
		if e != nil {
			return e
		}
		v.Point = p.Point
	}
	orientation := r.Graph.ProjectionOrientation
	if r.FreeAxis == 1 {
		orientation = -orientation
	}
	r.Nodes = [2]SplineGraphPolynomialJunction{
		{Vertex: 4, RayCycle: [4]int{4, 1, 3, 0}, Regions: [4]int{2, 4, 1, 0}},
		{Vertex: 5, RayCycle: [4]int{2, 5, 1, 4}, Regions: [4]int{3, 4, 2, 0}},
	}
	for i := range r.Nodes {
		n := &r.Nodes[i]
		for j, power := range r.Power {
			slope, e := polynomialValue(polynomialDerivative(power), free[i], b)
			if e != nil {
				return e
			}
			n.Tangents[j] = chart(big.NewRat(1, 1), slope)
		}
		if orientation < 0 {
			slices.Reverse(n.RayCycle[1:])
			slices.Reverse(n.Regions[:])
		}
	}
	r.orderRegions(orientation, pair.StripSigns[0])
	between := func(x, y *big.Rat) *big.Rat {
		return new(big.Rat).Quo(new(big.Rat).Add(x, y), big.NewRat(2, 1))
	}
	nodeParameter := r.Vertices[4].Parameters[1-r.FreeAxis]
	r.Regions[0].Witness = chart(new(big.Rat).Set(free[0]), between(new(big.Rat).SetFloat64(d.VMin), nodeParameter))
	r.Regions[4].Witness = chart(new(big.Rat).Set(free[0]), between(nodeParameter, new(big.Rat).SetFloat64(d.VMax)))
	for i := 0; i < 3; i++ {
		x := between(stations[i], stations[i+1])
		f, e := polynomialValue(r.Power[0], x, b)
		if e != nil {
			return e
		}
		g, e := polynomialValue(r.Power[1], x, b)
		if e != nil {
			return e
		}
		r.Regions[i+1].Witness = chart(x, between(f, g))
	}
	for _, reg := range r.Regions {
		for _, x := range reg.Witness {
			if e := b.check(x); e != nil {
				return e
			}
		}
	}
	return ctx.Err()
}

// The lower branch on the three slabs is 0,1,0. These closed boundaries
// have positive orientation in the free/dependent chart. The middle lens
// has two distinct edges with the same endpoints and no perimeter arc.
func (r *SplineGraphTwoPolynomialNodes) orderRegions(orientation, sign int) {
	perimeter := func(start, end int) *SplineGraphPolynomialPerimeter {
		return &SplineGraphPolynomialPerimeter{start, end, r.Graph.ProjectionOrientation}
	}
	r.Regions = [5]SplineGraphPolynomialRegion{
		{Edges: []SplineGraphPolynomialUse{{2, true}, {4, true}, {0, true}}, Perimeter: perimeter(0, 1), HeightSide: sign},
		{Edges: []SplineGraphPolynomialUse{{0, false}, {3, true}}, Perimeter: perimeter(2, 0), HeightSide: -sign},
		{Edges: []SplineGraphPolynomialUse{{4, false}, {1, true}}, HeightSide: -sign},
		{Edges: []SplineGraphPolynomialUse{{5, true}, {2, false}}, Perimeter: perimeter(1, 3), HeightSide: -sign},
		{Edges: []SplineGraphPolynomialUse{{3, false}, {1, false}, {5, false}}, Perimeter: perimeter(3, 2), HeightSide: sign},
	}
	if orientation >= 0 {
		return
	}
	for i := range r.Regions {
		reg := &r.Regions[i]
		slices.Reverse(reg.Edges)
		for j := range reg.Edges {
			reg.Edges[j].Reversed = !reg.Edges[j].Reversed
		}
		if p := reg.Perimeter; p != nil {
			p.Start, p.End = p.End, p.Start
		}
	}
}
