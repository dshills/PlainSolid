package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestSplineDistanceComposedWeightedCubicUse(t *testing.T) {
	ctx := context.Background()
	p, _, _, _ := bezierSeamFixture(t, 4)
	curve, e := geom.NewBezier([]geom.Vec3{{X: 0, Y: .25}, {X: .25, Y: .5}, {X: .75, Y: .25}, {X: 1, Y: .75}}, []float64{1, .5, .75, 1})
	if e != nil {
		t.Fatal(e)
	}
	original, e := geom.NewExactSplineCurve(curve, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	composed, e := p.ComposeParameterCurve(ctx, original, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	target, e := composed.EvaluateJet(.375)
	if e != nil {
		t.Fatal(e)
	}
	for _, reversed := range []bool{false, true} {
		use := Use{Curve: SplineUV{Curve: curve, Range: UVInterval{Min: .25, Max: .75}, Reversed: reversed}}
		uv, model, cells, hit, e := composedSplineTrimWitness(ctx, p, use, target.Point, 16, SplineBoundaryDistanceOptions{SquaredTolerance: 1e-12})
		if e != nil || !hit || cells > 16 {
			t.Fatal(uv, model, cells, hit, e)
		}
		expected, e := original.EvaluateJet(.375)
		if e != nil {
			t.Fatal(e)
		}
		if uv.Min.X > expected.Point.X || uv.Max.X < expected.Point.X || uv.Min.Y > expected.Point.Y || uv.Max.Y < expected.Point.Y {
			t.Fatal("lost original restricted/reversed parameter", uv, expected.Point)
		}
		_, bound := splinePointBoxSquared(target.Point, model)
		if bound.Cmp(big.NewRat(1, 1000000000000)) > 0 {
			t.Fatal("wrong original composed witness", bound)
		}
	}
	// Exhaustion of this optional upper-witness quota cannot turn a curve's
	// lower bound into a face bound or produce a complete face certificate.
	use := Use{Curve: SplineUV{Curve: curve, Range: UVInterval{Min: .25, Max: .75}}}
	_, _, cells, _, e := composedSplineTrimWitness(ctx, p, use, target.Point, 1, SplineBoundaryDistanceOptions{SquaredTolerance: 1e-30})
	if e != nil || cells > 1 {
		t.Fatal(cells, e)
	}
}

func TestSplineDistanceComposedUseQuota(t *testing.T) {
	s := Solid{faces: []Face{{Outer: make([]Use, 1025)}}}
	r, e := s.BoundSplineBoundaryDistance(context.Background(), geom.Vec3{}, SplineBoundaryDistanceOptions{TrimWitnessCells: 1})
	if !errors.Is(e, parametric.ErrBudget) || r.HasWitness || r.Complete || r.SquaredMin != 0 || !math.IsInf(r.SquaredMax, 1) {
		t.Fatal(r, e)
	}
}
