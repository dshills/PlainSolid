package trimmed

import (
	"context"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

// A plane containing an entire original edge cannot supply either point-only
// fan proof. The raw affine discovery also rejects its multi-side edge ends.
func TestSplineTwoCornerCoincidentEdgeRejected(t *testing.T) {
	ctx := context.Background()
	stock, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, .125)
	raw, _ := makeOpenSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 0}, 0)
	tool, e := raw.Transformed(geom.Affine{{1, 0, 0, 0}, {0, 1, 0, 0}, {.5, 0, 1, 0}})
	if e != nil {
		t.Fatal(e)
	}
	if r, e := CertifySplineFacePointContact(ctx, stock, tool, [2]int{4, 5}, SplineRectangleEmbeddingOptions{}); e == nil || r != nil {
		t.Fatal("zero edge treated as isolated")
	}
	if r, e := CombineSplinePlaneClip(ctx, stock, tool, 5, parametric.Intersection, SplineRectangleEmbeddingOptions{}); e == nil || len(r.Solid.faces) != 0 {
		t.Fatal("coincident edge accepted")
	}
}
