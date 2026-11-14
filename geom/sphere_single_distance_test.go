package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"reflect"
	"testing"
)

func TestSphereSingleNearestReductions(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	for _, tc := range []struct {
		a, b     Vec3
		ra, rb   float64
		op       SphereSetOperation
		p        [3]*big.Rat
		owners   BoreBoundaryMask
		distance int64
		family   bool
	}{
		{Vec3{}, Vec3{X: 1}, 5, 1, SphereSetUnion, [3]*big.Rat{r(0, 1), r(0, 1), r(0, 1)}, 1, 25, true},
		{Vec3{}, Vec3{X: 4}, 5, 1, SphereSetUnion, [3]*big.Rat{r(5, 1), r(0, 1), r(0, 1)}, 1, 0, false},
		{Vec3{}, Vec3{X: 4}, 5, 1, SphereSetIntersection, [3]*big.Rat{r(5, 1), r(0, 1), r(0, 1)}, 2, 0, false},
		{Vec3{X: 4}, Vec3{}, 1, 5, SphereSetUnion, [3]*big.Rat{r(5, 1), r(0, 1), r(0, 1)}, 2, 0, false},
		{Vec3{X: 4}, Vec3{}, 1, 5, SphereSetIntersection, [3]*big.Rat{r(5, 1), r(0, 1), r(0, 1)}, 1, 0, false},
		{Vec3{}, Vec3{}, 5, 5, SphereSetUnion, [3]*big.Rat{r(0, 1), r(0, 1), r(0, 1)}, 3, 25, true},
		{Vec3{}, Vec3{}, 5, 5, SphereSetIntersection, [3]*big.Rat{r(5, 1), r(0, 1), r(0, 1)}, 3, 0, false},
		{Vec3{}, Vec3{X: 4}, 2, 2, SphereSetDifference, [3]*big.Rat{r(2, 1), r(0, 1), r(0, 1)}, 1, 0, false},
		{Vec3{}, Vec3{X: 5}, 2, 1, SphereSetDifference, [3]*big.Rat{r(0, 1), r(0, 1), r(0, 1)}, 1, 4, true},
	} {
		s, _ := NewSpherePairMaterial(tc.a, tc.ra, tc.b, tc.rb, tc.op)
		g, e := s.NearestBoundaryRational(ctx, tc.p, SphereFullBoundaryDistanceOptions{})
		if e != nil || g.Kind != SphereNearestSingle || g.Single == nil || g.Owners != tc.owners || g.Lower.Cmp(r(tc.distance, 1)) != 0 || g.Upper.Cmp(g.Lower) != 0 {
			t.Fatal(tc, g, e)
		}
		w := g.Single.Witness
		if w.Owner != tc.owners || w.WholeSphere != tc.family || w.MaterialNormalSign != 1 || g.Single.ExcludedOwners != 3&^tc.owners || g.Single.Operations != g.Operations {
			t.Fatal("complete reduced owner/family", g)
		}
		if tc.distance == 0 {
			p := Vec3{}
			p.X, _ = tc.p[0].Float64()
			p.Y, _ = tc.p[1].Float64()
			p.Z, _ = tc.p[2].Float64()
			loc, owners, e := s.Classify(p)
			if e != nil || loc != BoreBoundary || owners != tc.owners {
				t.Fatal("existing regularized ownership", loc, owners, e)
			}
		}
		for _, n := range []int{g.Operations, g.Operations - 1} {
			v, e := s.NearestBoundaryRational(ctx, tc.p, SphereFullBoundaryDistanceOptions{Arithmetic: SurfacePatchOptions{MaxOperations: n}})
			if n == g.Operations {
				if e != nil {
					t.Fatal(e)
				}
			} else if e == nil || !reflect.DeepEqual(v, SpherePairNearest{}) {
				t.Fatal("single aggregate budget", e)
			}
		}
		g.Single.Witness.Center[0].SetInt64(999)
		again, e := s.NearestBoundaryRational(ctx, tc.p, SphereFullBoundaryDistanceOptions{})
		if e != nil || again.Single.Witness.Center[0].Cmp(r(999, 1)) == 0 {
			t.Fatal("immutable original source")
		}
	}
	for _, op := range []SphereSetOperation{SphereSetUnion, SphereSetIntersection} {
		s, _ := NewSpherePairMaterial(Vec3{X: 2, Y: 3, Z: 4}, 5, Vec3{X: 2, Y: 3, Z: 4}, 5, op)
		g, e := s.NearestBoundaryRational(ctx, [3]*big.Rat{r(3, 1), r(4, 1), r(4, 1)}, SphereFullBoundaryDistanceOptions{})
		truth := 27 - 10*math.Sqrt(2)
		if e != nil || g.Owners != 3 {
			t.Fatal(g, e)
		}
		lo, _ := g.Lower.Float64()
		hi, _ := g.Upper.Float64()
		if truth < lo-1e-13 || truth > hi+1e-13 {
			t.Fatal("coincident irrational foot")
		}
	}
	// Exact internal-tangent removal is not a nearest support owner for Union.
	for _, x := range []float64{math.Nextafter(4, 0), 4, math.Nextafter(4, 5)} {
		s, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: x}, 1, SphereSetUnion)
		g, e := s.NearestBoundaryRational(ctx, [3]*big.Rat{r(5, 1), r(0, 1), r(0, 1)}, SphereFullBoundaryDistanceOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if x <= 4 && g.Kind != SphereNearestSingle || x > 4 && g.Kind != SphereNearestTransverseAxis {
			t.Fatal("exact containment transition", x, g.Kind)
		}
	}
	for _, tc := range []struct {
		b      Vec3
		ra, rb float64
		op     SphereSetOperation
	}{{Vec3{}, 5, 5, SphereSetDifference}, {Vec3{X: 4}, 2, 2, SphereSetIntersection}, {Vec3{}, 1, 5, SphereSetDifference}} {
		s, _ := NewSpherePairMaterial(Vec3{}, tc.ra, tc.b, tc.rb, tc.op)
		v, e := s.NearestBoundaryRational(ctx, [3]*big.Rat{r(0, 1), r(0, 1), r(0, 1)}, SphereFullBoundaryDistanceOptions{})
		if !errors.Is(e, ErrDomain) || !reflect.DeepEqual(v, SpherePairNearest{}) {
			t.Fatal("empty regularized material", tc, v, e)
		}
	}
}
