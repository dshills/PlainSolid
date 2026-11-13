package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
)

func TestExactRationalFloatUnderflowAndOverflow(t *testing.T) {
	power := func(e int) *big.Rat {
		if e >= 0 {
			return new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), uint(e)))
		}
		return new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), uint(-e)))
	}
	for _, e := range []int{-1100, -1075, -1074, -1073, 0, 1023, 1100} {
		for _, sign := range []int64{-1, 1} {
			x := new(big.Rat).Mul(big.NewRat(sign, 1), power(e))
			f, err := RepresentedExactScalar(x)
			representable := e >= -1074 && e <= 1023
			if representable {
				if err != nil || new(big.Rat).SetFloat64(f).Cmp(x) != 0 {
					t.Fatal(e, sign, f, err)
				}
			} else if !errors.Is(err, ErrUnrepresentable) {
				t.Fatal("rounded scalar accepted", e, sign, f, err)
			}
			lo, hi := rationalFloatBound(x, true), rationalFloatBound(x, false)
			if !math.IsInf(lo, -1) && new(big.Rat).SetFloat64(lo).Cmp(x) > 0 {
				t.Fatal("bad lower", e, sign, lo)
			}
			if !math.IsInf(hi, 1) && new(big.Rat).SetFloat64(hi).Cmp(x) < 0 {
				t.Fatal("bad upper", e, sign, hi)
			}
			if e < -1074 {
				if sign > 0 && (lo != 0 || hi != math.SmallestNonzeroFloat64) {
					t.Fatal(lo, hi)
				}
				if sign < 0 && (lo != -math.SmallestNonzeroFloat64 || hi != 0) {
					t.Fatal(lo, hi)
				}
			}
		}
	}
	if f, e := RepresentedExactScalar(new(big.Rat)); e != nil || f != 0 {
		t.Fatal(f, e)
	}
	if _, e := RepresentedExactScalar(nil); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	_, e := RepresentedExactPoint([3]*big.Rat{power(-1100), new(big.Rat), new(big.Rat)})
	if !errors.Is(e, ErrUnrepresentable) {
		t.Fatal(e)
	}
	squared := power(-2200)
	if lo, hi := outwardSqrtRat(squared, true), outwardSqrtRat(squared, false); lo != 0 || hi != math.SmallestNonzeroFloat64 {
		t.Fatal(lo, hi)
	}
}

func TestLosslessCurveExportRejectsUnderflow(t *testing.T) {
	s, _ := NewBezier([]Vec3{{X: 1}, {X: 1}}, nil)
	curve := isoIntersectionFixture(t, s)
	scale := Scaling(Vec3{X: math.Ldexp(1, -550), Y: 1, Z: 1})
	var e error
	curve, e = curve.Transform(scale, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	curve, e = curve.Transform(scale, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = curve.ToBSpline(); !errors.Is(e, ErrUnrepresentable) {
		t.Fatal("lossless spline export rounded underflow", e)
	}
	if _, e = curve.ExactPoint(.5); !errors.Is(e, ErrUnrepresentable) {
		t.Fatal("lossless point export rounded underflow", e)
	}
	jet, e := curve.patch.EvaluateRational(context.Background(), big.NewRat(1, 2), new(big.Rat), SurfacePatchEvaluationOptions{})
	if e != nil || jet.Point[0].Sign() <= 0 || jet.PointBounds.Min.X != 0 || jet.PointBounds.Max.X != math.SmallestNonzeroFloat64 {
		t.Fatal(jet, e)
	}
}
