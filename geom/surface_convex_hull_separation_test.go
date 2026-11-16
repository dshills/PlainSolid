package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestSurfaceConvexHullSeparatesCurvedWeightedSupports(t *testing.T) {
	ctx := context.Background()
	a := curvedDiamondPatch(t, new(big.Rat), false)
	s := big.NewRat(5, 4)
	b := curvedDiamondPatch(t, s, true)
	old, err := SeparateSurfacePatches(a, b, SurfaceSeparationOptions{})
	if err != nil || old.Separated {
		t.Fatal("fixture must overlap on the coordinate and center-normal projections", old, err)
	}
	cut, err := SeparateSurfacePatchesConvexHull(ctx, a, b, SurfaceConvexHullSeparationOptions{})
	if err != nil || !cut.Separated || cut.Iterations < 2 {
		t.Fatal("exact support search did not separate the two original curved charts", cut, err)
	}
	// Each horizontal control hull projects as the unit diamond shifted
	// by (s,s). Independently project all nine poles on each support,
	// including the unique rational weighted curved pole above the plane.
	var maxA, minB *big.Rat
	for i := int64(0); i <= 2; i++ {
		for j := int64(0); j <= 2; j++ {
			z := new(big.Rat)
			if i == 1 && j == 1 {
				z.SetFrac64(1, 8)
			}
			x, y := big.NewRat(i-j, 2), big.NewRat(i+j-2, 2)
			pa := new(big.Rat).Add(new(big.Rat).Mul(x, cut.Direction[0]), new(big.Rat).Mul(y, cut.Direction[1]))
			pa.Add(pa, new(big.Rat).Mul(z, cut.Direction[2]))
			pb := new(big.Rat).Add(pa, new(big.Rat).Mul(s, new(big.Rat).Add(cut.Direction[0], cut.Direction[1])))
			if maxA == nil || pa.Cmp(maxA) > 0 {
				maxA = pa
			}
			if minB == nil || pb.Cmp(minB) < 0 {
				minB = pb
			}
		}
	}
	if want := new(big.Rat).Sub(minB, maxA); want.Sign() <= 0 || cut.Gap.Cmp(want) != 0 {
		t.Fatal("separator does not match independent original pole projections", cut.Gap, want)
	}
	oldCover, err := BoundSurfaceSurface(a, b, SurfaceSurfaceIsolationOptions{SeparatingProjections: true, ParameterTolerance: 1})
	if err != nil || !oldCover.Complete || len(oldCover.Candidates) != 1 {
		t.Fatal("legacy isolation silently lost the candidate", oldCover, err)
	}
	newCover, err := BoundSurfaceSurfaceContext(ctx, a, b, SurfaceSurfaceIsolationOptions{SeparatingProjections: true,
		ConvexHullIterations: 16, ParameterTolerance: 1})
	if err != nil || !newCover.Complete || newCover.ConvexHullExclusions != 1 || len(newCover.Candidates) != 0 || newCover.VisitedPairs != 1 {
		t.Fatal("full-domain isolation did not use the new certificate", newCover, err)
	}
	one, err := SeparateSurfacePatchesConvexHull(ctx, a, b, SurfaceConvexHullSeparationOptions{MaxIterations: 1})
	if err != nil || one.Separated || one.Gap != nil {
		t.Fatal("one iteration made an unsupported conclusion", one, err)
	}
	short, err := SeparateSurfacePatchesConvexHull(ctx, a, b, SurfaceConvexHullSeparationOptions{MaxOperations: cut.ExactOperations - 1})
	if !errors.Is(err, ErrConvergence) || short.Separated || short.Gap != nil {
		t.Fatal("exhausted work published a partial certificate", short, err)
	}
}

func TestSurfaceConvexHullPreservesTouchesAndExactSmallGaps(t *testing.T) {
	ctx := context.Background()
	a := curvedDiamondPatch(t, new(big.Rat), false)
	for _, offset := range []*big.Rat{big.NewRat(9, 10), big.NewRat(1, 1)} {
		b := curvedDiamondPatch(t, offset, true)
		result, err := BoundSurfaceSurface(a, b, SurfaceSurfaceIsolationOptions{ConvexHullIterations: 16, ParameterTolerance: 1})
		if err != nil || !result.Complete || result.ConvexHullExclusions != 0 || len(result.Candidates) != 1 {
			t.Fatal("overlapping or touching control hull was excluded", offset, result, err)
		}
	}
	gap := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))
	b := curvedDiamondPatch(t, new(big.Rat).Add(big.NewRat(1, 1), gap), true)
	p, err := SeparateSurfacePatchesConvexHull(ctx, b, a, SurfaceConvexHullSeparationOptions{})
	if err != nil || !p.Separated || p.Gap.Sign() <= 0 {
		t.Fatal("strict gap smaller than a double-precision ULP was lost", p, err)
	}
	pending, err := BoundSurfaceSurface(a, b, SurfaceSurfaceIsolationOptions{ConvexHullIterations: 16, MaxOperations: 36})
	if !errors.Is(err, ErrConvergence) || pending.Complete || len(pending.Pending) != 1 ||
		pending.Pending[0].DomainA != a.Domain() || pending.Pending[0].DomainB != b.Domain() {
		t.Fatal("work loss dropped an untested original pair", pending, err)
	}
}

func TestSurfaceConvexHullDoesNotExcludeVolumetricOverlap(t *testing.T) {
	ctx := context.Background()
	makePatch := func(poles [4][3]int64, denominator int64) *SurfacePatch {
		t.Helper()
		h := make([][4]*big.Rat, len(poles))
		for i, p := range poles {
			h[i] = [4]*big.Rat{big.NewRat(p[0], denominator), big.NewRat(p[1], denominator),
				big.NewRat(p[2], denominator), big.NewRat(1, 1)}
		}
		patch, err := NewExactHomogeneousBezierSurface(ctx, 1, 1, h, SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		return patch
	}
	a := makePatch([4][3]int64{{-1, -1, -1}, {3, -1, -1}, {-1, 3, -1}, {-1, -1, 3}}, 1)
	// Every pole of B is strictly inside the tetrahedral hull of A:
	// all coordinates exceed -1 and their sum is less than 1.
	b := makePatch([4][3]int64{{1, 3, 2}, {2, 3, 1}, {1, 4, 2}, {2, 3, 2}}, 20)
	for _, order := range [][2]*SurfacePatch{{a, b}, {b, a}} {
		got, err := SeparateSurfacePatchesConvexHull(ctx, order[0], order[1], SurfaceConvexHullSeparationOptions{})
		if err != nil || got.Separated || got.Gap != nil || got.Iterations < 4 || got.Iterations == 32 {
			t.Fatal("full-dimensional hull containment was misclassified", got, err)
		}
	}
}

func curvedDiamondPatch(t *testing.T, offset *big.Rat, reverse bool) *SurfacePatch {
	t.Helper()
	var controls [][4]*big.Rat
	for row := int64(0); row <= 2; row++ {
		for col := int64(0); col <= 2; col++ {
			i, j := row, col
			if reverse {
				i, j = 2-i, 2-j
			}
			x := new(big.Rat).Add(offset, big.NewRat(i-j, 2))
			y := new(big.Rat).Add(offset, big.NewRat(i+j-2, 2))
			z, w := new(big.Rat), big.NewRat(1, 1)
			if i == 1 && j == 1 {
				z, w = big.NewRat(1, 8), big.NewRat(3, 2)
			}
			controls = append(controls, [4]*big.Rat{x.Mul(x, w), y.Mul(y, w), z.Mul(z, w), w})
		}
	}
	p, err := NewExactHomogeneousBezierSurface(context.Background(), 2, 2, controls, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return p
}
