package geom

import (
	"context"
	"errors"
	"math"
	"testing"
)

func exactCompareCurve(t *testing.T, c *BSpline) *ExactSplineCurve {
	t.Helper()
	r, e := NewExactSplineCurve(c, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func TestExactCurveComparisonPartitionsDomainsAndReversal(t *testing.T) {
	a, _ := NewBSpline(1, []Vec3{{}, {1.5, 0, 0}, {3, 0, 0}}, []float64{0, 0, .5, 1, 1}, nil)
	b, _ := NewBSpline(1, []Vec3{{}, {1, 0, 0}, {3, 0, 0}}, []float64{2, 2, 3, 5, 5}, nil)
	ac, bc := exactCompareCurve(t, a), exactCompareCurve(t, b)
	r, e := CompareExactCurves(ac, bc, ExactCurveComparisonOptions{NormalizeDomains: true})
	if e != nil || !r.Complete || !r.Equal || r.ComparedSpans != 3 {
		t.Fatal(r, e)
	}
	_, e = CompareExactCurves(ac, bc, ExactCurveComparisonOptions{})
	if !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}
	reverse, _ := NewBSpline(1, []Vec3{{3, 0, 0}, {1, 0, 0}, {}}, []float64{-4, -4, 0, 2, 2}, nil)
	r, e = CompareExactCurves(ac, exactCompareCurve(t, reverse), ExactCurveComparisonOptions{NormalizeDomains: true, ReverseB: true})
	if e != nil || !r.Equal {
		t.Fatal(r, e)
	}
	r, e = CompareExactCurves(ac, bc, ExactCurveComparisonOptions{NormalizeDomains: true, ReverseB: true})
	if e != nil || !r.Complete || r.Equal {
		t.Fatal(r, e)
	}
}
func TestExactCurveComparisonDegreeAndRationalMultiplier(t *testing.T) {
	a, _ := NewBezier([]Vec3{{}, {1, 0, 0}}, nil)
	ac := exactCompareCurve(t, a)
	quadratic, _ := NewBezier([]Vec3{{}, {.5, 0, 0}, {1, 0, 0}}, nil)
	r, e := CompareExactCurves(ac, exactCompareCurve(t, quadratic), ExactCurveComparisonOptions{})
	if e != nil || !r.Equal {
		t.Fatal(r, e)
	}
	// Exact homogeneous numerator t+2t², denominator 1+2t, so C(t)=t.
	// Stored weights deliberately bypass constructor normalization to preserve
	// these represented integers exactly (normalizing by 3 would round them).
	rational := &BSpline{degree: 2, poles: []Vec3{{}, {.25, 0, 0}, {1, 0, 0}}, knots: []float64{0, 0, 0, 1, 1, 1}, weights: []float64{1, 2, 3}}
	r, e = CompareExactCurves(ac, exactCompareCurve(t, rational), ExactCurveComparisonOptions{})
	if e != nil || !r.Equal {
		t.Fatal(r, e)
	}
	perturbed, e := ac.Transform(Translation(Vec3{0, math.Ldexp(1, -100), 0}), SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e = CompareExactCurves(ac, perturbed, ExactCurveComparisonOptions{})
	if e != nil || !r.Complete || r.Equal || r.DifferentAxis != 1 {
		t.Fatal(r, e)
	}
}
func TestExactCurveComparisonLimitsAndCancellation(t *testing.T) {
	c, _ := NewBezier([]Vec3{{}, {1, 2, 0}, {2, 0, 0}}, nil)
	a := exactCompareCurve(t, c)
	r, e := CompareExactCurves(a, a, ExactCurveComparisonOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) || r.Complete {
		t.Fatal(r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, e = CompareExactCurvesContext(ctx, a, a, ExactCurveComparisonOptions{})
	if !errors.Is(e, context.Canceled) || r.Complete {
		t.Fatal(r, e)
	}
}
