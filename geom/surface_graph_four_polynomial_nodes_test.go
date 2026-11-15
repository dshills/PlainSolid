package geom

import (
	"context"
	"errors"
	"math/big"
	"reflect"
	"slices"
	"testing"
)

func fourPolynomialNodeGap(t *testing.T, linear ...[2]int64) rationalPolynomial {
	t.Helper()
	p := rationalPolynomial{big.NewRat(1, 4096)}
	budget := &polynomialBudget{maxOperations: 1000, maxBits: 1024}
	for _, factor := range linear {
		var e error
		p, e = polynomialProduct(p, rationalPolynomial{big.NewRat(-factor[0], 1), big.NewRat(factor[1], 1)}, budget)
		if e != nil {
			t.Fatal(e)
		}
	}
	return p
}

func fourPolynomialNodeFixture(t *testing.T, axis int, gap rationalPolynomial) (*SurfacePatch, *SurfacePatch, [2]rationalPolynomial) {
	t.Helper()
	f := rationalPolynomial{big.NewRat(1, 4), new(big.Rat), new(big.Rat), new(big.Rat), big.NewRat(1, 8)}
	g, e := graphPolynomialSum(f, gap, 1, &polynomialBudget{maxOperations: 1000, maxBits: 1024})
	if e != nil {
		t.Fatal(e)
	}
	negative := clonePolynomial(g)
	for _, c := range negative {
		c.Neg(c)
	}
	a, b, _ := bivariateGraphFixture(t, 4, axis, rationalPolynomial{big.NewRat(1, 1)}, negative)
	return a, b, [2]rationalPolynomial{f, g}
}

func TestSplineGraphFourPolynomialNodes(t *testing.T) {
	ctx := context.Background()
	// The original quartic gap is (8u-1)(8u-3)(8u-5)(8u-7)/4096.
	gap := fourPolynomialNodeGap(t, [2]int64{1, 8}, [2]int64{3, 8}, [2]int64{5, 8}, [2]int64{7, 8})
	wantAreas := [7]*big.Rat{
		big.NewRat(841, 3072), big.NewRat(157, 122880), big.NewRat(19, 30720), big.NewRat(11, 30720),
		big.NewRat(19, 30720), big.NewRat(157, 122880), big.NewRat(8873, 12288),
	}
	wantSlopeGaps := [4]int64{-3, 1, -1, 3}
	for axis := 0; axis < 2; axis++ {
		a, b, powers := fourPolynomialNodeFixture(t, axis, gap)
		for order, sources := range [][2]*SurfacePatch{{a, b}, {b, a}} {
			r, e := DiscoverSplineGraphFourPolynomialNodes(ctx, sources[0], sources[1], axis, GraphPointContactOptions{})
			if e != nil || !r.Proven || r.FreeAxis != axis || len(r.CertifiedCells) != 4 || r.SeparationScale.Cmp(big.NewRat(1, 1)) != 0 {
				t.Fatal("complete original quartic factors and signed union cells", axis, order, e)
			}
			for i, source := range powers {
				if !slices.EqualFunc(r.Power[i], source, func(a, b *big.Rat) bool { return a.Cmp(b) == 0 }) {
					t.Fatal("original quartic power coefficients changed", axis, order, i, r.Power[i], source)
				}
			}
			for i, node := range r.Nodes {
				vertex := r.Vertices[node.Vertex]
				x := big.NewRat(int64(2*i+1), 8)
				slopeGap := new(big.Rat).Sub(node.Tangents[1][1-axis], node.Tangents[0][1-axis])
				if node.Vertex != 4+i || vertex.Parameters[axis].Cmp(x) != 0 ||
					vertex.Parameters[1-axis].Cmp(testGraphPowerValue(powers[0], x)) != 0 || slopeGap.Cmp(big.NewRat(wantSlopeGaps[i], 32)) != 0 {
					t.Fatal("exact original junction and transverse derivative", i, vertex, slopeGap)
				}
			}
			uv, xyz, e := r.RepresentedVertices()
			if e != nil {
				t.Fatal("lossless native coordinates", e)
			}
			for i := range uv {
				for j := range uv[:i] {
					if uv[i] == uv[j] || xyz[i] == xyz[j] {
						t.Fatal("all original vertices remain distinct", i, j)
					}
				}
			}
			checkFourPolynomialNodeRegions(t, r, sources, 1-2*order)
			// The exact integrals of the original power polynomials give
			// the expected areas above. An independent Green integral of
			// each published region uses its Bézier branch controls.
			total := new(big.Rat)
			for i, region := range r.Regions {
				area := testPolynomialStripArea(axis, r.Edges[:], region)
				if i == 6 {
					// The upper square perimeter adds the unit-wide
					// constant-dependent edge at dependent coordinate 1.
					area.Add(area, big.NewRat(1, 1))
				}
				if area.Cmp(wantAreas[i]) != 0 {
					t.Fatal("independent integrated original region area", axis, order, i, area, wantAreas[i])
				}
				total.Add(total, area)
			}
			if total.Cmp(big.NewRat(1, 1)) != 0 {
				t.Fatal("complete original chart area", total)
			}
			for _, limit := range []int{r.ExactOperations - 1, r.ExactOperations} {
				q, e := DiscoverSplineGraphFourPolynomialNodes(ctx, sources[0], sources[1], axis, GraphPointContactOptions{MaxOperations: limit})
				if (e == nil) != (limit == r.ExactOperations) || (e != nil && !reflect.DeepEqual(q, SplineGraphFourPolynomialNodes{ExactOperations: q.ExactOperations})) {
					t.Fatal("atomic exact work boundary", axis, order, e)
				}
			}
		}
	}
}

func checkFourPolynomialNodeRegions(t *testing.T, r SplineGraphFourPolynomialNodes, sources [2]*SurfacePatch, sign int) {
	t.Helper()
	ctx := context.Background()
	for i, edge := range r.Edges {
		if edge.Branch != i/5 || len(edge.Controls) != 5 {
			t.Fatal("original quartic edge order", i, edge)
		}
		for j := range edge.Controls {
			uv := testGraphBezierValue(edge.Controls, big.NewRat(int64(j), 4))
			if uv[1-r.FreeAxis].Cmp(testGraphPowerValue(r.Power[edge.Branch], uv[r.FreeAxis])) != 0 {
				t.Fatal("exact original quartic restriction", i, uv)
			}
			a, e := sources[0].EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			b, e := sources[1].EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{})
			if e != nil || !reflect.DeepEqual(a.Point, b.Point) ||
				j == 0 && !reflect.DeepEqual(a.Point, r.Vertices[edge.Start].Point) ||
				j == 4 && !reflect.DeepEqual(a.Point, r.Vertices[edge.End].Point) {
				t.Fatal("exact lift on both original roofs and bound endpoints", i, uv, e)
			}
		}
	}

	var uses [10][2]int
	for i, region := range r.Regions {
		a, e := sources[0].EvaluateRational(ctx, region.Witness[0], region.Witness[1], SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		b, e := sources[1].EvaluateRational(ctx, region.Witness[0], region.Witness[1], SurfacePatchEvaluationOptions{})
		want := -sign
		if i == 0 || i == 6 {
			want = sign
		}
		if e != nil || region.HeightSide != want || a.Point[2].Cmp(b.Point[2]) != want {
			t.Fatal("original source material sign", i, region, e)
		}
		begin, current := -1, -1
		for _, use := range region.Edges {
			edge := r.Edges[use.Edge]
			start, end, dir := edge.Start, edge.End, 0
			if use.Reversed {
				start, end, dir = end, start, 1
			}
			uses[use.Edge][dir]++
			if current < 0 {
				begin = start
			} else if current != start {
				t.Fatal("directed original boundaries must be continuous", i, region)
			}
			current = end
		}
		if i >= 2 && i <= 4 {
			if region.Perimeter != nil || begin != current || len(region.Edges) != 2 ||
				r.Edges[region.Edges[0].Edge].Branch == r.Edges[region.Edges[1].Edge].Branch {
				t.Fatal("each bounded lens retains both distinct branch sides", i, region)
			}
		} else if p := region.Perimeter; p == nil || p.Start != current || p.End != begin ||
			p.Start >= 4 || p.End >= 4 || p.ParameterDirection != r.Graph.ProjectionOrientation {
			t.Fatal("original perimeter closes in positive model order", i, region)
		}
	}
	for _, use := range uses {
		if use != [2]int{1, 1} {
			t.Fatal("each original edge has its two opposite sector uses", uses)
		}
	}
	checkPolynomialNodeRays(t, r.Nodes[:], r.Edges[:], r.Regions[:], r.Graph.ProjectionOrientation)
}

func TestSplineGraphFourPolynomialNodesOriginalUnits(t *testing.T) {
	gap := fourPolynomialNodeGap(t, [2]int64{1, 8}, [2]int64{3, 8}, [2]int64{5, 8}, [2]int64{7, 8})
	a, b, _ := fourPolynomialNodeFixture(t, 0, gap)
	sources := [2]*SurfacePatch{a, b}
	for i, p := range sources {
		for j := range p.knotsU {
			p.knotsU[j] = 2 + 3*p.knotsU[j]
		}
		for j := range p.knotsV {
			p.knotsV[j] = -1 + 2*p.knotsV[j]
		}
		var e error
		sources[i], e = p.Transformed(Scaling(Vec3{X: -1, Y: 1, Z: 1}))
		if e != nil {
			t.Fatal(e)
		}
	}
	r, e := DiscoverSplineGraphFourPolynomialNodes(context.Background(), sources[0], sources[1], 0, GraphPointContactOptions{})
	if e != nil || !r.Proven || r.Graph.ProjectionOrientation != -1 || r.SeparationScale.Cmp(big.NewRat(2, 81)) != 0 {
		t.Fatal("reflected original chart and scaled physical separation", r, e)
	}
	for i, n := range []int64{19, 25, 31, 37} {
		if r.Vertices[4+i].Parameters[0].Cmp(big.NewRat(n, 8)) != 0 {
			t.Fatal("original nonunit root station", i, r.Vertices[4+i])
		}
	}
	checkFourPolynomialNodeRegions(t, r, sources, 1)
}

func TestSplineGraphFourPolynomialNodesScope(t *testing.T) {
	ctx := context.Background()
	nonbinary := fourPolynomialNodeGap(t, [2]int64{1, 6}, [2]int64{3, 8}, [2]int64{5, 8}, [2]int64{7, 8})
	a, b, _ := fourPolynomialNodeFixture(t, 0, nonbinary)
	r, e := DiscoverSplineGraphFourPolynomialNodes(ctx, a, b, 0, GraphPointContactOptions{})
	if e != nil || !r.Proven || r.Vertices[4].Parameters[0].Cmp(big.NewRat(1, 6)) != 0 {
		t.Fatal("exact rational geometry at a nonbinary station", r, e)
	}
	if uv, xyz, e := r.RepresentedVertices(); !errors.Is(e, ErrUnrepresentable) || uv != ([8]Vec2{}) || xyz != ([8]Vec3{}) {
		t.Fatal("independent lossless native vertex gate", e)
	}
	irrational, e := polynomialProduct(
		rationalPolynomial{big.NewRat(2, 1), big.NewRat(-16, 1), big.NewRat(16, 1)},
		fourPolynomialNodeGap(t, [2]int64{3, 8}, [2]int64{5, 8}), &polynomialBudget{maxOperations: 1000, maxBits: 1024},
	)
	if e != nil {
		t.Fatal(e)
	}
	for name, tc := range map[string]struct {
		gap  rationalPolynomial
		want error
	}{
		"coincident":       {nil, ErrDegenerate},
		"two crossings":    {twoNodeGap(4), ErrConvergence},
		"three crossings":  {threeNodeGraphGap(4), ErrConvergence},
		"quintic branch":   {fourPolynomialNodeGap(t, [2]int64{1, 8}, [2]int64{2, 8}, [2]int64{3, 8}, [2]int64{5, 8}, [2]int64{7, 8}), ErrConvergence},
		"double root":      {fourPolynomialNodeGap(t, [2]int64{1, 8}, [2]int64{3, 8}, [2]int64{3, 8}, [2]int64{7, 8}), ErrDegenerate},
		"endpoint contact": {fourPolynomialNodeGap(t, [2]int64{0, 8}, [2]int64{3, 8}, [2]int64{5, 8}, [2]int64{7, 8}), ErrDomain},
		"irrational":       {irrational, ErrUnrepresentable},
	} {
		t.Run(name, func(t *testing.T) {
			a, b, _ := fourPolynomialNodeFixture(t, 0, tc.gap)
			r, e := DiscoverSplineGraphFourPolynomialNodes(ctx, a, b, 0, GraphPointContactOptions{})
			if !errors.Is(e, tc.want) || !reflect.DeepEqual(r, SplineGraphFourPolynomialNodes{ExactOperations: r.ExactOperations}) {
				t.Fatal("wrong rejection or partial geometry", r, e)
			}
		})
	}
	gap := fourPolynomialNodeGap(t, [2]int64{1, 8}, [2]int64{3, 8}, [2]int64{5, 8}, [2]int64{7, 8})
	a, b, _ = fourPolynomialNodeFixture(t, 0, gap)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := DiscoverSplineGraphFourPolynomialNodes(canceled, a, b, 0, GraphPointContactOptions{}); !errors.Is(e, context.Canceled) || r.Proven {
		t.Fatal("canceled proof published geometry", e)
	}
	if r, e := DiscoverSplineGraphFourPolynomialNodes(ctx, a, b, 0, GraphPointContactOptions{MaxCells: 1}); !errors.Is(e, ErrConvergence) || r.Proven {
		t.Fatal("original union-knot budget ignored", e)
	}
	a.net[(a.countU-1)*a.countV][2].Add(a.net[(a.countU-1)*a.countV][2], big.NewRat(1, 1024))
	if r, e := DiscoverSplineGraphFourPolynomialNodes(ctx, a, b, 0, GraphPointContactOptions{}); !errors.Is(e, ErrConvergence) ||
		!reflect.DeepEqual(r, SplineGraphFourPolynomialNodes{ExactOperations: r.ExactOperations}) {
		t.Fatal("later original support cell changed", r, e)
	}
}
