package geom

import (
	"context"
	"errors"
	"math/big"
	"reflect"
	"slices"
	"testing"
)

func mixedGraphGap(numerator, denominator int64) rationalPolynomial {
	// (numerator-denominator*u)(2u-1)^2(1+u)/128.
	return rationalPolynomial{
		big.NewRat(numerator, 128), big.NewRat(-3*numerator-denominator, 128),
		big.NewRat(3*denominator, 128), big.NewRat(4*numerator, 128), big.NewRat(-4*denominator, 128),
	}
}

func TestSplineGraphPolynomialMixedContact(t *testing.T) {
	ctx := context.Background()
	for _, fixture := range []struct {
		rootNumerator int64
		roots         [2]*big.Rat
		orders        [2]int
		gaps          [2]*big.Rat
		areas         [3]int64 // numerators over 163840
		cycles        [2][4]int
	}{
		{1, [2]*big.Rat{big.NewRat(1, 4), big.NewRat(1, 2)}, [2]int{1, 2},
			[2]*big.Rat{big.NewRat(-5, 512), big.NewRat(-3, 32)}, [3]int64{121, 9, 1008}, [2][4]int{{4, 1, 3, 0}, {5, 2, 1, 4}}},
		{3, [2]*big.Rat{big.NewRat(1, 2), big.NewRat(3, 4)}, [2]int{2, 1},
			[2]*big.Rat{big.NewRat(3, 32), big.NewRat(-7, 512)}, [3]int64{592, 11, 219}, [2][4]int{{1, 4, 3, 0}, {5, 2, 4, 1}}},
	} {
		for axis := 0; axis < 2; axis++ {
			a, b, powers := polynomialPairGraphFixture(t, axis, mixedGraphGap(fixture.rootNumerator, 4))
			for order, sources := range [][2]*SurfacePatch{{a, b}, {b, a}} {
				r, e := DiscoverSplineGraphPolynomialMixedContact(ctx, sources[0], sources[1], axis, GraphPointContactOptions{})
				if e != nil || !r.Proven || r.FreeAxis != axis || len(r.CertifiedCells) != 4 ||
					!reflect.DeepEqual(r.Power, [2][]*big.Rat{powers[0], powers[1]}) {
					t.Fatal("complete original mixed-degree arrangement", fixture.rootNumerator, axis, order, e)
				}
				if r.ContactOrder != fixture.orders || !reflect.DeepEqual(r.DerivativeGap, fixture.gaps) ||
					!reflect.DeepEqual(r.Cofactor, []*big.Rat{big.NewRat(-1, 8), big.NewRat(-1, 8)}) {
					t.Fatal("exact simple factor, squared factor, and local derivative orders", r.ContactOrder, r.DerivativeGap, r.Cofactor)
				}
				for i, node := range r.Nodes {
					v := r.Vertices[node.Vertex]
					x := fixture.roots[i]
					if node.Vertex != 4+i || v.Parameters[axis].Cmp(x) != 0 || v.Parameters[1-axis].Cmp(testGraphPowerValue(powers[0], x)) != 0 {
						t.Fatal("exact original contact station", i, v)
					}
					for j, tangent := range node.Tangents {
						slope := testGraphPowerValue(polynomialDerivative(powers[j]), x)
						if tangent[axis].Cmp(big.NewRat(1, 1)) != 0 || tangent[1-axis].Cmp(slope) != 0 {
							t.Fatal("original branch derivative at each contact", i, j, tangent)
						}
					}
				}
				if uv, p, e := r.RepresentedVertices(); e != nil || uv[4] == uv[5] || p[4] == p[5] {
					t.Fatal("lossless native vertex conversion", uv, p, e)
				}
				checkPolynomialMixedContactRegions(t, r, sources, 1-2*order, fixture.cycles)
				for i, area := range fixture.areas {
					if got := testPolynomialStripArea(axis, r.Edges[:], r.Regions[i+1]); got.Cmp(big.NewRat(area, 163840)) != 0 {
						t.Fatal("independent integrated original between-branch area", i, got, area)
					}
				}
				for _, limit := range []int{r.ExactOperations - 1, r.ExactOperations} {
					q, e := DiscoverSplineGraphPolynomialMixedContact(ctx, sources[0], sources[1], axis, GraphPointContactOptions{MaxOperations: limit})
					if (e == nil) != (limit == r.ExactOperations) || (e != nil && !reflect.DeepEqual(q, SplineGraphPolynomialMixedContact{ExactOperations: q.ExactOperations})) {
						t.Fatal("atomic original proof budget", axis, order, e)
					}
				}
			}
		}
	}
}

func checkPolynomialMixedContactRegions(t *testing.T, r SplineGraphPolynomialMixedContact, sources [2]*SurfacePatch, sign int, cycles [2][4]int) {
	t.Helper()
	checkOriginalPolynomialContactRegions(t, polynomialContactRegionTest{
		r.FreeAxis, r.Graph.ProjectionOrientation, r.Power, r.Vertices[:], r.Edges[:], r.Nodes[:], r.Regions[:], r.ContactOrder[:],
	}, sources, sign, cycles[:])
}

type polynomialContactRegionTest struct {
	FreeAxis, ProjectionOrientation int
	Power                           [2][]*big.Rat
	Vertices                        []SplineGraphPolynomialVertex
	Edges                           []SplineGraphPolynomialPiece
	Nodes                           []SplineGraphPolynomialJunction
	Regions                         []SplineGraphPolynomialRegion
	ContactOrder                    []int
}

func checkOriginalPolynomialContactRegions(t *testing.T, r polynomialContactRegionTest, sources [2]*SurfacePatch, sign int, cycles [][4]int) {
	t.Helper()
	ctx := context.Background()
	for _, edge := range r.Edges {
		power := r.Power[edge.Branch]
		if len(edge.Controls) != len(power) {
			t.Fatal("original polynomial degree changed", edge)
		}
		for i := range power {
			uv := testGraphBezierValue(edge.Controls, big.NewRat(int64(i), int64(len(power)-1)))
			if uv[1-r.FreeAxis].Cmp(testGraphPowerValue(power, uv[r.FreeAxis])) != 0 {
				t.Fatal("wrong restriction of original graph branch", edge, uv)
			}
			a, e := sources[0].EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			b, e := sources[1].EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{})
			if e != nil || !reflect.DeepEqual(a.Point, b.Point) {
				t.Fatal("original supports must give the same model lift", edge, uv, e)
			}
			if i == 0 && !reflect.DeepEqual(a.Point, r.Vertices[edge.Start].Point) ||
				i == len(power)-1 && !reflect.DeepEqual(a.Point, r.Vertices[edge.End].Point) {
				t.Fatal("exact original contact and terminal vertices", edge, uv)
			}
		}
	}

	uses := make([][2]int, len(r.Edges))
	for i, region := range r.Regions {
		u, v := region.Witness[0], region.Witness[1]
		a, e := sources[0].EvaluateRational(ctx, u, v, SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		b, e := sources[1].EvaluateRational(ctx, u, v, SurfacePatchEvaluationOptions{})
		want := -sign
		if i == 0 || i == len(r.Regions)-1 {
			want = sign
		}
		if e != nil || region.HeightSide != want || a.Point[2].Cmp(b.Point[2]) != want {
			t.Fatal("signed original roof order", i, region, e)
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
				t.Fatal("discontinuous original sector boundary", i, region)
			}
			current = end
		}
		if i >= 2 && i < len(r.Regions)-2 {
			if region.Perimeter != nil || current != begin || len(region.Edges) != 2 || region.Edges[0].Edge == region.Edges[1].Edge {
				t.Fatal("interior lens must contain two original curved sides", region)
			}
		} else if p := region.Perimeter; p == nil || p.Start != current || p.End != begin || p.Start >= 4 || p.End >= 4 || p.ParameterDirection != r.ProjectionOrientation {
			t.Fatal("original perimeter closes in positive model order", i, region)
		}
	}
	for _, use := range uses {
		if use != [2]int{1, 1} {
			t.Fatal("both opposite sector uses of each original edge", uses)
		}
	}

	for i, node := range r.Nodes {
		wantRegions := [4]int{i + 2, len(r.Regions) - 1, i + 1, 0}
		expected := cycles[i]
		if r.ProjectionOrientation*(1-2*r.FreeAxis) < 0 {
			slices.Reverse(expected[1:])
			slices.Reverse(wantRegions[:])
		}
		if node.RayCycle != expected || node.Regions != wantRegions {
			t.Fatal("ordered model rays and retained sectors at both contact types", i, node)
		}
		for j, id := range node.RayCycle {
			edge := r.Edges[id]
			next := node.RayCycle[(j+1)%4]
			if (edge.Start == node.Vertex) == (edge.End == node.Vertex) {
				t.Fatal("contact ray must start at the original junction", i, j, edge)
			}
			matched := 0
			for _, use := range r.Regions[node.Regions[j]].Edges {
				if use.Edge == id || use.Edge == next {
					matched++
				}
			}
			if matched != 2 || r.Regions[node.Regions[j]].HeightSide == r.Regions[node.Regions[(j+1)%4]].HeightSide {
				t.Fatal("local ray cycle and signed sector incidence", i, j)
			}
		}
		if r.ContactOrder[i] == 1 {
			checkPolynomialNodeRays(t, []SplineGraphPolynomialJunction{node}, r.Edges, r.Regions, r.ProjectionOrientation)
		} else {
			checkQuadraticTangentOneSidedRays(t, node, r)
		}
	}
}

func checkQuadraticTangentOneSidedRays(t *testing.T, node SplineGraphPolynomialJunction, r polynomialContactRegionTest) {
	t.Helper()
	cycle := node.RayCycle
	if r.ProjectionOrientation*(1-2*r.FreeAxis) < 0 {
		slices.Reverse(cycle[1:])
	}
	check := func(lower, upper int, parameter *big.Rat) {
		lo := testGraphBezierValue(r.Edges[lower].Controls, parameter)
		hi := testGraphBezierValue(r.Edges[upper].Controls, parameter)
		if lo[r.FreeAxis].Cmp(hi[r.FreeAxis]) != 0 || lo[1-r.FreeAxis].Cmp(hi[1-r.FreeAxis]) >= 0 {
			t.Fatal("one-sided tangent rays must retain original branch order", node, lo, hi)
		}
	}
	check(cycle[0], cycle[1], big.NewRat(1, 4))
	check(cycle[3], cycle[2], big.NewRat(3, 4))
}

func TestSplineGraphPolynomialMixedContactOriginalUnits(t *testing.T) {
	a, b, _ := polynomialPairGraphFixture(t, 0, mixedGraphGap(1, 4))
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
	r, e := DiscoverSplineGraphPolynomialMixedContact(context.Background(), sources[0], sources[1], 0, GraphPointContactOptions{})
	if e != nil || !r.Proven || r.Graph.ProjectionOrientation != -1 ||
		r.Vertices[4].Parameters[0].Cmp(big.NewRat(11, 4)) != 0 || r.Vertices[5].Parameters[0].Cmp(big.NewRat(7, 2)) != 0 ||
		!reflect.DeepEqual(r.DerivativeGap, [2]*big.Rat{big.NewRat(-5, 768), big.NewRat(-1, 48)}) ||
		!reflect.DeepEqual(r.Cofactor, []*big.Rat{big.NewRat(-1, 324), big.NewRat(-1, 324)}) {
		t.Fatal("original reflected nonunit chart stations, derivatives and cofactor", r, e)
	}
	checkPolynomialMixedContactRegions(t, r, sources, 1, [2][4]int{{4, 1, 3, 0}, {5, 2, 1, 4}})
}

func TestSplineGraphPolynomialMixedContactScope(t *testing.T) {
	ctx := context.Background()
	a, b, _ := polynomialPairGraphFixture(t, 0, mixedGraphGap(1, 3))
	r, e := DiscoverSplineGraphPolynomialMixedContact(ctx, a, b, 0, GraphPointContactOptions{})
	if e != nil || !r.Proven || r.Vertices[4].Parameters[0].Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatal("rational geometry at a nonbinary station", r, e)
	}
	if uv, xyz, e := r.RepresentedVertices(); !errors.Is(e, ErrUnrepresentable) || uv != ([6]Vec2{}) || xyz != ([6]Vec3{}) {
		t.Fatal("independent native conversion gate", e)
	}
	for name, tc := range map[string]struct {
		gap  rationalPolynomial
		want error
	}{
		"equal branches": {nil, ErrDegenerate},
		"two simple":     {twoNodeGap(4), ErrConvergence},
		"only tangency":  {polynomialTangencyGap(2), ErrConvergence},
		"triple":         {rationalPolynomial{big.NewRat(-1, 64), big.NewRat(3, 32), big.NewRat(-3, 16), big.NewRat(1, 8)}, ErrDegenerate},
		"extra simple":   {rationalPolynomial{big.NewRat(3, 128), big.NewRat(-28, 128), big.NewRat(92, 128), big.NewRat(-128, 128), big.NewRat(64, 128)}, ErrConvergence},
		"endpoint":       {mixedGraphGap(0, 4), ErrDomain},
		"irrational":     {rationalPolynomial{big.NewRat(-1, 128), big.NewRat(4, 128), big.NewRat(-2, 128), big.NewRat(-8, 128), big.NewRat(8, 128)}, ErrUnrepresentable},
	} {
		t.Run(name, func(t *testing.T) {
			a, b, _ := polynomialPairGraphFixture(t, 0, tc.gap)
			r, e := DiscoverSplineGraphPolynomialMixedContact(ctx, a, b, 0, GraphPointContactOptions{})
			if !errors.Is(e, tc.want) || !reflect.DeepEqual(r, SplineGraphPolynomialMixedContact{ExactOperations: r.ExactOperations}) {
				t.Fatal("missing atomic contact classification", r, e)
			}
		})
	}
	a, b, _ = polynomialPairGraphFixture(t, 0, mixedGraphGap(3, 4))
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := DiscoverSplineGraphPolynomialMixedContact(canceled, a, b, 0, GraphPointContactOptions{}); !errors.Is(e, context.Canceled) || r.Proven {
		t.Fatal("canceled contact proof", r, e)
	}
	for _, gap := range []rationalPolynomial{nil, mixedGraphGap(3, 4)} {
		a, b, _ = polynomialPairGraphFixture(t, 0, gap)
		a.net[(a.countU-1)*a.countV][2].Add(a.net[(a.countU-1)*a.countV][2], big.NewRat(1, 1024))
		r, e := DiscoverSplineGraphPolynomialMixedContact(ctx, a, b, 0, GraphPointContactOptions{})
		if !errors.Is(e, ErrConvergence) || !reflect.DeepEqual(r, SplineGraphPolynomialMixedContact{ExactOperations: r.ExactOperations}) {
			t.Fatal("later original owner cell changed", r, e)
		}
	}
}
