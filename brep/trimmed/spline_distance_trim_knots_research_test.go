//go:build research

package trimmed

import (
	"bytes"
	"context"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestSplineDistanceKnotWitnessNativeReload(t *testing.T) {
	ctx := context.Background()
	stock := rationalBezierGraphStock(t, 4)
	tool, _ := makeOpenSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 3. / 16}, 0)
	cut, e := CombineRationalGraphSplinePlaneClip(ctx, stock, tool, 5, parametric.Intersection, SplineRectangleEmbeddingOptions{})
	if e != nil {
		t.Fatal(e)
	}
	id := cut.StockFaces[4][0]
	refined, e := cut.Solid.faces[id].Surface.(*geom.BSplineSurface).InsertUKnot(.5, 1)
	if e != nil {
		t.Fatal(e)
	}
	cut.Solid.faces[id].Surface = refined
	var data bytes.Buffer
	if e = WriteJSON(&data, cut.Solid); e != nil {
		t.Fatal(e)
	}
	loaded, e := ReadJSONContext(ctx, &data, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	face := loaded.faces[id]
	p, e := geom.NewSurfacePatch(face.Surface.(*geom.BSplineSurface), geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, use := range face.Outer {
		if _, ok := use.Curve.(ExactBezierUV); !ok {
			continue
		}
		sample, used, hit, e := composedKnotTrimWitness(ctx, p, use, geom.Vec3{X: .3, Y: .6, Z: 3. / 16}, 32, SplineBoundaryDistanceOptions{})
		if e != nil || !hit || used < 2 {
			t.Fatal(sample, used, hit, e)
		}
		q := use.Curve.(ExactBezierUV)
		uv, _, e := q.EvaluateRational(sample.Parameter)
		if e != nil {
			t.Fatal(e)
		}
		original, e := p.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if original.PointBounds != sample.Point {
			t.Fatal("original source binding", original.PointBounds, sample.Point)
		}
		// The public search uses the same original face index after native reload.
		query := geom.Vec3{X: sample.Point.Min.X, Y: sample.Point.Min.Y, Z: sample.Point.Min.Z}
		got, e := loaded.BoundSplineBoundaryDistance(ctx, query, SplineBoundaryDistanceOptions{TrimWitnessCells: 128, MaxCells: 1, SquaredTolerance: 1e-12})
		if e != nil || !got.Complete || !got.HasWitness || got.SquaredMax > 1e-12 {
			t.Fatal(got, e)
		}
		return
	}
	t.Fatal("missing original exact coedge")
}
