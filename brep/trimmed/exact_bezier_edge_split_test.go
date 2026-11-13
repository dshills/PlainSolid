package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestSplitExactBezierTrimEdge(t *testing.T) {
	ctx := context.Background()
	stock := rationalBezierGraphStockFrom(t, func(k int, y float64) float64 {
		// At X=1/2, 1+3X²=7/4. Choose a nonconstant quartic numerator
		// with a factor of seven at this sample, so both trim UVs are binary.
		z := [5]float64{0, -1. / 256, 0, 0, 1. / 16}[k]
		return z + (1+float64(polynomialChoose(k, 2))/2)*y/4
	})
	tool, _ := makeOpenSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{3, 3, 3. / 16}, 0)
	r, e := CombineRationalGraphSplinePlaneClip(ctx, stock, tool, 5, parametric.Intersection, SplineRectangleEmbeddingOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var edge = -1
	for _, u := range r.Solid.faces[r.StockFaces[4][0]].Outer {
		if q, ok := u.Curve.(ExactBezierUV); ok && len(q.control) == 5 {
			edge = u.Edge
		}
	}
	if edge < 0 {
		t.Fatal("missing quartic edge")
	}
	t0 := big.NewRat(1, 2)
	q := r.Solid.edges[edge].Curve.(surfaceTrimCurve).curve.(ExactBezierUV)
	if _, _, e = q.Split(big.NewRat(1, 3)); !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("nonrepresentable split", e)
	}
	if bad, _, e := r.Solid.SplitExactBezierTrimEdge(ctx, edge, big.NewRat(1, 3), edgecheck.Options{}); !errors.Is(e, geom.ErrUnrepresentable) || len(bad.faces) != 0 {
		t.Fatal("nonrepresentable solid split was not atomic", e)
	}
	out, ids, e := r.Solid.SplitExactBezierTrimEdge(ctx, edge, t0, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	if ids.First != edge || len(out.vertices) != len(r.Solid.vertices)+1 || len(out.edges) != len(r.Solid.edges)+1 || len(out.faces) != len(r.Solid.faces) ||
		out.Topology().EulerCharacteristic != r.Solid.Topology().EulerCharacteristic {
		t.Fatal("wrong edit topology", ids, out.Topology())
	}
	if out.vertices[ids.Vertex] != (geom.Vec3{X: .5, Y: 27. / 64, Z: 3. / 16}) {
		t.Fatal("wrong exact new model vertex", out.vertices[ids.Vertex])
	}
	for i, id := range [2]int{ids.First, ids.Second} {
		c := out.edges[id].Curve.(surfaceTrimCurve).curve.(ExactBezierUV)
		newUV, _, e := c.EvaluateRational(big.NewRat(1, 7))
		if e != nil {
			t.Fatal(e)
		}
		original, _, e := q.EvaluateRational(big.NewRat(int64(1+7*i), 14))
		if e != nil || original[0].Cmp(newUV[0]) != 0 || original[1].Cmp(newUV[1]) != 0 {
			t.Fatal("changed curve under reparameterization", e)
		}
	}
	before, e := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteJSON(&b, out); e != nil {
		t.Fatal(e)
	}
	reloaded, e := ReadJSONContext(ctx, &b, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	after, e := reloaded.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(before.Volume-after.Volume) > 1e-9 || before.Centroid.Sub(after.Centroid).Length() > 1e-9 {
		t.Fatal("split or reload changed material", before, after, e)
	}
}
