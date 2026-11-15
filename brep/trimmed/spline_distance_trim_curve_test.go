package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestSplineDistanceComposedQuarticTrim(t *testing.T) {
	ctx := context.Background()
	stock := rationalBezierGraphStock(t, 4)
	tool, _ := makeOpenSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 3. / 16}, 0)
	cut, err := CombineRationalGraphSplinePlaneClip(ctx, stock, tool, 5, parametric.Intersection, SplineRectangleEmbeddingOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	if err = WriteJSON(&data, cut.Solid); err != nil {
		t.Fatal(err)
	}
	loaded, err := ReadJSONContext(ctx, &data, JSONLimits{})
	if err != nil {
		t.Fatal(err)
	}
	// Query the independently trimmed original nonaffine lower face, not the
	// planar cap which shares its section. Its analytic graph is
	// z=x^4/16+(1+x)y/4, with section z=3/16.
	face := loaded.faces[cut.StockFaces[4][0]]
	s := Solid{faces: []Face{face}}
	var trim ExactBezierUV
	for _, u := range face.Outer {
		if q, ok := u.Curve.(ExactBezierUV); ok {
			trim = q
		}
	}
	if len(trim.control) != 5 {
		t.Fatal("missing original quartic")
	}
	p, _ := geom.NewSurfacePatch(face.Surface.(*geom.BSplineSurface), geom.SurfacePatchOptions{})
	uv, _, err := trim.EvaluateRational(big.NewRat(1, 4))
	if err != nil {
		t.Fatal(err)
	}
	jet, err := p.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	xyz := [3]float64{}
	for i, x := range jet.Point {
		xyz[i], _ = x.Float64()
	}
	query := geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]}
	if math.Abs(query.X*query.X*query.X*query.X/16+(1+query.X)*query.Y/4-3./16) > 1e-15 {
		t.Fatal("independent graph")
	}
	opts := SplineBoundaryDistanceOptions{SquaredTolerance: 1e-12, MaxCells: 1}
	old, e := s.BoundSplineBoundaryDistance(ctx, query, opts)
	if e == nil || old.Complete {
		t.Fatal("fixture no longer demonstrates missing boundary witness", old, e)
	}
	opts.TrimWitnessCells = 64
	got, e := s.BoundSplineBoundaryDistance(ctx, query, opts)
	if e != nil || !got.Complete || !got.HasWitness || got.Face != 0 || got.SquaredMin != 0 || got.SquaredMax > 1e-12 || got.TrimWitnessCells > 64 {
		t.Fatal(got, e)
	}
	t.Logf("default incomplete %g; composed witness gap %g, cells %d", old.SquaredMax, got.SquaredMax, got.TrimWitnessCells)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = s.BoundSplineBoundaryDistance(canceled, query, opts); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	opts.TrimWitnessCells = -1
	if _, e = s.BoundSplineBoundaryDistance(ctx, query, opts); !errors.Is(e, geom.ErrInvalid) {
		t.Fatal(e)
	}
}

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
