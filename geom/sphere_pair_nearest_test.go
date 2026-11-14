package geom

import (
	"context"
	"errors"
	"math/big"
	"reflect"
	"testing"
)

func TestSpherePairNearestDispatch(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	for _, tc := range []struct {
		a, b   Vec3
		ra, rb float64
		op     SphereSetOperation
		p      [3]*big.Rat
		kind   SphereNearestKind
	}{
		{Vec3{}, Vec3{X: 6}, 2, 1, SphereSetUnion, [3]*big.Rat{r(7, 2), r(0, 1), r(0, 1)}, SphereNearestSeparatedUnion},
		{Vec3{}, Vec3{X: 4}, 2, 2, SphereSetUnion, [3]*big.Rat{r(2, 1), r(0, 1), r(0, 1)}, SphereNearestSeparatedUnion},
		{Vec3{}, Vec3{X: 1}, 5, 1, SphereSetDifference, [3]*big.Rat{r(1, 1), r(0, 1), r(0, 1)}, SphereNearestCavity},
		{Vec3{}, Vec3{X: 2, Y: 2, Z: 2}, 5, 5, SphereSetUnion, [3]*big.Rat{r(2, 1), r(0, 1), r(1, 1)}, SphereNearestTransverseIsolated},
		{Vec3{}, Vec3{X: 2, Y: 2, Z: 2}, 5, 5, SphereSetIntersection, [3]*big.Rat{r(1, 1), r(1, 1), r(1, 1)}, SphereNearestTransverseAxis},
		{Vec3{}, Vec3{X: 2, Y: 2, Z: 2}, 5, 3, SphereSetDifference, [3]*big.Rat{r(2, 1), r(2, 1), r(2, 1)}, SphereNearestTransverseAxis},
	} {
		s, _ := NewSpherePairMaterial(tc.a, tc.ra, tc.b, tc.rb, tc.op)
		g, e := s.NearestBoundaryRational(ctx, tc.p, SphereFullBoundaryDistanceOptions{})
		if e != nil || g.Kind != tc.kind {
			t.Fatal(tc, g, e)
		}
		count := 0
		for _, present := range []bool{g.Separated != nil, g.Cavity != nil, g.Isolated != nil, g.Axis != nil} {
			if present {
				count++
			}
		}
		if count != 1 {
			t.Fatal("ambiguous payload")
		}
		directOps := 0
		switch g.Kind {
		case SphereNearestSeparatedUnion:
			q, e := s.NearestSeparatedUnionBoundaryRational(ctx, tc.p, SphereFullBoundaryDistanceOptions{})
			if e != nil {
				t.Fatal(e)
			}
			directOps = q.Operations
			q.Operations = g.Operations
			if !reflect.DeepEqual(q, *g.Separated) || g.Owners != q.Owners {
				t.Fatal("changed separated proof")
			}
		case SphereNearestCavity:
			q, e := s.NearestCavityBoundaryRational(ctx, tc.p, SphereFullBoundaryDistanceOptions{})
			if e != nil {
				t.Fatal(e)
			}
			directOps = q.Operations
			q.Operations = g.Operations
			if !reflect.DeepEqual(q, *g.Cavity) || g.Owners != q.Owners || !q.Candidates[1].WholeSphere {
				t.Fatal("lost complete cavity family")
			}
		case SphereNearestTransverseIsolated:
			q, e := s.NearestTransverseBoundaryRational(ctx, tc.p, SphereFullBoundaryDistanceOptions{})
			if e != nil {
				t.Fatal(e)
			}
			directOps = q.Operations
			q.Operations = g.Operations
			if !reflect.DeepEqual(q, *g.Isolated) || g.Owners != q.Owners {
				t.Fatal("changed isolated proof")
			}
		case SphereNearestTransverseAxis:
			q, e := s.NearestTransverseAxisBoundaryRational(ctx, tc.p, SphereFullBoundaryDistanceOptions{})
			if e != nil {
				t.Fatal(e)
			}
			directOps = q.Operations
			q.Operations = g.Operations
			if !reflect.DeepEqual(q, *g.Axis) || g.Owners != q.Owners {
				t.Fatal("lost complete axis families")
			}
		}
		if g.Operations <= directOps {
			t.Fatal("uncharged dispatch")
		}
		for _, n := range []int{g.Operations, g.Operations - 1, directOps} {
			v, e := s.NearestBoundaryRational(ctx, tc.p, SphereFullBoundaryDistanceOptions{Arithmetic: SurfacePatchOptions{MaxOperations: n}})
			if n == g.Operations {
				if e != nil {
					t.Fatal(e)
				}
			} else if e == nil || !reflect.DeepEqual(v, SpherePairNearest{}) {
				t.Fatal("work ledger reset/partial result", n, e)
			}
		}
		g.Lower.SetInt64(999)
		again, e := s.NearestBoundaryRational(ctx, tc.p, SphereFullBoundaryDistanceOptions{})
		if e != nil || again.Lower.Cmp(r(999, 1)) == 0 {
			t.Fatal("source ownership")
		}
	}
	for _, tc := range []struct {
		b  Vec3
		r  float64
		op SphereSetOperation
	}{{Vec3{}, 6, SphereSetDifference}, {Vec3{X: 10}, 1, SphereSetIntersection}, {Vec3{X: 4}, 1, SphereSetDifference}} {
		s, _ := NewSpherePairMaterial(Vec3{}, 5, tc.b, tc.r, tc.op)
		v, e := s.NearestBoundaryRational(ctx, [3]*big.Rat{r(0, 1), r(0, 1), r(0, 1)}, SphereFullBoundaryDistanceOptions{})
		if !errors.Is(e, ErrDomain) || !reflect.DeepEqual(v, SpherePairNearest{}) {
			t.Fatal("unsupported regime", tc, v, e)
		}
	}
	// An arbitrarily small exact off-axis displacement must not select a family.
	s, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 2, Y: 2, Z: 2}, 5, SphereSetUnion)
	eps := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 120))
	p := [3]*big.Rat{new(big.Rat).Add(r(1, 1), eps), r(1, 1), r(1, 1)}
	g, e := s.NearestBoundaryRational(ctx, p, SphereFullBoundaryDistanceOptions{})
	if e != nil || g.Kind != SphereNearestTransverseIsolated {
		t.Fatal("exact axis dispatch", g, e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	v, e := s.NearestBoundaryRational(cancelled, p, SphereFullBoundaryDistanceOptions{})
	if !errors.Is(e, context.Canceled) || !reflect.DeepEqual(v, SpherePairNearest{}) {
		t.Fatal(e)
	}
}
