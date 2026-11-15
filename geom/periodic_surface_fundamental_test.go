package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestPeriodicFundamentalExactMapAndContinuity(t *testing.T) {
	data := BSplineSurfaceData{
		DegreeU: 2, DegreeV: 1, CountU: 3, CountV: 2,
		KnotsV:  []float64{2, 2, 4, 4},
		Poles:   []Vec3{{0, 0, 0}, {0, 0, 2}, {1, 0, 0}, {1, 0, 2}, {0, 1, 0}, {0, 1, 2}},
		Weights: []float64{1, 2, 3, 2, 4, 1},
	}
	surface, err := NewPeriodicBSplineSurface(data, &PeriodicSplineAxis{Start: -1.25, Period: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	o := PeriodicSurfaceFundamentalOptions{ContinuityOrder: 1}
	fundamental, err := surface.ExactFundamentalPatch(ctx, o)
	if err != nil || fundamental.Wrapped == nil || fundamental.Seam.PositionMatched != [2]bool{true, false} || fundamental.Seam.FirstPartialsMatched != [2]bool{true, false} {
		t.Fatalf("wrapped proof = %+v, %v", fundamental, err)
	}
	if fundamental.Maps[0].Offset.Cmp(big.NewRat(-5, 4)) != 0 || fundamental.Maps[0].Scale.Cmp(big.NewRat(1, 3)) != 0 || fundamental.Maps[1].Offset.Sign() != 0 || fundamental.Maps[1].Scale.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatalf("wrong parameter correspondence %+v", fundamental.Maps)
	}
	d := fundamental.Wrapped.Domain()
	if d != (ParameterRectangle{0, 3, 2, 4}) {
		t.Fatalf("working rectangle = %+v", d)
	}
	for _, u := range []*big.Rat{big.NewRat(-7, 3), big.NewRat(0, 1), big.NewRat(1, 1), big.NewRat(17, 2)} {
		v := big.NewRat(13, 4)
		original := new(big.Rat).Add(big.NewRat(-5, 4), new(big.Rat).Mul(u, big.NewRat(1, 3)))
		expected, err := surface.EvaluateRational(ctx, original, v, SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		actual, err := fundamental.Wrapped.EvaluateRational(ctx, u, v, SurfacePatchEvaluationOptions{})
		if err != nil {
			t.Fatal(err)
		}
		for axis := range actual.Point {
			if actual.Point[axis].Cmp(expected.Point[axis]) != 0 || new(big.Rat).Mul(actual.Du[axis], big.NewRat(3, 1)).Cmp(expected.Du[axis]) != 0 || actual.Dv[axis].Cmp(expected.Dv[axis]) != 0 {
				t.Fatalf("source and working jets do not agree at %v,%v", u, v)
			}
		}
	}
	o.MaxOperations = fundamental.ExactOperations
	full, err := surface.ExactFundamentalPatch(ctx, o)
	if err != nil || full.Wrapped == nil || full.ExactOperations != fundamental.ExactOperations {
		t.Fatalf("exact operation limit = %+v, %v", full, err)
	}
	o.MaxOperations--
	limited, err := surface.ExactFundamentalPatch(ctx, o)
	if !errors.Is(err, ErrConvergence) || limited.Wrapped != nil {
		t.Fatalf("past-budget seam proof = %+v, %v", limited, err)
	}
}

func TestPeriodicFundamentalPlaneSectionAcrossSeam(t *testing.T) {
	surface := periodicDistanceSurface(t)
	ctx := context.Background()
	fundamental, err := surface.ExactFundamentalPatch(ctx, PeriodicSurfaceFundamentalOptions{})
	if err != nil || fundamental.Wrapped == nil || fundamental.Seam.PositionMatched != [2]bool{true, false} {
		t.Fatalf("C0 fundamental = %+v, %v", fundamental, err)
	}
	if r, err := surface.ExactFundamentalPatch(ctx, PeriodicSurfaceFundamentalOptions{ContinuityOrder: 1}); !errors.Is(err, ErrDegenerate) || r.Wrapped != nil {
		t.Fatalf("a degree-one polygonal seam cannot prove matching partials: %+v, %v", r, err)
	}
	plane, err := NewPlane(Vec3{Z: 2.5}, Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	if strict, err := TraceWrappedSurfacePlaneContext(ctx, fundamental.Wrapped, plane, SurfaceTraceOptions{}); !errors.Is(err, ErrConvergence) || strict.Complete || len(strict.SeamJoins) != 0 {
		t.Fatalf("strict seam mode accepted a crease: complete=%t, joins=%d, error=%v", strict.Complete, len(strict.SeamJoins), err)
	}
	trace, err := TraceWrappedSurfacePlaneContext(ctx, fundamental.Wrapped, plane, SurfaceTraceOptions{AllowC0SeamContinuation: true})
	if err != nil || !trace.Complete || len(trace.Contours) != 1 || !trace.Contours[0].Closed || len(trace.SeamJoins) != 1 {
		t.Fatalf("section did not close across the original seam: complete=%t, contours=%d, joins=%d, error=%v", trace.Complete, len(trace.Contours), len(trace.SeamJoins), err)
	}
	if len(trace.Contours[0].Arcs) != 3 || trace.SeamJoins[0].Axis != 0 || !trace.SeamJoins[0].RootCertificate.C0Continuation {
		t.Fatalf("section lost an original facet or seam: %+v", trace)
	}
}
