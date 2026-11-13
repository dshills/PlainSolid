package trimmed

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestSplinePlaneClipNoOriginalVertex(t *testing.T) {
	ctx := context.Background()
	stock, _ := makeOpenSplineBox(t, [3]float64{0, 0, 0}, [3]float64{1, 1, 1}, .125)
	base, _ := makeCornerShearBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 0}, 0)
	tool, e := base.Transformed(geom.Affine{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, .125}})
	if e != nil {
		t.Fatal(e)
	}
	for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Difference} {
		result, e := CombineSplinePlaneClip(ctx, stock, tool, 5, op, SplineRectangleEmbeddingOptions{})
		if e != nil {
			t.Fatal(op, e)
		}
		mass, e := result.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		volume, center := 11./32, geom.Vec3{X: 6. / 11, Y: 6. / 11, Z: 43. / 198}
		if op == parametric.Difference {
			volume = 21. / 32
			center = geom.Vec3{X: 10. / 21, Y: 10. / 21, Z: 263. / 378}
		}
		if math.Abs(mass.Volume-volume) > 1e-10 || mass.Centroid.Sub(center).Length() > 1e-10 {
			t.Fatal("independent integral", mass, volume, center)
		}
		if result.Solid.Topology().EulerCharacteristic != 2 || len(result.SectionEdges) != 4 || len(result.ClippedStockFaces) != 4 {
			t.Fatal("topology")
		}
		owned := map[int]bool{}
		for _, ids := range result.StockEdges {
			for _, id := range ids {
				if owned[id] {
					t.Fatal("duplicate ancestry")
				}
				owned[id] = true
			}
		}
		for _, id := range result.SectionEdges {
			if owned[id] {
				t.Fatal("section ancestry")
			}
			owned[id] = true
		}
		if len(owned) != len(result.Solid.edges) {
			t.Fatal("missing ancestry")
		}
		cap := result.Solid.faces[result.ToolFaces[5][0]]
		if cap.Surface != tool.faces[5].Surface || cap.Reversed != (op == parametric.Difference) {
			t.Fatal("cap orientation")
		}
		retained := 0
		for _, id := range result.OriginalVertices {
			if id >= 0 {
				retained++
			}
		}
		if retained != 4 {
			t.Fatal("original vertices", retained)
		}
		var native bytes.Buffer
		if e = WriteJSON(&native, result.Solid); e != nil {
			t.Fatal(e)
		}
		loaded, e := ReadJSONContext(ctx, &native, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		reload, e := loaded.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(reload.Volume-volume) > 1e-10 {
			t.Fatal("native reload", e)
		}
		mesh, e := result.Solid.Tessellate(ctx, parametric.TessellationOptions{Deflection: .05})
		if e != nil {
			t.Fatal(e)
		}
		if e = mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		contacts, e := collision.SelfIntersectionsBVHContext(ctx, mesh, collision.Options{})
		if e != nil || !contacts.Complete || len(contacts.Contacts) != 0 {
			t.Fatal("contacts", e)
		}
	}
	for _, offset := range []float64{0, -.125} {
		tool, e := base.Transformed(geom.Affine{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, offset}})
		if e != nil {
			t.Fatal(e)
		}
		if bad, e := CombineSplinePlaneClip(ctx, stock, tool, 5, parametric.Intersection, SplineRectangleEmbeddingOptions{}); e == nil || len(bad.Solid.Faces()) != 0 {
			t.Fatal("corner/nonaffine crossing", offset)
		}
	}
	tangent, _ := makeOpenSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 0}, 0)
	if bad, e := CombineSplinePlaneClip(ctx, stock, tangent, 5, parametric.Intersection, SplineRectangleEmbeddingOptions{}); e == nil || len(bad.Solid.Faces()) != 0 {
		t.Fatal("tangency")
	}
	if bad, e := CombineSplinePlaneClip(ctx, stock, tool, 5, parametric.Intersection, SplineRectangleEmbeddingOptions{MaxFaces: 1}); e == nil || len(bad.Solid.Faces()) != 0 {
		t.Fatal("budget")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if bad, e := CombineSplinePlaneClip(canceled, stock, tool, 5, parametric.Intersection, SplineRectangleEmbeddingOptions{}); e != context.Canceled || len(bad.Solid.Faces()) != 0 {
		t.Fatal("cancel", e)
	}
}
