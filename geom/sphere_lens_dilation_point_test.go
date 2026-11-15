package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
)

func TestClassifyDilatedLensExactCapsRimAndJoins(t *testing.T) {
	ctx := context.Background()
	s, err := NewSpherePairMaterial(Vec3{}, 5, Vec3{Z: 6}, 5, SphereSetIntersection)
	if err != nil {
		t.Fatal(err)
	}
	q := func(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }
	p := func(x, z float64) [3]*big.Rat { return [3]*big.Rat{q(x), q(0), q(z)} }
	const both = SpherePairFirst | SpherePairSecond
	for _, tc := range []struct {
		p      [3]*big.Rat
		d      *big.Rat
		want   BorePointLocation
		owners BoreBoundaryMask
	}{
		{p(4, 3), q(0), BoreBoundary, both},
		{p(4, 3), q(.5), BoreInside, 0},
		{p(4.5, 3), q(.5), BoreBoundary, both},
		{p(math.Nextafter(4.5, 0), 3), q(.5), BoreInside, 0},
		{p(math.Nextafter(4.5, 5), 3), q(.5), BoreOutside, 0},
		{p(0, .5), q(.5), BoreBoundary, SpherePairSecond},
		{p(0, 5.5), q(.5), BoreBoundary, SpherePairFirst},
		{p(0, 6), q(.5), BoreOutside, 0},
		{[3]*big.Rat{big.NewRat(22, 5), q(0), big.NewRat(33, 10)}, q(.5), BoreBoundary, both},
		{p(16, 3), q(12), BoreBoundary, both},
	} {
		r, err := s.ClassifyDilatedLensRational(ctx, tc.p, tc.d, SphereFullBoundaryDistanceOptions{SqrtBits: 8})
		if err != nil || r.Location != tc.want || r.Owners != tc.owners || r.SquaredDistance[0] == nil || r.Operations <= 0 {
			t.Fatalf("%v d=%v: %+v, %v, want %v/%v", tc.p, tc.d, r, err, tc.want, tc.owners)
		}
		if tc.want == BoreBoundary {
			d2 := new(big.Rat).Mul(tc.d, tc.d)
			if r.SquaredDistance[0].Cmp(d2) > 0 || r.SquaredDistance[1].Cmp(d2) < 0 {
				t.Fatalf("distance bracket %+v does not enclose %v", r.SquaredDistance, d2)
			}
		}
	}
}

func TestClassifyDilatedLensIrrationalObliqueRimAndLimits(t *testing.T) {
	ctx := context.Background()
	s, err := NewSpherePairMaterial(Vec3{}, 2, Vec3{X: 2, Y: 2}, 2, SphereSetIntersection)
	if err != nil {
		t.Fatal(err)
	}
	// Coplanar with the circle, perpendicular to its oblique centerline. The
	// nearest distance is 2−sqrt(2), and neither cap's full radial foot is kept.
	p := [3]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 1), big.NewRat(2, 1)}
	for _, tc := range []struct {
		threshold *big.Rat
		location  BorePointLocation
	}{{big.NewRat(1, 2), BoreOutside}, {big.NewRat(3, 4), BoreInside}} {
		r, err := s.ClassifyDilatedLensRational(ctx, p, tc.threshold, SphereFullBoundaryDistanceOptions{})
		if err != nil || r.Location != tc.location || r.Owners != 0 {
			t.Fatalf("%v: %+v %v", tc.threshold, r, err)
		}
		if lo, _ := r.SquaredDistance[0].Float64(); math.Abs(lo-math.Pow(2-math.Sqrt2, 2)) > 1e-14 {
			t.Fatalf("distance bracket %+v", r.SquaredDistance)
		}
	}
	for _, tc := range []struct {
		threshold *big.Rat
		o         SphereFullBoundaryDistanceOptions
		want      error
	}{{big.NewRat(-1, 2), SphereFullBoundaryDistanceOptions{}, ErrInvalid}, {big.NewRat(1, 2), SphereFullBoundaryDistanceOptions{Arithmetic: SurfacePatchOptions{MaxOperations: 4}}, ErrConvergence}} {
		r, err := s.ClassifyDilatedLensRational(ctx, p, tc.threshold, tc.o)
		if !errors.Is(err, tc.want) || r != (SphereLensDilationPoint{}) {
			t.Fatalf("unfinished classification admitted: %+v %v", r, err)
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if r, err := s.ClassifyDilatedLensRational(ctx, p, big.NewRat(1, 2), SphereFullBoundaryDistanceOptions{}); !errors.Is(err, context.Canceled) || r != (SphereLensDilationPoint{}) {
		t.Fatalf("canceled classification: %+v %v", r, err)
	}
}
