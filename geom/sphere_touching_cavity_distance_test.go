package geom

import (
	"context"
	"errors"
	"math/big"
	"reflect"
	"testing"
)

func TestSphereTouchingCavityNearest(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	s, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 4}, 1, SphereSetDifference)
	for _, tc := range []struct {
		p                          [3]*big.Rat
		distance                   *big.Rat
		owners                     BoreBoundaryMask
		contact, same, innerFamily bool
	}{
		{[3]*big.Rat{r(9, 2), r(0, 1), r(0, 1)}, r(1, 4), 3, true, true, false},
		{[3]*big.Rat{r(5, 1), r(0, 1), r(0, 1)}, r(0, 1), 3, true, true, false},
		{[3]*big.Rat{r(6, 1), r(0, 1), r(0, 1)}, r(1, 1), 3, true, true, false},
		{[3]*big.Rat{r(4, 1), r(0, 1), r(0, 1)}, r(1, 1), 3, true, false, true},
		{[3]*big.Rat{r(20, 7), r(15, 7), r(0, 1)}, r(100, 49), 3, false, false, false},
		{[3]*big.Rat{r(4, 1), r(1, 1), r(0, 1)}, r(0, 1), 2, false, false, false},
		{[3]*big.Rat{r(0, 1), r(0, 1), r(0, 1)}, r(9, 1), 2, false, false, false},
	} {
		g, e := s.NearestTouchingCavityBoundaryRational(ctx, tc.p, SphereFullBoundaryDistanceOptions{})
		if e != nil || g.Boundary.Owners != tc.owners || g.Boundary.Lower.Cmp(tc.distance) != 0 || g.Boundary.Upper.Cmp(tc.distance) != 0 || g.ContactNearest != tc.contact || g.CoincidentSingletons != tc.same || g.Boundary.Candidates[1].WholeSphere != tc.innerFamily {
			t.Fatal(tc, g, e)
		}
		if g.ContactOwners != 3 || g.Contact[0].Cmp(r(5, 1)) != 0 || g.Contact[1].Sign() != 0 || g.Contact[2].Sign() != 0 || g.Boundary.Candidates[1].MaterialNormalSign != -1 || g.Operations != g.Boundary.Operations {
			t.Fatal("shared exact contact/material", g)
		}
		if tc.same {
			for _, w := range g.Boundary.Candidates {
				for j := 0; j < 3; j++ {
					point := new(big.Rat).Add(w.Center[j], new(big.Rat).Quo(w.Numerator[j], w.Root[0]))
					if point.Cmp(g.Contact[j]) != 0 {
						t.Fatal("proved shared foot")
					}
				}
			}
		}
		if tc.p[0].Cmp(r(20, 7)) == 0 {
			for i, w := range g.Boundary.Candidates {
				want := [2][2]*big.Rat{{r(4, 1), r(3, 1)}, {r(60, 17), r(15, 17)}}
				for j := 0; j < 2; j++ {
					point := new(big.Rat).Add(w.Center[j], new(big.Rat).Quo(w.Numerator[j], w.Root[0]))
					if point.Cmp(want[i][j]) != 0 {
						t.Fatal("independent distinct tied feet")
					}
				}
			}
		}
		loc, owners, e := s.Classify(Vec3{X: 5})
		if e != nil || loc != BoreBoundary || owners != 3 {
			t.Fatal("established singular material contact")
		}
		unified, e := s.NearestBoundaryRational(ctx, tc.p, SphereFullBoundaryDistanceOptions{})
		if e != nil || unified.Kind != SphereNearestTouchingCavity || unified.Owners != tc.owners || unified.Touching == nil {
			t.Fatal("unified tangent proof", unified, e)
		}
		for _, n := range []int{unified.Operations, unified.Operations - 1} {
			v, e := s.NearestBoundaryRational(ctx, tc.p, SphereFullBoundaryDistanceOptions{Arithmetic: SurfacePatchOptions{MaxOperations: n}})
			if n == unified.Operations {
				if e != nil {
					t.Fatal(e)
				}
			} else if e == nil || !reflect.DeepEqual(v, SpherePairNearest{}) {
				t.Fatal("aggregate contact work", e)
			}
		}
		if _, e := s.NearestCavityBoundaryRational(ctx, tc.p, SphereCavityDistanceOptions{}); !errors.Is(e, ErrDomain) {
			t.Fatal("strict API changed")
		}
		g.Contact[0].SetInt64(999)
		again, e := s.NearestTouchingCavityBoundaryRational(ctx, tc.p, SphereFullBoundaryDistanceOptions{})
		if e != nil || again.Contact[0].Cmp(r(5, 1)) != 0 {
			t.Fatal("owned contact")
		}
	}
	// Oblique represented source axis; exact contact has nonbinary coordinates.
	oblique, _ := NewSpherePairMaterial(Vec3{X: 2, Y: -1, Z: 3}, 6, Vec3{X: 5, Y: 3, Z: 3}, 1, SphereSetDifference)
	p := [3]*big.Rat{r(53, 10), r(17, 5), r(3, 1)}
	g, e := oblique.NearestTouchingCavityBoundaryRational(ctx, p, SphereFullBoundaryDistanceOptions{})
	if e != nil || !g.CoincidentSingletons || g.Boundary.Lower.Cmp(r(1, 4)) != 0 || g.Contact[0].Cmp(r(28, 5)) != 0 || g.Contact[1].Cmp(r(19, 5)) != 0 {
		t.Fatal("oblique nonbinary contact", g, e)
	}
	eps := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 120))
	p = [3]*big.Rat{new(big.Rat).Add(r(28, 5), new(big.Rat).Mul(r(4, 1), eps)), new(big.Rat).Sub(r(19, 5), new(big.Rat).Mul(r(3, 1), eps)), r(3, 1)}
	near, e := oblique.NearestTouchingCavityBoundaryRational(ctx, p, SphereFullBoundaryDistanceOptions{SqrtBits: 8})
	if e != nil || near.Boundary.Owners != 1 || near.ContactNearest {
		t.Fatal("subfloat unequal curvature order", near, e)
	}
	strict, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 3}, 1, SphereSetDifference)
	v, e := strict.NearestTouchingCavityBoundaryRational(ctx, p, SphereFullBoundaryDistanceOptions{})
	if !errors.Is(e, ErrDomain) || v.Boundary.Lower != nil {
		t.Fatal("tangent-only gate")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	v, e = s.NearestTouchingCavityBoundaryRational(cancelled, p, SphereFullBoundaryDistanceOptions{})
	if !errors.Is(e, context.Canceled) || v.Boundary.Lower != nil {
		t.Fatal(e)
	}
}
