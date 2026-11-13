package geom

import (
	"context"
	"errors"
	"math"
	"testing"
)

func isoIntersectionFixture(t *testing.T, curve *BSpline) *SurfaceIsoCurve {
	t.Helper()
	p, e := curveDistancePatch(curve, 1000000, 16384)
	if e != nil {
		t.Fatal(e)
	}
	c, e := p.IsoCurve(1, .3, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func TestSurfaceIsoExactPlaneIntersections(t *testing.T) {
	plane, _ := NewPlane(Vec3{}, Vec3{0, 0, 1})
	for _, tc := range []struct {
		z                   []float64
		count, multiplicity int
		coincident          bool
	}{{[]float64{-1, 1}, 1, 1, false}, {[]float64{.25, -.25, .25}, 1, 2, false}, {[]float64{0, 0}, 0, 0, true}, {[]float64{.5, -.5, -.5, .5}, 2, 1, false}} {
		poles := make([]Vec3, len(tc.z))
		for i, z := range tc.z {
			poles[i] = Vec3{float64(i), 0, z}
		}
		original, _ := NewBezier(poles, nil)
		c := isoIntersectionFixture(t, original)
		r, e := c.IntersectPlane(plane, SplinePlaneOptions{})
		if e != nil || len(r.Hits) != tc.count || (len(r.Coincident) > 0) != tc.coincident {
			t.Fatal(r, e)
		}
		oracle, e := IntersectSplinePlane(original, plane, SplinePlaneOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for i, h := range r.Hits {
			if h.Multiplicity != tc.multiplicity || h.ParameterMin > oracle.Hits[i].ParameterMax || h.ParameterMax < oracle.Hits[i].ParameterMin {
				t.Fatal(r, oracle)
			}
		}
	}
}
func TestSurfaceIsoExactPlaneNoRoundedReconstruction(t *testing.T) {
	s, _ := NewBSplineSurface(1, 1, 2, 2, []Vec3{{0, 0, 0}, {0, 1, 0}, {1, 0, 0}, {1, 1, 0}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, []float64{1, 1, 2, 2})
	p, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	c, _ := p.IsoCurve(0, .5, SurfacePatchOptions{})
	plane, _ := NewPlane(Vec3{2.0 / 3, 0, 0}, Vec3{1, 0, 0})
	r, e := c.IntersectPlane(plane, SplinePlaneOptions{})
	if e != nil || len(r.Hits) != 0 || len(r.Coincident) != 0 {
		t.Fatal(r, e)
	}
}
func TestSurfaceIsoPlaneKnotsLimitsAndCancellation(t *testing.T) {
	original, _ := NewBezier([]Vec3{{0, 0, -1}, {1, 0, 1}}, nil)
	original, _ = original.InsertKnot(.5, 1)
	c := isoIntersectionFixture(t, original)
	plane, _ := NewPlane(Vec3{}, Vec3{0, 0, 1})
	r, e := c.IntersectPlane(plane, SplinePlaneOptions{})
	if e != nil || len(r.Hits) != 2 || r.Hits[0].Parameter != .5 || r.Hits[1].Parameter != .5 {
		t.Fatal(r, e)
	}
	_, e = c.IntersectPlane(plane, SplinePlaneOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = c.IntersectPlaneContext(ctx, plane, SplinePlaneOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	curve, _ := NewBezier([]Vec3{{1, 0, 0}, {1, 1, 0}, {0, 1, 0}}, []float64{1, math.Sqrt(.5), 1})
	c = isoIntersectionFixture(t, curve)
	plane, _ = NewPlane(Vec3{.5, 0, 0}, Vec3{1, 0, 0})
	r, e = c.IntersectPlane(plane, SplinePlaneOptions{})
	if e != nil || len(r.Hits) != 1 || math.Abs(r.Hits[0].Point.Y-math.Sqrt(.75)) > 1e-8 {
		t.Fatal(r, e)
	}
}
