package geom

import (
	"errors"
	"testing"
)

func TestEvaluateSurfacePairTubeAffine(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	a, _ = a.Trim(ParameterRectangle{.2, .8, 0, 1})
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{s, .4, t - .3} })
	proof, e := NewVerifiedSurfaceTube(a, b, SurfacePairTubeOptions{})
	if e != nil || proof.FreeTangentSign() == 0 {
		t.Fatal(proof, e)
	}
	for _, free := range []float64{.2, .5, .8} {
		r, e := EvaluateSurfacePairTube(a, b, SurfaceTubeEvaluationOptions{FreeAxis: 0, Parameter: free})
		if e != nil || !r.Complete || !r.Bounds.Contains(Vec3{free, .4, 0}) {
			t.Fatal(r, e)
		}
		if r.ParametersA.Min.X != free || r.ParametersA.Max.X != free {
			t.Fatal(r)
		}
		p, e := proof.Evaluate(SurfaceTubeEvaluationOptions{Parameter: free})
		if e != nil || p.Bounds != r.Bounds || p.ParametersA != r.ParametersA || p.ParametersB != r.ParametersB ||
			p.ExactOperations >= r.ExactOperations {
			t.Fatal(p, r, e)
		}
	}
}
func TestEvaluateSurfacePairTubeNonlinear(t *testing.T) {
	source := quarterCylinder(t)
	a, _ := NewSurfacePatch(source, SurfacePatchOptions{})
	a, _ = a.Trim(ParameterRectangle{.25, .35, 0, 1})
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{2 * s, 2 * t, .75} })
	r, e := EvaluateSurfacePairTube(a, b, SurfaceTubeEvaluationOptions{FreeAxis: 0, Parameter: .31})
	if e != nil || !r.Complete {
		t.Fatal(r, e)
	}
	point, _ := source.Eval(.31, .375)
	if point.Distance(r.Point) > 1e-8 {
		t.Fatal(point, r)
	}
	if r.ParametersA.Min.Y > .375 || r.ParametersA.Max.Y < .375 {
		t.Fatal(r)
	}
	if _, e = EvaluateSurfacePairTube(a, b, SurfaceTubeEvaluationOptions{FreeAxis: 0, Parameter: .9}); !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}
	if r, e = EvaluateSurfacePairTube(a, b, SurfaceTubeEvaluationOptions{FreeAxis: 0, Parameter: .31, MaxOperations: 1}); !errors.Is(e, ErrConvergence) || r.Complete {
		t.Fatal(r, e)
	}
}

func TestEvaluateSurfacePairTubeContractsNonlinearSection(t *testing.T) {
	a, _ := NewSurfacePatch(quadraticContactSurface(t, 0), SurfacePatchOptions{})
	a, _ = a.Trim(ParameterRectangle{.35, .45, .4, .6})
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{s, .5, t - .1} })
	r, e := EvaluateSurfacePairTube(a, b, SurfaceTubeEvaluationOptions{FreeAxis: 0, Parameter: .4})
	if e != nil || !r.Complete || r.Point.Distance(Vec3{.4, .5, .01}) > 1e-8 {
		t.Fatal(r, e)
	}
	partial, e := EvaluateSurfacePairTube(a, b, SurfaceTubeEvaluationOptions{FreeAxis: 0, Parameter: .4, MaxIterations: 1})
	if !errors.Is(e, ErrConvergence) || partial.Complete || partial.ParametersB.Min.Y > .11 || partial.ParametersB.Max.Y < .11 {
		t.Fatal(partial, e)
	}
}
