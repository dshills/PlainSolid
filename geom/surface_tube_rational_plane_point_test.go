package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func rationalPlanePointTube(t *testing.T, reverse, curved bool) VerifiedSurfaceTube {
	t.Helper()
	makePatch := func(us, vs [2]float64, point func(float64, float64) Vec3) *SurfacePatch {
		var poles []Vec3
		for _, u := range us {
			for _, v := range vs {
				poles = append(poles, point(u, v))
			}
		}
		s, e := NewBSplineSurface(1, 1, 2, 2, poles, []float64{us[0], us[0], us[1], us[1]}, []float64{vs[0], vs[0], vs[1], vs[1]}, nil)
		if e != nil {
			t.Fatal(e)
		}
		p, e := NewSurfacePatch(s, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return p
	}
	a := makePatch([2]float64{-.125, .125}, [2]float64{.25, .75}, func(u, v float64) Vec3 { return Vec3{X: u, Y: v} })
	b := makePatch([2]float64{0, 1}, [2]float64{.5, 1}, func(u, v float64) Vec3 {
		if reverse {
			v = 1.5 - v
		}
		x := 0.
		if curved {
			x = u * v / 128
		}
		return Vec3{X: x, Y: u, Z: -1 + (1+u/2)*v}
	})
	tube, e := NewVerifiedSurfaceTube(a, b, SurfacePairTubeOptions{FreeAxis: 1})
	if e != nil {
		t.Fatal("fixture tube", e)
	}
	return tube
}
func TestRationalPlaneTubePoint(t *testing.T) {
	ctx := context.Background()
	for _, reverse := range []bool{false, true} {
		tube := rationalPlanePointTube(t, reverse, false)
		if p, e := tube.EvaluateRationalGraph(ctx, big.NewRat(1, 3), TubeAlgebraicGraphEndpointOptions{}); e == nil || p.Proven {
			t.Fatal("nonaffine target unexpectedly XY")
		}
		p, e := tube.EvaluateRationalPlane(ctx, big.NewRat(1, 3), TubeAlgebraicGraphEndpointOptions{})
		if e != nil || !p.Proven || !p.SourceIsoTransverse || p.ModelEvent == nil {
			t.Fatal("exact plane evaluation", e)
		}
		last := big.NewRat(6, 7)
		if reverse {
			last = big.NewRat(9, 14)
		}
		for i, want := range [4]*big.Rat{new(big.Rat), big.NewRat(1, 3), big.NewRat(1, 3), last} {
			q, e := p.Parameters[i].Classify(ctx, ExactCurvePlaneEventOptions{})
			if e != nil || q.Kind != ExactRootKindRational || q.Rational.Cmp(want) != 0 {
				t.Fatal("original coordinate", i, q, e)
			}
		}
		for _, q := range []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 4)} {
			if p, e := tube.EvaluateRationalPlane(ctx, q, TubeAlgebraicGraphEndpointOptions{}); e != nil || !p.Proven {
				t.Fatal("closed free endpoints", e)
			}
		}
	}
	tube := rationalPlanePointTube(t, false, false)
	// Swap support ownership without changing the physical event or free units.
	swapped, err := NewVerifiedSurfaceTube(tube.b, tube.a, SurfacePairTubeOptions{FreeAxis: 3})
	if err != nil {
		t.Fatal(err)
	}
	paired, err := swapped.EvaluateRationalPlane(ctx, big.NewRat(1, 3), TubeAlgebraicGraphEndpointOptions{})
	if err != nil || !paired.Proven {
		t.Fatal("swapped owners", err)
	}
	for i, want := range [4]*big.Rat{big.NewRat(1, 3), big.NewRat(6, 7), new(big.Rat), big.NewRat(1, 3)} {
		got, err := paired.Parameters[i].Classify(ctx, ExactCurvePlaneEventOptions{})
		if err != nil || got.Rational == nil || got.Rational.Cmp(want) != 0 {
			t.Fatal("swapped parameter", i, err)
		}
	}
	for _, row := range []struct {
		q    *big.Rat
		o    TubeAlgebraicGraphEndpointOptions
		want error
	}{
		{big.NewRat(1, 1), TubeAlgebraicGraphEndpointOptions{}, ErrDomain},
		{big.NewRat(1, 3), TubeAlgebraicGraphEndpointOptions{SurfaceCurveAgreementOptions: SurfaceCurveAgreementOptions{SurfacePatchOptions: SurfacePatchOptions{MaxOperations: 1}}}, ErrConvergence},
	} {
		p, e := tube.EvaluateRationalPlane(ctx, row.q, row.o)
		if !errors.Is(e, row.want) || p.Proven || p.ModelEvent != nil {
			t.Fatal("atomic failure", e)
		}
	}
	p, e := rationalPlanePointTube(t, false, true).EvaluateRationalPlane(ctx, big.NewRat(1, 3), TubeAlgebraicGraphEndpointOptions{})
	if e == nil || p.Proven || p.ModelEvent != nil {
		t.Fatal("nonplanar support accepted", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := tube.EvaluateRationalPlane(canceled, big.NewRat(1, 3), TubeAlgebraicGraphEndpointOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
