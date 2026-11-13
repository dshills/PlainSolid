package geom

import (
	"errors"
	"math"
	"math/rand"
	"testing"
)

func TestAnalyticSurfaceProjections(t *testing.T) {
	frame, _ := FrameFromNormal(Vec3{2, -1, 3}, Vec3{1, 2, 3})
	plane, _ := NewPlaneSurface(frame)
	cylinder, _ := NewCylinderSurface(frame, 2)
	sphere, _ := NewSphereSurface(frame, 3)
	torus, _ := NewTorusSurface(frame, 4, 1)
	type projector interface {
		Project(Vec3) (SurfaceProjection, error)
	}
	surfaces := []projector{plane, cylinder, sphere, torus}
	r := rand.New(rand.NewSource(9931))
	for _, surface := range surfaces {
		for i := 0; i < 100; i++ {
			p := Vec3{r.Float64()*20 - 10, r.Float64()*20 - 10, r.Float64()*20 - 10}
			result, e := surface.Project(p)
			if e != nil || result.Residual > 2e-13 || !finite(result.Distance) {
				t.Fatal(result, e)
			}
			local := frame.ToLocal(p)
			want := 0.0
			switch surface.(type) {
			case PlaneSurface:
				want = math.Abs(local.Z)
			case CylinderSurface:
				want = math.Abs(math.Hypot(local.X, local.Y) - 2)
			case SphereSurface:
				want = math.Abs(local.Length() - 3)
			case TorusSurface:
				want = math.Abs(math.Hypot(math.Hypot(local.X, local.Y)-4, local.Z) - 1)
			}
			if math.Abs(result.Distance-want) > 2e-13 {
				t.Fatal(result, want)
			}
		}
	}
	identity, _ := FrameFromNormal(Vec3{}, Vec3{0, 0, 1})
	cylinder, _ = NewCylinderSurface(identity, 2)
	sphere, _ = NewSphereSurface(identity, 3)
	torus, _ = NewTorusSurface(identity, 4, 1)
	result, e := cylinder.Project(Vec3{0, 0, 7})
	if e != nil || result.Status != ProjectionNonUnique {
		t.Fatal(result, e)
	}
	result, e = sphere.Project(Vec3{0, 0, 7})
	if e != nil || result.Status != ProjectionSingularParameters {
		t.Fatal(result, e)
	}
	result, e = sphere.Project(Vec3{})
	if e != nil || result.Status != ProjectionNonUnique {
		t.Fatal(result, e)
	}
	for _, p := range []Vec3{{0, 0, 2}, {4, 0, 0}} {
		result, e = torus.Project(p)
		if e != nil || result.Status != ProjectionNonUnique {
			t.Fatal(result, e)
		}
	}
}
func TestGlobalSplineProjection(t *testing.T) {
	circle, _ := NewCircle(Vec3{}, Vec3{0, 0, 1}, 2)
	spline, _ := circle.RationalBSpline()
	p := Vec3{3, 4, 2}
	result, e := spline.Project(p, CurveProjectionOptions{DistanceTolerance: 1e-7})
	want := math.Sqrt(13)
	if e != nil || math.Abs(result.Distance-want) > 1e-7 || result.ErrorBound > 1e-7 || result.LowerBound > want+1e-12 {
		t.Fatal(result, e, want)
	}
	point, _ := spline.Eval(result.Parameter)
	near(t, result.Point, point, 1e-13)
	result, e = spline.Project(Vec3{}, CurveProjectionOptions{DistanceTolerance: 1e-3, MaxSplits: 4096})
	if e != nil || math.Abs(result.Distance-2) > 1e-12 || result.ErrorBound > 1e-3 {
		t.Fatal(result, e)
	}
	line, _ := NewBezier([]Vec3{{0, 0, 0}, {10, 0, 0}}, nil)
	result, e = line.Project(Vec3{3, 2, 0}, CurveProjectionOptions{})
	if e != nil || result.Distance-2 > 1e-7 {
		t.Fatal(result, e)
	}
	result, e = spline.Project(p, CurveProjectionOptions{DistanceTolerance: 1e-15, MaxSplits: 1})
	if !errors.Is(e, ErrConvergence) || result.Splits != 1 || !finite(result.Distance) || result.ErrorBound <= 0 {
		t.Fatal(result, e)
	}
}
func TestSplineProjectionRejectsUnsupported(t *testing.T) {
	unclamped, _ := NewBSpline(2, []Vec3{{0, 0, 0}, {1, 1, 0}, {2, 0, 0}}, []float64{-2, -1, 0, 1, 2, 3}, nil)
	if _, e := unclamped.Project(Vec3{}, CurveProjectionOptions{}); e == nil {
		t.Fatal("unclamped accepted")
	}
	jump, _ := NewBSpline(1, []Vec3{{0, 0, 0}, {1, 0, 0}, {3, 0, 0}, {4, 0, 0}}, []float64{0, 0, 1, 1, 2, 2}, nil)
	if _, e := jump.Project(Vec3{}, CurveProjectionOptions{}); e == nil {
		t.Fatal("discontinuous accepted")
	}
}
func TestMultimodalSplineProjection(t *testing.T) {
	curve, _ := NewBezier([]Vec3{{-3, 0, 0}, {5, 8, 0}, {-5, 8, 0}, {3, 0, 0}}, []float64{1, 2, 2, 1})
	for _, p := range []Vec3{{0, 1, 2}, {0, 5, 0}, {-4, 2, 0}, {4, 6, 1}} {
		result, e := curve.Project(p, CurveProjectionOptions{DistanceTolerance: 1e-6})
		if e != nil {
			t.Fatal(result, e)
		}
		sampled := math.Inf(1)
		for i := 0; i <= 10000; i++ {
			q, _ := curve.Eval(float64(i) / 10000)
			sampled = math.Min(sampled, p.Distance(q))
		}
		if result.Distance > sampled+1e-6 || sampled < result.LowerBound-1e-10 {
			t.Fatal(result, sampled)
		}
	}
}
