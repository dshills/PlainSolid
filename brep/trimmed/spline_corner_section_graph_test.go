package trimmed

import (
	"context"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestSplineCornerSectionGraph(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	fan := cornerLedgerFixture(t)
	partial, e := DiagnoseSplineCornerSections(ctx, fan, SplineCornerSectionLedgerOptions{})
	if e != nil {
		t.Fatal(e)
	}
	full, e := ResolveSplineCornerAffineSections(ctx, partial, SplineRectangleEmbeddingOptions{})
	if e != nil {
		t.Fatal(e)
	}
	graph, e := BuildSplineCornerSectionGraph(ctx, full, SplineRectangleEmbeddingOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if !graph.PendingAssembly() || len(graph.Nodes()) != 4 || len(graph.Arcs()) != 4 {
		t.Fatal("graph")
	}
	vertices, edges := 0, map[int]bool{}
	for _, node := range graph.Nodes() {
		if node.Vertex >= 0 {
			vertices++
			if node.Vertex != fan.vertex || node.Edge != -1 || node.Parameter != nil {
				t.Fatal("vertex ownership")
			}
		} else {
			if node.Edge < 0 || node.Parameter == nil || edges[node.Edge] {
				t.Fatal("edge ownership")
			}
			edges[node.Edge] = true
		}
	}
	if vertices != 1 || len(edges) != 3 {
		t.Fatal("endpoint ownership")
	}
	nodes := graph.Nodes()
	nodes[0].Point[0].SetInt64(99)
	if graph.Nodes()[0].Point[0].RatString() == "99" {
		t.Fatal("alias")
	}
	if bad, e := BuildSplineCornerSectionGraph(ctx, partial, SplineRectangleEmbeddingOptions{}); e == nil || bad != nil {
		t.Fatal("incomplete ledger")
	}
	for mode := 0; mode < 3; mode++ {
		bad := *full
		bad.pairs = full.Pairs()
		switch mode {
		case 0:
			bad.pairs = bad.pairs[:35]
		case 1:
			bad.pairs[0] = bad.pairs[1]
		case 2:
			for i, p := range bad.pairs {
				if p.Kind == SplinePairBranches {
					bad.pairs[i].Branches = 2
					break
				}
			}
		}
		if r, e := BuildSplineCornerSectionGraph(ctx, &bad, SplineRectangleEmbeddingOptions{}); e == nil || r != nil {
			t.Fatal("missing/extra/wrong graph", mode)
		}
	}
	if bad, e := BuildSplineCornerSectionGraph(ctx, full, SplineRectangleEmbeddingOptions{Patches: geom.SurfacePatchOptions{MaxOperations: 1}}); e == nil || bad != nil {
		t.Fatal("budget")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if bad, e := BuildSplineCornerSectionGraph(canceled, full, SplineRectangleEmbeddingOptions{}); e != context.Canceled || bad != nil {
		t.Fatal("cancel", e)
	}
}
