package geom

import (
	"errors"
	"math"
	"testing"
)

func TestSplinePlaneCrossings(t *testing.T) {
	plane, _ := NewPlane(Vec3{}, Vec3{0, 0, 1})
	curve, _ := NewBezier([]Vec3{{0, 0, -4}, {1.0 / 3, 0, 3.5}, {2.0 / 3, 0, -1.75}, {1, 0, 4}}, []float64{1, 2, 4, 1})
	result, e := IntersectSplinePlane(curve, plane, SplinePlaneOptions{})
	if e != nil || len(result.Hits) != 3 {
		t.Fatal(result, e)
	}
	for i, want := range []float64{.2, .5, .8} {
		hit := result.Hits[i]
		if math.Abs(hit.Parameter-want) > 1e-10 || hit.ParameterMin > want || hit.ParameterMax < want || hit.Residual > 1e-8 || hit.Multiplicity != 1 || !hit.Crossing || hit.Tangent {
			t.Fatal(hit, want)
		}
	}
}
func TestSplinePlaneTangencies(t *testing.T) {
	plane, _ := NewPlane(Vec3{}, Vec3{0, 0, 1})
	for _, tt := range []struct {
		z            []float64
		root         float64
		multiplicity int
	}{{[]float64{.25, -.25, .25}, .5, 2}, {[]float64{1, -2, 4}, 1.0 / 3, 2}, {[]float64{-.125, .125, -.125, .125}, .5, 3}} {
		poles := make([]Vec3, len(tt.z))
		for i, z := range tt.z {
			poles[i] = Vec3{float64(i) / float64(len(poles)-1), 0, z}
		}
		curve, _ := NewBezier(poles, nil)
		result, e := IntersectSplinePlane(curve, plane, SplinePlaneOptions{})
		if e != nil || len(result.Hits) != 1 {
			t.Fatal(tt, result, e)
		}
		hit := result.Hits[0]
		if math.Abs(hit.Parameter-tt.root) > 1e-10 || hit.Multiplicity != tt.multiplicity || !hit.Tangent || hit.Crossing != (tt.multiplicity%2 == 1) {
			t.Fatal(tt, hit)
		}
	}
}
func TestSplinePlaneCoincidentAndNear(t *testing.T) {
	plane, _ := NewPlane(Vec3{}, Vec3{0, 0, 1})
	for _, z := range []float64{0, 1e-200} {
		curve, _ := NewBezier([]Vec3{{0, 0, z}, {1, 2, z}, {2, 0, z}}, nil)
		result, e := IntersectSplinePlane(curve, plane, SplinePlaneOptions{})
		if e != nil || len(result.Hits) != 0 || ((len(result.Coincident) == 1) != (z == 0)) {
			t.Fatal(z, result, e)
		}
	}
	curve, _ := NewBSpline(1, []Vec3{{0, 0, 0}, {1, 0, 0}, {2, 0, 1}}, []float64{0, 0, 1, 2, 2}, nil)
	result, e := IntersectSplinePlane(curve, plane, SplinePlaneOptions{})
	if e != nil || len(result.Coincident) != 1 || len(result.Hits) != 1 || result.Coincident[0].Start != 0 || result.Coincident[0].End != 1 || result.Hits[0].Parameter != 1 {
		t.Fatal(result, e)
	}
}
func TestSplinePlaneKnotsAndDiscontinuities(t *testing.T) {
	circle, _ := NewCircle(Vec3{}, Vec3{0, 0, 1}, 1)
	curve, _ := circle.RationalBSpline()
	plane, _ := NewPlane(Vec3{}, Vec3{1, 0, 0})
	result, e := IntersectSplinePlane(curve, plane, SplinePlaneOptions{})
	if e != nil || len(result.Hits) != 4 {
		t.Fatal(result, e)
	}
	for i, want := range []float64{1, 1, 3, 3} {
		if result.Hits[i].Parameter != want || result.Hits[i].Multiplicity != 1 {
			t.Fatal(result)
		}
	}
	plane, _ = NewPlane(Vec3{0, 1, 0}, Vec3{0, 1, 0})
	result, e = IntersectSplinePlane(curve, plane, SplinePlaneOptions{})
	if e != nil || len(result.Hits) != 2 {
		t.Fatal(result, e)
	}
	for _, hit := range result.Hits {
		if hit.Parameter != 1 || hit.Multiplicity != 2 || !hit.Tangent {
			t.Fatal(hit)
		}
	}
	jump, _ := NewBSpline(1, []Vec3{{0, 0, -1}, {1, 0, 0}, {2, 0, 0}, {3, 0, 1}}, []float64{0, 0, 1, 1, 2, 2}, nil)
	plane, _ = NewPlane(Vec3{}, Vec3{0, 0, 1})
	result, e = IntersectSplinePlane(jump, plane, SplinePlaneOptions{})
	if e != nil || len(result.Hits) != 2 {
		t.Fatal(result, e)
	}
	near(t, result.Hits[0].Point, Vec3{1, 0, 0}, 0)
	near(t, result.Hits[1].Point, Vec3{2, 0, 0}, 0)
}
func TestSplinePlaneLimits(t *testing.T) {
	plane, _ := NewPlane(Vec3{}, Vec3{0, 0, 1})
	curve, _ := NewBezier([]Vec3{{0, 0, 1}, {.5, 0, -2}, {1, 0, 4}}, nil)
	for _, options := range []SplinePlaneOptions{{MaxOperations: 1}, {MaxSubdivisions: 1}, {MaxDegree: 1}, {MaxCoefficientBits: 64, ParameterTolerance: 1e-100}} {
		_, e := IntersectSplinePlane(curve, plane, options)
		if !errors.Is(e, ErrConvergence) {
			t.Fatal(options, e)
		}
	}
	unclamped, _ := NewBSpline(2, []Vec3{{0, 0, -1}, {1, 0, 1}, {2, 0, -1}}, []float64{-2, -1, 0, 1, 2, 3}, nil)
	if _, e := IntersectSplinePlane(unclamped, plane, SplinePlaneOptions{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}
