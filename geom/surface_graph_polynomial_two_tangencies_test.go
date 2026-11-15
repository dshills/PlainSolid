package geom

import (
	"context"
	"errors"
	"math/big"
	"reflect"
	"slices"
	"testing"
)

func twoTangenciesGraphFixture(t *testing.T, axis int, gap rationalPolynomial) (*SurfacePatch, *SurfacePatch, [2]rationalPolynomial) {
	t.Helper()
	lower := rationalPolynomial{big.NewRat(1, 4), big.NewRat(3, 8), big.NewRat(-3, 8), big.NewRat(1, 8)}
	upper, e := graphPolynomialSum(lower, gap, 1, &polynomialBudget{maxOperations: 1000, maxBits: 1024})
	if e != nil {
		t.Fatal(e)
	}
	negative := clonePolynomial(upper)
	for _, c := range negative {
		c.Neg(c)
	}
	a, b, _ := bivariateGraphFixture(t, 3, axis, rationalPolynomial{big.NewRat(1, 1)}, negative)
	return a, b, [2]rationalPolynomial{lower, upper}
}

func TestSplineGraphPolynomialTwoTangencies(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		roots [2][2]int64
		scale *big.Rat
		areas [5]*big.Rat
		gap   *big.Rat
	}{
		{
			roots: [2][2]int64{{1, 4}, {3, 4}}, scale: big.NewRat(1, 1), gap: big.NewRat(1, 2),
			areas: [5]*big.Rat{big.NewRat(11, 32), big.NewRat(19, 7680), big.NewRat(1, 960), big.NewRat(19, 7680), big.NewRat(2497, 3840)},
		},
		{
			roots: [2][2]int64{{1, 8}, {5, 8}}, scale: big.NewRat(7, 8), gap: big.NewRat(7, 16),
			areas: [5]*big.Rat{big.NewRat(11, 32), big.NewRat(791, 3932160), big.NewRat(7, 7680), big.NewRat(12411, 1310720), big.NewRat(317359, 491520)},
		},
	} {
		raw0, raw1 := tc.roots[0], tc.roots[1]
		delta := fourPolynomialNodeGap(t, raw0, raw0, raw1, raw1)
		multiplier := new(big.Rat).Quo(tc.scale, delta[4])
		for _, c := range delta {
			c.Mul(c, multiplier)
		}
		for axis := 0; axis < 2; axis++ {
			a, b, powers := twoTangenciesGraphFixture(t, axis, delta)
			for order, sources := range [][2]*SurfacePatch{{a, b}, {b, a}} {
				r, e := DiscoverSplineGraphPolynomialTwoTangencies(ctx, sources[0], sources[1], axis, GraphPointContactOptions{})
				if e != nil || !r.Proven || r.FreeAxis != axis || len(r.CertifiedCells) != 4 || r.SeparationScale.Cmp(tc.scale) != 0 {
					t.Fatal("complete original cubic and quartic factorization", tc.roots, axis, order, r.SeparationScale, len(r.CertifiedCells), e)
				}
				for i, power := range r.Power {
					if !slices.EqualFunc(power, powers[i], func(a, b *big.Rat) bool { return a.Cmp(b) == 0 }) {
						t.Fatal("original branch coefficients", i, power, powers[i])
					}
				}
				for i, n := range r.Nodes {
					x := big.NewRat(tc.roots[i][0], tc.roots[i][1])
					v := r.Vertices[n.Vertex]
					slope := testGraphPowerValue(polynomialDerivative(powers[0]), x)
					if n.Vertex != 4+i || v.Parameters[axis].Cmp(x) != 0 ||
						v.Parameters[1-axis].Cmp(testGraphPowerValue(powers[0], x)) != 0 ||
						r.SecondDerivativeGaps[i].Cmp(tc.gap) != 0 || !reflect.DeepEqual(n.Tangents[0], n.Tangents[1]) ||
						n.Tangents[0][axis].Cmp(big.NewRat(1, 1)) != 0 || n.Tangents[0][1-axis].Cmp(slope) != 0 {
						t.Fatal("original station, equal first derivatives, and strict second-order separation", i, v, n)
					}
				}
				checkTwoTangenciesRegions(t, r, sources, 1-2*order)
				uv, xyz, e := r.RepresentedVertices()
				if e != nil {
					t.Fatal("lossless native coordinates of exact branch vertices", e)
				}
				for i := range uv {
					for j := range uv[:i] {
						if uv[i] == uv[j] || xyz[i] == xyz[j] {
							t.Fatal("two separate tangencies must retain all six vertex identities", i, j)
						}
					}
				}
				total := new(big.Rat)
				for i, region := range r.Regions {
					// Compare Green's boundary integral of the published Bézier
					// curves with direct integration of f and g-f on each slab
					// and of 1-g above both original branches.
					area := testPolynomialStripArea(axis, r.Edges[:], region)
					if i == 4 {
						area.Add(area, big.NewRat(1, 1)) // upper perimeter
					}
					if area.Cmp(tc.areas[i]) != 0 {
						t.Fatal("independently integrated exact source area", tc.roots, axis, order, i, area, tc.areas[i])
					}
					total.Add(total, area)
				}
				if total.Cmp(big.NewRat(1, 1)) != 0 {
					t.Fatal("complete union of original regions", total)
				}
				for _, limit := range []int{r.ExactOperations - 1, r.ExactOperations} {
					q, e := DiscoverSplineGraphPolynomialTwoTangencies(ctx, sources[0], sources[1], axis, GraphPointContactOptions{MaxOperations: limit})
					if (e == nil) != (limit == r.ExactOperations) ||
						(e != nil && !reflect.DeepEqual(q, SplineGraphPolynomialTwoTangencies{ExactOperations: q.ExactOperations})) {
						t.Fatal("atomic exact work boundary", tc.roots, axis, order, e)
					}
				}
			}
		}
	}
}

func checkTwoTangenciesRegions(t *testing.T, r SplineGraphPolynomialTwoTangencies, sources [2]*SurfacePatch, sign int) {
	t.Helper()
	checkOriginalPolynomialContactRegions(t, polynomialContactRegionTest{
		r.FreeAxis, r.Graph.ProjectionOrientation, r.Power, r.Vertices[:], r.Edges[:], r.Nodes[:], r.Regions[:], []int{2, 2},
	}, sources, sign, [][4]int{{1, 4, 3, 0}, {2, 5, 4, 1}})
}

func TestSplineGraphPolynomialTwoTangenciesOriginalUnits(t *testing.T) {
	gap := twoCrossingsTangentGap(t, [2]int64{1, 4}, [2]int64{1, 4}, [2]int64{3, 4}, [2]int64{3, 4})
	a, b, _ := twoTangenciesGraphFixture(t, 1, gap)
	sources := [2]*SurfacePatch{a, b}
	for i, p := range sources {
		for j := range p.knotsU {
			p.knotsU[j] = -1 + 2*p.knotsU[j]
		}
		for j := range p.knotsV {
			p.knotsV[j] = 2 + 3*p.knotsV[j]
		}
		var e error
		sources[i], e = p.Transformed(Scaling(Vec3{X: -1, Y: 1, Z: 1}))
		if e != nil {
			t.Fatal(e)
		}
	}
	r, e := DiscoverSplineGraphPolynomialTwoTangencies(context.Background(), sources[0], sources[1], 1, GraphPointContactOptions{})
	if e != nil || !r.Proven || r.Graph.ProjectionOrientation != -1 || r.SeparationScale.Cmp(big.NewRat(2, 81)) != 0 ||
		!reflect.DeepEqual(r.SecondDerivativeGaps, [2]*big.Rat{big.NewRat(1, 9), big.NewRat(1, 9)}) {
		t.Fatal("reflected model with independent original free and dependent units", e)
	}
	for i, n := range []int64{11, 17} {
		if r.Vertices[4+i].Parameters[1].Cmp(big.NewRat(n, 4)) != 0 {
			t.Fatal("original nonunit root station", i, r.Vertices[4+i])
		}
	}
	checkTwoTangenciesRegions(t, r, sources, 1)
}

func TestSplineGraphPolynomialTwoTangenciesScope(t *testing.T) {
	ctx := context.Background()
	squaredPair := func(a, b [2]int64) rationalPolynomial {
		return twoCrossingsTangentGap(t, a, a, b, b)
	}
	atQuarter, atHalf, atThreeQuarters := [2]int64{1, 4}, [2]int64{2, 4}, [2]int64{3, 4}
	a, b, _ := twoTangenciesGraphFixture(t, 0, squaredPair([2]int64{1, 5}, atThreeQuarters))
	r, e := DiscoverSplineGraphPolynomialTwoTangencies(ctx, a, b, 0, GraphPointContactOptions{})
	if e != nil || !r.Proven || r.Vertices[4].Parameters[0].Cmp(big.NewRat(1, 5)) != 0 {
		t.Fatal("exact geometry at an interior nonbinary tangency", e)
	}
	if uv, xyz, e := r.RepresentedVertices(); !errors.Is(e, ErrUnrepresentable) || uv != ([6]Vec2{}) || xyz != ([6]Vec3{}) {
		t.Fatal("nonbinary tangency must fail lossless native coordinates", e)
	}
	quadratic := rationalPolynomial{big.NewRat(1, 8), big.NewRat(-1, 1), big.NewRat(1, 1)}
	irrational, e := polynomialProduct(quadratic, quadratic, &polynomialBudget{maxOperations: 1000, maxBits: 1024})
	if e != nil {
		t.Fatal(e)
	}
	for name, tc := range map[string]struct {
		gap  rationalPolynomial
		want error
	}{
		"coincident":            {nil, ErrDegenerate},
		"quadruple root":        {squaredPair(atHalf, atHalf), ErrDegenerate},
		"one interior double":   {squaredPair(atQuarter, [2]int64{5, 4}), ErrConvergence},
		"endpoint contact":      {squaredPair([2]int64{0, 4}, atThreeQuarters), ErrDomain},
		"crossing":              {mixedGraphGap(1, 4), ErrConvergence},
		"mixed profile":         {twoCrossingsTangentGap(t, atQuarter, atHalf, atHalf, atThreeQuarters), ErrConvergence},
		"quintic branch":        {twoCrossingsTangentGap(t, atQuarter, atQuarter, atThreeQuarters, atThreeQuarters, [2]int64{5, 4}), ErrConvergence},
		"irrational tangencies": {irrational, ErrUnrepresentable},
	} {
		t.Run(name, func(t *testing.T) {
			a, b, _ := twoTangenciesGraphFixture(t, 0, tc.gap)
			r, e := DiscoverSplineGraphPolynomialTwoTangencies(ctx, a, b, 0, GraphPointContactOptions{})
			if !errors.Is(e, tc.want) || !reflect.DeepEqual(r, SplineGraphPolynomialTwoTangencies{ExactOperations: r.ExactOperations}) {
				t.Fatal("wrong rejection or partial original arrangement", e)
			}
		})
	}
	a, b, _ = twoTangenciesGraphFixture(t, 0, squaredPair(atQuarter, atThreeQuarters))
	if r, e := DiscoverSplineGraphPolynomialTwoTangencies(ctx, a, b, 0, GraphPointContactOptions{MaxCells: 1}); !errors.Is(e, ErrConvergence) || r.Proven {
		t.Fatal("union-knot proof ignored the cell bound", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := DiscoverSplineGraphPolynomialTwoTangencies(canceled, a, b, 0, GraphPointContactOptions{}); !errors.Is(e, context.Canceled) || r.Proven {
		t.Fatal("canceled source proof published a geometry", e)
	}
	a.net[(a.countU-1)*a.countV][2].Add(a.net[(a.countU-1)*a.countV][2], big.NewRat(1, 1024))
	if r, e := DiscoverSplineGraphPolynomialTwoTangencies(ctx, a, b, 0, GraphPointContactOptions{}); !errors.Is(e, ErrConvergence) || r.Proven {
		t.Fatal("changed later original support cell", e)
	}
}
