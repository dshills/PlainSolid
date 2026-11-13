package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestSpherePairHorizontalBoundaryCycle(t *testing.T) {
	for _, op := range []SphereSetOperation{SphereSetUnion, SphereSetIntersection, SphereSetDifference} {
		s, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 4}, 3, op)
		r, e := s.HorizontalBoundaryCycle(context.Background(), 0, SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(op, e)
		}
		if r.Arcs[0].Source != SpherePairFirst || r.Arcs[1].Source != SpherePairSecond || !r.Arcs[0].CCW || r.Arcs[1].CCW != (op != SphereSetDifference) {
			t.Fatal("directed ancestry", r)
		}
		// Independent exact circle extrema identify the selected half-plane.
		samples := [2]int64{-5, 7}
		if op == SphereSetIntersection {
			samples = [2]int64{5, 1}
		} else if op == SphereSetDifference {
			samples = [2]int64{-5, 1}
		}
		for i, a := range r.Arcs {
			value := new(big.Rat).Add(new(big.Rat).Mul(a.Covector[0], big.NewRat(samples[i], 1)), a.Offset)
			if value.Sign()*a.InteriorSign <= 0 {
				t.Fatal("wrong source-circle arc", op, i, a)
			}
			if a.StartContact == a.EndContact {
				t.Fatal("collapsed arc")
			}
		}
		for _, p := range r.Contacts {
			if p.Base[0].Cmp(big.NewRat(4, 1)) != 0 || new(big.Rat).Mul(new(big.Rat).Mul(p.Vector[1], p.Vector[1]), p.Radicand).Cmp(big.NewRat(9, 1)) != 0 {
				t.Fatal("exact(4,+/-3) contacts", p)
			}
		}
		// At height one endpoints remain irrational: no float pole reconstruction.
		r, e = s.HorizontalBoundaryCycle(context.Background(), 1, SurfacePatchEvaluationOptions{})
		if e != nil || new(big.Rat).Mul(new(big.Rat).Mul(r.Contacts[0].Vector[1], r.Contacts[0].Vector[1]), r.Contacts[0].Radicand).Cmp(big.NewRat(8, 1)) != 0 {
			t.Fatal("irrational endpoints", r, e)
		}
	}
}
func TestSpherePairBoundaryCycleRejectsUnsupported(t *testing.T) {
	cases := []struct {
		b    Vec3
		r, h float64
	}{{Vec3{X: 8}, 3, 0}, {Vec3{}, 5, 0}, {Vec3{X: 4}, 3, 3}, {Vec3{X: 20}, 3, 0}, {Vec3{}, 1, 0}}
	for _, c := range cases {
		s, _ := NewSpherePairMaterial(Vec3{}, 5, c.b, c.r, SphereSetUnion)
		r, e := s.HorizontalBoundaryCycle(context.Background(), c.h, SurfacePatchEvaluationOptions{})
		if e != ErrDomain || r.Height != nil || len(r.Contacts) != 0 {
			t.Fatal(c, r, e)
		}
	}
	s, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 4}, 3, SphereSetUnion)
	r, e := s.HorizontalBoundaryCycle(context.Background(), 0, SurfacePatchEvaluationOptions{MaxOperations: 1})
	if e == nil || r.Height != nil {
		t.Fatal("atomic budget", r, e)
	}
}

func TestSpherePairBoundaryCycleReversedTranslated(t *testing.T) {
	// Same two circles under a scaled 3-4-5 rotation and translation. The
	// source-extremum witnesses are integer points, not trigonometric samples.
	a, b := Vec3{X: 10, Y: -2}, Vec3{X: 22, Y: 14}
	for _, swap := range []bool{false, true} {
		first, second, ra, rb := a, b, 25.0, 15.0
		samples := [2][2]int64{{-5, -22}, {31, 26}}
		if swap {
			first, second, ra, rb = b, a, 15, 25
			samples[0], samples[1] = samples[1], samples[0]
		}
		s, _ := NewSpherePairMaterial(first, ra, second, rb, SphereSetUnion)
		r, e := s.HorizontalBoundaryCycle(context.Background(), 0, SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for i, arc := range r.Arcs {
			v := new(big.Rat).Set(arc.Offset)
			for j := 0; j < 2; j++ {
				v.Add(v, new(big.Rat).Mul(arc.Covector[j], big.NewRat(samples[i][j], 1)))
			}
			if v.Sign()*arc.InteriorSign <= 0 || !arc.CCW {
				t.Fatal("rotated outside arc", swap, i, arc)
			}
		}
	}
}
