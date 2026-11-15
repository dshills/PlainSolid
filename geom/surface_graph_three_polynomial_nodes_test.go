package geom

import (
	"context"
	"errors"
	"math/big"
	"reflect"
	"testing"
)

func threeNodeGraphGap(firstDenominator int64) rationalPolynomial {
	// (1-firstDenominator*u)(1-2u)(3-4u)(1+u)/128. The final factor is
	// strictly positive on the chart, while the others give distinct crossings.
	return rationalPolynomial{
		big.NewRat(3, 128), big.NewRat(-3*firstDenominator-7, 128), big.NewRat(7*firstDenominator-2, 128),
		big.NewRat(2*firstDenominator+8, 128), big.NewRat(-8*firstDenominator, 128),
	}
}

func TestSplineGraphThreePolynomialNodes(t *testing.T) {
	ctx := context.Background()
	for _, axis := range []int{0, 1} {
		a, b, want := polynomialPairGraphFixture(t, axis, threeNodeGraphGap(4))
		for order, pair := range [][2]*SurfacePatch{{a, b}, {b, a}} {
			r, e := DiscoverSplineGraphThreePolynomialNodes(ctx, pair[0], pair[1], axis, GraphPointContactOptions{})
			if e != nil || !r.Proven || r.FreeAxis != axis || len(r.CertifiedCells) != 4 ||
				!reflect.DeepEqual(r.Power, [2][]*big.Rat{want[0], want[1]}) {
				t.Fatal("original mixed-degree three-node arrangement", axis, order, r, e)
			}
			for i := range r.Nodes {
				x := big.NewRat(int64(i+1), 4)
				v := r.Vertices[4+i]
				if r.Nodes[i].Vertex != 4+i || v.Parameters[axis].Cmp(x) != 0 || v.Parameters[1-axis].Cmp(testGraphPowerValue(want[0], x)) != 0 {
					t.Fatal("wrong exact original free station or ordinate", i, v)
				}
			}
			if u, p, e := r.RepresentedVertices(); e != nil || u[4] == u[5] || u[5] == u[6] || p[4] == p[5] || p[5] == p[6] {
				t.Fatal("native representability of three separate points", u, p, e)
			}
			checkThreePolynomialNodeOwners(t, r, pair, 1-2*order)
			for i, want := range []int64{289, 41, 49, 521} {
				if got := testPolynomialStripArea(axis, r.Edges[:], r.Regions[i+1]); got.Cmp(big.NewRat(want, 122880)) != 0 {
					t.Fatal("independent analytical strip area", i, got, want)
				}
			}
			for _, limit := range []int{r.ExactOperations - 1, r.ExactOperations} {
				q, e := DiscoverSplineGraphThreePolynomialNodes(ctx, pair[0], pair[1], axis, GraphPointContactOptions{MaxOperations: limit})
				if (e == nil) != (limit == r.ExactOperations) || (e != nil && !reflect.DeepEqual(q, SplineGraphThreePolynomialNodes{ExactOperations: q.ExactOperations})) {
					t.Fatal("atomic exact work limit", e)
				}
			}
			if q, e := DiscoverSplineGraphTwoPolynomialNodes(ctx, pair[0], pair[1], axis, GraphPointContactOptions{}); e == nil || q.Proven {
				t.Fatal("two-node API accepted a third crossing", e)
			}
		}
	}
}

func checkThreePolynomialNodeOwners(t *testing.T, r SplineGraphThreePolynomialNodes, pair [2]*SurfacePatch, exteriorSign int) {
	t.Helper()
	ctx := context.Background()
	for _, v := range r.Vertices {
		for _, owner := range pair {
			p, e := owner.EvaluateRational(ctx, v.Parameters[0], v.Parameters[1], SurfacePatchEvaluationOptions{})
			if e != nil || !reflect.DeepEqual(p.Point, v.Point) {
				t.Fatal("exact original vertex lift", v, p.Point, e)
			}
		}
	}
	for _, edge := range r.Edges {
		if len(edge.Controls) != len(r.Power[edge.Branch]) {
			t.Fatal("source curve degree changed", edge)
		}
		for _, t0 := range []*big.Rat{new(big.Rat), big.NewRat(1, 3), big.NewRat(1, 1)} {
			uv := testGraphBezierValue(edge.Controls, t0)
			if uv[1-r.FreeAxis].Cmp(testGraphPowerValue(r.Power[edge.Branch], uv[r.FreeAxis])) != 0 {
				t.Fatal("wrong original branch restriction", edge, t0)
			}
			a, e := pair[0].EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			b, e := pair[1].EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{})
			if e != nil || !reflect.DeepEqual(a.Point, b.Point) {
				t.Fatal("paired original source lifts", uv, a.Point, b.Point, e)
			}
		}
	}
	var uses [8][2]int
	for i, region := range r.Regions {
		sign := -exteriorSign
		if i == 0 || i == 5 {
			sign = exteriorSign
		}
		u, v := region.Witness[0], region.Witness[1]
		a, e := pair[0].EvaluateRational(ctx, u, v, SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		b, e := pair[1].EvaluateRational(ctx, u, v, SurfacePatchEvaluationOptions{})
		if e != nil || region.HeightSide != sign || a.Point[2].Cmp(b.Point[2]) != sign {
			t.Fatal("original height sign", i, region, e)
		}
		begin, current := -1, -1
		for _, use := range region.Edges {
			edge := r.Edges[use.Edge]
			start, end, dir := edge.Start, edge.End, 0
			if use.Reversed {
				start, end, dir = end, start, 1
			}
			uses[use.Edge][dir]++
			if current == -1 {
				begin = start
			} else if current != start {
				t.Fatal("disconnected original region boundary", i, region)
			}
			current = end
		}
		if i == 2 || i == 3 {
			if region.Perimeter != nil || current != begin || len(region.Edges) != 2 || region.Edges[0].Edge == region.Edges[1].Edge {
				t.Fatal("middle lens must retain two distinct branch restrictions", i, region)
			}
		} else if p := region.Perimeter; p == nil || p.Start != current || p.End != begin || p.ParameterDirection != r.Graph.ProjectionOrientation {
			t.Fatal("wrong original perimeter route", i, region)
		}
	}
	for _, use := range uses {
		if use != ([2]int{1, 1}) {
			t.Fatal("each source branch restriction must have two opposite uses", uses)
		}
	}
	checkPolynomialNodeRays(t, r.Nodes[:], r.Edges[:], r.Regions[:], r.Graph.ProjectionOrientation)
}

func TestSplineGraphThreePolynomialNodesOriginalUnits(t *testing.T) {
	ctx := context.Background()
	a, b, _ := polynomialPairGraphFixture(t, 0, threeNodeGraphGap(4))
	pair := [2]*SurfacePatch{a, b}
	for i, p := range pair {
		for j := range p.knotsU {
			p.knotsU[j] = 2 + 3*p.knotsU[j]
		}
		for j := range p.knotsV {
			p.knotsV[j] = -1 + 2*p.knotsV[j]
		}
		var e error
		pair[i], e = p.Transformed(Scaling(Vec3{X: -1, Y: 1, Z: 1}))
		if e != nil {
			t.Fatal(e)
		}
	}
	r, e := DiscoverSplineGraphThreePolynomialNodes(ctx, pair[0], pair[1], 0, GraphPointContactOptions{})
	if e != nil || !r.Proven || r.Graph.ProjectionOrientation != -1 || r.Vertices[4].Parameters[0].Cmp(big.NewRat(11, 4)) != 0 ||
		r.Vertices[5].Parameters[0].Cmp(big.NewRat(7, 2)) != 0 || r.Vertices[6].Parameters[0].Cmp(big.NewRat(17, 4)) != 0 {
		t.Fatal("reflected nonunit original chart", r, e)
	}
	checkThreePolynomialNodeOwners(t, r, pair, 1)
}

func TestSplineGraphThreePolynomialNodesScope(t *testing.T) {
	ctx := context.Background()
	a, b, _ := polynomialPairGraphFixture(t, 0, threeNodeGraphGap(3))
	r, e := DiscoverSplineGraphThreePolynomialNodes(ctx, a, b, 0, GraphPointContactOptions{})
	if e != nil || !r.Proven || r.Vertices[4].Parameters[0].Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatal("exact nonbinary original arrangement", e)
	}
	if u, p, e := r.RepresentedVertices(); !errors.Is(e, ErrUnrepresentable) || u != ([7]Vec2{}) || p != ([7]Vec3{}) {
		t.Fatal("native representability should be a separate atomic gate", e)
	}
	fourNodes := rationalPolynomial{big.NewRat(1, 8)}
	budget := &polynomialBudget{maxOperations: 1000, maxBits: 1024}
	for _, n := range []int64{1, 3, 5, 7} {
		fourNodes, e = polynomialProduct(fourNodes, rationalPolynomial{big.NewRat(-n, 8), big.NewRat(1, 1)}, budget)
		if e != nil {
			t.Fatal(e)
		}
	}
	for name, tc := range map[string]struct {
		delta rationalPolynomial
		want  error
	}{
		"coincident":     {nil, ErrDegenerate},
		"tangent":        {polynomialTangencyGap(2), ErrDegenerate},
		"two crossings":  {twoNodeGap(4), ErrConvergence},
		"four crossings": {fourNodes, ErrConvergence},
		"endpoint":       {rationalPolynomial{new(big.Rat), big.NewRat(3, 128), big.NewRat(-1, 8), big.NewRat(1, 8)}, ErrDomain},
		"irrational":     {rationalPolynomial{big.NewRat(-7, 2048), big.NewRat(15, 512), big.NewRat(-5, 64), big.NewRat(1, 16)}, ErrUnrepresentable},
	} {
		t.Run(name, func(t *testing.T) {
			a, b, _ := polynomialPairGraphFixture(t, 0, tc.delta)
			r, e := DiscoverSplineGraphThreePolynomialNodes(ctx, a, b, 0, GraphPointContactOptions{})
			if !errors.Is(e, tc.want) || !reflect.DeepEqual(r, SplineGraphThreePolynomialNodes{ExactOperations: r.ExactOperations}) {
				t.Fatal("wrong rejection or partial geometry", r, e)
			}
		})
	}
	a, b, _ = polynomialPairGraphFixture(t, 0, threeNodeGraphGap(4))
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := DiscoverSplineGraphThreePolynomialNodes(canceled, a, b, 0, GraphPointContactOptions{}); !errors.Is(e, context.Canceled) || r.Proven {
		t.Fatal("canceled arrangement was published", e)
	}
	a.net[(a.countU-1)*a.countV][2].Add(a.net[(a.countU-1)*a.countV][2], big.NewRat(1, 1024))
	if r, e := DiscoverSplineGraphThreePolynomialNodes(ctx, a, b, 0, GraphPointContactOptions{}); !errors.Is(e, ErrConvergence) || r.Proven || len(r.CertifiedCells) != 0 {
		t.Fatal("later original source span was never verified", r, e)
	}
}
