package trimmed

import (
	"context"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestSplineCornerSectionReversedEdgeRanges(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	a, _ := makeOpenSplineBox(t, [3]float64{0, 0, 0}, [3]float64{1, 1, 1}, .125)
	b, _ := makeCornerShearBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 0}, 0)
	vertices, edges, faces := a.Vertices(), a.Edges(), a.Faces()
	for id, old := range edges {
		curve, e := geom.NewBSpline(1, []geom.Vec3{vertices[old.End], vertices[old.Start]}, []float64{2, 2, 6, 6}, nil)
		if e != nil {
			t.Fatal(e)
		}
		edges[id] = parametric.Edge{Start: old.End, End: old.Start, Curve: curve, Range: parametric.Interval{Min: 2, Max: 6}}
	}
	for i := range faces {
		for j := range faces[i].Outer {
			faces[i].Outer[j].Reversed = !faces[i].Outer[j].Reversed
		}
	}
	a, e := NewSolidContext(ctx, vertices, edges, faces, a.tolerance)
	if e != nil {
		t.Fatal(e)
	}
	fan := cornerLedgerFromSolids(t, a, b)
	ledger, e := DiagnoseSplineCornerSections(ctx, fan, SplineCornerSectionLedgerOptions{})
	if e != nil {
		t.Fatal(e)
	}
	ledger, e = ResolveSplineCornerAffineSections(ctx, ledger, SplineRectangleEmbeddingOptions{})
	if e != nil {
		t.Fatal(e)
	}
	graph, e := BuildSplineCornerSectionGraph(ctx, ledger, SplineRectangleEmbeddingOptions{})
	if e != nil {
		t.Fatal(e)
	}
	count := 0
	for _, node := range graph.Nodes() {
		if node.Vertex >= 0 {
			continue
		}
		count++
		parameter, exact := node.Parameter.Float64()
		if !exact || parameter <= 2 || parameter >= 6 {
			t.Fatal("nonunit parameter", node.Parameter)
		}
		p, e := a.edges[node.Edge].Curve.Eval(parameter)
		if e != nil {
			t.Fatal(e)
		}
		for i, q := range node.Point {
			v, exact := q.Float64()
			if !exact || v != []float64{p.X, p.Y, p.Z}[i] {
				t.Fatal("independent source evaluation", node, p)
			}
		}
	}
	if count != 3 {
		t.Fatal("edge events")
	}
}
