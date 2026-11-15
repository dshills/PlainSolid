package geom

import (
	"context"
	"errors"
	"math/big"
	"reflect"
	"slices"
	"testing"
)

func threeCrossingsTangencyGraphGap(t *testing.T, stations [4]*big.Rat, tangent int) rationalPolynomial {
	t.Helper()
	factors := [5]*big.Rat{stations[0], stations[1], stations[2], stations[3], stations[tangent]}
	delta := rationalPolynomial{big.NewRat(-1, 1)}
	b := &polynomialBudget{maxOperations: 1000, maxBits: 1024}
	for _, station := range factors {
		var e error
		delta, e = polynomialProduct(delta, rationalPolynomial{new(big.Rat).Neg(station), big.NewRat(1, 1)}, b)
		if e != nil {
			t.Fatal(e)
		}
	}
	return delta
}

func threeCrossingsTangencyStations() [4]*big.Rat {
	return [4]*big.Rat{big.NewRat(1, 8), big.NewRat(3, 8), big.NewRat(5, 8), big.NewRat(7, 8)}
}

func TestSplineGraphPolynomialThreeCrossingsTangency(t *testing.T) {
	ctx := context.Background()
	stations := threeCrossingsTangencyStations()
	for tangent, tc := range []struct {
		gaps   [4]*big.Rat
		areas  [7]int64 // exact numerators over 7864320
		cycles [4][4]int
	}{
		{
			[4]*big.Rat{big.NewRat(3, 16), big.NewRat(-1, 128), big.NewRat(1, 64), big.NewRat(-9, 128)},
			[7]int64{2693881, 887, 544, 1056, 3104, 8423, 5156425},
			[4][4]int{{1, 6, 5, 0}, {7, 2, 6, 1}, {3, 8, 2, 7}, {9, 4, 8, 3}},
		},
		{
			[4]*big.Rat{big.NewRat(-3, 128), big.NewRat(-1, 16), big.NewRat(1, 128), big.NewRat(-3, 64)},
			[7]int64{2696425, 3399, 672, 352, 1888, 5911, 5155673},
			[4][4]int{{6, 1, 5, 0}, {7, 2, 1, 6}, {3, 8, 2, 7}, {9, 4, 8, 3}},
		},
		{
			[4]*big.Rat{big.NewRat(-3, 64), big.NewRat(1, 128), big.NewRat(1, 16), big.NewRat(-3, 128)},
			[7]int64{2698073, 5911, 1888, 352, 672, 3399, 5154025},
			[4][4]int{{6, 1, 5, 0}, {2, 7, 1, 6}, {3, 8, 7, 2}, {9, 4, 8, 3}},
		},
		{
			[4]*big.Rat{big.NewRat(-9, 128), big.NewRat(1, 64), big.NewRat(-1, 128), big.NewRat(-3, 16)},
			[7]int64{2698825, 8423, 3104, 1056, 544, 887, 5151481},
			[4][4]int{{6, 1, 5, 0}, {2, 7, 1, 6}, {8, 3, 7, 2}, {9, 4, 3, 8}},
		},
	} {
		orders := [4]int{1, 1, 1, 1}
		orders[tangent] = 2
		delta := threeCrossingsTangencyGraphGap(t, stations, tangent)
		for axis := 0; axis < 2; axis++ {
			a, b, powers := polynomialPairGraphFixture(t, axis, delta)
			for order, sources := range [][2]*SurfacePatch{{a, b}, {b, a}} {
				r, e := DiscoverSplineGraphPolynomialThreeCrossingsTangency(ctx, sources[0], sources[1], axis, GraphPointContactOptions{})
				if e != nil || !r.Proven || r.FreeAxis != axis || len(r.CertifiedCells) != 4 ||
					r.SeparationScale.Cmp(big.NewRat(-1, 1)) != 0 || r.ContactOrder != orders ||
					!reflect.DeepEqual(r.DerivativeGap, tc.gaps) {
					t.Fatal("complete original quintic separation and contact orders", tangent, axis, order, e)
				}
				for i, power := range r.Power {
					if !slices.EqualFunc(power, powers[i], func(x, y *big.Rat) bool { return x.Cmp(y) == 0 }) {
						t.Fatal("original cubic and quintic powers", i, power, powers[i])
					}
				}
				checkThreeCrossingsTangencyRegions(t, r, sources, 1-2*order, stations, tc.cycles)
				uv, xyz, e := r.RepresentedVertices()
				if e != nil {
					t.Fatal("the eight original vertices have lossless coordinates", e)
				}
				for i := range uv {
					for j := range uv[:i] {
						if uv[i] == uv[j] || xyz[i] == xyz[j] {
							t.Fatal("distinct original chart and model vertices", i, j)
						}
					}
				}
				total := new(big.Rat)
				for i, region := range r.Regions {
					// Compare the exact area from the published Bézier
					// boundary to direct integration of original powers.
					area := testPolynomialStripArea(axis, r.Edges[:], region)
					if i == 6 {
						area.Add(area, big.NewRat(1, 1))
					}
					if want := big.NewRat(tc.areas[i], 7864320); area.Cmp(want) != 0 {
						t.Fatal("independently integrated original region area", tangent, axis, order, i, area, want)
					}
					total.Add(total, area)
				}
				if total.Cmp(big.NewRat(1, 1)) != 0 {
					t.Fatal("complete chart area retained", total)
				}
				for _, limit := range []int{r.ExactOperations - 1, r.ExactOperations} {
					q, e := DiscoverSplineGraphPolynomialThreeCrossingsTangency(ctx, sources[0], sources[1], axis, GraphPointContactOptions{MaxOperations: limit})
					if (e == nil) != (limit == r.ExactOperations) ||
						(e != nil && !reflect.DeepEqual(q, SplineGraphPolynomialThreeCrossingsTangency{ExactOperations: q.ExactOperations})) {
						t.Fatal("atomic exact work boundary", tangent, axis, order, e)
					}
				}
			}
		}
	}
}

func checkThreeCrossingsTangencyRegions(t *testing.T, r SplineGraphPolynomialThreeCrossingsTangency, sources [2]*SurfacePatch, sign int, stations [4]*big.Rat, cycles [4][4]int) {
	t.Helper()
	for i, n := range r.Nodes {
		x := stations[i]
		v := r.Vertices[n.Vertex]
		axis := r.FreeAxis
		if n.Vertex != 4+i || v.Parameters[axis].Cmp(x) != 0 ||
			v.Parameters[1-axis].Cmp(testGraphPowerValue(r.Power[0], x)) != 0 {
			t.Fatal("exact original contact station", i, v)
		}
		var derivative [2]*big.Rat
		for j, power := range r.Power {
			p := polynomialDerivative(power)
			if n.Tangents[j][axis].Cmp(big.NewRat(1, 1)) != 0 || n.Tangents[j][1-axis].Cmp(testGraphPowerValue(p, x)) != 0 {
				t.Fatal("exact original graph first derivative", i, j, n)
			}
			if r.ContactOrder[i] == 2 {
				p = polynomialDerivative(p)
			}
			derivative[j] = testGraphPowerValue(p, x)
		}
		gap := new(big.Rat).Sub(derivative[1], derivative[0])
		sameSlope := n.Tangents[0][1-axis].Cmp(n.Tangents[1][1-axis]) == 0
		if gap.Sign() == 0 || r.DerivativeGap[i].Cmp(gap) != 0 || sameSlope != (r.ContactOrder[i] == 2) {
			t.Fatal("exact derivative gap of the stated contact order", i, derivative, r.DerivativeGap[i])
		}
	}
	checkOriginalPolynomialContactRegions(t, polynomialContactRegionTest{
		r.FreeAxis, r.Graph.ProjectionOrientation, r.Power, r.Vertices[:], r.Edges[:], r.Nodes[:], r.Regions[:], r.ContactOrder[:],
	}, sources, sign, cycles[:])
}

func TestSplineGraphPolynomialThreeCrossingsTangencyDegreeTen(t *testing.T) {
	ctx := context.Background()
	stations := threeCrossingsTangencyStations()
	delta := threeCrossingsTangencyGraphGap(t, stations, 0)
	base := rationalPolynomial{big.NewRat(1, 4), new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat), big.NewRat(1, 8)}
	upper, e := graphPolynomialSum(base, delta, 1, &polynomialBudget{maxOperations: 1000, maxBits: 1024})
	if e != nil {
		t.Fatal(e)
	}
	for _, c := range upper {
		c.Neg(c)
	}
	a, b, _ := bivariateGraphFixture(t, 5, 1, rationalPolynomial{big.NewRat(1, 1)}, upper)
	sources := [2]*SurfacePatch{a, b}
	for i, p := range sources {
		if p.degreeV != 10 {
			t.Fatal("fixture must retain the degree-ten source product", p.degreeV)
		}
		for j := range p.knotsU {
			p.knotsU[j] = -1 + 2*p.knotsU[j]
		}
		for j := range p.knotsV {
			p.knotsV[j] = 2 + 3*p.knotsV[j]
		}
		sources[i], e = p.Transformed(Scaling(Vec3{X: -1, Y: 1, Z: 1}))
		if e != nil {
			t.Fatal(e)
		}
	}
	r, e := DiscoverSplineGraphPolynomialThreeCrossingsTangency(ctx, sources[0], sources[1], 1, GraphPointContactOptions{})
	if e != nil || !r.Proven || r.Graph.ProjectionOrientation != -1 || r.SeparationScale.Cmp(big.NewRat(-2, 243)) != 0 ||
		len(r.Power[0]) != 6 || len(r.Power[1]) != 6 || !reflect.DeepEqual(r.DerivativeGap,
		[4]*big.Rat{big.NewRat(1, 24), big.NewRat(-1, 192), big.NewRat(1, 96), big.NewRat(-3, 64)}) {
		t.Fatal("two quintic source branches in nonunit reflected chart", e)
	}
	checkThreeCrossingsTangencyRegions(t, r, sources, 1,
		[4]*big.Rat{big.NewRat(19, 8), big.NewRat(25, 8), big.NewRat(31, 8), big.NewRat(37, 8)},
		[4][4]int{{1, 6, 5, 0}, {7, 2, 6, 1}, {3, 8, 2, 7}, {9, 4, 8, 3}})
}

func TestSplineGraphPolynomialThreeCrossingsTangencyScope(t *testing.T) {
	ctx := context.Background()
	stations := threeCrossingsTangencyStations()
	gap := threeCrossingsTangencyGraphGap(t, stations, 0)
	a, b, _ := polynomialPairGraphFixture(t, 0, gap)
	if q, e := DiscoverSplineGraphPolynomialTwoCrossingsTangency(ctx, a, b, 0, GraphPointContactOptions{}); !errors.Is(e, ErrConvergence) || q.Proven {
		t.Fatal("quartic mixed proof widened to admit quintic branches", e)
	}
	if q, e := DiscoverSplineGraphTwoPolynomials(ctx, a, b, 0, GraphPointContactOptions{}); !errors.Is(e, ErrConvergence) || q.Proven {
		t.Fatal("disjoint branch proof widened to admit the new profile", e)
	}
	unrepresented := stations
	unrepresented[0] = big.NewRat(1, 7)
	c, d, _ := polynomialPairGraphFixture(t, 0, threeCrossingsTangencyGraphGap(t, unrepresented, 2))
	r, e := DiscoverSplineGraphPolynomialThreeCrossingsTangency(ctx, c, d, 0, GraphPointContactOptions{})
	if e != nil || !r.Proven || r.Vertices[4].Parameters[0].Cmp(big.NewRat(1, 7)) != 0 {
		t.Fatal("exact rational nonbinary geometry", e)
	}
	if uv, xyz, e := r.RepresentedVertices(); !errors.Is(e, ErrUnrepresentable) || uv != ([8]Vec2{}) || xyz != ([8]Vec3{}) {
		t.Fatal("separate lossless native coordinate rejection", e)
	}
	polyBudget := &polynomialBudget{maxOperations: 10000, maxBits: 1024}
	irrational, e := polynomialProduct(twoCrossingsTangentGap(t, [2]int64{3, 8}, [2]int64{3, 8}, [2]int64{1, 2}),
		rationalPolynomial{big.NewRat(1, 8), big.NewRat(-1, 1), big.NewRat(1, 1)}, polyBudget)
	if e != nil {
		t.Fatal(e)
	}
	sextic, e := polynomialProduct(gap, rationalPolynomial{big.NewRat(1, 1), big.NewRat(1, 8)}, polyBudget)
	if e != nil {
		t.Fatal(e)
	}
	end := stations
	end[0] = new(big.Rat)
	coalesced := stations
	coalesced[1] = coalesced[0]
	for name, tc := range map[string]struct {
		gap  rationalPolynomial
		want error
	}{
		"coincident":      {nil, ErrDegenerate},
		"endpoint":        {threeCrossingsTangencyGraphGap(t, end, 0), ErrDomain},
		"coalesced":       {threeCrossingsTangencyGraphGap(t, coalesced, 0), ErrDegenerate},
		"irrational":      {irrational, ErrUnrepresentable},
		"sextic":          {sextic, ErrConvergence},
		"four simple":     {fourPolynomialNodeGap(t, [2]int64{1, 8}, [2]int64{3, 8}, [2]int64{5, 8}, [2]int64{7, 8}), ErrConvergence},
		"only one double": {twoCrossingsTangentGap(t, [2]int64{1, 4}, [2]int64{1, 4}), ErrConvergence},
	} {
		t.Run(name, func(t *testing.T) {
			a, b, _ := polynomialPairGraphFixture(t, 0, tc.gap)
			r, e := DiscoverSplineGraphPolynomialThreeCrossingsTangency(ctx, a, b, 0, GraphPointContactOptions{})
			if !errors.Is(e, tc.want) || !reflect.DeepEqual(r, SplineGraphPolynomialThreeCrossingsTangency{ExactOperations: r.ExactOperations}) {
				t.Fatal("wrong rejection or partial original geometry", e)
			}
		})
	}
	if q, e := DiscoverSplineGraphPolynomialThreeCrossingsTangency(ctx, a, b, 0, GraphPointContactOptions{MaxCells: 1}); !errors.Is(e, ErrConvergence) || q.Proven {
		t.Fatal("original cell quota ignored", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if q, e := DiscoverSplineGraphPolynomialThreeCrossingsTangency(canceled, a, b, 0, GraphPointContactOptions{}); !errors.Is(e, context.Canceled) || q.Proven {
		t.Fatal("canceled proof published geometry", e)
	}
	a.net[(a.countU-1)*a.countV][2].Add(a.net[(a.countU-1)*a.countV][2], big.NewRat(1, 1024))
	if q, e := DiscoverSplineGraphPolynomialThreeCrossingsTangency(ctx, a, b, 0, GraphPointContactOptions{}); !errors.Is(e, ErrConvergence) || q.Proven {
		t.Fatal("changed later original support cell accepted", e)
	}
}
