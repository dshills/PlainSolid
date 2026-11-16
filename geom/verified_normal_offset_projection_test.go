package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestVerifiedNormalOffsetDirectionalBoundsPreserveObliqueSmallGap(t *testing.T) {
	ctx := context.Background()
	a := offsetSeparationTiltedPlane(t, new(big.Rat))
	gap := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 80))
	b := offsetSeparationTiltedPlane(t, gap)
	n := [3]*big.Rat{big.NewRat(-1, 2), new(big.Rat), big.NewRat(1, 1)}
	// Both analytic planes have constant z-x/2. Equal normal offsets
	// preserve their original 2^-80 separation in this direction.
	x, err := a.ProjectionBoundsRational(ctx, a.Domain(), n, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	y, err := b.ProjectionBoundsRational(ctx, b.Domain(), n, SurfacePatchOptions{})
	if err != nil || x.Upper.Cmp(y.Lower) >= 0 ||
		new(big.Rat).Sub(y.Lower, x.Lower).Cmp(gap) != 0 || new(big.Rat).Sub(y.Upper, x.Upper).Cmp(gap) != 0 {
		t.Fatal("independent oblique projections did not retain the exact source gap", x, y, err)
	}
	reverse := [3]*big.Rat{new(big.Rat).Neg(n[0]), new(big.Rat), new(big.Rat).Neg(n[2])}
	neg, err := a.ProjectionBoundsRational(ctx, a.Domain(), reverse, SurfacePatchOptions{})
	if err != nil || neg.Lower.Cmp(new(big.Rat).Neg(x.Upper)) != 0 || neg.Upper.Cmp(new(big.Rat).Neg(x.Lower)) != 0 {
		t.Fatal("reversing the direction did not reverse the interval", neg, err)
	}
	limited, err := a.ProjectionBoundsRational(ctx, a.Domain(), n, SurfacePatchOptions{MaxOperations: x.ExactOperations - 1})
	if !errors.Is(err, ErrConvergence) || limited.Lower != nil || limited.Upper != nil {
		t.Fatal("short work exposed an uncertified projection", limited, err)
	}
	naive, err := CertifyVerifiedNormalOffsetSeparation(ctx, a, b, VerifiedNormalOffsetSeparationOptions{MaxPairs: 1})
	if !errors.Is(err, ErrConvergence) || naive.Complete || len(naive.Unresolved) == 0 {
		t.Fatal("one world-aligned box must leave the sub-ULP pair unresolved", naive, err)
	}
	separated, err := CertifyVerifiedNormalOffsetSeparation(ctx, a, b, VerifiedNormalOffsetSeparationOptions{
		MaxPairs: 1, Directions: [][3]*big.Rat{n},
	})
	if err != nil || !separated.Complete || separated.VisitedPairs != 1 || len(separated.Unresolved) != 0 ||
		separated.ProjectedGap.Sign() <= 0 || separated.ProjectedGap.Cmp(gap) > 0 {
		t.Fatal("one exact oblique projection should exclude the original pair", separated, err)
	}
	separated.ProjectionDirection[0].SetInt64(9)
	if n[0].Cmp(big.NewRat(-1, 2)) != 0 {
		t.Fatal("projection certificate aliases the input direction")
	}
}
