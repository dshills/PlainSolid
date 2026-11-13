package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestSpherePairPlaneRationalLine(t *testing.T) {
	ctx := context.Background()
	s, _ := NewSpherePairMaterial(Vec3{}, 1, Vec3{X: 20}, 1, SphereSetUnion)
	v, e := s.PlaneMaterial(ctx, Vec3{}, Vec3{Y: 1, Z: -1}, SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	o := ToroidLineOptions{ParameterTolerance: 1e-10}
	org := [2]*big.Rat{big.NewRat(1, 3), big.NewRat(2, 3)}
	for _, sign := range []int64{1, -1} {
		dir := [2]*big.Rat{big.NewRat(2*sign, 3), new(big.Rat)}
		r, e := v.ClipRationalLine(ctx, org, dir, big.NewRat(-2, 1), big.NewRat(2, 1), o)
		if e != nil || len(r.Intervals) != 1 || len(r.Points) != 0 {
			t.Fatal(r, e)
		}
		wantA, wantB := big.NewRat(-1, 1), new(big.Rat)
		if sign < 0 {
			wantA, wantB = new(big.Rat), big.NewRat(1, 1)
		}
		for i, root := range []*ExactAlgebraicRoot{r.Intervals[0].Start, r.Intervals[0].End} {
			want := wantA
			if i == 1 {
				want = wantB
			}
			q, _, e := NewExactRationalRoot(ctx, want, ExactCurvePlaneEventOptions{})
			if e != nil {
				t.Fatal(e)
			}
			cmp, e := root.Compare(ctx, q, ExactCurvePlaneEventOptions{})
			if e != nil || !cmp.Proven || cmp.Order != 0 {
				t.Fatal(cmp, e)
			}
		}
		if r.Intervals[0].StartBoundary != SpherePairFirst || r.Intervals[0].EndBoundary != SpherePairFirst {
			t.Fatal(r)
		}
	}
	r, e := v.ClipRationalLine(ctx, org, [2]*big.Rat{big.NewRat(2, 3), new(big.Rat)}, big.NewRat(-2, 3), big.NewRat(-1, 3), o)
	if e != nil || len(r.Intervals) != 1 || r.Intervals[0].StartBoundary != 0 || r.Intervals[0].Start.Interval().Lower.Cmp(big.NewRat(-2, 3)) != 0 {
		t.Fatal(r, e)
	}
	// Exact boundary start moving outward is an isolated retained endpoint.
	r, e = v.ClipRationalLine(ctx, org, [2]*big.Rat{big.NewRat(2, 3), new(big.Rat)}, new(big.Rat), big.NewRat(1, 3), o)
	if e != nil || len(r.Intervals) != 0 || len(r.Points) != 1 || r.Points[0].Boundary != SpherePairFirst {
		t.Fatal(r, e)
	}
}
func TestSpherePairPlaneRationalLinePoleAndLimits(t *testing.T) {
	ctx := context.Background()
	s, _ := NewSpherePairMaterial(Vec3{}, 1, Vec3{X: 20}, 1, SphereSetUnion)
	v, _ := s.PlaneMaterial(ctx, Vec3{X: 3}, Vec3{X: 1, Y: 2, Z: 2}, SurfacePatchEvaluationOptions{})
	org := [2]*big.Rat{big.NewRat(-8, 3), big.NewRat(2, 3)}
	dir := [2]*big.Rat{big.NewRat(1, 3), new(big.Rat)}
	o := ToroidLineOptions{ParameterTolerance: 1e-10}
	r, e := v.ClipRationalLine(ctx, org, dir, big.NewRat(-1, 3), big.NewRat(1, 3), o)
	if e != nil || len(r.Points) != 1 || len(r.Intervals) != 0 || r.Points[0].Multiplicity != 2 {
		t.Fatal(r, e)
	}
	o.Surface.MaxOperations = 1
	r, e = v.ClipRationalLine(ctx, org, dir, big.NewRat(-1, 3), big.NewRat(1, 3), o)
	if e == nil || len(r.Points) != 0 || len(r.Intervals) != 0 {
		t.Fatal("atomic", r, e)
	}
}

func TestSpherePairPlaneRationalLineIrrationalRoots(t *testing.T) {
	ctx := context.Background()
	s, _ := NewSpherePairMaterial(Vec3{}, 1, Vec3{X: 20}, 1, SphereSetUnion)
	v, _ := s.PlaneMaterial(ctx, Vec3{}, Vec3{Y: 1, Z: -1}, SurfacePatchEvaluationOptions{})
	r, e := v.ClipRationalLine(ctx, [2]*big.Rat{new(big.Rat), big.NewRat(1, 3)}, [2]*big.Rat{big.NewRat(1, 1), new(big.Rat)}, big.NewRat(-4, 3), big.NewRat(4, 3), ToroidLineOptions{ParameterTolerance: 1e-10})
	if e != nil || len(r.Intervals) != 1 {
		t.Fatal(r, e)
	}
	for i, root := range []*ExactAlgebraicRoot{r.Intervals[0].Start, r.Intervals[0].End} {
		lo, hi := "-1", "0"
		if i == 1 {
			lo, hi = "0", "1"
		}
		q, e := ImportExactRoot(ctx, ExactRootRecipe{Version: 1, Coefficients: []string{"-7", "0", "9"}, Lower: lo, Upper: hi}, ExactRootCodecOptions{})
		if e != nil {
			t.Fatal(e)
		}
		cmp, e := root.Compare(ctx, q, ExactCurvePlaneEventOptions{})
		if e != nil || !cmp.Proven || cmp.Order != 0 {
			t.Fatal(cmp, e)
		}
	}
}
