package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestQuadraticPlaneProofAdversaries(t *testing.T) {
	ctx := context.Background()
	stock, surfaces := quadraticClipStock(t)
	if _, e := CertifyQuadraticPlaneFaceSection(ctx, stock, 4, .125, SplineRectangleEmbeddingOptions{}); e == nil {
		t.Fatal("original vertex crossing")
	}
	for _, height := range []float64{.125, .375} {
		tool, _ := makeOpenSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, height}, 0)
		if r, e := CombineQuadraticSplinePlaneClip(ctx, stock, tool, 5, parametric.Intersection, SplineRectangleEmbeddingOptions{}); e == nil || len(r.Solid.faces) != 0 {
			t.Fatal("vertex/edge event", height)
		}
	}
	data := surfaces[4].Data()
	data.Weights = make([]float64, len(data.Poles))
	for i := range data.Weights {
		data.Weights[i] = 1
	}
	data.Weights[4] = 2
	weighted, e := geom.NewBSplineSurface(data.DegreeU, data.DegreeV, data.CountU, data.CountV, data.Poles, data.KnotsU, data.KnotsV, data.Weights)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(weighted, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = quadraticPlaneTrim(p, big.NewRat(3, 16)); e == nil {
		t.Fatal("unsupported positive weights")
	}
	// Two individually regular branches v=1/4 and3/4 must not be mistaken for one.
	data = surfaces[4].Data()
	for i := range data.Poles {
		j := i % data.CountV
		data.Poles[i].Z = 3. / 16
		if j == 1 {
			data.Poles[i].Z = -5. / 16
		}
	}
	multiple, e := geom.NewBSplineSurface(data.DegreeU, data.DegreeV, data.CountU, data.CountV, data.Poles, data.KnotsU, data.KnotsV, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e = geom.NewSurfacePatch(multiple, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = quadraticPlaneTrim(p, new(big.Rat)); e == nil {
		t.Fatal("extra branch")
	}
	broken, e := surfaces[4].InsertUKnot(.5, 3)
	if e != nil {
		t.Fatal(e)
	}
	data = broken.Data()
	for i := data.CountU / 2; i < data.CountU; i++ {
		for j := 0; j < data.CountV; j++ {
			data.Poles[i*data.CountV+j].Z += .125
		}
	}
	broken, e = geom.NewBSplineSurface(data.DegreeU, data.DegreeV, data.CountU, data.CountV, data.Poles, data.KnotsU, data.KnotsV, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e = geom.NewSurfacePatch(broken, geom.SurfacePatchOptions{})
	if e == nil {
		if _, _, e = quadraticPlaneTrim(p, big.NewRat(3, 16)); e == nil {
			t.Fatal("discontinuous branch")
		}
	}
	if _, e = CertifyQuadraticPlaneFaceSection(ctx, stock, 4, 3./16, SplineRectangleEmbeddingOptions{Patches: geom.SurfacePatchOptions{MaxOperations: 1}}); e == nil {
		t.Fatal("budget")
	}
	tool, _ := makeOpenSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 3. / 16}, 0)
	if r, e := CombineQuadraticSplinePlaneClip(ctx, stock, tool, 5, parametric.Intersection, SplineRectangleEmbeddingOptions{Patches: geom.SurfacePatchOptions{MaxOperations: 1}}); e == nil || len(r.Solid.faces) != 0 {
		t.Fatal("assembly work budget")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = CertifyQuadraticPlaneFaceSection(canceled, stock, 4, 3./16, SplineRectangleEmbeddingOptions{}); e != context.Canceled {
		t.Fatal("cancel", e)
	}
}
func TestSewExactQuadraticWholeInterior(t *testing.T) {
	ctx := context.Background()
	stock, _ := quadraticClipStock(t)
	proof, e := CertifyQuadraticPlaneFaceSection(ctx, stock, 4, 3./16, SplineRectangleEmbeddingOptions{})
	if e != nil {
		t.Fatal(e)
	}
	edge := func(q ExactQuadraticUV) parametric.Edge {
		_, p := q.Endpoints()
		return parametric.Edge{Start: 0, End: 1, Curve: surfaceTrimCurve{stock.faces[4].Surface, p[0], p[1], q}, Range: parametric.Interval{Max: 1}}
	}
	q := proof.Trim()
	a, b := edge(q), edge(reversePlaneQuadratic(q))
	if yes, e := compareSewSplineEdges(ctx, a, b, true, edgecheck.Options{}); e != nil || !yes {
		t.Fatal("reverse equality", e)
	}
	if yes, e := compareSewSplineEdges(ctx, a, b, false, edgecheck.Options{}); e != nil || yes {
		t.Fatal("wrong direction", e)
	}
	h := q.HomogeneousControls()
	h[1][1].Add(h[1][1], big.NewRat(1, 32))
	bad, e := NewExactQuadraticUV(h)
	if e != nil {
		t.Fatal(e)
	}
	if yes, e := compareSewSplineEdges(ctx, a, edge(bad), false, edgecheck.Options{}); e != nil || yes {
		t.Fatal("same ends unequal interior", e)
	}
	b = edge(q)
	b.Range.Max = .5
	if yes, e := compareSewSplineEdges(ctx, a, b, false, edgecheck.Options{}); e != nil || yes {
		t.Fatal("unsupported subrange", e)
	}
}
