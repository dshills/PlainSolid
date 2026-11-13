package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func affineTubeLineFixture(t *testing.T, rational bool) VerifiedSurfaceTube {
	t.Helper()
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	a, _ = a.Trim(ParameterRectangle{UMin: .49, UMax: .51, VMin: .49, VMax: .51})
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{s, .5, t - .25} })
	if rational {
		source := &BSplineSurface{degreeU: 2, degreeV: 1, countU: 3, countV: 2, knotsU: []float64{0, 0, 0, 1, 1, 1}, knotsV: []float64{0, 0, 1, 1}, weights: []float64{1, 1, 2, 2, 3, 3}, poles: []Vec3{{0, .5, -.25}, {0, .5, .75}, {.25, .5, -.25}, {.25, .5, .75}, {1, .5, -.25}, {1, .5, .75}}}
		b, _ = NewSurfacePatch(source, SurfacePatchOptions{})
	}
	b, _ = b.Trim(ParameterRectangle{UMin: .48, UMax: .52, VMin: .24, VMax: .26})
	tube, e := NewVerifiedSurfaceTube(a, b, SurfacePairTubeOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return tube
}
func affineTubeLineEndpoints() ([4]*big.Rat, [4]*big.Rat) {
	start := [4]*big.Rat{new(big.Rat).SetFloat64(.49), big.NewRat(1, 2), new(big.Rat).SetFloat64(.49), big.NewRat(1, 4)}
	end := [4]*big.Rat{new(big.Rat).SetFloat64(.51), big.NewRat(1, 2), new(big.Rat).SetFloat64(.51), big.NewRat(1, 4)}
	return start, end
}
func TestVerifiedTubeAffineParameterLine(t *testing.T) {
	for _, rational := range []bool{false, true} {
		tube := affineTubeLineFixture(t, rational)
		start, end := affineTubeLineEndpoints()
		for _, reverse := range []bool{false, true} {
			if reverse {
				start, end = end, start
			}
			r, e := tube.CertifyAffineParameterLine(context.Background(), start, end, TubeAffineLineOptions{})
			if e != nil || !r.Proven || r.Curve == nil {
				t.Fatal(rational, reverse, r, e)
			}
			point, e := r.Curve.ExactPoint(0)
			if e != nil {
				t.Fatal(e)
			}
			x, _ := start[0].Float64()
			if point != (Vec3{x, .5, 0}) {
				t.Fatal(point)
			}
			r.Start[0].SetInt64(99)
			r.ModelStart[0].SetInt64(99)
			point, e = r.Curve.ExactPoint(0)
			if e != nil || point.X != x {
				t.Fatal("aliased endpoint", point, e)
			}
		}
	}
}
func TestVerifiedTubeAffineLineRejectsWrongAndCurved(t *testing.T) {
	tube := affineTubeLineFixture(t, false)
	start, end := affineTubeLineEndpoints()
	end[1] = big.NewRat(51, 100)
	r, e := tube.CertifyAffineParameterLine(context.Background(), start, end, TubeAffineLineOptions{})
	if !errors.Is(e, ErrConvergence) || r.Proven {
		t.Fatal(r, e)
	}
	start, end = affineTubeLineEndpoints()
	end[0] = big.NewRat(1, 2)
	_, e = tube.CertifyAffineParameterLine(context.Background(), start, end, TubeAffineLineOptions{})
	if !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}
	a, _ := NewSurfacePatch(quadraticContactSurface(t, 0), SurfacePatchOptions{})
	a, _ = a.Trim(ParameterRectangle{UMin: .35, UMax: .45, VMin: .4, VMax: .6})
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{s, .5, t - .125} })
	curved, e := NewVerifiedSurfaceTube(a, b, SurfacePairTubeOptions{})
	if e != nil {
		t.Fatal(e)
	}
	first, last := new(big.Rat).SetFloat64(.35), new(big.Rat).SetFloat64(.45)
	height := func(x *big.Rat) *big.Rat {
		delta := new(big.Rat).Sub(x, big.NewRat(1, 2))
		return new(big.Rat).Add(new(big.Rat).Mul(delta, delta), big.NewRat(1, 8))
	}
	start = [4]*big.Rat{first, big.NewRat(1, 2), new(big.Rat).Set(first), height(first)}
	end = [4]*big.Rat{last, big.NewRat(1, 2), new(big.Rat).Set(last), height(last)}
	r, e = curved.CertifyAffineParameterLine(context.Background(), start, end, TubeAffineLineOptions{})
	if !errors.Is(e, ErrConvergence) || r.Proven {
		t.Fatal(r, e)
	}
}
func TestVerifiedTubeAffineLineLimits(t *testing.T) {
	tube := affineTubeLineFixture(t, false)
	start, end := affineTubeLineEndpoints()
	r, e := tube.CertifyAffineParameterLine(context.Background(), start, end, TubeAffineLineOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) || r.Proven {
		t.Fatal(r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = tube.CertifyAffineParameterLine(ctx, start, end, TubeAffineLineOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	start[0] = nil
	_, e = tube.CertifyAffineParameterLine(context.Background(), start, end, TubeAffineLineOptions{})
	if !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}

func TestVerifiedTubeAffineLineNonbinaryDependentEndpoint(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	a, _ = a.Trim(ParameterRectangle{UMin: .49, UMax: .51, VMin: .49, VMax: .51})
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{s, .5, 3*t - 1} })
	b, _ = b.Trim(ParameterRectangle{UMin: .48, UMax: .52, VMin: .25, VMax: .4})
	tube, e := NewVerifiedSurfaceTube(a, b, SurfacePairTubeOptions{})
	if e != nil {
		t.Fatal(e)
	}
	start, end := affineTubeLineEndpoints()
	start[3], end[3] = big.NewRat(1, 3), big.NewRat(1, 3)
	r, e := tube.CertifyAffineParameterLine(context.Background(), start, end, TubeAffineLineOptions{})
	if e != nil || !r.Proven || r.Start[3].Cmp(big.NewRat(1, 3)) != 0 || r.End[3].Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatal(r, e)
	}
	start[3].SetInt64(4)
	if r.Start[3].Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatal("input alias")
	}
}

func TestVerifiedTubeAffineParameterLineRejectsCurvedModelImage(t *testing.T) {
	build := func(second bool) *SurfacePatch {
		poles := make([]Vec3, 6)
		for i := 0; i < 3; i++ {
			for j := 0; j < 2; j++ {
				x, y, z := float64(i)/2, float64(j), 0.0
				if i == 2 {
					z = 1
				}
				if second {
					y = .5
					z += float64(j) - .25
				}
				poles[2*i+j] = Vec3{x, y, z}
			}
		}
		s, _ := NewBSplineSurface(2, 1, 3, 2, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 1, 1}, nil)
		p, _ := NewSurfacePatch(s, SurfacePatchOptions{})
		return p
	}
	a, _ := build(false).Trim(ParameterRectangle{UMin: .49, UMax: .51, VMin: .49, VMax: .51})
	b, _ := build(true).Trim(ParameterRectangle{UMin: .48, UMax: .52, VMin: .1, VMax: .4})
	tube, e := NewVerifiedSurfaceTube(a, b, SurfacePairTubeOptions{})
	if e != nil {
		t.Fatal(e)
	}
	start, end := affineTubeLineEndpoints()
	r, e := tube.CertifyAffineParameterLine(context.Background(), start, end, TubeAffineLineOptions{})
	if !errors.Is(e, ErrConvergence) || r.Proven {
		t.Fatal(r, e)
	}
}
