package geom

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/internal/slowtest"
	"reflect"
	"slices"
	"testing"
)

func simpleCubicCrossingGap(t *testing.T, simple, cubic, scale *big.Rat) rationalPolynomial {
	t.Helper()
	linear := rationalPolynomial{new(big.Rat).Neg(new(big.Rat).Mul(simple, scale)), new(big.Rat).Set(scale)}
	return cubicCrossingGraphGap(t, cubic, linear)
}

func TestSplineGraphPolynomialSimpleCubicCrossings(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	quarter, threeQuarters := big.NewRat(1, 4), big.NewRat(3, 4)
	for _, tc := range []struct {
		cubic, simple, scale *big.Rat
		orders               [2]int
		gaps                 [2]*big.Rat
		areas                [5]*big.Rat
	}{
		{
			cubic: quarter, simple: threeQuarters, scale: big.NewRat(1, 2), orders: [2]int{3, 1},
			gaps:  [2]*big.Rat{big.NewRat(-3, 2), big.NewRat(1, 16)},
			areas: [5]*big.Rat{big.NewRat(439, 1280), big.NewRat(7, 20480), big.NewRat(1, 1280), big.NewRat(97, 20480), big.NewRat(1667, 2560)},
		},
		{
			cubic: threeQuarters, simple: quarter, scale: big.NewRat(3, 4), orders: [2]int{1, 3},
			gaps:  [2]*big.Rat{big.NewRat(-3, 32), big.NewRat(9, 4)},
			areas: [5]*big.Rat{big.NewRat(877, 2560), big.NewRat(291, 40960), big.NewRat(3, 2560), big.NewRat(21, 40960), big.NewRat(3321, 5120)},
		},
	} {
		delta := simpleCubicCrossingGap(t, tc.simple, tc.cubic, tc.scale)
		for axis := 0; axis < 2; axis++ {
			a, b, powers := polynomialPairGraphFixture(t, axis, delta)
			for order, sources := range [][2]*SurfacePatch{{a, b}, {b, a}} {
				r, e := DiscoverSplineGraphPolynomialSimpleCubicCrossings(ctx, sources[0], sources[1], axis, GraphPointContactOptions{})
				if e != nil || !r.Proven || r.FreeAxis != axis || len(r.CertifiedCells) != 4 || r.ContactOrder != tc.orders ||
					r.SeparationScale.Cmp(tc.scale) != 0 {
					t.Fatal("complete original quartic separation at the two distinct orders", tc.cubic, axis, order, e)
				}
				for i, power := range r.Power {
					if !slices.EqualFunc(power, powers[i], func(a, b *big.Rat) bool { return a.Cmp(b) == 0 }) ||
						r.DerivativeGap[i].Cmp(tc.gaps[i]) != 0 {
						t.Fatal("original branch coefficients and exact crossing derivative gap", i, power, r.DerivativeGap)
					}
				}
				checkSimpleCubicCrossingRegions(t, r, sources, 1-2*order, [2]*big.Rat{quarter, threeQuarters})

				uv, xyz, e := r.RepresentedVertices()
				if e != nil {
					t.Fatal("lossless native coordinates of exact branch vertices", e)
				}
				for i := range uv {
					for j := range uv[:i] {
						if uv[i] == uv[j] || xyz[i] == xyz[j] {
							t.Fatal("independent native vertex identities", i, j)
						}
					}
				}
				total := new(big.Rat)
				for i, region := range r.Regions {
					// Compare Green's boundary integral to the
					// original power polynomials on each of the three slabs.
					area := testPolynomialStripArea(axis, r.Edges[:], region)
					if i == 4 {
						area.Add(area, big.NewRat(1, 1))
					}
					if area.Cmp(tc.areas[i]) != 0 {
						t.Fatal("independently integrated exact source area", tc.cubic, axis, order, i, area, tc.areas[i])
					}
					total.Add(total, area)
				}
				if total.Cmp(big.NewRat(1, 1)) != 0 {
					t.Fatal("complete union of original regions", total)
				}
				for _, limit := range []int{r.ExactOperations - 1, r.ExactOperations} {
					q, e := DiscoverSplineGraphPolynomialSimpleCubicCrossings(ctx, sources[0], sources[1], axis, GraphPointContactOptions{MaxOperations: limit})
					if (e == nil) != (limit == r.ExactOperations) ||
						(e != nil && !reflect.DeepEqual(q, SplineGraphPolynomialSimpleCubicCrossings{ExactOperations: q.ExactOperations})) {
						t.Fatal("atomic exact work boundary", tc.cubic, axis, order, e)
					}
				}
			}
		}
	}
}

func checkSimpleCubicCrossingRegions(t *testing.T, r SplineGraphPolynomialSimpleCubicCrossings, sources [2]*SurfacePatch, sign int, stations [2]*big.Rat) {
	t.Helper()
	axis := r.FreeAxis
	for i, node := range r.Nodes {
		x := stations[i]
		vertex := r.Vertices[node.Vertex]
		if node.Vertex != 4+i || vertex.Parameters[axis].Cmp(x) != 0 ||
			vertex.Parameters[1-axis].Cmp(testGraphPowerValue(r.Power[0], x)) != 0 {
			t.Fatal("exact original vertex at the expected free station", i, vertex)
		}
		order := r.ContactOrder[i]
		var contactDerivative [2]*big.Rat
		for branch, power := range r.Power {
			derivative := polynomialDerivative(power)
			if node.Tangents[branch][axis].Cmp(big.NewRat(1, 1)) != 0 ||
				node.Tangents[branch][1-axis].Cmp(testGraphPowerValue(derivative, x)) != 0 {
				t.Fatal("original chart slope at each crossing", i, branch, node)
			}
			if order == 3 {
				derivative = polynomialDerivative(derivative)
				if testGraphPowerValue(derivative, x).Cmp(r.CubicSecondDerivative) != 0 {
					t.Fatal("coincident original second derivatives at the cubic crossing", i, branch)
				}
				derivative = polynomialDerivative(derivative)
			}
			contactDerivative[branch] = testGraphPowerValue(derivative, x)
		}
		gap := new(big.Rat).Sub(contactDerivative[1], contactDerivative[0])
		if gap.Sign() == 0 || r.DerivativeGap[i].Cmp(gap) != 0 {
			t.Fatal("exact derivative separation of the original branches", i, contactDerivative)
		}
		if sameSlope := node.Tangents[0][1-axis].Cmp(node.Tangents[1][1-axis]) == 0; sameSlope != (order == 3) {
			t.Fatal("coincident original first derivatives only at the cubic crossing", i)
		}
	}
	checkOriginalPolynomialContactRegions(t, polynomialContactRegionTest{
		r.FreeAxis, r.Graph.ProjectionOrientation, r.Power, r.Vertices[:], r.Edges[:], r.Nodes[:], r.Regions[:], r.ContactOrder[:],
	}, sources, sign, [][4]int{{4, 1, 3, 0}, {2, 5, 1, 4}})
}

func TestSplineGraphPolynomialSimpleCubicCrossingsOriginalUnits(t *testing.T) {
	gap := simpleCubicCrossingGap(t, big.NewRat(3, 4), big.NewRat(1, 4), big.NewRat(1, 2))
	a, b, _ := polynomialPairGraphFixture(t, 1, gap)
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
	r, e := DiscoverSplineGraphPolynomialSimpleCubicCrossings(context.Background(), sources[0], sources[1], 1, GraphPointContactOptions{})
	if e != nil || !r.Proven || r.Graph.ProjectionOrientation != -1 || r.SeparationScale.Cmp(big.NewRat(1, 81)) != 0 ||
		r.CubicSecondDerivative.Cmp(big.NewRat(-1, 8)) != 0 ||
		!reflect.DeepEqual(r.DerivativeGap, [2]*big.Rat{big.NewRat(-1, 9), big.NewRat(1, 24)}) {
		t.Fatal("original nonunit derivative orders and reflected model rays", e)
	}
	checkSimpleCubicCrossingRegions(t, r, sources, 1, [2]*big.Rat{big.NewRat(11, 4), big.NewRat(17, 4)})
}

func TestSplineGraphPolynomialSimpleCubicCrossingsScope(t *testing.T) {
	ctx := context.Background()
	quarter, half, threeQuarters := big.NewRat(1, 4), big.NewRat(1, 2), big.NewRat(3, 4)
	scale := big.NewRat(1, 2)
	gap := simpleCubicCrossingGap(t, threeQuarters, quarter, scale)
	a, b, _ := polynomialPairGraphFixture(t, 0, simpleCubicCrossingGap(t, threeQuarters, big.NewRat(1, 5), scale))
	r, e := DiscoverSplineGraphPolynomialSimpleCubicCrossings(ctx, a, b, 0, GraphPointContactOptions{})
	if e != nil || !r.Proven || r.Vertices[4].Parameters[0].Cmp(big.NewRat(1, 5)) != 0 {
		t.Fatal("exact nonbinary cubic crossing station", e)
	}
	if uv, xyz, e := r.RepresentedVertices(); !errors.Is(e, ErrUnrepresentable) || uv != ([6]Vec2{}) || xyz != ([6]Vec3{}) {
		t.Fatal("nonbinary station must fail only the independent native conversion", e)
	}
	quintic, e := polynomialProduct(gap, rationalPolynomial{big.NewRat(1, 1), big.NewRat(1, 8)}, &polynomialBudget{maxOperations: 1000, maxBits: 1024})
	if e != nil {
		t.Fatal(e)
	}
	for name, tc := range map[string]struct {
		gap  rationalPolynomial
		want error
	}{
		"quintic branch":       {quintic, ErrConvergence},
		"coincident":           {nil, ErrDegenerate},
		"coalesced crossings":  {simpleCubicCrossingGap(t, half, half, scale), ErrDegenerate},
		"simple root at end":   {simpleCubicCrossingGap(t, big.NewRat(1, 1), quarter, scale), ErrDomain},
		"cubic root at start":  {simpleCubicCrossingGap(t, threeQuarters, new(big.Rat), scale), ErrDomain},
		"only one cubic":       {cubicCrossingGraphGap(t, half, rationalPolynomial{big.NewRat(-1, 4)}), ErrConvergence},
		"two simple roots":     {twoNodeGap(4), ErrConvergence},
		"simple and quadratic": {mixedGraphGap(1, 4), ErrDegenerate},
		"two quadratic roots":  {twoCrossingsTangentGap(t, [2]int64{1, 4}, [2]int64{1, 4}, [2]int64{3, 4}, [2]int64{3, 4}), ErrDegenerate},
	} {
		t.Run(name, func(t *testing.T) {
			a, b, _ := polynomialPairGraphFixture(t, 0, tc.gap)
			r, e := DiscoverSplineGraphPolynomialSimpleCubicCrossings(ctx, a, b, 0, GraphPointContactOptions{})
			if !errors.Is(e, tc.want) || !reflect.DeepEqual(r, SplineGraphPolynomialSimpleCubicCrossings{ExactOperations: r.ExactOperations}) {
				t.Fatal("wrong rejection or partial original arrangement", e)
			}
		})
	}
	a, b, _ = polynomialPairGraphFixture(t, 0, gap)
	if q, e := DiscoverSplineGraphPolynomialCubicCrossing(ctx, a, b, 0, GraphPointContactOptions{}); !errors.Is(e, ErrConvergence) || q.Proven {
		t.Fatal("single cubic proof admitted an additional contact", e)
	}
	if q, e := DiscoverSplineGraphTwoPolynomialNodes(ctx, a, b, 0, GraphPointContactOptions{}); !errors.Is(e, ErrDegenerate) || q.Proven {
		t.Fatal("two-transverse-node proof admitted a cubic crossing", e)
	}
	if q, e := DiscoverSplineGraphPolynomialMixedContact(ctx, a, b, 0, GraphPointContactOptions{}); !errors.Is(e, ErrDegenerate) || q.Proven {
		t.Fatal("mixed quadratic proof admitted a cubic crossing", e)
	}
	if q, e := DiscoverSplineGraphPolynomialSimpleCubicCrossings(ctx, a, b, 0, GraphPointContactOptions{MaxCells: 1}); !errors.Is(e, ErrConvergence) || q.Proven {
		t.Fatal("source knot cells ignored the work cap", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if q, e := DiscoverSplineGraphPolynomialSimpleCubicCrossings(canceled, a, b, 0, GraphPointContactOptions{}); !errors.Is(e, context.Canceled) || q.Proven {
		t.Fatal("canceled source proof published a geometry", e)
	}
	a.net[(a.countU-1)*a.countV][2].Add(a.net[(a.countU-1)*a.countV][2], big.NewRat(1, 1024))
	if q, e := DiscoverSplineGraphPolynomialSimpleCubicCrossings(ctx, a, b, 0, GraphPointContactOptions{}); !errors.Is(e, ErrConvergence) || q.Proven {
		t.Fatal("changed later original support cell", e)
	}
}
