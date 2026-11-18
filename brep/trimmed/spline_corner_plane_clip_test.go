package trimmed

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestSplineCornerPlaneClip(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	fan := cornerLedgerFixture(t)
	partial, e := DiagnoseSplineCornerSections(ctx, fan, SplineCornerSectionLedgerOptions{})
	if e != nil {
		t.Fatal(e)
	}
	ledger, e := ResolveSplineCornerAffineSections(ctx, partial, SplineRectangleEmbeddingOptions{})
	if e != nil {
		t.Fatal(e)
	}
	graph, e := BuildSplineCornerSectionGraph(ctx, ledger, SplineRectangleEmbeddingOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		op     parametric.BooleanOp
		volume float64
	}{{parametric.Intersection, 7. / 32}, {parametric.Difference, 25. / 32}} {
		result, e := CombineSplineCornerPlaneClip(ctx, graph, tc.op, SplineRectangleEmbeddingOptions{})
		if e != nil {
			t.Fatal(tc.op, e)
		}
		t.Log(tc.op, result.Solid.Topology())
		if result.Solid.Topology().EulerCharacteristic != 2 || len(result.Solid.faces) != 6 || len(result.ClippedStockFaces) != 4 || len(result.KeptStockFaces) != 1 || len(result.DiscardedStockFaces) != 1 {
			t.Fatal("topology/classification")
		}
		if result.OriginalVertices[fan.vertex] < 0 || result.Solid.vertices[result.OriginalVertices[fan.vertex]] != fan.anchors[0].solids[0].vertices[fan.vertex] {
			t.Fatal("original corner")
		}
		mass, e := result.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(mass.Volume-tc.volume) > 1e-10 {
			t.Fatal("analytic volume", mass, e)
		}
		wantCenter := geom.Vec3{X: 4. / 7, Y: 4. / 7, Z: 41. / 252}
		if tc.op == parametric.Difference {
			wantCenter = geom.Vec3{X: 12. / 25, Y: 12. / 25, Z: 571. / 900}
		}
		if mass.Centroid.Sub(wantCenter).Length() > 1e-10 {
			t.Fatal("analytic centroid", mass.Centroid, wantCenter)
		}
		for source, ids := range result.StockFaces {
			for _, id := range ids {
				if result.Solid.faces[id].Surface != fan.anchors[0].solids[0].faces[source].Surface {
					t.Fatal("changed stock support")
				}
			}
		}
		if len(result.ToolFaces[5]) != 1 || result.Solid.faces[result.ToolFaces[5][0]].Surface != fan.anchors[0].solids[1].faces[5].Surface {
			t.Fatal("cap support")
		}
		owned := map[int]bool{}
		for _, ids := range result.StockEdges {
			for _, id := range ids {
				if owned[id] {
					t.Fatal("duplicate source edge")
				}
				owned[id] = true
			}
		}
		for _, id := range result.SectionEdges {
			if owned[id] {
				t.Fatal("section misowned")
			}
			owned[id] = true
		}
		if len(owned) != len(result.Solid.edges) || len(result.SectionEdges) != 4 {
			t.Fatal("edge ancestry")
		}
		var data bytes.Buffer
		if e = WriteJSON(&data, result.Solid); e != nil {
			t.Fatal(e)
		}
		restored, e := ReadJSONContext(ctx, &data, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		m, e := restored.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(m.Volume-tc.volume) > 1e-10 {
			t.Fatal("reload", e, m)
		}
		mesh, e := restored.Tessellate(ctx, parametric.TessellationOptions{Deflection: .05})
		if e != nil {
			t.Fatal(e)
		}
		if e = mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		contacts, e := collision.SelfIntersectionsBVHContext(ctx, mesh, collision.Options{})
		if e != nil || !contacts.Complete || len(contacts.Contacts) != 0 {
			t.Fatal("display contact", e)
		}
	}
	if bad, e := CombineSplineCornerPlaneClip(ctx, graph, parametric.Union, SplineRectangleEmbeddingOptions{}); e == nil || len(bad.Solid.Faces()) != 0 {
		t.Fatal("unsupported union")
	}
	if bad, e := CombineSplineCornerPlaneClip(ctx, graph, parametric.Intersection, SplineRectangleEmbeddingOptions{Plane: geom.SurfaceSupportingPlaneOptions{MaxOperations: 1}}); e == nil || len(bad.Solid.Faces()) != 0 {
		t.Fatal("halfspace budget")
	}
	// Private adversarial mutation tests the mandatory inactive-halfspace gate:
	// an otherwise parallel cutter floor enters the stock. Public graph evidence
	// is immutable, so callers cannot perform this mutation.
	narrow, _ := makeCornerShearBox(t, [3]float64{-1, -1, -.125}, [3]float64{2, 2, 0}, 0)
	badFan := *fan
	badAnchor := *fan.anchors[0]
	badAnchor.solids[1] = narrow
	badFan.anchors[0] = &badAnchor
	badLedger := *ledger
	badLedger.fan = &badFan
	badGraph := *graph
	badGraph.ledger = &badLedger
	if bad, e := CombineSplineCornerPlaneClip(ctx, &badGraph, parametric.Intersection, SplineRectangleEmbeddingOptions{}); e == nil || len(bad.Solid.Faces()) != 0 {
		t.Fatal("unproved second active plane")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if bad, e := CombineSplineCornerPlaneClip(canceled, graph, parametric.Intersection, SplineRectangleEmbeddingOptions{}); e != context.Canceled || len(bad.Solid.Faces()) != 0 {
		t.Fatal("cancel", e)
	}
}
