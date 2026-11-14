package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func sphereLineRootEquals(t *testing.T, root *ExactAlgebraicRoot, want *big.Rat) {
	t.Helper()
	q, _, e := NewExactRationalRoot(context.Background(), want, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cmp, e := root.Compare(context.Background(), q, ExactCurvePlaneEventOptions{})
	if e != nil || !cmp.Proven || cmp.Order != 0 {
		t.Fatal("exact original parameter", want, cmp, e)
	}
}
func TestSpherePairRationalLineOblique(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	o := ToroidLineOptions{ParameterTolerance: 1e-9}
	org := [3]*big.Rat{r(1, 3), r(1, 3), r(1, 6)}
	dir := [3]*big.Rat{r(2, 9), r(2, 9), r(1, 9)}
	for _, tc := range []struct {
		op     SphereSetOperation
		lo, hi *big.Rat
		lm, hm BoreBoundaryMask
	}{{SphereSetUnion, r(-33, 2), r(45, 2), 1, 2}, {SphereSetIntersection, r(-15, 2), r(27, 2), 2, 1}, {SphereSetDifference, r(-33, 2), r(-15, 2), 1, 2}} {
		s, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 2, Y: 2, Z: 1}, 5, tc.op)
		for _, sign := range []int64{1, -1} {
			d := dir
			for j := range d {
				d[j] = new(big.Rat).Mul(r(sign, 1), dir[j])
			}
			line, e := s.ClipRationalLine(ctx, org, d, r(-30, 1), r(30, 1), o)
			if e != nil || len(line.Intervals) != 1 || len(line.Points) != 0 {
				t.Fatal(line, e)
			}
			lo, hi, lm, hm := tc.lo, tc.hi, tc.lm, tc.hm
			if sign < 0 {
				lo, hi, lm, hm = new(big.Rat).Neg(hi), new(big.Rat).Neg(lo), hm, lm
			}
			v := line.Intervals[0]
			sphereLineRootEquals(t, v.Start, lo)
			sphereLineRootEquals(t, v.End, hi)
			if v.StartBoundary != lm || v.EndBoundary != hm || v.Boundary != 0 {
				t.Fatal("original ownership/reversal", v)
			}
		}
	}
	s, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 2, Y: 2, Z: 1}, 5, SphereSetUnion)
	eps := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 120))
	lo := new(big.Rat).Add(r(-33, 2), eps)
	hi := new(big.Rat).Add(lo, eps)
	line, e := s.ClipRationalLine(ctx, org, dir, lo, hi, o)
	if e != nil || len(line.Intervals) != 1 || line.Intervals[0].StartBoundary != 0 || line.Intervals[0].EndBoundary != 0 {
		t.Fatal("subfloat raw clip", line, e)
	}
	sphereLineRootEquals(t, line.Intervals[0].Start, lo)
	sphereLineRootEquals(t, line.Intervals[0].End, hi)
	single, e := s.ClipRationalLine(ctx, org, dir, lo, lo, o)
	if e != nil || len(single.Points) != 1 || single.Points[0].Boundary != 0 || len(single.Intervals) != 0 {
		t.Fatal("clipped interior singleton", single, e)
	}
	at, e := s.ClipRationalLine(ctx, org, dir, r(-33, 2), r(-33, 2), o)
	if e != nil || len(at.Points) != 1 || at.Points[0].Boundary != 1 {
		t.Fatal("exact retained endpoint", at, e)
	}
	// Root/source input ownership survives caller mutation after a successful clip.
	lo.SetInt64(999)
	org[0].SetInt64(999)
	sphereLineRootEquals(t, line.Intervals[0].End, hi)
}
func TestSpherePairRationalLineRegimes(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	o := ToroidLineOptions{ParameterTolerance: 1e-9}
	origin := [3]*big.Rat{r(0, 1), r(0, 1), r(0, 1)}
	dx := [3]*big.Rat{r(1, 1), r(0, 1), r(0, 1)}
	// Internally tangent cavity: a material interval and an isolated singular point.
	cavity, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 4}, 1, SphereSetDifference)
	line, e := cavity.ClipRationalLine(ctx, origin, dx, r(-6, 1), r(6, 1), o)
	if e != nil || len(line.Intervals) != 1 || len(line.Points) != 1 {
		t.Fatal(line, e)
	}
	sphereLineRootEquals(t, line.Intervals[0].Start, r(-5, 1))
	sphereLineRootEquals(t, line.Intervals[0].End, r(3, 1))
	sphereLineRootEquals(t, line.Points[0].Parameter, r(5, 1))
	if line.Intervals[0].StartBoundary != 1 || line.Intervals[0].EndBoundary != 2 || line.Points[0].Boundary != 3 {
		t.Fatal("singular original owners")
	}
	dy := [3]*big.Rat{r(0, 1), r(1, 1), r(0, 1)}
	touch, e := cavity.ClipRationalLine(ctx, [3]*big.Rat{r(5, 1), r(0, 1), r(0, 1)}, dy, r(-1, 1), r(1, 1), o)
	if e != nil || len(touch.Intervals) != 0 || len(touch.Points) != 1 || touch.Points[0].Boundary != 3 || touch.Points[0].Multiplicity != 2 {
		t.Fatal("isolated tangent contact", touch, e)
	}
	for _, op := range []SphereSetOperation{SphereSetUnion, SphereSetIntersection, SphereSetDifference} {
		s, _ := NewSpherePairMaterial(Vec3{X: -1}, 1, Vec3{X: 1}, 1, op)
		v, e := s.ClipRationalLine(ctx, origin, dy, r(-1, 1), r(1, 1), o)
		if e != nil {
			t.Fatal(e)
		}
		if op == SphereSetIntersection {
			if len(v.Points) != 0 || len(v.Intervals) != 0 {
				t.Fatal("empty regularized tangent common")
			}
		} else {
			mask := BoreBoundaryMask(3)
			if op == SphereSetDifference {
				mask = 1
			}
			if len(v.Points) != 1 || v.Points[0].Boundary != mask || v.Points[0].Multiplicity != 2 {
				t.Fatal("regularized touching owner", op, v)
			}
		}
	}
	for _, op := range []SphereSetOperation{SphereSetUnion, SphereSetIntersection, SphereSetDifference} {
		s, _ := NewSpherePairMaterial(Vec3{}, 1, Vec3{}, 1, op)
		v, e := s.ClipRationalLine(ctx, origin, dx, r(-2, 1), r(2, 1), o)
		if e != nil {
			t.Fatal(e)
		}
		if op == SphereSetDifference {
			if len(v.Intervals) != 0 || len(v.Points) != 0 {
				t.Fatal("empty identical cut")
			}
		} else if len(v.Intervals) != 1 || v.Intervals[0].StartBoundary != 3 || v.Intervals[0].EndBoundary != 3 {
			t.Fatal("coincident source identities", v)
		}
	}

	for _, tc := range []struct {
		a, b   Vec3
		ra, rb float64
		op     SphereSetOperation
		ends   [][2]int64
		masks  [][2]BoreBoundaryMask
	}{
		{Vec3{}, Vec3{X: 4}, 1, 1, SphereSetUnion, [][2]int64{{-1, 1}, {3, 5}}, [][2]BoreBoundaryMask{{1, 1}, {2, 2}}},
		{Vec3{}, Vec3{X: 1}, 5, 1, SphereSetDifference, [][2]int64{{-5, 0}, {2, 5}}, [][2]BoreBoundaryMask{{1, 2}, {2, 1}}},
		{Vec3{}, Vec3{X: 1}, 5, 1, SphereSetIntersection, [][2]int64{{0, 2}}, [][2]BoreBoundaryMask{{2, 2}}},
		{Vec3{}, Vec3{X: 4}, 5, 1, SphereSetUnion, [][2]int64{{-5, 5}}, [][2]BoreBoundaryMask{{1, 1}}},
		{Vec3{X: -1}, Vec3{X: 1}, 1, 1, SphereSetUnion, [][2]int64{{-2, 0}, {0, 2}}, [][2]BoreBoundaryMask{{1, 3}, {3, 2}}},
	} {
		s, _ := NewSpherePairMaterial(tc.a, tc.ra, tc.b, tc.rb, tc.op)
		v, e := s.ClipRationalLine(ctx, origin, dx, r(-6, 1), r(6, 1), o)
		if e != nil || len(v.Intervals) != len(tc.ends) || len(v.Points) != 0 {
			t.Fatal("complete original regime", tc, v, e)
		}
		for i, span := range v.Intervals {
			sphereLineRootEquals(t, span.Start, r(tc.ends[i][0], 1))
			sphereLineRootEquals(t, span.End, r(tc.ends[i][1], 1))
			if span.StartBoundary != tc.masks[i][0] || span.EndBoundary != tc.masks[i][1] {
				t.Fatal("retained span ancestry", span)
			}
		}
	}
	// Strict containment excludes redundant source contacts under existing policy.
	inside, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 1}, 1, SphereSetUnion)
	v, e := inside.ClipRationalLine(ctx, origin, dx, r(-6, 1), r(6, 1), o)
	if e != nil || len(v.Intervals) != 1 || v.Intervals[0].StartBoundary != 1 || v.Intervals[0].EndBoundary != 1 {
		t.Fatal("redundant sphere mask", v, e)
	}
	o.Surface.MaxOperations = 1
	v, e = inside.ClipRationalLine(ctx, origin, dx, r(-6, 1), r(6, 1), o)
	if e == nil || len(v.Intervals) != 0 || len(v.Points) != 0 {
		t.Fatal("atomic budget", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	v, e = inside.ClipRationalLine(cancelled, origin, dx, r(-6, 1), r(6, 1), ToroidLineOptions{ParameterTolerance: 1e-9})
	if !errors.Is(e, context.Canceled) || len(v.Intervals) != 0 || len(v.Points) != 0 {
		t.Fatal(e)
	}
}
func TestSpherePairRationalLineAlgebraicAndSubfloat(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	s, _ := NewSpherePairMaterial(Vec3{}, 1, Vec3{X: 20}, 1, SphereSetUnion)
	o := ToroidLineOptions{ParameterTolerance: 1e-6}
	d := [3]*big.Rat{r(1, 1), r(0, 1), r(0, 1)}
	v, e := s.ClipRationalLine(ctx, [3]*big.Rat{r(1, 3), r(1, 3), r(0, 1)}, d, r(-2, 1), r(2, 1), o)
	if e != nil || len(v.Intervals) != 1 {
		t.Fatal(v, e)
	}
	for i, root := range []*ExactAlgebraicRoot{v.Intervals[0].Start, v.Intervals[0].End} {
		lo, hi := "-2", "0"
		if i == 1 {
			lo, hi = "0", "2"
		}
		want, e := ImportExactRoot(ctx, ExactRootRecipe{Version: 1, Coefficients: []string{"-7", "6", "9"}, Lower: lo, Upper: hi}, ExactRootCodecOptions{})
		if e != nil {
			t.Fatal(e)
		}
		cmp, e := root.Compare(ctx, want, ExactCurvePlaneEventOptions{})
		if e != nil || !cmp.Proven || cmp.Order != 0 {
			t.Fatal("independent irrational roots", cmp, e)
		}
	}
	eps := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))
	for _, sign := range []int64{-1, 1} {
		y := new(big.Rat).Add(r(1, 1), new(big.Rat).Mul(r(sign, 1), eps))
		v, e := s.ClipRationalLine(ctx, [3]*big.Rat{r(0, 1), y, r(0, 1)}, d, r(-1, 1), r(1, 1), o)
		if e != nil {
			t.Fatal(e)
		}
		if sign > 0 {
			if len(v.Intervals) != 0 || len(v.Points) != 0 {
				t.Fatal("subfloat exterior")
			}
		} else {
			if len(v.Intervals) != 1 {
				t.Fatal("subfloat interior", v)
			}
			cmp, e := v.Intervals[0].Start.Compare(ctx, v.Intervals[0].End, ExactCurvePlaneEventOptions{})
			if e != nil || !cmp.Proven || cmp.Order >= 0 {
				t.Fatal("distinct close roots", cmp, e)
			}
		}
	}
}
