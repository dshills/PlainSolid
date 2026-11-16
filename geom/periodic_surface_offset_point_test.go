package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestPeriodicSurfaceNormalOffsetAtExactFocalDistance(t *testing.T) {
	ctx := context.Background()
	data := BSplineSurfaceData{DegreeU: 2, DegreeV: 1, CountU: 3, CountV: 3, KnotsU: []float64{-1, -1, -1, 1, 1, 1}}
	for i, x := range []float64{-1, 0, 1} {
		for _, y := range []float64{0, 1, 0} {
			data.Poles = append(data.Poles, Vec3{x, y, []float64{1, -1, 1}[i]})
		}
	}
	s, err := NewPeriodicBSplineSurface(data, nil, &PeriodicSplineAxis{Start: 0, Period: 3})
	if err != nil {
		t.Fatal(err)
	}
	// The first cyclic strip is (u, v, u²) with normal (-2u,0,1)/sqrt(1+4u²).
	// The nonzero curvature is 2/(1+4u²)^(3/2).
	at, err := s.EvaluateSecondJetRational(ctx, new(big.Rat), big.NewRat(1, 4), SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		d    *big.Rat
		sign int
	}{{big.NewRat(-5, 1), 1}, {new(big.Rat), 1}, {big.NewRat(49, 100), 1}, {big.NewRat(1, 2), 0}, {big.NewRat(51, 100), -1}} {
		got, err := at.CheckNormalOffset(ctx, tc.d, SurfacePatchOptions{})
		if err != nil || got.FirstMinorSign != tc.sign || got.DeterminantSign != tc.sign || got.FocalFree != (tc.sign > 0) {
			t.Fatalf("d=%s: got %+v, %v", tc.d, got, err)
		}
	}
	at, err = s.EvaluateSecondJetRational(ctx, big.NewRat(1, 2), big.NewRat(1, 4), SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// These distances surround the irrational threshold sqrt(2), but have
	// the same float64 representation.
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(22), nil)
	floor := new(big.Int).Sqrt(new(big.Int).Mul(new(big.Int).Mul(scale, scale), big.NewInt(2)))
	lo, hi := new(big.Rat).SetFrac(floor, scale), new(big.Rat).SetFrac(new(big.Int).Add(floor, big.NewInt(1)), scale)
	lo64, _ := lo.Float64()
	hi64, _ := hi.Float64()
	if lo64 != hi64 {
		t.Fatal("fixture does not exercise equal float64 values")
	}
	inside, err := at.CheckNormalOffset(ctx, lo, SurfacePatchOptions{})
	if err != nil || !inside.FocalFree || inside.DeterminantSign != 1 {
		t.Fatal("rounded-away positive clearance", inside, err)
	}
	outside, err := at.CheckNormalOffset(ctx, hi, SurfacePatchOptions{})
	if err != nil || outside.FocalFree || outside.DeterminantSign != -1 {
		t.Fatal("rounded-away focal crossing", outside, err)
	}
	limited, err := at.CheckNormalOffset(ctx, lo, SurfacePatchOptions{MaxOperations: inside.ExactOperations - 1})
	if !errors.Is(err, ErrConvergence) || limited.FocalFree || limited.FirstMinorSign != 0 || limited.DeterminantSign != 0 {
		t.Fatal("exhausted proof published signs", limited, err)
	}
}

func TestPeriodicSurfaceNormalOffsetRejectsPairOfFocalCrossings(t *testing.T) {
	ctx := context.Background()
	tensor := func(x, y, z int64) [3]*big.Rat {
		return [3]*big.Rat{big.NewRat(x, 1), big.NewRat(y, 1), big.NewRat(z, 1)}
	}
	// S=(u,v,u²+v²/2) at the origin has upward signed curvatures 2 and 1.
	var s RationalPeriodicSurfaceSecondJet
	s.Du, s.Dv = tensor(1, 0, 0), tensor(0, 1, 0)
	s.Duu, s.Duv, s.Dvv = tensor(0, 0, 2), tensor(0, 0, 0), tensor(0, 0, 1)
	both, err := s.CheckNormalOffset(ctx, big.NewRat(2, 1), SurfacePatchOptions{})
	if err != nil || both.FirstMinorSign != -1 || both.DeterminantSign != 1 || both.FocalFree {
		t.Fatal("positive determinant alone cannot certify the full displacement", both, err)
	}
	s.Du, s.Dv, s.Duu, s.Dvv = s.Dv, s.Du, s.Dvv, s.Duu
	reversed, err := s.CheckNormalOffset(ctx, big.NewRat(2, 1), SurfacePatchOptions{})
	if err != nil || !reversed.FocalFree || reversed.DeterminantSign != 1 {
		t.Fatal("reversed chart must flip the signed curvature", reversed, err)
	}
	s.Du = s.Dv
	if _, err := s.CheckNormalOffset(ctx, big.NewRat(1, 1), SurfacePatchOptions{}); !errors.Is(err, ErrDegenerate) {
		t.Fatal("degenerate tangent pair is not a normal chart", err)
	}
}
