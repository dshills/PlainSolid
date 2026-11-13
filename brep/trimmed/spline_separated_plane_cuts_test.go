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

func TestSplineSeparatedNonparallelPlaneCuts(t *testing.T) {
	ctx := context.Background()
	stock, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, .125)
	tools := [2]Solid{slabTool(t, true, .125, .125), slabTool(t, false, .5, .25)}
	for _, swap := range []bool{false, true} {
		inputs, active := tools, [2]int{4, 5}
		if swap {
			inputs = [2]Solid{tools[1], tools[0]}
			active = [2]int{5, 4}
		}
		r, e := CombineSplineSeparatedPlaneCuts(ctx, stock, inputs, active, SplineRectangleEmbeddingOptions{})
		if e != nil {
			t.Fatal(e)
		}
		// Direct integration between l=1/8+(x+y)/8 and h=1/2+(x+y)/4.
		m, e := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
		wantArea := 2 + math.Sqrt(66)/8 + 3*math.Sqrt(2)/4
		if e != nil || math.Abs(m.Volume-.5) > 1e-10 || math.Abs(m.Area-wantArea) > 1e-10 || m.Centroid.Sub(geom.Vec3{X: 25. / 48, Y: 25. / 48, Z: 65. / 128}).Length() > 1e-10 {
			t.Fatal("independent mass", m, e)
		}
		if r.Solid.Topology().EulerCharacteristic != 2 || len(r.Solid.faces) != 6 || len(r.Solid.edges) != 12 || len(r.Solid.vertices) != 8 {
			t.Fatal("topology")
		}
		owned := map[int]bool{}
		for _, ids := range r.StockEdges {
			for _, id := range ids {
				if owned[id] {
					t.Fatal("duplicate stock edge")
				}
				owned[id] = true
			}
		}
		for k := 0; k < 2; k++ {
			if len(r.SectionEdges[k]) != 4 || len(r.ToolFaces[k][active[k]]) != 1 {
				t.Fatal("cap ancestry")
			}
			cap := r.Solid.faces[r.ToolFaces[k][active[k]][0]]
			if cap.Surface != inputs[k].faces[active[k]].Surface || cap.Reversed != inputs[k].faces[active[k]].Reversed {
				t.Fatal("physical cap support")
			}
			for _, id := range r.SectionEdges[k] {
				if owned[id] {
					t.Fatal("duplicate cap edge")
				}
				owned[id] = true
			}
		}
		if len(owned) != 12 {
			t.Fatal("ancestry coverage")
		}
		for i, ids := range r.StockFaces {
			if i < 4 {
				if len(ids) != 1 || r.Solid.faces[ids[0]].Surface != stock.faces[i].Surface {
					t.Fatal("stock support")
				}
			} else if len(ids) != 0 {
				t.Fatal("excluded support")
			}
		}
		for _, id := range r.OriginalVertices {
			if id != -1 {
				t.Fatal("unexpected corner")
			}
		}
		var native bytes.Buffer
		if e = WriteJSON(&native, r.Solid); e != nil {
			t.Fatal(e)
		}
		reload, e := ReadJSONContext(ctx, &native, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		mass, e := reload.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(mass.Volume-.5) > 1e-10 {
			t.Fatal("reload", e)
		}
		mesh, e := r.Solid.Tessellate(ctx, parametric.TessellationOptions{Deflection: .02})
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
	if r, e := CombineSplineParallelSlab(ctx, stock, tools, [2]int{4, 5}, SplineRectangleEmbeddingOptions{}); e == nil || len(r.Solid.faces) != 0 {
		t.Fatal("old parallel contract widened")
	}
	for _, upper := range []float64{.5, .875} {
		// Each independent section is valid, but the two caps cross or touch.
		bad := [2]Solid{slabTool(t, true, .375, .25), slabTool(t, false, upper, 0)}
		for k, face := range [2]int{4, 5} {
			if _, e := CombineSplinePlaneClip(ctx, stock, bad[k], face, parametric.Intersection, SplineRectangleEmbeddingOptions{}); e != nil {
				t.Fatal("bad fixture has unresolved individual section", e)
			}
		}
		if r, e := CombineSplineSeparatedPlaneCuts(ctx, stock, bad, [2]int{4, 5}, SplineRectangleEmbeddingOptions{}); e == nil || len(r.Solid.faces) != 0 || len(r.StockFaces) != 0 {
			t.Fatal("cap contact accepted", upper)
		}
	}
	if r, e := CombineSplineSeparatedPlaneCuts(ctx, stock, tools, [2]int{4, 5}, SplineRectangleEmbeddingOptions{Patches: geom.SurfacePatchOptions{MaxOperations: 1}}); e == nil || len(r.Solid.faces) != 0 {
		t.Fatal("budget")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := CombineSplineSeparatedPlaneCuts(canceled, stock, tools, [2]int{4, 5}, SplineRectangleEmbeddingOptions{}); e != context.Canceled {
		t.Fatal("cancel", e)
	}
}
