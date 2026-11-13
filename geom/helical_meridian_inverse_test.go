package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestExactHelicalMeridianInverse(t *testing.T) {
	ctx := context.Background()
	f, _ := NewFrame(Vec3{2, 3, 4}, Vec3{0, 1, 0}, Vec3{1, 0, 0})
	for _, hand := range []HelixHandedness{RightHanded, LeftHanded} {
		s, e := NewHelicalProfileSurface(f, Vec2{2, 0}, Vec2{3, 2}, 4, hand, -2, 2)
		if e != nil {
			t.Fatal(e)
		}
		// t=1/4, v=1/3: world=(3+2/3,3,4+h*(2+1/3)).
		point := [3]*big.Rat{big.NewRat(11, 3), big.NewRat(3, 1), new(big.Rat).Add(big.NewRat(4, 1), big.NewRat(7*int64(hand), 3))}
		got, e := s.InvertQuarterTurnPoint(ctx, big.NewRat(1, 4), point, SurfacePatchEvaluationOptions{})
		if e != nil || !got.OnSurface || !got.OnMeridian || got.ExcludedAtTurn || got.ProfileParameter.Cmp(big.NewRat(1, 3)) != 0 {
			t.Fatal(got, e)
		}

		for _, v := range []int64{0, 1} {
			endpoint := [3]*big.Rat{big.NewRat(3+2*v, 1), big.NewRat(3, 1), big.NewRat(4+(2+v)*int64(hand), 1)}
			r, e := s.InvertQuarterTurnPoint(ctx, big.NewRat(1, 4), endpoint, SurfacePatchEvaluationOptions{})
			if e != nil || !r.OnSurface || r.Boundary != int(2*v-1) {
				t.Fatal(r, e)
			}
		}
		// Exact tiny off-meridian displacement must never be swallowed by tolerance.
		off := point
		off[1] = new(big.Rat).Add(point[1], new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 1100)))
		bad, e := s.InvertQuarterTurnPoint(ctx, big.NewRat(1, 4), off, SurfacePatchEvaluationOptions{})
		if e != nil || !bad.ExcludedAtTurn || bad.OnSurface {
			t.Fatal(bad, e)
		}
		if _, e := s.InvertQuarterTurnPoint(ctx, big.NewRat(1, 3), point, SurfacePatchEvaluationOptions{}); !errors.Is(e, ErrConvergence) {
			t.Fatal(e)
		}
		if _, e := s.InvertQuarterTurnPoint(ctx, big.NewRat(1, 4), point, SurfacePatchEvaluationOptions{MaxOperations: 1}); !errors.Is(e, ErrConvergence) {
			t.Fatal(e)
		}
		// t=-1/4,v=2 is on the infinite meridian but outside this bounded flank.
		outside := [3]*big.Rat{big.NewRat(5, 1), big.NewRat(3, 1), big.NewRat(4-4*int64(hand), 1)}
		r, e := s.InvertQuarterTurnPoint(ctx, big.NewRat(-1, 4), outside, SurfacePatchEvaluationOptions{})
		if e != nil || !r.OnMeridian || r.OnSurface || !r.ExcludedAtTurn || r.ProfileParameter.Cmp(big.NewRat(2, 1)) != 0 {
			t.Fatal(r, e)
		}
	}
}
