package geom

import (
	"context"
	"errors"
	"math/big"
	"reflect"
	"slices"
	"testing"
)

func arrangementGap(t *testing.T, stations []*big.Rat, orders []int, cofactor rationalPolynomial) rationalPolynomial {
	t.Helper()
	delta := cofactor
	budget := &polynomialBudget{maxOperations: 1000, maxBits: 1024}
	for i, station := range stations {
		factor := rationalPolynomial{new(big.Rat).Neg(station), big.NewRat(1, 1)}
		for range orders[i] {
			var e error
			delta, e = polynomialProduct(delta, factor, budget)
			if e != nil {
				t.Fatal(e)
			}
		}
	}
	return delta
}

func TestSplineGraphPolynomialArrangement(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name        string
		stations    []*big.Rat
		orders      []int
		cofactor    rationalPolynomial
		lower       []int
		cycles      [][4]int
		areas       []int64 // numerators over denominator
		denominator int64
	}{
		{
			name: "five crossings", stations: []*big.Rat{big.NewRat(1, 8), big.NewRat(1, 4), big.NewRat(1, 2), big.NewRat(3, 4), big.NewRat(7, 8)},
			orders: []int{1, 1, 1, 1, 1}, cofactor: rationalPolynomial{big.NewRat(-1, 1)}, lower: []int{0, 1, 0, 1, 0, 1},
			cycles: [][4]int{{7, 1, 6, 0}, {2, 8, 1, 7}, {9, 3, 8, 2}, {4, 10, 3, 9}, {11, 5, 10, 4}},
			areas:  []int64{1079430, 1421, 125, 368, 368, 125, 1421, 2062470}, denominator: 3145728,
		},
		{
			name: "cubic then quadratic", stations: []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 4)},
			orders: []int{3, 2}, cofactor: rationalPolynomial{big.NewRat(-1, 1)}, lower: []int{0, 1, 1},
			cycles: [][4]int{{4, 1, 3, 0}, {5, 2, 1, 4}},
			areas:  []int64{41997, 59, 32, 211, 80581}, denominator: 122880,
		},
		{
			name: "quadratic then cubic", stations: []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 4)},
			orders: []int{2, 3}, cofactor: rationalPolynomial{big.NewRat(-1, 1)}, lower: []int{0, 0, 1},
			cycles: [][4]int{{1, 4, 3, 0}, {5, 2, 4, 1}},
			areas:  []int64{42181, 211, 32, 59, 80397}, denominator: 122880,
		},
		{
			name: "cubic with quadratic quotient", stations: []*big.Rat{big.NewRat(1, 4)},
			orders: []int{3}, cofactor: rationalPolynomial{big.NewRat(-65, 256), big.NewRat(1, 1), big.NewRat(-1, 1)}, lower: []int{0, 1},
			cycles: [][4]int{{3, 1, 2, 0}},
			areas:  []int64{1307697, 799, 43983, 2579681}, denominator: 3932160,
		},
		{
			name: "order five", stations: []*big.Rat{big.NewRat(1, 2)},
			orders: []int{5}, cofactor: rationalPolynomial{big.NewRat(-1, 1)}, lower: []int{0, 1},
			cycles: [][4]int{{3, 1, 2, 0}},
			areas:  []int64{131, 1, 1, 251}, denominator: 384,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			delta := arrangementGap(t, tc.stations, tc.orders, tc.cofactor)
			sameRat := func(a, b *big.Rat) bool { return a.Cmp(b) == 0 }
			for axis := 0; axis < 2; axis++ {
				a, b, powers := polynomialPairGraphFixture(t, axis, delta)
				for order, sources := range [][2]*SurfacePatch{{a, b}, {b, a}} {
					r, e := DiscoverSplineGraphPolynomialArrangement(ctx, sources[0], sources[1], axis, GraphPointContactOptions{})
					if e != nil || !r.Proven || r.FreeAxis != axis || len(r.CertifiedCells) != 4 ||
						!slices.Equal(r.ContactOrder, tc.orders) || !slices.Equal(r.LowerBranches, tc.lower) ||
						len(r.Nodes) != len(tc.orders) || len(r.Regions) != len(tc.areas) || len(r.Edges) != 2*(len(tc.orders)+1) ||
						r.CofactorSign != -1 || !slices.EqualFunc(r.Cofactor, tc.cofactor, sameRat) {
						t.Fatal("complete original factor and slab ledger", axis, order, r.ContactOrder, r.Cofactor, e)
					}
					for i, p := range r.Power {
						if !slices.EqualFunc(p, powers[i], sameRat) {
							t.Fatal("original source branch powers", i, p, powers[i])
						}
					}
					checkPolynomialArrangementJets(t, r, tc.stations, tc.orders, tc.cofactor)
					checkOriginalPolynomialContactRegions(t, polynomialContactRegionTest{
						r.FreeAxis, r.Graph.ProjectionOrientation, r.Power, r.Vertices, r.Edges, r.Nodes, r.Regions, r.ContactOrder,
					}, sources, 1-2*order, tc.cycles)
					uv, xyz, e := r.RepresentedVertices()
					if e != nil || len(uv) != len(tc.orders)+4 || len(xyz) != len(uv) {
						t.Fatal("lossless exact native coordinates", e)
					}
					for i := range uv {
						for j := range uv[:i] {
							if uv[i] == uv[j] || xyz[i] == xyz[j] {
								t.Fatal("distinct original vertex identities", i, j)
							}
						}
					}
					total := new(big.Rat)
					for i, region := range r.Regions {
						area := testPolynomialStripArea(axis, r.Edges, region)
						if i == len(r.Regions)-1 {
							area.Add(area, big.NewRat(1, 1))
						}
						if want := big.NewRat(tc.areas[i], tc.denominator); area.Cmp(want) != 0 {
							t.Fatal("exact independent integration of original power form", i, area, want)
						}
						total.Add(total, area)
					}
					if total.Cmp(big.NewRat(1, 1)) != 0 {
						t.Fatal("complete signed coverage of original chart", total)
					}
					for _, limit := range []int{r.ExactOperations - 1, r.ExactOperations} {
						q, e := DiscoverSplineGraphPolynomialArrangement(ctx, sources[0], sources[1], axis, GraphPointContactOptions{MaxOperations: limit})
						if (e == nil) != (limit == r.ExactOperations) ||
							(e != nil && !reflect.DeepEqual(q, SplineGraphPolynomialArrangement{ExactOperations: q.ExactOperations})) {
							t.Fatal("atomic exact work boundary", axis, order, e)
						}
					}
				}
			}
		})
	}
}

func checkPolynomialArrangementJets(t *testing.T, r SplineGraphPolynomialArrangement, stations []*big.Rat, orders []int, cofactor rationalPolynomial) {
	t.Helper()
	for i, node := range r.Nodes {
		station, order := stations[i], orders[i]
		axis := r.FreeAxis
		vertex := r.Vertices[node.Vertex]
		if node.Vertex != 4+i || vertex.Parameters[axis].Cmp(station) != 0 ||
			vertex.Parameters[1-axis].Cmp(testGraphPowerValue(r.Power[0], station)) != 0 {
			t.Fatal("junction refers to the original exact contact station", i, vertex)
		}
		derivatives := [2]rationalPolynomial{r.Power[0], r.Power[1]}
		for n := 1; n <= order; n++ {
			for k := range derivatives {
				derivatives[k] = polynomialDerivative(derivatives[k])
				if n == 1 && (node.Tangents[k][axis].Cmp(big.NewRat(1, 1)) != 0 ||
					node.Tangents[k][1-axis].Cmp(testGraphPowerValue(derivatives[k], station)) != 0) {
					t.Fatal("original branch tangent at the contact", i, k)
				}
			}
			if n < order && testGraphPowerValue(derivatives[0], station).Cmp(testGraphPowerValue(derivatives[1], station)) != 0 {
				t.Fatal("original derivatives through the lower orders must coincide", i, n)
			}
		}
		// The leading Taylor coefficient comes from the other original
		// factors evaluated at this station, independently of re-expansion.
		gap := testGraphPowerValue(cofactor, station)
		for factor := 2; factor <= order; factor++ {
			gap.Mul(gap, big.NewRat(int64(factor), 1))
		}
		for j, other := range stations {
			if i == j {
				continue
			}
			separation := new(big.Rat).Sub(station, other)
			for range orders[j] {
				gap.Mul(gap, separation)
			}
		}
		if gap.Sign() == 0 || r.DerivativeGap[i].Cmp(gap) != 0 {
			t.Fatal("independent signed derivative separation", i, r.DerivativeGap[i], gap)
		}
	}
}

func TestSplineGraphPolynomialArrangementAllOrders(t *testing.T) {
	stations := []*big.Rat{big.NewRat(1, 8), big.NewRat(1, 4), big.NewRat(1, 2), big.NewRat(3, 4), big.NewRat(7, 8)}
	var checked int
	var check func([]int, int, int)
	check = func(orders []int, degree, remaining int) {
		if remaining > 0 {
			for n := 1; n <= remaining; n++ {
				check(append(orders, n), degree, remaining-n)
			}
			return
		}
		cofactor := rationalPolynomial{big.NewRat(int64(1-2*(degree%2)), 16)}
		roots := stations[:len(orders)]
		a, b, _ := polynomialPairGraphFixture(t, 0, arrangementGap(t, roots, orders, cofactor))
		r, e := DiscoverSplineGraphPolynomialArrangement(context.Background(), a, b, 0, GraphPointContactOptions{})
		if e != nil || !r.Proven || !slices.Equal(r.ContactOrder, orders) {
			t.Fatal("ordered composition of a supported degree", orders, e)
		}
		checkPolynomialArrangementJets(t, r, roots, orders, cofactor)
		checked++
	}
	for degree := 1; degree <= 5; degree++ {
		check(nil, degree, degree)
	}
	if checked != 31 {
		t.Fatal("every nonempty ordered multiplicity sequence through total degree five", checked)
	}
}

func TestSplineGraphPolynomialArrangementResidualRoot(t *testing.T) {
	stations := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 4)}
	a, b, powers := polynomialPairGraphFixture(t, 0, arrangementGap(t, stations, []int{2, 2}, rationalPolynomial{big.NewRat(1, 1)}))
	result, e := DiscoverSplineGraphPolynomialArrangement(context.Background(), a, b, 0, GraphPointContactOptions{})
	if e != nil || !slices.Equal(result.ContactOrder, []int{2, 2}) {
		t.Fatal("complete arrangement must retain both even contacts", e)
	}
	partial := SplineGraphPolynomialArrangement{
		Power: [2][]*big.Rat{powers[0], powers[1]}, Nodes: make([]SplineGraphPolynomialJunction, 1),
	}
	budget := &polynomialBudget{maxOperations: 10000, maxBits: 1024}
	if e = partial.certifyContactFactors(context.Background(), []*big.Rat{new(big.Rat), stations[0], big.NewRat(1, 1)}, budget); !errors.Is(e, ErrConvergence) {
		t.Fatal("quotient with equal endpoint signs still has an omitted even root", e)
	}
}

func TestSplineGraphPolynomialArrangementScope(t *testing.T) {
	ctx := context.Background()
	root := big.NewRat(1, 5)
	delta := arrangementGap(t, []*big.Rat{root}, []int{5}, rationalPolynomial{big.NewRat(-1, 4)})
	a, b, _ := polynomialPairGraphFixture(t, 0, delta)
	r, e := DiscoverSplineGraphPolynomialArrangement(ctx, a, b, 0, GraphPointContactOptions{})
	if e != nil || !r.Proven || !slices.Equal(r.ContactOrder, []int{5}) || r.Vertices[4].Parameters[0].Cmp(root) != 0 {
		t.Fatal("rational order-five contact has exact geometry", e)
	}
	if u, v, e := r.RepresentedVertices(); !errors.Is(e, ErrUnrepresentable) || u != nil || v != nil {
		t.Fatal("native conversion must independently reject its nonbinary station", e)
	}
	if q, e := DiscoverSplineGraphPolynomialQuarticTangency(ctx, a, b, 0, GraphPointContactOptions{}); !errors.Is(e, ErrConvergence) || q.Proven {
		t.Fatal("legacy named mode must retain its quartic degree bound", e)
	}
	irrational := arrangementGap(t, []*big.Rat{big.NewRat(1, 4)}, []int{1}, rationalPolynomial{big.NewRat(-1, 128), big.NewRat(1, 16), big.NewRat(-1, 16)})
	// The retained quotient may have real roots outside the chart.
	outside := rationalPolynomial{big.NewRat(1, 8), big.NewRat(-1, 16)}
	a, b, _ = polynomialPairGraphFixture(t, 0, arrangementGap(t, []*big.Rat{big.NewRat(1, 2)}, []int{2}, outside))
	q, e := DiscoverSplineGraphPolynomialArrangement(ctx, a, b, 0, GraphPointContactOptions{})
	if e != nil || !q.Proven || !slices.Equal(q.ContactOrder, []int{2}) || q.CofactorSign != 1 ||
		!slices.EqualFunc(q.Cofactor, outside, func(a, b *big.Rat) bool { return a.Cmp(b) == 0 }) {
		t.Fatal("a real outside root may remain in the exact signed quotient", e)
	}
	for name, tc := range map[string]struct {
		gap  rationalPolynomial
		want error
	}{
		"coincident": {nil, ErrDegenerate},
		"disjoint":   {rationalPolynomial{big.NewRat(1, 16)}, ErrConvergence},
		"endpoint":   {arrangementGap(t, []*big.Rat{new(big.Rat)}, []int{4}, rationalPolynomial{big.NewRat(1, 16)}), ErrDomain},
		"irrational": {irrational, ErrUnrepresentable},
		"sextic":     {arrangementGap(t, []*big.Rat{big.NewRat(1, 2)}, []int{6}, rationalPolynomial{big.NewRat(1, 16)}), ErrConvergence},
	} {
		t.Run(name, func(t *testing.T) {
			a, b, _ := polynomialPairGraphFixture(t, 0, tc.gap)
			r, e := DiscoverSplineGraphPolynomialArrangement(ctx, a, b, 0, GraphPointContactOptions{})
			if !errors.Is(e, tc.want) || !reflect.DeepEqual(r, SplineGraphPolynomialArrangement{ExactOperations: r.ExactOperations}) {
				t.Fatal("unsupported sources must fail without publishing an arrangement", e)
			}
		})
	}
	a, b, _ = polynomialPairGraphFixture(t, 0, rationalPolynomial{big.NewRat(1, 16)})
	if r, e := DiscoverSplineGraphTwoPolynomials(ctx, a, b, 0, GraphPointContactOptions{}); e != nil || !r.Proven {
		t.Fatal("ordinary disjoint arrangement still accepts the source pair", e)
	}
	a, b, _ = polynomialPairGraphFixture(t, 0, delta)
	if r, e := DiscoverSplineGraphPolynomialArrangement(ctx, a, b, 0, GraphPointContactOptions{MaxCells: 1}); !errors.Is(e, ErrConvergence) || r.Proven {
		t.Fatal("original union cell cap ignored", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := DiscoverSplineGraphPolynomialArrangement(canceled, a, b, 0, GraphPointContactOptions{}); !errors.Is(e, context.Canceled) || r.Proven {
		t.Fatal("canceled proof published original geometry", e)
	}
	a.net[(a.countU-1)*a.countV][2].Add(a.net[(a.countU-1)*a.countV][2], big.NewRat(1, 1024))
	if r, e := DiscoverSplineGraphPolynomialArrangement(ctx, a, b, 0, GraphPointContactOptions{}); !errors.Is(e, ErrConvergence) || r.Proven {
		t.Fatal("later original knot cell changed", e)
	}
}

func TestSplineGraphPolynomialArrangementOriginalUnits(t *testing.T) {
	ctx := context.Background()
	stations := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 4)}
	orders := []int{3, 2}
	gap := arrangementGap(t, stations, orders, rationalPolynomial{big.NewRat(-1, 1)})
	base := rationalPolynomial{big.NewRat(1, 4), new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat), big.NewRat(1, 8)}
	upper, e := graphPolynomialSum(base, gap, 1, &polynomialBudget{maxOperations: 1000, maxBits: 1024})
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
			t.Fatal("both quintic branches must retain degree-ten source coefficients")
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
	r, e := DiscoverSplineGraphPolynomialArrangement(ctx, sources[0], sources[1], 1, GraphPointContactOptions{})
	if e != nil || !r.Proven || r.Graph.ProjectionOrientation != -1 || !slices.Equal(r.ContactOrder, orders) ||
		len(r.Cofactor) != 1 || r.Cofactor[0].Cmp(big.NewRat(-2, 243)) != 0 {
		t.Fatal("two quintic branches in reflected nonunit original charts", e)
	}
	checkPolynomialArrangementJets(t, r, []*big.Rat{big.NewRat(11, 4), big.NewRat(17, 4)}, orders, rationalPolynomial{big.NewRat(-2, 243)})
	checkOriginalPolynomialContactRegions(t, polynomialContactRegionTest{
		r.FreeAxis, r.Graph.ProjectionOrientation, r.Power, r.Vertices, r.Edges, r.Nodes, r.Regions, r.ContactOrder,
	}, sources, 1, [][4]int{{4, 1, 3, 0}, {5, 2, 1, 4}})
}
