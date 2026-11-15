package geom

import (
	"context"
	"errors"
	"math/big"
	"reflect"
	"slices"
	"testing"
)

func cubicCrossingGraphGap(t *testing.T, root *big.Rat, cofactor rationalPolynomial) rationalPolynomial {
	t.Helper()
	linear := rationalPolynomial{new(big.Rat).Neg(root), big.NewRat(1, 1)}
	p := cofactor
	b := &polynomialBudget{maxOperations: 1000, maxBits: 1024}
	for range 3 {
		var e error
		p, e = polynomialProduct(p, linear, b)
		if e != nil {
			t.Fatal(e)
		}
	}
	return p
}

func TestSplineGraphPolynomialCubicCrossing(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		station    *big.Rat
		separation rationalPolynomial
		thirdGap   *big.Rat
		areas      [4]*big.Rat
	}{
		{
			station: big.NewRat(1, 2), separation: rationalPolynomial{big.NewRat(-1, 4)}, thirdGap: big.NewRat(-3, 2),
			areas: [4]*big.Rat{big.NewRat(87, 256), big.NewRat(1, 256), big.NewRat(1, 256), big.NewRat(167, 256)},
		},
		{
			station: big.NewRat(3, 8), separation: rationalPolynomial{big.NewRat(-1, 4), big.NewRat(-1, 4)}, thirdGap: big.NewRat(-33, 16),
			areas: [4]*big.Rat{big.NewRat(170849, 524288), big.NewRat(3483, 2621440), big.NewRat(9375, 524288), big.NewRat(1716837, 2621440)},
		},
	} {
		delta := cubicCrossingGraphGap(t, tc.station, tc.separation)
		for axis := 0; axis < 2; axis++ {
			a, b, powers := polynomialPairGraphFixture(t, axis, delta)
			for order, sources := range [][2]*SurfacePatch{{a, b}, {b, a}} {
				r, e := DiscoverSplineGraphPolynomialCubicCrossing(ctx, sources[0], sources[1], axis, GraphPointContactOptions{})
				sameRat := func(a, b *big.Rat) bool { return a.Cmp(b) == 0 }
				if e != nil || !r.Proven || r.FreeAxis != axis || len(r.CertifiedCells) != 4 ||
					!slices.EqualFunc(r.Separation, tc.separation, sameRat) || r.ThirdDerivativeGap.Cmp(tc.thirdGap) != 0 {
					t.Fatal("complete original cubic separation and exact third derivative", tc.station, axis, order, e)
				}
				for i, power := range r.Power {
					if !slices.EqualFunc(power, powers[i], sameRat) {
						t.Fatal("original graph power coefficients", i, power, powers[i])
					}
				}
				checkCubicCrossingRegions(t, r, sources, 1-2*order, tc.station)
				uv, xyz, e := r.RepresentedVertices()
				if e != nil {
					t.Fatal("lossless native coordinates of exact branch vertices", e)
				}
				for i := range uv {
					for j := range uv[:i] {
						if uv[i] == uv[j] || xyz[i] == xyz[j] {
							t.Fatal("distinct model coordinates for every exact vertex", i, j)
						}
					}
				}
				total := new(big.Rat)
				for i, region := range r.Regions {
					// The area from the published Bézier boundary must
					// match direct integration of the selected original
					// branch powers, including the order swap in both
					// of the exterior regions.
					area := testPolynomialStripArea(axis, r.Edges[:], region)
					if i == 3 {
						area.Add(area, big.NewRat(1, 1))
					}
					if area.Cmp(tc.areas[i]) != 0 {
						t.Fatal("independent original-region area", tc.station, axis, order, i, area, tc.areas[i])
					}
					total.Add(total, area)
				}
				if total.Cmp(big.NewRat(1, 1)) != 0 {
					t.Fatal("complete unit-chart coverage", total)
				}
				for _, limit := range []int{r.ExactOperations - 1, r.ExactOperations} {
					q, e := DiscoverSplineGraphPolynomialCubicCrossing(ctx, sources[0], sources[1], axis, GraphPointContactOptions{MaxOperations: limit})
					if (e == nil) != (limit == r.ExactOperations) ||
						(e != nil && !reflect.DeepEqual(q, SplineGraphPolynomialCubicCrossing{ExactOperations: q.ExactOperations})) {
						t.Fatal("atomic exact work boundary", tc.station, axis, order, e)
					}
				}
			}
		}
	}
}

func checkCubicCrossingRegions(t *testing.T, r SplineGraphPolynomialCubicCrossing, sources [2]*SurfacePatch, sign int, station *big.Rat) {
	t.Helper()
	v := r.Vertices[r.Node.Vertex]
	axis := r.FreeAxis
	if r.Node.Vertex != 4 || v.Parameters[axis].Cmp(station) != 0 ||
		v.Parameters[1-axis].Cmp(testGraphPowerValue(r.Power[0], station)) != 0 {
		t.Fatal("original order-three crossing vertex", v)
	}
	var third [2]*big.Rat
	for i, power := range r.Power {
		first := polynomialDerivative(power)
		second := polynomialDerivative(first)
		slope := testGraphPowerValue(first, station)
		third[i] = testGraphPowerValue(polynomialDerivative(second), station)
		if r.Node.Tangents[i][axis].Cmp(big.NewRat(1, 1)) != 0 ||
			r.Node.Tangents[i][1-axis].Cmp(slope) != 0 || r.SecondDerivative.Cmp(testGraphPowerValue(second, station)) != 0 {
			t.Fatal("both original branches have identical first and second derivatives", i, slope, r.SecondDerivative)
		}
	}
	if r.ThirdDerivativeGap.Cmp(new(big.Rat).Sub(third[1], third[0])) != 0 {
		t.Fatal("independent evaluation of the original third derivatives", third)
	}
	checkOriginalPolynomialContactRegions(t, polynomialContactRegionTest{
		r.FreeAxis, r.Graph.ProjectionOrientation, r.Power, r.Vertices[:], r.Edges[:], []SplineGraphPolynomialJunction{r.Node},
		r.Regions[:], []int{3},
	}, sources, sign, [][4]int{{3, 1, 2, 0}})
}

func TestSplineGraphPolynomialCubicCrossingOriginalUnits(t *testing.T) {
	delta := cubicCrossingGraphGap(t, big.NewRat(1, 2), rationalPolynomial{big.NewRat(-1, 4)})
	a, b, _ := polynomialPairGraphFixture(t, 1, delta)
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
	r, e := DiscoverSplineGraphPolynomialCubicCrossing(context.Background(), sources[0], sources[1], 1, GraphPointContactOptions{})
	if e != nil || !r.Proven || r.Graph.ProjectionOrientation != -1 || len(r.Separation) != 1 ||
		r.Separation[0].Cmp(big.NewRat(-1, 54)) != 0 || r.SecondDerivative.Cmp(big.NewRat(-1, 12)) != 0 ||
		r.ThirdDerivativeGap.Cmp(big.NewRat(-1, 9)) != 0 {
		t.Fatal("independent chart units and reflected model orientation", e)
	}
	checkCubicCrossingRegions(t, r, sources, 1, big.NewRat(7, 2))
}

func TestSplineGraphPolynomialCubicCrossingScope(t *testing.T) {
	ctx := context.Background()
	half := big.NewRat(1, 2)
	negative := rationalPolynomial{big.NewRat(-1, 4)}
	a, b, _ := polynomialPairGraphFixture(t, 0, cubicCrossingGraphGap(t, big.NewRat(1, 3), negative))
	r, e := DiscoverSplineGraphPolynomialCubicCrossing(ctx, a, b, 0, GraphPointContactOptions{})
	if e != nil || !r.Proven || r.Vertices[4].Parameters[0].Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatal("exact geometry at an interior nonbinary cubic crossing", e)
	}
	if uv, xyz, e := r.RepresentedVertices(); !errors.Is(e, ErrUnrepresentable) || uv != ([5]Vec2{}) || xyz != ([5]Vec3{}) {
		t.Fatal("nonbinary crossing must fail lossless native coordinates", e)
	}
	delta := cubicCrossingGraphGap(t, half, negative)
	for name, tc := range map[string]struct {
		gap  rationalPolynomial
		want error
	}{
		"coincident":          {nil, ErrDegenerate},
		"quadruple root":      {cubicCrossingGraphGap(t, half, rationalPolynomial{big.NewRat(-1, 8), big.NewRat(1, 4)}), ErrDegenerate},
		"tangent":             {twoCrossingsTangentGap(t, [2]int64{1, 2}, [2]int64{1, 2}), ErrDegenerate},
		"transverse":          {rationalPolynomial{big.NewRat(1, 8), big.NewRat(-1, 4)}, ErrConvergence},
		"endpoint":            {cubicCrossingGraphGap(t, new(big.Rat), negative), ErrDomain},
		"extra high crossing": {cubicCrossingGraphGap(t, half, rationalPolynomial{big.NewRat(-3, 16), big.NewRat(1, 4)}), ErrConvergence},
		"extra low crossing":  {cubicCrossingGraphGap(t, half, rationalPolynomial{big.NewRat(-1, 16), big.NewRat(1, 4)}), ErrConvergence},
	} {
		t.Run(name, func(t *testing.T) {
			a, b, _ := polynomialPairGraphFixture(t, 0, tc.gap)
			r, e := DiscoverSplineGraphPolynomialCubicCrossing(ctx, a, b, 0, GraphPointContactOptions{})
			if !errors.Is(e, tc.want) || !reflect.DeepEqual(r, SplineGraphPolynomialCubicCrossing{ExactOperations: r.ExactOperations}) {
				t.Fatal("wrong rejection or partial original arrangement", e)
			}
		})
	}
	a, b, _ = polynomialPairGraphFixture(t, 0, delta)
	if r, e := DiscoverSplineGraphPolynomialNode(ctx, a, b, 0, GraphPointContactOptions{}); !errors.Is(e, ErrConvergence) || r.Proven {
		t.Fatal("ordinary transverse proof widened to include a cubic crossing", e)
	}
	if r, e := DiscoverSplineGraphPolynomialTangency(ctx, a, b, 0, GraphPointContactOptions{}); !errors.Is(e, ErrDegenerate) || r.Proven {
		t.Fatal("ordinary quadratic proof widened to include a cubic crossing", e)
	}
	if r, e := DiscoverSplineGraphPolynomialCubicCrossing(ctx, a, b, 0, GraphPointContactOptions{MaxCells: 1}); !errors.Is(e, ErrConvergence) || r.Proven {
		t.Fatal("union-knot proof ignored its cell quota", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := DiscoverSplineGraphPolynomialCubicCrossing(canceled, a, b, 0, GraphPointContactOptions{}); !errors.Is(e, context.Canceled) || r.Proven {
		t.Fatal("canceled source proof published a geometry", e)
	}
	a.net[(a.countU-1)*a.countV][2].Add(a.net[(a.countU-1)*a.countV][2], big.NewRat(1, 1024))
	if r, e := DiscoverSplineGraphPolynomialCubicCrossing(ctx, a, b, 0, GraphPointContactOptions{}); !errors.Is(e, ErrConvergence) || r.Proven {
		t.Fatal("changed later original support cell", e)
	}
}
