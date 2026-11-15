package geom

import (
	"context"
	"errors"
	"math/big"
	"reflect"
	"slices"
	"testing"
)

func twoCrossingsTangentGap(t *testing.T, linear ...[2]int64) rationalPolynomial {
	t.Helper()
	// Reuse the exact test product of the original monic factors expressed
	// as integer linear forms, with scale 1/256 for these roof fixtures.
	gap := fourPolynomialNodeGap(t, linear...)
	for _, c := range gap {
		c.Mul(c, big.NewRat(16, 1))
	}
	return gap
}

func TestSplineGraphPolynomialTwoCrossingsTangency(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		tangent int64
		orders  [3]int
		gaps    [3]*big.Rat
		areas   [6]int64 // numerators over 61440
		cycles  [3][4]int
	}{
		{1, [3]int{2, 1, 1}, [3]*big.Rat{big.NewRat(1, 4), big.NewRat(-1, 64), big.NewRat(1, 16)},
			[6]int64{16873, 97, 7, 23, 367, 44073}, [3][4]int{{1, 5, 4, 0}, {6, 2, 5, 1}, {3, 7, 2, 6}}},
		{2, [3]int{1, 2, 1}, [3]*big.Rat{big.NewRat(-1, 32), big.NewRat(-1, 8), big.NewRat(1, 32)},
			[6]int64{16880, 232, 8, 8, 232, 44080}, [3][4]int{{5, 1, 4, 0}, {6, 2, 1, 5}, {3, 7, 2, 6}}},
		{3, [3]int{1, 1, 2}, [3]*big.Rat{big.NewRat(-1, 16), big.NewRat(1, 64), big.NewRat(1, 4)},
			[6]int64{16873, 367, 23, 7, 97, 44073}, [3][4]int{{5, 1, 4, 0}, {2, 6, 1, 5}, {3, 7, 6, 2}}},
	} {
		// f=1/4+u^4/8 and g-f=(u-1/4)(u-1/2)(u-3/4)(u-tangent/4).
		gap := twoCrossingsTangentGap(t, [2]int64{1, 4}, [2]int64{2, 4}, [2]int64{3, 4}, [2]int64{tc.tangent, 4})
		for axis := 0; axis < 2; axis++ {
			a, b, powers := fourPolynomialNodeFixture(t, axis, gap)
			for order, sources := range [][2]*SurfacePatch{{a, b}, {b, a}} {
				r, e := DiscoverSplineGraphPolynomialTwoCrossingsTangency(ctx, sources[0], sources[1], axis, GraphPointContactOptions{})
				if e != nil || !r.Proven || r.FreeAxis != axis || len(r.CertifiedCells) != 4 || r.SeparationScale.Cmp(big.NewRat(1, 1)) != 0 {
					t.Fatal("complete original union-knot quartic identity", tc.tangent, axis, order, e)
				}
				for i, power := range powers {
					if !slices.EqualFunc(r.Power[i], power, func(x, y *big.Rat) bool { return x.Cmp(y) == 0 }) {
						t.Fatal("changed original quartic branch", i, power, r.Power[i])
					}
				}
				if r.ContactOrder != tc.orders || !reflect.DeepEqual(r.DerivativeGap, tc.gaps) {
					t.Fatal("exact typed contact orders and derivative gaps", tc.tangent, axis, order, r.ContactOrder, r.DerivativeGap)
				}
				for i, node := range r.Nodes {
					x := big.NewRat(int64(i+1), 4)
					vertex := r.Vertices[node.Vertex]
					if node.Vertex != 4+i || vertex.Parameters[axis].Cmp(x) != 0 ||
						vertex.Parameters[1-axis].Cmp(testGraphPowerValue(powers[0], x)) != 0 {
						t.Fatal("original contact vertex and free station", i, vertex)
					}
					for j, tangent := range node.Tangents {
						slope := testGraphPowerValue(polynomialDerivative(powers[j]), x)
						if tangent[axis].Cmp(big.NewRat(1, 1)) != 0 || tangent[1-axis].Cmp(slope) != 0 {
							t.Fatal("original exact branch tangent", i, j, tangent, slope)
						}
					}
				}
				uv, xyz, e := r.RepresentedVertices()
				if e != nil {
					t.Fatal("lossless native vertex conversion", e)
				}
				for i := range uv {
					for j := range uv[:i] {
						if uv[i] == uv[j] || xyz[i] == xyz[j] {
							t.Fatal("distinct originals must remain distinct native vertices", i, j)
						}
					}
				}
				checkOriginalPolynomialContactRegions(t, polynomialContactRegionTest{
					r.FreeAxis, r.Graph.ProjectionOrientation, r.Power, r.Vertices[:], r.Edges[:], r.Nodes[:], r.Regions[:], r.ContactOrder[:],
				}, sources, 1-2*order, tc.cycles[:])
				total := new(big.Rat)
				for i, region := range r.Regions {
					// Compare Green's exact boundary integral of the published
					// Bézier curves with the integrated original power forms.
					area := testPolynomialStripArea(axis, r.Edges[:], region)
					if i == 5 {
						area.Add(area, big.NewRat(1, 1)) // upper perimeter
					}
					if want := big.NewRat(tc.areas[i], 61440); area.Cmp(want) != 0 {
						t.Fatal("independently integrated original region", tc.tangent, axis, order, i, area, want)
					}
					total.Add(total, area)
				}
				if total.Cmp(big.NewRat(1, 1)) != 0 {
					t.Fatal("complete unit-chart coverage", total)
				}
				for _, limit := range []int{r.ExactOperations - 1, r.ExactOperations} {
					q, e := DiscoverSplineGraphPolynomialTwoCrossingsTangency(ctx, sources[0], sources[1], axis, GraphPointContactOptions{MaxOperations: limit})
					if (e == nil) != (limit == r.ExactOperations) ||
						(e != nil && !reflect.DeepEqual(q, SplineGraphPolynomialTwoCrossingsTangency{ExactOperations: q.ExactOperations})) {
						t.Fatal("atomic exact work boundary", tc.tangent, axis, order, e)
					}
				}
			}
		}
	}
}

func TestSplineGraphPolynomialTwoCrossingsTangencyOriginalUnits(t *testing.T) {
	ctx := context.Background()
	gap := twoCrossingsTangentGap(t, [2]int64{1, 4}, [2]int64{2, 4}, [2]int64{2, 4}, [2]int64{3, 4})
	a, b, _ := fourPolynomialNodeFixture(t, 1, gap)
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
	r, e := DiscoverSplineGraphPolynomialTwoCrossingsTangency(ctx, sources[0], sources[1], 1, GraphPointContactOptions{})
	if e != nil || !r.Proven || r.Graph.ProjectionOrientation != -1 || r.SeparationScale.Cmp(big.NewRat(2, 81)) != 0 ||
		!reflect.DeepEqual(r.DerivativeGap, [3]*big.Rat{big.NewRat(-1, 48), big.NewRat(-1, 36), big.NewRat(1, 48)}) {
		t.Fatal("exact reflected original units and derivative orders", r.SeparationScale, r.DerivativeGap, e)
	}
	for i, n := range []int64{11, 14, 17} {
		if r.Vertices[4+i].Parameters[1].Cmp(big.NewRat(n, 4)) != 0 {
			t.Fatal("original nonunit root station", i, r.Vertices[4+i])
		}
	}
	checkOriginalPolynomialContactRegions(t, polynomialContactRegionTest{
		r.FreeAxis, r.Graph.ProjectionOrientation, r.Power, r.Vertices[:], r.Edges[:], r.Nodes[:], r.Regions[:], r.ContactOrder[:],
	}, sources, 1, [][4]int{{5, 1, 4, 0}, {6, 2, 1, 5}, {3, 7, 2, 6}})
}

func TestSplineGraphPolynomialTwoCrossingsTangencyScope(t *testing.T) {
	ctx := context.Background()
	nonbinary := twoCrossingsTangentGap(t, [2]int64{1, 5}, [2]int64{2, 4}, [2]int64{2, 4}, [2]int64{3, 4})
	a, b, _ := fourPolynomialNodeFixture(t, 0, nonbinary)
	r, e := DiscoverSplineGraphPolynomialTwoCrossingsTangency(ctx, a, b, 0, GraphPointContactOptions{})
	if e != nil || !r.Proven || r.Vertices[4].Parameters[0].Cmp(big.NewRat(1, 5)) != 0 {
		t.Fatal("exact geometry retains an interior nonbinary crossing", r.ContactOrder, e)
	}
	if uv, xyz, e := r.RepresentedVertices(); !errors.Is(e, ErrUnrepresentable) || uv != ([7]Vec2{}) || xyz != ([7]Vec3{}) {
		t.Fatal("lossless native coordinates are checked separately", e)
	}
	irrational, e := polynomialProduct(
		rationalPolynomial{big.NewRat(2, 1), big.NewRat(-16, 1), big.NewRat(16, 1)},
		twoCrossingsTangentGap(t, [2]int64{2, 4}, [2]int64{2, 4}), &polynomialBudget{maxOperations: 1000, maxBits: 1024},
	)
	if e != nil {
		t.Fatal(e)
	}
	for name, tc := range map[string]struct {
		gap  rationalPolynomial
		want error
	}{
		"coincident":       {nil, ErrDegenerate},
		"two double roots": {twoCrossingsTangentGap(t, [2]int64{1, 4}, [2]int64{1, 4}, [2]int64{3, 4}, [2]int64{3, 4}), ErrConvergence},
		"one crossing":     {mixedGraphGap(1, 4), ErrConvergence},
		"four crossings":   {fourPolynomialNodeGap(t, [2]int64{1, 8}, [2]int64{3, 8}, [2]int64{5, 8}, [2]int64{7, 8}), ErrConvergence},
		"quintic branch":   {twoCrossingsTangentGap(t, [2]int64{1, 8}, [2]int64{3, 8}, [2]int64{3, 8}, [2]int64{5, 8}, [2]int64{7, 8}), ErrConvergence},
		"triple root":      {twoCrossingsTangentGap(t, [2]int64{1, 4}, [2]int64{1, 4}, [2]int64{1, 4}, [2]int64{3, 4}), ErrDegenerate},
		"endpoint contact": {twoCrossingsTangentGap(t, [2]int64{0, 4}, [2]int64{2, 4}, [2]int64{2, 4}, [2]int64{3, 4}), ErrDomain},
		"irrational":       {irrational, ErrUnrepresentable},
	} {
		t.Run(name, func(t *testing.T) {
			a, b, _ := fourPolynomialNodeFixture(t, 0, tc.gap)
			r, e := DiscoverSplineGraphPolynomialTwoCrossingsTangency(ctx, a, b, 0, GraphPointContactOptions{})
			if !errors.Is(e, tc.want) || !reflect.DeepEqual(r, SplineGraphPolynomialTwoCrossingsTangency{ExactOperations: r.ExactOperations}) {
				t.Fatal("wrong rejection or partial source arrangement", name, e)
			}
		})
	}
	gap := twoCrossingsTangentGap(t, [2]int64{1, 4}, [2]int64{2, 4}, [2]int64{2, 4}, [2]int64{3, 4})
	a, b, _ = fourPolynomialNodeFixture(t, 0, gap)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := DiscoverSplineGraphPolynomialTwoCrossingsTangency(canceled, a, b, 0, GraphPointContactOptions{}); !errors.Is(e, context.Canceled) || r.Proven {
		t.Fatal("canceled proof published an arrangement", e)
	}
	if r, e := DiscoverSplineGraphPolynomialTwoCrossingsTangency(ctx, a, b, 0, GraphPointContactOptions{MaxCells: 1}); !errors.Is(e, ErrConvergence) || r.Proven {
		t.Fatal("union-knot cell budget ignored", e)
	}
	for _, candidate := range []rationalPolynomial{nil, gap} {
		a, b, _ = fourPolynomialNodeFixture(t, 0, candidate)
		a.net[(a.countU-1)*a.countV][2].Add(a.net[(a.countU-1)*a.countV][2], big.NewRat(1, 1024))
		r, e := DiscoverSplineGraphPolynomialTwoCrossingsTangency(ctx, a, b, 0, GraphPointContactOptions{})
		if !errors.Is(e, ErrConvergence) || !reflect.DeepEqual(r, SplineGraphPolynomialTwoCrossingsTangency{ExactOperations: r.ExactOperations}) {
			t.Fatal("later original owner cell changed", e)
		}
	}
}
