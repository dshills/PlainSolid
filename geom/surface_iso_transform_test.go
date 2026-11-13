package geom

import (
	"errors"
	"math"
	"testing"
)

func TestExactIsoTransformEvaluationAndPlane(t *testing.T) {
	curve, _ := NewBezier([]Vec3{{1, 0, 0}, {1, 1, 0}, {0, 1, 0}}, []float64{1, math.Sqrt(.5), 1})
	iso := isoIntersectionFixture(t, curve)
	m := Affine{{2, 1, 0, 3}, {0, -3, 0, 4}, {0, 0, 2, 5}}
	mapped, e := iso.Transform(m, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, u := range []float64{0, .2, .5, .9, 1} {
		p, _ := iso.Eval(u)
		q, e := mapped.Eval(u)
		if e != nil {
			t.Fatal(e)
		}
		near(t, q, m.Apply(p), 2e-14)
		d, _ := iso.Derivative(u)
		dm, e := mapped.Derivative(u)
		if e != nil {
			t.Fatal(e)
		}
		near(t, dm, m.ApplyVector(d), 2e-14)
	}
	plane, _ := NewPlane(Vec3{0, 0, 5}, Vec3{0, 0, 1})
	r, e := mapped.IntersectPlane(plane, SplinePlaneOptions{})
	if e != nil || len(r.Coincident) != 1 || len(r.Hits) != 0 {
		t.Fatal(r, e)
	}
	// y=2.5 corresponds exactly to source y=.5, with x=3+sqrt(3)+.5.
	plane, _ = NewPlane(Vec3{0, 2.5, 0}, Vec3{0, 1, 0})
	r, e = mapped.IntersectPlane(plane, SplinePlaneOptions{})
	if e != nil || len(r.Hits) != 1 || math.Abs(r.Hits[0].Point.X-(3.5+math.Sqrt(3))) > 1e-8 {
		t.Fatal(r, e)
	}
}
func TestExactIsoTransformRetainsSubULPTranslation(t *testing.T) {
	curve, _ := NewBezier([]Vec3{{1, 0, 0}, {1, 1, 0}}, nil)
	iso := isoIntersectionFixture(t, curve)
	mapped, e := iso.Transform(Translation(Vec3{math.Ldexp(1, -54), 0, 0}), SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	plane, _ := NewPlane(Vec3{1, 0, 0}, Vec3{1, 0, 0})
	r, e := mapped.BoundPlaneRange(plane, SurfacePlaneRangeOptions{})
	if e != nil || r.StrictSign != 1 || r.Minimum.Min != math.Ldexp(1, -54) || r.Minimum.Max != math.Ldexp(1, -54) {
		t.Fatal(r, e)
	}
	hits, e := mapped.IntersectPlane(plane, SplinePlaneOptions{})
	if e != nil || len(hits.Hits) != 0 || len(hits.Coincident) != 0 {
		t.Fatal(hits, e)
	}
	original, e := iso.IntersectPlane(plane, SplinePlaneOptions{})
	if e != nil || len(original.Coincident) != 1 {
		t.Fatal(original, e)
	}
}
func TestExactIsoTransformSingularAndLimits(t *testing.T) {
	c, _ := NewBezier([]Vec3{{}, {1, 2, 3}}, nil)
	iso := isoIntersectionFixture(t, c)
	collapsed, e := iso.Transform(Affine{}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := collapsed.BoundCurvature(CurveDerivativeBoundOptions{})
	if e != nil || !r.Cells[0].IdenticallyStationary {
		t.Fatal(r, e)
	}
	_, e = iso.Transform(Identity(), SurfacePatchOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	bad := Identity()
	bad[0][0] = math.NaN()
	_, e = iso.Transform(bad, SurfacePatchOptions{})
	if !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}
