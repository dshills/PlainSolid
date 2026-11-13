package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestSplineCornerAnchor(t *testing.T) {
	ctx := context.Background()
	a, _ := makeOpenSplineBox(t, [3]float64{0, 0, 0}, [3]float64{1, 1, 1}, .125)
	b, _ := makeCornerShearBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 0}, 0)
	boxes := [2]geom.ParameterRectangle{{UMax: .125, VMax: .125}, {UMin: -.125, UMax: .125, VMin: 0, VMax: .25}}
	options := geom.SurfaceTubeBoundaryOptions{Anchor: [4]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat)}}
	r, e := CertifySplineFaceCornerAnchor(ctx, a, b, [2]int{0, 5}, boxes, 0, options, SplineRectangleEmbeddingOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if !r.PendingFan() || r.FacePair() != ([2]int{0, 5}) || r.Operand() != 0 || a.vertices[r.Vertex()] != (geom.Vec3{}) || r.Edges()[0] == r.Edges()[1] {
		t.Fatal("original ownership")
	}
	for i, id := range r.Edges() {
		edge := a.edges[id]
		if r.EdgeEnds()[i] == 0 && edge.Start != r.Vertex() || r.EdgeEnds()[i] == 1 && edge.End != r.Vertex() {
			t.Fatal("edge-end direction")
		}
		if edge.Start != r.Vertex() && edge.End != r.Vertex() {
			t.Fatal("wrong edge")
		}
	}
	for _, v := range r.Point() {
		if v.Sign() != 0 {
			t.Fatal(v)
		}
	}
	badSource := a
	badSource.edges = append([]parametric.Edge(nil), a.edges...)
	badEdge := r.Edges()[0]
	if r.EdgeEnds()[0] == 0 {
		badSource.edges[badEdge].Start = (r.Vertex() + 1) % len(a.vertices)
	} else {
		badSource.edges[badEdge].End = (r.Vertex() + 1) % len(a.vertices)
	}
	if bad, e := CertifySplineFaceCornerAnchor(ctx, badSource, b, [2]int{0, 5}, boxes, 0, options, SplineRectangleEmbeddingOptions{}); e == nil || bad != nil {
		t.Fatal("nonincident edge")
	}
	shifted, e := a.Transformed(geom.Affine{{1, 0, 0, 1}, {0, 1, 0, 0}, {0, 0, 1, 0}})
	if e != nil {
		t.Fatal(e)
	}
	if bad, e := CertifySplineFaceCornerAnchor(ctx, shifted, b, [2]int{0, 5}, boxes, 0, options, SplineRectangleEmbeddingOptions{}); e == nil || bad != nil {
		t.Fatal("shifted source")
	}
	options.Anchor[0].SetInt64(1)
	if r.Point()[0].Sign() != 0 {
		t.Fatal("alias")
	}
	if bad, e := CertifySplineFaceCornerAnchor(ctx, a, b, [2]int{0, 5}, boxes, 0, options, SplineRectangleEmbeddingOptions{}); e == nil || bad != nil {
		t.Fatal("false anchor")
	}
	options.Anchor[0].SetInt64(0)
	for _, pair := range [][2]int{{1, 5}, {0, 4}, {-1, 5}} {
		if bad, e := CertifySplineFaceCornerAnchor(ctx, a, b, pair, boxes, 0, options, SplineRectangleEmbeddingOptions{}); e == nil || bad != nil {
			t.Fatal("source mismatch", pair)
		}
	}
	if bad, e := CertifySplineFaceCornerAnchor(ctx, a, b, [2]int{0, 5}, boxes, 1, options, SplineRectangleEmbeddingOptions{}); e == nil || bad != nil {
		t.Fatal("wrong owner")
	}
	options.MaxOperations = 1
	if bad, e := CertifySplineFaceCornerAnchor(ctx, a, b, [2]int{0, 5}, boxes, 0, options, SplineRectangleEmbeddingOptions{}); e == nil || bad != nil {
		t.Fatal("budget")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if bad, e := CertifySplineFaceCornerAnchor(canceled, a, b, [2]int{0, 5}, boxes, 0, options, SplineRectangleEmbeddingOptions{}); e != context.Canceled || bad != nil {
		t.Fatal("cancel", e)
	}
}
