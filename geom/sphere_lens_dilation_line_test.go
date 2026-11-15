package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
)

func TestClipDilatedLensLineCapRimJoinAndTangency(t *testing.T) {
	ctx := context.Background()
	s, err := NewSpherePairMaterial(Vec3{}, 5, Vec3{Z: 6}, 5, SphereSetIntersection)
	if err != nil {
		t.Fatal(err)
	}
	q := func(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }
	p := func(x, y, z float64) [3]*big.Rat { return [3]*big.Rat{q(x), q(y), q(z)} }
	const both = SpherePairFirst | SpherePairSecond
	o := ToroidLineOptions{ParameterTolerance: 1e-9}
	checkRoot := func(root *ExactAlgebraicRoot, value *big.Rat) {
		t.Helper()
		exact, _, err := NewExactRationalRoot(ctx, value, ExactCurvePlaneEventOptions{})
		if err != nil {
			t.Fatal(err)
		}
		comparison, err := root.Compare(ctx, exact, ExactCurvePlaneEventOptions{})
		if err != nil || !comparison.Proven || comparison.Order != 0 {
			t.Fatalf("root %+v != %v: %+v %v", root.Interval(), value, comparison, err)
		}
	}
	for _, tc := range []struct {
		origin, direction [3]*big.Rat
		displacement      float64
		w                 [2]*big.Rat
		owners            [2]BoreBoundaryMask
	}{
		{p(0, 0, 3), p(2, 0, 0), 0, [2]*big.Rat{q(-2), q(2)}, [2]BoreBoundaryMask{both, both}},
		{p(0, 0, 3), p(2, 0, 0), .5, [2]*big.Rat{q(-2.25), q(2.25)}, [2]BoreBoundaryMask{both, both}},
		{p(0, 0, 3), p(2, 0, 0), 12, [2]*big.Rat{q(-8), q(8)}, [2]BoreBoundaryMask{both, both}},
		{p(0, 0, 0), p(0, 0, 1), .5, [2]*big.Rat{q(.5), q(5.5)}, [2]BoreBoundaryMask{SpherePairSecond, SpherePairFirst}},
		{[3]*big.Rat{big.NewRat(22, 5), q(0), q(0)}, p(0, 0, 1), .5, [2]*big.Rat{big.NewRat(27, 10), big.NewRat(33, 10)}, [2]BoreBoundaryMask{both, both}},
	} {
		r, err := s.ClipDilatedLensRationalLine(ctx, tc.origin, tc.direction, q(-20), q(20), q(tc.displacement), o)
		if err != nil || r.Interval == nil || r.Point != nil {
			t.Fatalf("line %+v: %+v %v", tc, r, err)
		}
		checkRoot(r.Interval.Start, tc.w[0])
		checkRoot(r.Interval.End, tc.w[1])
		if r.Interval.StartBoundary != tc.owners[0] || r.Interval.EndBoundary != tc.owners[1] {
			t.Fatalf("line %+v: owners %+v", tc, r.Interval)
		}
	}
	line, err := s.ClipDilatedLensRationalLine(ctx, p(0, 4.5, 3), p(1, 0, 0), q(-1), q(1), q(.5), o)
	if err != nil || line.Point == nil || line.Interval != nil || line.Point.Owners != both {
		t.Fatalf("isolated tangency: %+v %v", line, err)
	}
	checkRoot(line.Point.Parameter, q(0))
	line, err = s.ClipDilatedLensRationalLine(ctx, p(0, 0, 3), p(1, 0, 0), q(0), q(1), q(.5), o)
	if err != nil || line.Interval == nil || line.Interval.StartBoundary != 0 || line.Interval.EndBoundary != 0 {
		t.Fatalf("clipped inside interval: %+v %v", line, err)
	}
	line, err = s.ClipDilatedLensRationalLine(ctx, p(0, 0, 3), p(1, 0, 0), q(0), q(0), q(.5), o)
	if err != nil || line.Point == nil || line.Point.Owners != 0 {
		t.Fatalf("clipped interior point: %+v %v", line, err)
	}
}

func TestClipDilatedLensLineObliqueAlgebraicRimAndLimits(t *testing.T) {
	ctx := context.Background()
	s, err := NewSpherePairMaterial(Vec3{}, 2, Vec3{X: 2, Y: 2}, 2, SphereSetIntersection)
	if err != nil {
		t.Fatal(err)
	}
	q := func(v int64) *big.Rat { return big.NewRat(v, 1) }
	org, dir := [3]*big.Rat{q(1), q(1), q(0)}, [3]*big.Rat{q(0), q(0), q(1)}
	o := ToroidLineOptions{ParameterTolerance: 1e-9}
	r, err := s.ClipDilatedLensRationalLine(ctx, org, dir, q(-3), q(3), big.NewRat(1, 2), o)
	const both = SpherePairFirst | SpherePairSecond
	if err != nil || r.Interval == nil || r.Interval.StartBoundary != both || r.Interval.EndBoundary != both {
		t.Fatalf("oblique clip: %+v %v", r, err)
	}
	for i, root := range []*ExactAlgebraicRoot{r.Interval.Start, r.Interval.End} {
		// (±t−1/2)^2−2=0, with matching direction sign.
		proof, err := root.SignPolynomial(ctx, []*big.Rat{big.NewRat(-7, 4), q(int64(1 - 2*i)), q(1)}, ExactCurvePlaneEventOptions{})
		if err != nil || proof.Sign != 0 {
			t.Fatalf("root %d failed the analytic radical: %+v %v", i, proof, err)
		}
		lo, _ := root.Interval().Lower.Float64()
		if math.Abs(lo-float64(2*i-1)*(.5+math.Sqrt2)) > 1e-9 {
			t.Fatalf("root %d has another branch: %+v", i, root.Interval())
		}
	}
	r, err = s.ClipDilatedLensRationalLine(ctx, org, dir, q(2), q(3), big.NewRat(1, 2), o)
	if err != nil || r != (SphereLensDilationLine{}) {
		t.Fatalf("outside interval: %+v %v", r, err)
	}
	o.Surface.MaxOperations = 10
	r, err = s.ClipDilatedLensRationalLine(ctx, org, dir, q(-3), q(3), big.NewRat(1, 2), o)
	if !errors.Is(err, ErrConvergence) || r != (SphereLensDilationLine{}) {
		t.Fatalf("unproved interval: %+v %v", r, err)
	}
}

func TestClipDilatedLensLineRadicalPlaneBeyondCenter(t *testing.T) {
	ctx := context.Background()
	q := func(v int64) *big.Rat { return big.NewRat(v, 1) }
	o := ToroidLineOptions{ParameterTolerance: 1e-9}
	for _, swap := range []bool{false, true} {
		a, b, ra, rb := Vec3{}, Vec3{Z: 4}, 5., 2.
		cap := SpherePairSecond
		if swap {
			a, b, ra, rb = b, a, rb, ra
			cap = SpherePairFirst
		}
		s, err := NewSpherePairMaterial(a, ra, b, rb, SphereSetIntersection)
		if err != nil {
			t.Fatal(err)
		}
		// At z=37/8, the offset cap of the small sphere is retained. Its
		// expanded whole-sphere section is sqrt(3²−(5/8)²), whereas the
		// expanded larger sphere's section here is outside its trim.
		org, dir := [3]*big.Rat{q(0), q(0), big.NewRat(37, 8)}, [3]*big.Rat{q(1), q(0), q(0)}
		r, err := s.ClipDilatedLensRationalLine(ctx, org, dir, q(-8), q(8), q(1), o)
		if err != nil || r.Interval == nil || r.Interval.StartBoundary != cap || r.Interval.EndBoundary != cap {
			t.Fatalf("cap order swapped=%v: %+v %v", swap, r, err)
		}
		for _, root := range []*ExactAlgebraicRoot{r.Interval.Start, r.Interval.End} {
			proof, err := root.SignPolynomial(ctx, []*big.Rat{q(-551), q(0), q(64)}, ExactCurvePlaneEventOptions{})
			if err != nil || proof.Sign != 0 {
				t.Fatalf("cap endpoint %+v: %+v %v", root.Interval(), proof, err)
			}
		}
	}
}
