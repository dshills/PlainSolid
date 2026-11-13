package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestExactCardinalAxisLineRepeatedTurns(t *testing.T) {
	ctx := context.Background()
	f, _ := NewFrame(Vec3{}, Vec3{1, 0, 0}, Vec3{0, 0, 1})
	for _, hand := range []HelixHandedness{RightHanded, LeftHanded} {
		s, _ := NewHelicalProfileSurface(f, Vec2{2, 0}, Vec2{3, 1}, 2, hand, -1, 2)
		origin := [3]*big.Rat{big.NewRat(0, 1), big.NewRat(5*int64(hand), 2), big.NewRat(10, 1)}
		r, e := s.IntersectCardinalAxisLine(ctx, big.NewRat(9, 4), origin, big.NewRat(-2, 1), HelicalAxisLineOptions{})
		if e != nil || !r.Complete || len(r.Hits) != 3 || r.CandidateTurns != 3 {
			t.Fatal(r, e)
		}
		for i, hit := range r.Hits {
			if hit.Turn.Cmp(big.NewRat(int64(-3+4*i), 4)) != 0 || hit.ProfileParameter.Cmp(big.NewRat(1, 2)) != 0 || hit.LineParameter.Cmp(big.NewRat(int64(11-2*i), 2)) != 0 {
				t.Fatal(hit)
			}
			proof, e := s.InvertQuarterTurnPoint(ctx, hit.Turn, hit.Point, SurfacePatchEvaluationOptions{})
			if e != nil || !proof.OnSurface || proof.ProfileParameter.Cmp(hit.ProfileParameter) != 0 {
				t.Fatal(proof, e)
			}
		}

		closed, _ := NewHelicalProfileSurface(f, Vec2{2, 0}, Vec2{3, 1}, 2, hand, -.75, 1.25)
		endpoints, e := closed.IntersectCardinalAxisLine(ctx, big.NewRat(-3, 4), origin, big.NewRat(-2, 1), HelicalAxisLineOptions{})
		if e != nil || !endpoints.Complete || len(endpoints.Hits) != 3 {
			t.Fatal("closed turn ends", endpoints, e)
		}
		if bad, e := s.IntersectCardinalAxisLine(ctx, big.NewRat(1, 4), origin, big.NewRat(1, 1), HelicalAxisLineOptions{MaxOperations: 1}); !errors.Is(e, ErrConvergence) || bad.Complete {
			t.Fatal(bad, e)
		}
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		if _, e := s.IntersectCardinalAxisLine(cancelled, big.NewRat(1, 4), origin, big.NewRat(1, 1), HelicalAxisLineOptions{}); !errors.Is(e, context.Canceled) {
			t.Fatal(e)
		}
		clip := [2]*big.Rat{big.NewRat(4, 1), big.NewRat(5, 1)}
		r, e = s.IntersectCardinalAxisLine(ctx, big.NewRat(1, 4), origin, big.NewRat(-2, 1), HelicalAxisLineOptions{LineRange: &clip})
		if e != nil || !r.Complete || len(r.Hits) != 1 || r.Hits[0].Turn.Cmp(big.NewRat(1, 4)) != 0 || r.CandidateTurns != 3 {
			t.Fatal(r, e)
		}
		if bad, e := s.IntersectCardinalAxisLine(ctx, big.NewRat(1, 4), origin, big.NewRat(1, 1), HelicalAxisLineOptions{MaxContacts: 2}); !errors.Is(e, ErrConvergence) || bad.Complete || len(bad.Hits) != 0 {
			t.Fatal(bad, e)
		}
		off := origin
		off[0] = new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 1100))
		if bad, e := s.IntersectCardinalAxisLine(ctx, big.NewRat(1, 4), off, big.NewRat(1, 1), HelicalAxisLineOptions{}); e == nil || bad.Complete {
			t.Fatal("wrong phase ray", bad, e)
		}
		axis := [3]*big.Rat{big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(3, 1)}
		r, e = s.IntersectCardinalAxisLine(ctx, big.NewRat(1, 4), axis, big.NewRat(1, 1), HelicalAxisLineOptions{})
		if e != nil || !r.Complete || len(r.Hits) != 0 {
			t.Fatal(r, e)
		}
		cylinder, _ := NewHelicalProfileSurface(f, Vec2{2.5, 0}, Vec2{2.5, 1}, 2, hand, -1, 2)
		if bad, e := cylinder.IntersectCardinalAxisLine(ctx, big.NewRat(1, 4), origin, big.NewRat(1, 1), HelicalAxisLineOptions{}); !errors.Is(e, ErrConvergence) || bad.Complete {
			t.Fatal("coincidence", bad, e)
		}
	}
}
