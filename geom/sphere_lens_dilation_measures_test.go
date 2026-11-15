package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
)

func TestMeasureDilatedLensWithObliqueIrrationalRim(t *testing.T) {
	ctx := context.Background()
	a, b := Vec3{X: -10, Y: 8, Z: .5}, Vec3{X: -8, Y: 10, Z: .5}
	cap := 2 - math.Sqrt2
	volume0 := 2 * math.Pi * cap * cap * (2 - cap/3)
	area0 := 8 * math.Pi * cap
	curvature := math.Pi * (4*cap + math.Sqrt2*math.Pi/2)
	const d = .75
	volume := volume0 + d*area0 + d*d*curvature + 4*math.Pi/3*d*d*d
	area := area0 + 2*d*curvature + 4*math.Pi*d*d
	var first SphereLensParallelMeasures
	for _, swap := range []bool{false, true} {
		if swap {
			a, b = b, a
		}
		s, err := NewSpherePairMaterial(a, 2, b, 2, SphereSetIntersection)
		if err != nil {
			t.Fatal(err)
		}
		r, err := s.MeasureDilatedLensRational(ctx, big.NewRat(3, 4), SphereLensMeasureOptions{PrecisionBits: 40})
		if err != nil || r.Operations == 0 {
			t.Fatalf("oblique measures: %+v %v", r, err)
		}
		for _, check := range []struct {
			got      ExactParameterInterval
			expected float64
		}{{r.Volume, volume}, {r.Area, area}, {r.OriginalVolume, volume0}, {r.OriginalArea, area0}, {r.IntegratedMeanCurvature, curvature}, {r.RimRadius, math.Sqrt2}, {r.NormalAngle, math.Pi / 2}} {
			lo, _ := check.got.Lower.Float64()
			hi, _ := check.got.Upper.Float64()
			if lo > hi || math.Abs((lo+hi)/2-check.expected) > 1e-10 {
				t.Fatalf("bounds [%.17g, %.17g], expected %.17g", lo, hi, check.expected)
			}
		}
		for _, interval := range []ExactParameterInterval{r.Volume, r.Area} {
			if width := new(big.Rat).Sub(interval.Upper, interval.Lower); width.Sign() <= 0 || width.Cmp(lensMeasurePowerOfTwo(40)) > 0 {
				t.Fatalf("uncertified width %v", width)
			}
		}
		if swap && (r.Volume.Lower.Cmp(first.Volume.Lower) != 0 || r.Area.Upper.Cmp(first.Area.Upper) != 0) {
			t.Fatal("operand order changed the symmetric source result")
		}
		first = r
	}
}

func TestMeasureDilatedLensObtuseAngleAndPrecisionLimits(t *testing.T) {
	ctx := context.Background()
	s, err := NewSpherePairMaterial(Vec3{}, 5, Vec3{Z: 8}, 5, SphereSetIntersection)
	if err != nil {
		t.Fatal(err)
	}
	// The two cap heights are one, their shared rim radius is three, and
	// their normal angle is obtuse. Zero offset must reproduce the source.
	r, err := s.MeasureDilatedLensRational(ctx, new(big.Rat), SphereLensMeasureOptions{})
	if err != nil || r.RimRadius.Lower.Cmp(big.NewRat(3, 1)) != 0 || r.Volume.Lower.Cmp(r.OriginalVolume.Lower) != 0 || r.Area.Upper.Cmp(r.OriginalArea.Upper) != 0 {
		t.Fatalf("zero offset returned %+v %v", r, err)
	}
	lo, _ := r.IntegratedMeanCurvature.Lower.Float64()
	want := math.Pi * (4 + 3*math.Acos(-7./25))
	if math.Abs(lo-want) > 1e-11 {
		t.Fatalf("obtuse mean curvature %.15g want %.15g", lo, want)
	}
	for _, tc := range []struct {
		d    *big.Rat
		o    SphereLensMeasureOptions
		want error
	}{
		{big.NewRat(-1, 2), SphereLensMeasureOptions{}, ErrInvalid},
		{big.NewRat(1, 2), SphereLensMeasureOptions{PrecisionBits: 4}, ErrInvalid},
		{big.NewRat(1, 2), SphereLensMeasureOptions{Arithmetic: SurfacePatchOptions{MaxOperations: 10}}, ErrConvergence},
	} {
		r, err := s.MeasureDilatedLensRational(ctx, tc.d, tc.o)
		if !errors.Is(err, tc.want) || r != (SphereLensParallelMeasures{}) {
			t.Fatalf("unqualified measures: %+v %v", r, err)
		}
	}
}
