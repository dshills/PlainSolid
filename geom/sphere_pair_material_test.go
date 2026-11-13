package geom

import (
	"context"
	"math"
	"math/big"
	"testing"
)

func TestSpherePairMaterialIntervals(t *testing.T) {
	ctx := context.Background()
	o := ToroidLineOptions{ParameterTolerance: 1e-10}
	for _, tc := range []struct {
		op     SphereSetOperation
		lo, hi float64
		lm, hm BoreBoundaryMask
	}{{SphereSetUnion, -5, 7, SpherePairFirst, SpherePairSecond}, {SphereSetIntersection, 1, 5, SpherePairSecond, SpherePairFirst}, {SphereSetDifference, -5, 1, SpherePairFirst, SpherePairSecond}} {
		s, e := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 4}, 3, tc.op)
		if e != nil {
			t.Fatal(e)
		}
		r, e := s.ClipLine(ctx, Vec3{}, Vec3{X: 1}, -10, 10, o)
		if e != nil || len(r.Intervals) != 1 || len(r.Points) != 0 {
			t.Fatal(tc, r, e)
		}
		v := r.Intervals[0]
		if v.StartBounds.Lower.Cmp(new(big.Rat).SetFloat64(tc.lo)) != 0 || v.EndBounds.Upper.Cmp(new(big.Rat).SetFloat64(tc.hi)) != 0 || v.Boundary != 0 || v.StartBoundary != tc.lm || v.EndBoundary != tc.hm {
			t.Fatal(tc, v)
		}
	}
	s, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 4}, 3, SphereSetIntersection)
	r, e := s.ClipLine(ctx, Vec3{Y: 1}, Vec3{X: 1}, -10, 10, o)
	if e != nil || len(r.Intervals) != 1 {
		t.Fatal(r, e)
	}
	v := r.Intervals[0]
	lo := new(big.Rat).Sub(big.NewRat(4, 1), v.StartBounds.Upper)
	hi := new(big.Rat).Sub(big.NewRat(4, 1), v.StartBounds.Lower)
	sq := func(v *big.Rat) *big.Rat { return new(big.Rat).Mul(v, v) }
	if sq(lo).Cmp(big.NewRat(8, 1)) > 0 || sq(hi).Cmp(big.NewRat(8, 1)) < 0 || sq(v.EndBounds.Lower).Cmp(big.NewRat(24, 1)) > 0 || sq(v.EndBounds.Upper).Cmp(big.NewRat(24, 1)) < 0 {
		t.Fatal("independent radical endpoints", v)
	}
	// The intersection circle gives a single ray-tangent contact at x4,y3.
	r, e = s.ClipLine(ctx, Vec3{Y: 3}, Vec3{X: 2}, 0, 4, o)
	if e != nil || len(r.Intervals) != 0 || len(r.Points) != 1 || r.Points[0].Boundary != SpherePairFirst|SpherePairSecond || r.Points[0].Bounds.Lower.Cmp(big.NewRat(2, 1)) != 0 {
		t.Fatal("shared isolated line contact", r, e)
	}
}
func TestSpherePairRegularizationAndContainment(t *testing.T) {
	ctx := context.Background()
	o := ToroidLineOptions{ParameterTolerance: 1e-10}
	for _, op := range []SphereSetOperation{SphereSetUnion, SphereSetIntersection, SphereSetDifference} {
		s, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{}, 5, op)
		r, e := s.ClipLine(ctx, Vec3{}, Vec3{X: 1}, -6, 6, o)
		if e != nil {
			t.Fatal(e)
		}
		if op == SphereSetDifference {
			if len(r.Intervals)+len(r.Points) != 0 {
				t.Fatal("coincident difference remnants", r)
			}
		} else if len(r.Intervals) != 1 || r.Intervals[0].StartBoundary != SpherePairFirst|SpherePairSecond {
			t.Fatal("coincident ancestry", r)
		}
	}
	s, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 8}, 3, SphereSetIntersection)
	loc, mask, e := s.Classify(Vec3{X: 5})
	if e != nil || loc != BoreOutside || mask != 0 {
		t.Fatal("regularized external tangent common", loc, mask, e)
	}
	s, _ = NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 8}, 3, SphereSetUnion)
	r, e := s.ClipLine(ctx, Vec3{X: 5, Y: -1}, Vec3{Y: 1}, 0, 2, o)
	if e != nil || len(r.Points) != 1 || r.Points[0].Boundary != SpherePairFirst|SpherePairSecond {
		t.Fatal("singular union contact retained", r, e)
	}
	s, _ = NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 2}, 3, SphereSetUnion)
	loc, mask, e = s.Classify(Vec3{X: 5})
	if e != nil || loc != BoreBoundary || mask != SpherePairFirst {
		t.Fatal("hidden inner tangent support", loc, mask, e)
	}
	s, _ = NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 2}, 3, SphereSetIntersection)
	loc, mask, e = s.Classify(Vec3{X: 5})
	if e != nil || loc != BoreBoundary || mask != SpherePairSecond {
		t.Fatal("contained common support", loc, mask, e)
	}
	s, _ = NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 2}, 3, SphereSetDifference)
	loc, mask, e = s.Classify(Vec3{X: 5})
	if e != nil || loc != BoreBoundary || mask != SpherePairFirst|SpherePairSecond {
		t.Fatal("explicit singular cavity", loc, mask, e)
	}
	// Although the singular point belongs to the 3D regularized cavity closure,
	// on this axial line it is an isolated retained material point.
	r, e = s.ClipLine(ctx, Vec3{}, Vec3{X: 1}, 0, 6, o)
	if e != nil || len(r.Points) != 1 || r.Points[0].Bounds.Lower.Cmp(big.NewRat(5, 1)) != 0 {
		t.Fatal("cavity contact point", r, e)
	}
	s, _ = NewSpherePairMaterial(Vec3{X: 2}, 3, Vec3{}, 5, SphereSetDifference)
	loc, _, _ = s.Classify(Vec3{X: 5})
	if loc != BoreOutside {
		t.Fatal("contained removed body")
	}
}
func TestSpherePairExactLimits(t *testing.T) {
	a := Vec3{}
	b := Vec3{X: 4}
	s, _ := NewSpherePairMaterial(a, 5, b, 3, SphereSetDifference)
	b.X = 100
	a.X = 100
	for _, tc := range []struct {
		x    float64
		want BorePointLocation
	}{{math.Nextafter(1, 0), BoreInside}, {1, BoreBoundary}, {math.Nextafter(1, 2), BoreOutside}} {
		l, _, e := s.Classify(Vec3{X: tc.x})
		if e != nil || l != tc.want {
			t.Fatal(tc, l, e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	o := ToroidLineOptions{ParameterTolerance: 1e-10}
	r, e := s.ClipLine(ctx, Vec3{}, Vec3{X: 1}, -10, 10, o)
	if e != context.Canceled || len(r.Intervals)+len(r.Points) != 0 {
		t.Fatal(r, e)
	}
	o.Surface.MaxOperations = 1
	r, e = s.ClipLine(context.Background(), Vec3{}, Vec3{X: 1}, -10, 10, o)
	if e == nil || len(r.Intervals)+len(r.Points) != 0 {
		t.Fatal("atomic budget", r, e)
	}
	if _, e := NewSpherePairMaterial(Vec3{}, -1, Vec3{}, 1, SphereSetUnion); e == nil {
		t.Fatal("invalid radius")
	}
	if _, _, e := (SpherePairMaterial{}).Classify(Vec3{}); e == nil {
		t.Fatal("invalid query")
	}
}

func TestSpherePairTranslatedObliqueLine(t *testing.T) {
	// A represented Pythagorean line translated away from the origin.
	a := Vec3{X: 10, Y: -2, Z: 3}
	delta := Vec3{Y: 12, Z: 16}
	b := a.Add(delta)
	s, e := NewSpherePairMaterial(a, 25, b, 15, SphereSetIntersection)
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.ClipLine(context.Background(), a, Vec3{Y: 3, Z: 4}, -10, 10, ToroidLineOptions{ParameterTolerance: 1e-10})
	if e != nil || len(r.Intervals) != 1 || r.Intervals[0].StartBounds.Lower.Cmp(big.NewRat(1, 1)) != 0 || r.Intervals[0].EndBounds.Upper.Cmp(big.NewRat(5, 1)) != 0 {
		t.Fatal("translated rational direction", r, e)
	}
}
