package geom

import (
	"context"
	"errors"
	"math/big"
	"reflect"
	"testing"
)

func polynomialPairGraphFixture(t *testing.T, axis int, delta rationalPolynomial) (*SurfacePatch, *SurfacePatch, [2]rationalPolynomial) {
	t.Helper()
	// The source gap is (v-f)(v-g), g=f+delta. The rational tensor fixture
	// places a free-axis knot on A and a dependent-axis knot on B.
	f := rationalPolynomial{big.NewRat(1, 4), big.NewRat(3, 8), big.NewRat(-3, 8), big.NewRat(1, 8)}
	g, e := graphPolynomialSum(f, delta, 1, &polynomialBudget{maxOperations: 1000, maxBits: 1024})
	if e != nil {
		t.Fatal(e)
	}
	negative := clonePolynomial(g)
	for _, c := range negative {
		c.Neg(c)
	}
	a, b, _ := bivariateGraphFixture(t, 3, axis, rationalPolynomial{big.NewRat(1, 1)}, negative)
	return a, b, [2]rationalPolynomial{f, g}
}

func twoNodeGap(firstDenominator int64) rationalPolynomial {
	// ((firstDenominator*u-1)(2*u-1)(1+u^2))/64.
	return rationalPolynomial{
		big.NewRat(1, 64), big.NewRat(-firstDenominator-2, 64), big.NewRat(2*firstDenominator+1, 64),
		big.NewRat(-firstDenominator-2, 64), big.NewRat(2*firstDenominator, 64),
	}
}

func TestSplineGraphTwoPolynomialNodes(t *testing.T) {
	ctx := context.Background()
	for _, axis := range []int{0, 1} {
		a, b, want := polynomialPairGraphFixture(t, axis, twoNodeGap(4))
		for order, pair := range [][2]*SurfacePatch{{a, b}, {b, a}} {
			result, e := DiscoverSplineGraphTwoPolynomialNodes(ctx, pair[0], pair[1], axis, GraphPointContactOptions{})
			if e != nil || !result.Proven || len(result.CertifiedCells) != 4 || result.FreeAxis != axis {
				t.Fatal(axis, order, result, e)
			}
			if !reflect.DeepEqual(result.Power, [2][]*big.Rat{want[0], want[1]}) {
				t.Fatal("genuine cubic and quartic original factors", result.Power)
			}
			for i, x := range []*big.Rat{big.NewRat(1, 4), big.NewRat(1, 2)} {
				v := result.Vertices[4+i]
				if v.Parameters[axis].Cmp(x) != 0 || v.Parameters[1-axis].Cmp(testGraphPowerValue(want[0], x)) != 0 {
					t.Fatal("exact node stations", i, v)
				}
			}
			uv, xyz, e := result.RepresentedVertices()
			if e != nil || !xyz[4].IsFinite() || uv[4] == uv[5] {
				t.Fatal("exact native vertex admission", uv, xyz, e)
			}
			checkTwoPolynomialNodeGeometry(t, result, pair, 1-2*order)
			for i, wantArea := range []*big.Rat{big.NewRat(269, 163840), big.NewRat(61, 163840), big.NewRat(163, 10240)} {
				if area := testPolynomialStripArea(axis, result.Edges[:], result.Regions[i+1]); area.Cmp(wantArea) != 0 {
					t.Fatal("analytical between-branch area", i, area, wantArea)
				}
			}
			for _, limit := range []int{result.ExactOperations - 1, result.ExactOperations} {
				r, e := DiscoverSplineGraphTwoPolynomialNodes(ctx, pair[0], pair[1], axis, GraphPointContactOptions{MaxOperations: limit})
				if (e == nil) != (limit == result.ExactOperations) || (e != nil && !reflect.DeepEqual(r, SplineGraphTwoPolynomialNodes{ExactOperations: r.ExactOperations})) {
					t.Fatal("atomic quota", axis, order, e)
				}
			}
			if r, e := DiscoverSplineGraphPolynomialNode(ctx, pair[0], pair[1], axis, GraphPointContactOptions{}); e == nil || r.Proven {
				t.Fatal("single-node proof accepted two crossings", e)
			}
			if r, e := DiscoverSplineGraphTwoPolynomials(ctx, pair[0], pair[1], axis, GraphPointContactOptions{}); e == nil || r.Proven {
				t.Fatal("ordered proof accepted a crossing", e)
			}
		}
	}
}

func testPolynomialStripArea(axis int, edges []SplineGraphPolynomialPiece, region SplineGraphPolynomialRegion) *big.Rat {
	// Integrate -dependent * d(free) around the exact reported boundary. The
	// free coordinate is affine on each piece, so the mean of all its dependent
	// controls is the exact mean branch ordinate. Terminal arcs have fixed free
	// coordinate. The analytical oracles integrate ±(g-f) independently.
	area := new(big.Rat)
	for _, use := range region.Edges {
		controls := edges[use.Edge].Controls
		width := new(big.Rat).Sub(controls[len(controls)-1][axis], controls[0][axis])
		if !use.Reversed {
			width.Neg(width)
		}
		integral := new(big.Rat)
		for _, c := range controls {
			integral.Add(integral, c[1-axis])
		}
		integral.Quo(integral, big.NewRat(int64(len(controls)), 1)).Mul(integral, width)
		area.Add(area, integral)
	}
	if axis == 1 {
		area.Neg(area)
	}
	return area
}

func checkTwoPolynomialNodeGeometry(t *testing.T, r SplineGraphTwoPolynomialNodes, sources [2]*SurfacePatch, sign int) {
	t.Helper()
	ctx := context.Background()
	for i, vertex := range r.Vertices {
		for _, source := range sources {
			p, e := source.EvaluateRational(ctx, vertex.Parameters[0], vertex.Parameters[1], SurfacePatchEvaluationOptions{})
			if e != nil || !reflect.DeepEqual(p.Point, vertex.Point) {
				t.Fatal("original vertex lift", i, p.Point, vertex.Point, e)
			}
		}
	}
	for _, edge := range r.Edges {
		for _, param := range []*big.Rat{big.NewRat(0, 1), big.NewRat(1, 3), big.NewRat(1, 1)} {
			uv := testGraphBezierValue(edge.Controls, param)
			want := testGraphPowerValue(r.Power[edge.Branch], uv[r.FreeAxis])
			if uv[1-r.FreeAxis].Cmp(want) != 0 {
				t.Fatal("original branch restriction", edge, param)
			}
			a, e := sources[0].EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			b, e := sources[1].EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{})
			if e != nil || !reflect.DeepEqual(a.Point, b.Point) {
				t.Fatal("paired exact original owners", edge, param, a.Point, b.Point, e)
			}
		}
	}
	var uses [6][2]int
	for i, region := range r.Regions {
		want := [5]int{sign, -sign, -sign, -sign, sign}[i]
		u, v := region.Witness[0], region.Witness[1]
		a, e := sources[0].EvaluateRational(ctx, u, v, SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		b, e := sources[1].EvaluateRational(ctx, u, v, SurfacePatchEvaluationOptions{})
		if e != nil || region.HeightSide != want || a.Point[2].Cmp(b.Point[2]) != want {
			t.Fatal("signed original material", i, region, e)
		}
		begin, current := -1, -1
		for _, use := range region.Edges {
			edge := r.Edges[use.Edge]
			start, end, direction := edge.Start, edge.End, 0
			if use.Reversed {
				start, end, direction = end, start, 1
			}
			uses[use.Edge][direction]++
			if current == -1 {
				begin = start
			} else if current != start {
				t.Fatal("disconnected original sector boundary", region)
			}
			current = end
		}
		if i == 2 {
			if region.Perimeter != nil || current != begin || len(region.Edges) != 2 || region.Edges[0].Edge == region.Edges[1].Edge {
				t.Fatal("closed lens has two distinct curved sides", region)
			}
		} else if p := region.Perimeter; p == nil || p.Start != current || p.End != begin || p.Start > 3 || p.End > 3 || p.ParameterDirection != r.Graph.ProjectionOrientation {
			t.Fatal("original perimeter closes in positive model direction", region)
		}
	}
	for _, use := range uses {
		if use != ([2]int{1, 1}) {
			t.Fatal("contact edge has two opposite regional uses", uses)
		}
	}
	checkPolynomialNodeRays(t, r.Nodes[:], r.Edges[:], r.Regions[:], r.Graph.ProjectionOrientation)
}

func checkPolynomialNodeRays(t *testing.T, nodes []SplineGraphPolynomialJunction, edges []SplineGraphPolynomialPiece, regions []SplineGraphPolynomialRegion, orientation int) {
	t.Helper()
	for _, node := range nodes {
		var rays [4][2]*big.Rat
		for i, id := range node.RayCycle {
			edge := edges[id]
			if (edge.Start == node.Vertex) == (edge.End == node.Vertex) {
				t.Fatal("nonincident junction ray", node, edge)
			}
			ray := node.Tangents[edge.Branch]
			if edge.End == node.Vertex {
				ray = [2]*big.Rat{new(big.Rat).Neg(ray[0]), new(big.Rat).Neg(ray[1])}
			}
			rays[i] = ray
			region := regions[node.Regions[i]]
			matches := 0
			for _, use := range region.Edges {
				if use.Edge == id || use.Edge == node.RayCycle[(i+1)%4] {
					matches++
				}
			}
			if matches != 2 || region.HeightSide != -regions[node.Regions[(i+1)%4]].HeightSide {
				t.Fatal("local sector incidence and sign", node, region)
			}
		}
		for i, a := range rays {
			b := rays[(i+1)%4]
			cross := new(big.Rat).Sub(new(big.Rat).Mul(a[0], b[1]), new(big.Rat).Mul(a[1], b[0]))
			if cross.Sign()*orientation != 1 {
				t.Fatal("exact transverse model-CCW ray cycle", node, cross)
			}
		}
	}
}

func testGraphPowerValue(p rationalPolynomial, x *big.Rat) *big.Rat {
	v := new(big.Rat)
	for i := len(p) - 1; i >= 0; i-- {
		v.Mul(v, x).Add(v, p[i])
	}
	return v
}

func testGraphBezierValue(controls [][2]*big.Rat, x *big.Rat) [2]*big.Rat {
	work := append([][2]*big.Rat(nil), controls...)
	complement := new(big.Rat).Sub(big.NewRat(1, 1), x)
	for len(work) > 1 {
		for i := range work[:len(work)-1] {
			for j, p := range work[i] {
				work[i][j] = new(big.Rat).Add(new(big.Rat).Mul(complement, p), new(big.Rat).Mul(x, work[i+1][j]))
			}
		}
		work = work[:len(work)-1]
	}
	return work[0]
}

func TestSplineGraphTwoPolynomialNodesNativeAdmission(t *testing.T) {
	ctx := context.Background()
	a, b, _ := polynomialPairGraphFixture(t, 0, twoNodeGap(3))
	r, e := DiscoverSplineGraphTwoPolynomialNodes(ctx, a, b, 0, GraphPointContactOptions{})
	if e != nil || !r.Proven || r.Vertices[4].Parameters[0].Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatal("exact nonbinary rational arrangement", e)
	}
	for _, v := range r.Vertices[:4] {
		if _, e := RepresentedExactPoint(v.Point); e != nil {
			t.Fatal("fixture has a nonrepresentable terminal", e)
		}
	}
	if uv, xyz, e := r.RepresentedVertices(); !errors.Is(e, ErrUnrepresentable) || uv != ([6]Vec2{}) || xyz != ([6]Vec3{}) {
		t.Fatal("nonbinary node must fail native admission separately", e)
	}
}

func TestSplineGraphTwoPolynomialNodesOriginalUnitsAndOrientation(t *testing.T) {
	ctx := context.Background()
	a, b, _ := polynomialPairGraphFixture(t, 0, twoNodeGap(4))
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
	r, e := DiscoverSplineGraphTwoPolynomialNodes(ctx, pair[0], pair[1], 0, GraphPointContactOptions{})
	if e != nil || !r.Proven || r.Graph.ProjectionOrientation != -1 || r.Vertices[4].Parameters[0].Cmp(big.NewRat(11, 4)) != 0 ||
		r.Vertices[5].Parameters[0].Cmp(big.NewRat(7, 2)) != 0 || r.Vertices[5].Parameters[1].Cmp(big.NewRat(-9, 32)) != 0 {
		t.Fatal("original chart units and mirrored model", r, e)
	}
	checkTwoPolynomialNodeGeometry(t, r, pair, 1)
}

func TestSplineGraphTwoPolynomialNodesRejectsIncompleteArrangement(t *testing.T) {
	ctx := context.Background()
	for name, tc := range map[string]struct {
		delta rationalPolynomial
		want  error
	}{
		"coincident": {nil, ErrDegenerate},
		"tangent":    {rationalPolynomial{big.NewRat(1, 32), big.NewRat(-1, 8), big.NewRat(1, 8)}, ErrDegenerate},
		"endpoint":   {rationalPolynomial{new(big.Rat), big.NewRat(-1, 16), big.NewRat(1, 8)}, ErrDomain},
		"irrational": {rationalPolynomial{big.NewRat(7, 512), big.NewRat(-1, 16), big.NewRat(1, 16)}, ErrUnrepresentable},
		"extra":      {rationalPolynomial{big.NewRat(-3, 128), big.NewRat(11, 64), big.NewRat(-3, 8), big.NewRat(1, 4)}, ErrConvergence},
	} {
		t.Run(name, func(t *testing.T) {
			a, b, _ := polynomialPairGraphFixture(t, 0, tc.delta)
			r, e := DiscoverSplineGraphTwoPolynomialNodes(ctx, a, b, 0, GraphPointContactOptions{})
			if !errors.Is(e, tc.want) || !reflect.DeepEqual(r, SplineGraphTwoPolynomialNodes{ExactOperations: r.ExactOperations}) {
				t.Fatal("unexpected partial certificate", r, e)
			}
		})
	}
	a, b, _ := polynomialPairGraphFixture(t, 0, twoNodeGap(4))
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := DiscoverSplineGraphTwoPolynomialNodes(ctx, a, b, 0, GraphPointContactOptions{}); !errors.Is(e, context.Canceled) || r.Proven {
		t.Fatal("cancel", e)
	}
	// The last pole belongs to the later original free-axis knot span. The
	// first cell still proposes both exact factors of the unmodified pair.
	a.net[(a.countU-1)*a.countV][2].Add(a.net[(a.countU-1)*a.countV][2], big.NewRat(1, 1024))
	if r, e := DiscoverSplineGraphTwoPolynomialNodes(context.Background(), a, b, 0, GraphPointContactOptions{}); !errors.Is(e, ErrConvergence) || r.Proven || len(r.CertifiedCells) != 0 {
		t.Fatal("unverified later original cell accepted", e)
	}
}
