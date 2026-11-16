package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestVerifiedNormalOffsetSeparationExcludesOverlappingInitialBoxes(t *testing.T) {
	ctx := context.Background()
	a := offsetSeparationTiltedPlane(t, new(big.Rat))
	b := offsetSeparationTiltedPlane(t, big.NewRat(3, 4))
	aa, err := a.BoundsRational(ctx, a.Domain(), SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	bb, err := b.BoundsRational(ctx, b.Domain(), SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, overlap := aa.Bounds.Intersect(bb.Bounds); !overlap {
		t.Fatal("fixture should need subdivision")
	}
	// Both sources satisfy z=x/2+c. Their equal signed offsets translate
	// by the same normal vector, so the different values of c remain.
	for _, order := range [][2]*VerifiedNormalOffsetPatch{{a, b}, {b, a}} {
		got, err := CertifyVerifiedNormalOffsetSeparation(ctx, order[0], order[1], VerifiedNormalOffsetSeparationOptions{})
		if err != nil || !got.Complete || len(got.Unresolved) != 0 || got.ExcludedPairs <= 1 || got.VisitedPairs <= 1 {
			t.Fatal("independently disjoint offset images should be provable despite overlapping original boxes", got, err)
		}
	}
}

func TestVerifiedNormalOffsetSeparationPreservesContactAndPendingCover(t *testing.T) {
	ctx := context.Background()
	a := offsetSeparationTiltedPlane(t, new(big.Rat))
	for _, o := range []VerifiedNormalOffsetSeparationOptions{
		{MaxPairs: 1}, {MaxDepth: 2}, {Patch: SurfacePatchOptions{MaxOperations: 10}},
	} {
		got, err := CertifyVerifiedNormalOffsetSeparation(ctx, a, a, o)
		if !errors.Is(err, ErrConvergence) || got.Complete || len(got.Unresolved) == 0 {
			t.Fatal("identical geometry cannot be excluded", got, err)
		}
		for _, x := range []float64{.125, .5, .875} {
			for _, y := range []float64{.125, .5, .875} {
				covered := false
				for _, c := range got.Unresolved {
					covered = covered || pairCovers(SurfaceSurfaceCell{DomainA: c.A, DomainB: c.B}, x, y, x, y)
				}
				if !covered {
					t.Fatal("lost a known point in the coincident contact cover", x, y)
				}
			}
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	got, err := CertifyVerifiedNormalOffsetSeparation(canceled, a, a, VerifiedNormalOffsetSeparationOptions{})
	if !errors.Is(err, context.Canceled) || got.Complete || len(got.Unresolved) != 1 ||
		got.Unresolved[0].A != a.Domain() || got.Unresolved[0].B != a.Domain() {
		t.Fatal("cancellation lost the original possible pair", got, err)
	}
}

func offsetSeparationTiltedPlane(t *testing.T, c *big.Rat) *VerifiedNormalOffsetPatch {
	t.Helper()
	var net [][4]*big.Rat
	for _, u := range []int64{0, 1} {
		for _, v := range []int64{0, 1} {
			net = append(net, [4]*big.Rat{big.NewRat(2*u, 1), big.NewRat(v, 1),
				new(big.Rat).Add(big.NewRat(u, 1), c), big.NewRat(1, 1)})
		}
	}
	ctx := context.Background()
	source, err := NewExactHomogeneousBezierSurface(ctx, 1, 1, net, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	got, _, err := NewVerifiedNormalOffsetPatch(ctx, source, big.NewRat(1, 7),
		NormalOffsetProjection{Axes: [2]int{0, 1}}, NormalOffsetCoverageOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return got
}
