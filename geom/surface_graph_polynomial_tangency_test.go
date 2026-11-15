package geom

import (
	"context"
	"errors"
	"math/big"
	"reflect"
	"testing"
)

func polynomialTangencyGap(denominator int64) rationalPolynomial {
	// The quartic gap is ((denominator*u-1)^2)(1+u^2)/64.
	return rationalPolynomial{
		big.NewRat(1, 64), big.NewRat(-2*denominator, 64), big.NewRat(denominator*denominator+1, 64),
		big.NewRat(-2*denominator, 64), big.NewRat(denominator*denominator, 64),
	}
}

func TestSplineGraphPolynomialTangency(t *testing.T) {
	ctx := context.Background()
	for _, axis := range []int{0, 1} {
		a, b, want := polynomialPairGraphFixture(t, axis, polynomialTangencyGap(2))
		for order, pair := range [][2]*SurfacePatch{{a, b}, {b, a}} {
			r, e := DiscoverSplineGraphPolynomialTangency(ctx, pair[0], pair[1], axis, GraphPointContactOptions{})
			if e != nil || !r.Proven || r.FreeAxis != axis || len(r.CertifiedCells) != 4 ||
				!reflect.DeepEqual(r.Power, [2][]*big.Rat{want[0], want[1]}) {
				t.Fatal("complete original cubic/quartic tangency", axis, order, r, e)
			}
			v := r.Vertices[4]
			if v.Parameters[axis].Cmp(big.NewRat(1, 2)) != 0 || v.Parameters[1-axis].Cmp(big.NewRat(23, 64)) != 0 ||
				r.Tangent[axis].Cmp(big.NewRat(1, 1)) != 0 || r.Tangent[1-axis].Cmp(big.NewRat(3, 32)) != 0 ||
				r.SecondDerivativeGap.Cmp(big.NewRat(5, 32)) != 0 || len(r.Separation) != 3 ||
				r.Separation[0].Cmp(big.NewRat(1, 16)) != 0 || r.Separation[1].Sign() != 0 || r.Separation[2].Cmp(big.NewRat(1, 16)) != 0 {
				t.Fatal("original exact double root, tangent and quadratic separation", r)
			}
			checkSplineGraphPolynomialTangency(t, r, pair, 1-2*order)
			for i, want := range []*big.Rat{big.NewRat(41, 15360), big.NewRat(71, 15360)} {
				if got := testPolynomialStripArea(axis, r.Edges[:], r.Regions[1+i]); got.Cmp(want) != 0 {
					t.Fatal("analytical signed-wedge area", got, want)
				}
			}
			for _, limit := range []int{r.ExactOperations - 1, r.ExactOperations} {
				q, e := DiscoverSplineGraphPolynomialTangency(ctx, pair[0], pair[1], axis, GraphPointContactOptions{MaxOperations: limit})
				if (e == nil) != (limit == r.ExactOperations) || (e != nil && !reflect.DeepEqual(q, SplineGraphPolynomialTangency{ExactOperations: q.ExactOperations})) {
					t.Fatal("atomic quota", axis, order, e)
				}
			}
			if q, e := DiscoverSplineGraphPolynomialNode(ctx, pair[0], pair[1], axis, GraphPointContactOptions{}); e == nil || q.Proven {
				t.Fatal("tangent admitted as a crossing", e)
			}
			if q, e := DiscoverSplineGraphTwoPolynomialNodes(ctx, pair[0], pair[1], axis, GraphPointContactOptions{}); e == nil || q.Proven {
				t.Fatal("tangent admitted as a crossing pair", e)
			}
			if q, e := DiscoverSplineGraphTwoPolynomials(ctx, pair[0], pair[1], axis, GraphPointContactOptions{}); e == nil || q.Proven {
				t.Fatal("tangent admitted as permanently separated branches", e)
			}
		}
	}
}

func checkSplineGraphPolynomialTangency(t *testing.T, r SplineGraphPolynomialTangency, pair [2]*SurfacePatch, sign int) {
	t.Helper()
	ctx := context.Background()
	for _, vertex := range r.Vertices {
		for _, source := range pair {
			jet, e := source.EvaluateRational(ctx, vertex.Parameters[0], vertex.Parameters[1], SurfacePatchEvaluationOptions{})
			if e != nil || !reflect.DeepEqual(jet.Point, vertex.Point) {
				t.Fatal("exact original contact vertex", vertex, jet.Point, e)
			}
		}
	}
	for _, edge := range r.Edges {
		if len(edge.Controls) != len(r.Power[edge.Branch]) {
			t.Fatal("original degree was changed", edge)
		}
		for _, parameter := range []*big.Rat{new(big.Rat), big.NewRat(1, 3), big.NewRat(1, 1)} {
			uv := testGraphBezierValue(edge.Controls, parameter)
			x := uv[r.FreeAxis]
			if uv[1-r.FreeAxis].Cmp(testGraphPowerValue(r.Power[edge.Branch], x)) != 0 {
				t.Fatal("exact source branch restriction", edge, parameter)
			}
			a, e := pair[0].EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			b, e := pair[1].EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{})
			if e != nil || !reflect.DeepEqual(a.Point, b.Point) {
				t.Fatal("both original branch owners", uv, a.Point, b.Point, e)
			}
		}
	}
	cycle, sectors := [4]int{1, 3, 2, 0}, [4]int{2, 3, 1, 0}
	orientation := r.Graph.ProjectionOrientation * (1 - 2*r.FreeAxis)
	if orientation < 0 {
		cycle, sectors = [4]int{1, 0, 2, 3}, [4]int{0, 1, 3, 2}
	}
	if r.RayCycle != cycle || r.SectorRegions != sectors {
		t.Fatal("one-sided model-XY sector order", r.RayCycle, r.SectorRegions)
	}
	var uses [4][2]int
	for i, region := range r.Regions {
		want := [4]int{sign, -sign, -sign, sign}[i]
		u, v := region.Witness[0], region.Witness[1]
		a, e := pair[0].EvaluateRational(ctx, u, v, SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		b, e := pair[1].EvaluateRational(ctx, u, v, SurfacePatchEvaluationOptions{})
		if e != nil || a.Point[2].Cmp(b.Point[2]) != want || region.HeightSide != want {
			t.Fatal("signed one-sided height gap", region, a.Point, b.Point, e)
		}
		perimeter := region.Perimeter
		if perimeter == nil || perimeter.ParameterDirection != r.Graph.ProjectionOrientation {
			t.Fatal("missing original exterior route", region)
		}
		at := perimeter.End
		for _, use := range region.Edges {
			edge := r.Edges[use.Edge]
			start, end, dir := edge.Start, edge.End, 0
			if use.Reversed {
				start, end, dir = end, start, 1
			}
			uses[use.Edge][dir]++
			if at != start {
				t.Fatal("disconnected tangent sector", region)
			}
			at = end
		}
		if at != perimeter.Start {
			t.Fatal("original perimeter did not close", region)
		}
	}
	for _, use := range uses {
		if use != [2]int{1, 1} {
			t.Fatal("missing original opposite owner use", uses)
		}
	}
}

func TestSplineGraphPolynomialTangencyOriginalUnits(t *testing.T) {
	ctx := context.Background()
	a, b, _ := polynomialPairGraphFixture(t, 0, polynomialTangencyGap(2))
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
	r, e := DiscoverSplineGraphPolynomialTangency(ctx, pair[0], pair[1], 0, GraphPointContactOptions{})
	if e != nil || !r.Proven || r.Graph.ProjectionOrientation != -1 || r.Vertices[4].Parameters[0].Cmp(big.NewRat(7, 2)) != 0 ||
		r.Vertices[4].Parameters[1].Cmp(big.NewRat(-9, 32)) != 0 || r.Tangent[1].Cmp(big.NewRat(1, 16)) != 0 || r.SecondDerivativeGap.Cmp(big.NewRat(5, 144)) != 0 {
		t.Fatal("original scaled chart jets", r, e)
	}
	checkSplineGraphPolynomialTangency(t, r, pair, 1)

	a, b, _ = polynomialPairGraphFixture(t, 0, polynomialTangencyGap(3))
	r, e = DiscoverSplineGraphPolynomialTangency(ctx, a, b, 0, GraphPointContactOptions{})
	if e != nil || !r.Proven || r.Vertices[4].Parameters[0].Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatal("nonbinary rational geometry should remain exact", r, e)
	}
	if _, e := RepresentedExactPoint(r.Vertices[4].Point); !errors.Is(e, ErrUnrepresentable) {
		t.Fatal("rational geometry point should fail lossless native conversion", e)
	}
}

func TestSplineGraphPolynomialTangencyRejectsOtherContacts(t *testing.T) {
	ctx := context.Background()
	for name, tc := range map[string]struct {
		delta rationalPolynomial
		want  error
	}{
		"identical branches": {nil, ErrDegenerate},
		"separated":          {rationalPolynomial{big.NewRat(1, 64)}, ErrConvergence},
		"crossings":          {twoNodeGap(4), ErrConvergence},
		"triple":             {rationalPolynomial{big.NewRat(-1, 64), big.NewRat(3, 32), big.NewRat(-3, 16), big.NewRat(1, 8)}, ErrDegenerate},
		"quadruple":          {rationalPolynomial{big.NewRat(1, 512), big.NewRat(-1, 64), big.NewRat(3, 64), big.NewRat(-1, 16), big.NewRat(1, 32)}, ErrDegenerate},
		"two double roots":   {rationalPolynomial{big.NewRat(9, 2048), big.NewRat(-3, 64), big.NewRat(11, 64), big.NewRat(-1, 4), big.NewRat(1, 8)}, ErrConvergence},
		"endpoint":           {rationalPolynomial{new(big.Rat), new(big.Rat), big.NewRat(1, 8)}, ErrDomain},
		"irrational tangent": {rationalPolynomial{big.NewRat(1, 32), new(big.Rat), big.NewRat(-1, 8), new(big.Rat), big.NewRat(1, 8)}, ErrUnrepresentable},
	} {
		t.Run(name, func(t *testing.T) {
			a, b, _ := polynomialPairGraphFixture(t, 0, tc.delta)
			r, e := DiscoverSplineGraphPolynomialTangency(ctx, a, b, 0, GraphPointContactOptions{})
			if !errors.Is(e, tc.want) || !reflect.DeepEqual(r, SplineGraphPolynomialTangency{ExactOperations: r.ExactOperations}) {
				t.Fatal("missing atomic contact distinction", r, e)
			}
		})
	}
	a, b, _ := polynomialPairGraphFixture(t, 0, polynomialTangencyGap(2))
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := DiscoverSplineGraphPolynomialTangency(canceled, a, b, 0, GraphPointContactOptions{}); !errors.Is(e, context.Canceled) || r.Proven {
		t.Fatal("canceled proof was published", e)
	}
	a.net[(a.countU-1)*a.countV][2].Add(a.net[(a.countU-1)*a.countV][2], big.NewRat(1, 1024))
	if r, e := DiscoverSplineGraphPolynomialTangency(ctx, a, b, 0, GraphPointContactOptions{}); !errors.Is(e, ErrConvergence) || r.Proven || len(r.CertifiedCells) != 0 {
		t.Fatal("later original cell changed", r, e)
	}
	a, b, _ = polynomialPairGraphFixture(t, 0, nil)
	a.net[(a.countU-1)*a.countV][2].Add(a.net[(a.countU-1)*a.countV][2], big.NewRat(1, 1024))
	if r, e := DiscoverSplineGraphPolynomialTangency(ctx, a, b, 0, GraphPointContactOptions{}); !errors.Is(e, ErrConvergence) || r.Proven || len(r.CertifiedCells) != 0 {
		t.Fatal("coincident first cell cannot classify incomplete original coverage", r, e)
	}
}
