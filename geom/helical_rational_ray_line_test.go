package geom

import (
	"context"
	"math"
	"math/big"
	"testing"
)

func TestRationalRayHelicalContacts(t *testing.T) {
	ctx := context.Background()
	f, _ := NewFrame(Vec3{}, Vec3{1, 0, 0}, Vec3{0, 0, 1})
	for _, hand := range []HelixHandedness{RightHanded, LeftHanded} {
		s, _ := NewHelicalProfileSurface(f, Vec2{2, 0}, Vec2{3, 1}, 2, hand, -1, 2)
		origin := [3]*big.Rat{big.NewRat(3, 2), big.NewRat(2, 1), big.NewRat(10, 1)}
		r, e := s.IntersectRationalRayAxisLine(ctx, big.NewRat(3, 5), big.NewRat(4, 5), origin, big.NewRat(-2, 1), HelicalRayLineOptions{PhaseBits: 32})
		if e != nil || !r.Complete || len(r.Hits) != 3 {
			t.Fatal(r, e)
		}
		phase := math.Atan2(.8, .6) / (2 * math.Pi)
		if hand == LeftHanded {
			phase = 1 - phase
		}
		oracle := new(big.Rat).SetFloat64(phase)
		if oracle.Cmp(r.Phase[0]) < 0 || oracle.Cmp(r.Phase[1]) > 0 {
			t.Fatal("phase enclosure", r.Phase, phase)
		}
		for i, h := range r.Hits {
			turn := phase + float64(i-1)
			u := new(big.Rat).SetFloat64(turn)
			lambda := new(big.Rat).SetFloat64((2*turn + .5 - 10) / -2)
			if u.Cmp(h.Turn[0]) < 0 || u.Cmp(h.Turn[1]) > 0 || lambda.Cmp(h.LineParameter[0]) < 0 || lambda.Cmp(h.LineParameter[1]) > 0 || h.ProfileParameter.Cmp(big.NewRat(1, 2)) != 0 {
				t.Fatal(h)
			}
			if h.Point[0][0].Cmp(origin[0]) != 0 || h.Point[1][1].Cmp(origin[1]) != 0 {
				t.Fatal("radial coordinate changed")
			}
		}

		midpoint := new(big.Rat).Quo(new(big.Rat).Add(r.Hits[0].LineParameter[0], r.Hits[0].LineParameter[1]), big.NewRat(2, 1))
		ambiguous := [2]*big.Rat{midpoint, big.NewRat(100, 1)}
		if bad, e := s.IntersectRationalRayAxisLine(ctx, big.NewRat(3, 5), big.NewRat(4, 5), origin, big.NewRat(-2, 1), HelicalRayLineOptions{HelicalAxisLineOptions: HelicalAxisLineOptions{LineRange: &ambiguous}, PhaseBits: 32}); e == nil || bad.Complete || len(bad.Hits) != 0 {
			t.Fatal("ambiguous line boundary accepted", bad, e)
		}
		if bad, e := s.IntersectRationalRayAxisLine(ctx, big.NewRat(3, 5), big.NewRat(4, 5), origin, big.NewRat(-2, 1), HelicalRayLineOptions{HelicalAxisLineOptions: HelicalAxisLineOptions{MaxOperations: 1}}); e == nil || bad.Complete {
			t.Fatal("operation cap")
		}
		fine, e := s.IntersectRationalRayAxisLine(ctx, big.NewRat(3, 5), big.NewRat(4, 5), origin, big.NewRat(-2, 1), HelicalRayLineOptions{PhaseBits: 96})
		if e != nil || !fine.Complete || fine.Phase[0].Cmp(r.Phase[0]) < 0 || fine.Phase[1].Cmp(r.Phase[1]) > 0 {
			t.Fatal(fine, e)
		}
		clip := [2]*big.Rat{big.NewRat(3, 1), big.NewRat(5, 1)}
		clipped, e := s.IntersectRationalRayAxisLine(ctx, big.NewRat(3, 5), big.NewRat(4, 5), origin, big.NewRat(-2, 1), HelicalRayLineOptions{HelicalAxisLineOptions: HelicalAxisLineOptions{LineRange: &clip}})
		if e != nil || !clipped.Complete || len(clipped.Hits) != 2 {
			t.Fatal(clipped, e)
		}
		if bad, e := s.IntersectRationalRayAxisLine(ctx, big.NewRat(3, 5), big.NewRat(4, 5), origin, big.NewRat(1, 1), HelicalRayLineOptions{HelicalAxisLineOptions: HelicalAxisLineOptions{MaxContacts: 2}}); e == nil || bad.Complete || len(bad.Hits) > 0 {
			t.Fatal(bad, e)
		}
		if _, e := s.IntersectRationalRayAxisLine(ctx, big.NewRat(1, 2), big.NewRat(1, 2), origin, big.NewRat(1, 1), HelicalRayLineOptions{}); e == nil {
			t.Fatal("non-unit ray")
		}
	}
}
func TestHelicalAtanSeriesBounds(t *testing.T) {
	for _, x := range []*big.Rat{big.NewRat(-1, 3), big.NewRat(1, 5), big.NewRat(1, 239), big.NewRat(1, 2)} {
		b := &polynomialBudget{maxOperations: 100000, maxBits: 16384}
		bound, e := helicalAtanSmall(context.Background(), x, big.NewRat(1, 1000000000), 1000, b)
		if e != nil {
			t.Fatal(e)
		}
		xf, _ := x.Float64()
		q := new(big.Rat).SetFloat64(math.Atan(xf))
		if q.Cmp(bound[0]) < 0 || q.Cmp(bound[1]) > 0 {
			t.Fatal(bound, q)
		}
	}
}
