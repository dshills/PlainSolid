package geom

import (
	"context"
	"math"
	"math/big"
	"testing"
)

func TestHelicalRationalRayAllQuadrants(t *testing.T) {
	ctx := context.Background()
	f, _ := NewFrame(Vec3{1, 2, 3}, Vec3{0, 1, 0}, Vec3{1, 0, 0})
	for _, hand := range []HelixHandedness{RightHanded, LeftHanded} {
		s, _ := NewHelicalProfileSurface(f, Vec2{2, 0}, Vec2{3, 1}, 2, hand, -2, 2)
		for _, sx := range []int64{-1, 1} {
			for _, sy := range []int64{-1, 1} {
				cs, sn := big.NewRat(3*sx, 5), big.NewRat(4*sy, 5)
				origin := [3]*big.Rat{big.NewRat(11, 1), new(big.Rat).Add(big.NewRat(2, 1), big.NewRat(3*sx, 2)), big.NewRat(3+2*sy, 1)}
				r, e := s.IntersectRationalRayAxisLine(ctx, cs, sn, origin, big.NewRat(-2, 1), HelicalRayLineOptions{PhaseBits: 32})
				if e != nil || !r.Complete || len(r.Hits) != 4 {
					t.Fatal(sx, sy, hand, r, e)
				}
				expected := math.Atan2(float64(sy)*.8, float64(sx)*.6) / (2 * math.Pi)
				if expected < 0 {
					expected++
				}
				if hand == LeftHanded {
					expected = 1 - expected
				}
				oracle := new(big.Rat).SetFloat64(expected)
				if oracle.Cmp(r.Phase[0]) < 0 || oracle.Cmp(r.Phase[1]) > 0 {
					t.Fatal("phase", sx, sy, hand, expected, r.Phase)
				}
				for i, hit := range r.Hits {
					u := expected + float64(i-2)
					uq := new(big.Rat).SetFloat64(u)
					if uq.Cmp(hit.Turn[0]) < 0 || uq.Cmp(hit.Turn[1]) > 0 || hit.ProfileParameter.Cmp(big.NewRat(1, 2)) != 0 {
						t.Fatal(hit)
					}
					x := new(big.Rat).SetFloat64(1 + 2*u + .5)
					if x.Cmp(hit.Point[0][0]) < 0 || x.Cmp(hit.Point[0][1]) > 0 {
						t.Fatal("XYZ", hit)
					}
				}
				// Clip in original signed line units to retain exactly the two nonnegative
				// turns. Their lambda values lie in(2.75,4.75); negative turns lie above4.75.
				clip := [2]*big.Rat{big.NewRat(11, 4), big.NewRat(19, 4)}
				clipped, e := s.IntersectRationalRayAxisLine(ctx, cs, sn, origin, big.NewRat(-2, 1), HelicalRayLineOptions{HelicalAxisLineOptions: HelicalAxisLineOptions{LineRange: &clip}, PhaseBits: 48})
				if e != nil || !clipped.Complete || len(clipped.Hits) != 2 || clipped.Hits[0].Turn[0].Sign() <= 0 {
					t.Fatal("clip", clipped, e)
				}
			}
		}
	}
}
