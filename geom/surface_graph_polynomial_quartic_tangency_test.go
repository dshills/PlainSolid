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

func quarticTangencyGraphGap(t *testing.T, station, scale *big.Rat) rationalPolynomial {
	t.Helper()
	constant := new(big.Rat).Neg(new(big.Rat).Mul(scale, station))
	return cubicCrossingGraphGap(t, station, rationalPolynomial{constant, new(big.Rat).Set(scale)})
}

func TestSplineGraphPolynomialQuarticTangency(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for _, tc := range []struct {
		station, scale *big.Rat
		gap            *big.Rat
		areas          [4]*big.Rat
	}{
		{
			station: big.NewRat(1, 2), scale: big.NewRat(1, 2), gap: big.NewRat(12, 1),
			areas: [4]*big.Rat{big.NewRat(11, 32), big.NewRat(1, 320), big.NewRat(1, 320), big.NewRat(13, 20)},
		},
		{
			station: big.NewRat(3, 8), scale: big.NewRat(3, 4), gap: big.NewRat(18, 1),
			areas: [4]*big.Rat{big.NewRat(11, 32), big.NewRat(729, 655360), big.NewRat(1875, 131072), big.NewRat(52497, 81920)},
		},
	} {
		delta := quarticTangencyGraphGap(t, tc.station, tc.scale)
		for axis := 0; axis < 2; axis++ {
			a, b, powers := polynomialPairGraphFixture(t, axis, delta)
			for order, sources := range [][2]*SurfacePatch{{a, b}, {b, a}} {
				r, e := DiscoverSplineGraphPolynomialQuarticTangency(ctx, sources[0], sources[1], axis, GraphPointContactOptions{})
				if e != nil || !r.Proven || r.FreeAxis != axis || len(r.CertifiedCells) != 4 ||
					r.SeparationScale.Cmp(tc.scale) != 0 || r.FourthDerivativeGap.Cmp(tc.gap) != 0 {
					t.Fatal("complete original quartic separation on every source cell", tc.station, axis, order, e)
				}
				for i, power := range r.Power {
					if !slices.EqualFunc(power, powers[i], func(a, b *big.Rat) bool { return a.Cmp(b) == 0 }) {
						t.Fatal("original cubic and quartic powers", i, power, powers[i])
					}
				}
				checkQuarticTangencyRegions(t, r, sources, 1-2*order, tc.station)
				uv, xyz, e := r.RepresentedVertices()
				if e != nil {
					t.Fatal("lossless native coordinates of exact branch vertices", e)
				}
				for i := range uv {
					for j := range uv[:i] {
						if uv[i] == uv[j] || xyz[i] == xyz[j] {
							t.Fatal("distinct native identities for the five original vertices", i, j)
						}
					}
				}
				total := new(big.Rat)
				for i, region := range r.Regions {
					// Compare Green's boundary integral of the published
					// Bézier branches to the original powers on each side.
					area := testPolynomialStripArea(axis, r.Edges[:], region)
					if i == 3 {
						area.Add(area, big.NewRat(1, 1))
					}
					if area.Cmp(tc.areas[i]) != 0 {
						t.Fatal("independently integrated exact region area", tc.station, axis, order, i, area, tc.areas[i])
					}
					total.Add(total, area)
				}
				if total.Cmp(big.NewRat(1, 1)) != 0 {
					t.Fatal("complete four-region chart coverage", total)
				}
				for _, limit := range []int{r.ExactOperations - 1, r.ExactOperations} {
					q, e := DiscoverSplineGraphPolynomialQuarticTangency(ctx, sources[0], sources[1], axis, GraphPointContactOptions{MaxOperations: limit})
					if (e == nil) != (limit == r.ExactOperations) ||
						(e != nil && !reflect.DeepEqual(q, SplineGraphPolynomialQuarticTangency{ExactOperations: q.ExactOperations})) {
						t.Fatal("atomic exact work boundary", tc.station, axis, order, e)
					}
				}
			}
		}
	}
}

func checkQuarticTangencyRegions(t *testing.T, r SplineGraphPolynomialQuarticTangency, sources [2]*SurfacePatch, sign int, station *big.Rat) {
	t.Helper()
	v := r.Vertices[r.Node.Vertex]
	axis := r.FreeAxis
	if r.Node.Vertex != 4 || v.Parameters[axis].Cmp(station) != 0 ||
		v.Parameters[1-axis].Cmp(testGraphPowerValue(r.Power[0], station)) != 0 {
		t.Fatal("original order-four contact vertex", v)
	}
	var fourth [2]*big.Rat
	for i, power := range r.Power {
		first := polynomialDerivative(power)
		second := polynomialDerivative(first)
		third := polynomialDerivative(second)
		if r.Node.Tangents[i][axis].Cmp(big.NewRat(1, 1)) != 0 ||
			r.Node.Tangents[i][1-axis].Cmp(testGraphPowerValue(first, station)) != 0 ||
			r.SecondDerivative.Cmp(testGraphPowerValue(second, station)) != 0 ||
			r.ThirdDerivative.Cmp(testGraphPowerValue(third, station)) != 0 {
			t.Fatal("both original branches retain the same first three derivatives", i, r.Node)
		}
		fourth[i] = testGraphPowerValue(polynomialDerivative(third), station)
	}
	delta := new(big.Rat).Sub(fourth[1], fourth[0])
	if delta.Sign() <= 0 || r.FourthDerivativeGap.Cmp(delta) != 0 {
		t.Fatal("independently evaluated fourth-order separation", fourth)
	}
	checkOriginalPolynomialContactRegions(t, polynomialContactRegionTest{
		r.FreeAxis, r.Graph.ProjectionOrientation, r.Power, r.Vertices[:], r.Edges[:], []SplineGraphPolynomialJunction{r.Node}, r.Regions[:], []int{4},
	}, sources, sign, [][4]int{{1, 3, 2, 0}})
}

func TestSplineGraphPolynomialQuarticTangencyOriginalUnits(t *testing.T) {
	gap := quarticTangencyGraphGap(t, big.NewRat(1, 2), big.NewRat(1, 2))
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
	r, e := DiscoverSplineGraphPolynomialQuarticTangency(context.Background(), sources[0], sources[1], 1, GraphPointContactOptions{})
	if e != nil || !r.Proven || r.Graph.ProjectionOrientation != -1 || r.SeparationScale.Cmp(big.NewRat(1, 81)) != 0 ||
		r.SecondDerivative.Cmp(big.NewRat(-1, 12)) != 0 || r.ThirdDerivative.Cmp(big.NewRat(1, 18)) != 0 ||
		r.FourthDerivativeGap.Cmp(big.NewRat(8, 27)) != 0 {
		t.Fatal("original chart units and reflected model orientation", e)
	}
	checkQuarticTangencyRegions(t, r, sources, 1, big.NewRat(7, 2))
}

func TestSplineGraphPolynomialQuarticTangencyScope(t *testing.T) {
	ctx := context.Background()
	quarter, half, threeQuarters := big.NewRat(1, 4), big.NewRat(1, 2), big.NewRat(3, 4)
	scale := big.NewRat(1, 2)
	gap := quarticTangencyGraphGap(t, half, scale)
	a, b, _ := polynomialPairGraphFixture(t, 0, quarticTangencyGraphGap(t, big.NewRat(1, 5), scale))
	r, e := DiscoverSplineGraphPolynomialQuarticTangency(ctx, a, b, 0, GraphPointContactOptions{})
	if e != nil || !r.Proven || r.Vertices[4].Parameters[0].Cmp(big.NewRat(1, 5)) != 0 {
		t.Fatal("exact geometry at an interior nonbinary order-four point", e)
	}
	if uv, xyz, e := r.RepresentedVertices(); !errors.Is(e, ErrUnrepresentable) || uv != ([5]Vec2{}) || xyz != ([5]Vec3{}) {
		t.Fatal("nonbinary contact must fail lossless native conversion", e)
	}
	quintic, e := polynomialProduct(gap, rationalPolynomial{big.NewRat(1, 1), big.NewRat(1, 8)}, &polynomialBudget{maxOperations: 1000, maxBits: 1024})
	if e != nil {
		t.Fatal(e)
	}
	for name, tc := range map[string]struct {
		gap  rationalPolynomial
		want error
	}{
		"coincident":          {nil, ErrDegenerate},
		"quintic branch":      {quintic, ErrConvergence},
		"endpoint contact":    {quarticTangencyGraphGap(t, new(big.Rat), scale), ErrDomain},
		"cubic crossing":      {cubicCrossingGraphGap(t, half, rationalPolynomial{big.NewRat(-1, 4)}), ErrDegenerate},
		"quadratic tangency":  {twoCrossingsTangentGap(t, [2]int64{1, 2}, [2]int64{1, 2}), ErrDegenerate},
		"two quadratic roots": {twoCrossingsTangentGap(t, [2]int64{1, 4}, [2]int64{1, 4}, [2]int64{3, 4}, [2]int64{3, 4}), ErrDegenerate},
		"cubic then simple":   {simpleCubicCrossingGap(t, threeQuarters, quarter, scale), ErrDegenerate},
		"simple then cubic":   {simpleCubicCrossingGap(t, quarter, threeQuarters, scale), ErrConvergence},
	} {
		t.Run(name, func(t *testing.T) {
			a, b, _ := polynomialPairGraphFixture(t, 0, tc.gap)
			r, e := DiscoverSplineGraphPolynomialQuarticTangency(ctx, a, b, 0, GraphPointContactOptions{})
			if !errors.Is(e, tc.want) || !reflect.DeepEqual(r, SplineGraphPolynomialQuarticTangency{ExactOperations: r.ExactOperations}) {
				t.Fatal("wrong rejection or partial original arrangement", e)
			}
		})
	}
	a, b, _ = polynomialPairGraphFixture(t, 0, gap)
	if q, e := DiscoverSplineGraphPolynomialTangency(ctx, a, b, 0, GraphPointContactOptions{}); !errors.Is(e, ErrDegenerate) || q.Proven {
		t.Fatal("ordinary quadratic proof admitted an order-four contact", e)
	}
	if q, e := DiscoverSplineGraphPolynomialCubicCrossing(ctx, a, b, 0, GraphPointContactOptions{}); !errors.Is(e, ErrDegenerate) || q.Proven {
		t.Fatal("strict cubic proof admitted an order-four contact", e)
	}
	if q, e := DiscoverSplineGraphPolynomialQuarticTangency(ctx, a, b, 0, GraphPointContactOptions{MaxCells: 1}); !errors.Is(e, ErrConvergence) || q.Proven {
		t.Fatal("source union ignored its cell quota", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if q, e := DiscoverSplineGraphPolynomialQuarticTangency(canceled, a, b, 0, GraphPointContactOptions{}); !errors.Is(e, context.Canceled) || q.Proven {
		t.Fatal("canceled proof published geometry", e)
	}
	a.net[(a.countU-1)*a.countV][2].Add(a.net[(a.countU-1)*a.countV][2], big.NewRat(1, 1024))
	if q, e := DiscoverSplineGraphPolynomialQuarticTangency(ctx, a, b, 0, GraphPointContactOptions{}); !errors.Is(e, ErrConvergence) || q.Proven {
		t.Fatal("changed later original support cell", e)
	}
}
