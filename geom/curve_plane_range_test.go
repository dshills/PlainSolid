package geom

import (
	"context"
	"errors"
	"math"
	"testing"
)

func TestCurvePlaneCertifiedRange(t *testing.T) {
	plane, _ := NewPlane(Vec3{}, Vec3{0, 0, 1})
	for _, tc := range []struct {
		poles    []Vec3
		distance float64
	}{{[]Vec3{{0, 0, 2}, {1, 0, 2}}, 2}, {[]Vec3{{0, 0, -2}, {1, 0, -2}}, 2}, {[]Vec3{{0, 0, -2}, {1, 0, 3}}, 0}} {
		c, _ := NewBezier(tc.poles, nil)
		r, e := BoundCurvePlaneRange(c, plane, SurfacePlaneRangeOptions{})
		if e != nil || !r.Complete || r.DistanceLower != tc.distance || r.DistanceUpper != tc.distance {
			t.Fatal(r, e)
		}
	}
	// Stored nonunit normal must still give Euclidean distance.
	c, _ := NewBezier([]Vec3{{0, 0, 2}, {1, 0, 2}}, nil)
	r, e := BoundCurvePlaneRange(c, Plane{normal: Vec3{0, 0, 3}}, SurfacePlaneRangeOptions{})
	if e != nil || r.DistanceLower != 2 || r.DistanceUpper != 2 {
		t.Fatal(r, e)
	}
}
func TestCurvePlaneCertifiedRationalAndLimits(t *testing.T) {
	c, _ := NewBezier([]Vec3{{1, 0, 0}, {1, 1, 0}, {0, 1, 0}}, []float64{1, math.Sqrt(.5), 1})
	plane, _ := NewPlane(Vec3{}, Vec3{1, 1, 0})
	r, e := BoundCurvePlaneRange(c, plane, SurfacePlaneRangeOptions{Tolerance: .001})
	if e != nil || !r.Complete || r.DistanceLower > math.Sqrt(.5)+1e-14 || r.DistanceUpper < math.Sqrt(.5)-1e-14 || r.Maximum.Min > 1+1e-14 || r.Maximum.Max < 1-1e-14 {
		t.Fatal(r, e)
	}
	r, e = BoundCurvePlaneRange(c, plane, SurfacePlaneRangeOptions{MaxCells: 1})
	if !errors.Is(e, ErrConvergence) || r.Complete || !finite(r.DistanceUpper) {
		t.Fatal(r, e)
	}
	r, e = BoundCurvePlaneRangeContext(distanceCancelContext(4), c, plane, SurfacePlaneRangeOptions{})
	if !errors.Is(e, context.Canceled) || r.Complete || !finite(r.DistanceUpper) {
		t.Fatal(r, e)
	}
}
func TestCurvePlaneCertifiedTangent(t *testing.T) {
	c, _ := NewBezier([]Vec3{{0, 0, .25}, {.5, 0, -.25}, {1, 0, .25}}, nil)
	plane, _ := NewPlane(Vec3{}, Vec3{0, 0, 1})
	r, e := BoundCurvePlaneRange(c, plane, SurfacePlaneRangeOptions{Tolerance: 1e-4})
	if e != nil || !r.Complete || r.DistanceLower != 0 || r.DistanceUpper != 0 {
		t.Fatal(r, e)
	}
}
