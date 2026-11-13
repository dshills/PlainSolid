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

func slabTool(t *testing.T, lower bool, offset, slope float64) Solid {
	t.Helper()
	lo, hi := -2., 0.
	if lower {
		lo, hi = 0, 2
	}
	s, _ := makeOpenSplineBox(t, [3]float64{-1, -1, lo}, [3]float64{2, 2, hi}, 0)
	r, e := s.Transformed(geom.Affine{{1, 0, 0, 0}, {0, 1, 0, 0}, {slope, slope, 1, offset}})
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func TestSplineParallelSlab(t *testing.T) {
	ctx := context.Background()
	stock, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, .125)
	tools := [2]Solid{slabTool(t, true, .125, .25), slabTool(t, false, .375, .25)}
	for _, swap := range []bool{false, true} {
		inputs, active := tools, [2]int{4, 5}
		if swap {
			inputs = [2]Solid{tools[1], tools[0]}
			active = [2]int{5, 4}
		}
		r, e := CombineSplineParallelSlab(ctx, stock, inputs, active, SplineRectangleEmbeddingOptions{})
		if e != nil {
			t.Fatal(e)
		}
		// Integrate z between (x+y)/4+1/8 and (x+y)/4+3/8.
		m, e := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(m.Volume-.25) > 1e-10 || math.Abs(m.Area-(1+3*math.Sqrt(2)/2)) > 1e-10 || m.Centroid.Sub(geom.Vec3{X: .5, Y: .5, Z: .5}).Length() > 1e-10 {
			t.Fatal("integral", m, e)
		}
		if r.Solid.Topology().EulerCharacteristic != 2 || len(r.Solid.faces) != 6 || len(r.Solid.edges) != 12 || len(r.Solid.vertices) != 8 {
			t.Fatal("topology", r.Solid.Topology())
		}
		for k := 0; k < 2; k++ {
			if len(r.SectionEdges[k]) != 4 || len(r.ToolFaces[k][active[k]]) != 1 {
				t.Fatal("section ownership")
			}
			face := r.Solid.faces[r.ToolFaces[k][active[k]][0]]
			if face.Surface != inputs[k].faces[active[k]].Surface || face.Reversed != inputs[k].faces[active[k]].Reversed {
				t.Fatal("cap support")
			}
		}
		owned := map[int]bool{}
		for _, ids := range r.StockEdges {
			for _, id := range ids {
				if owned[id] {
					t.Fatal("duplicate source")
				}
				owned[id] = true
			}
		}
		for _, ids := range r.SectionEdges {
			for _, id := range ids {
				if owned[id] {
					t.Fatal("duplicate section")
				}
				owned[id] = true
			}
		}
		if len(owned) != 12 {
			t.Fatal("edge coverage")
		}
		for i, ids := range r.StockFaces {
			if i < 4 {
				if len(ids) != 1 || r.Solid.faces[ids[0]].Surface != stock.faces[i].Surface {
					t.Fatal("stock support")
				}
			} else if len(ids) != 0 {
				t.Fatal("excluded curved support")
			}
		}
		for _, v := range r.OriginalVertices {
			if v != -1 {
				t.Fatal("unexpected original corner")
			}
		}
		var buf bytes.Buffer
		if e = WriteJSON(&buf, r.Solid); e != nil {
			t.Fatal(e)
		}
		loaded, e := ReadJSONContext(ctx, &buf, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		lm, e := loaded.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(lm.Volume-.25) > 1e-10 {
			t.Fatal("reload", e)
		}
		mesh, e := r.Solid.Tessellate(ctx, parametric.TessellationOptions{Deflection: .02})
		if e != nil {
			t.Fatal(e)
		}
		if e = mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		c, e := collision.SelfIntersectionsBVHContext(ctx, mesh, collision.Options{})
		if e != nil || !c.Complete || len(c.Contacts) != 0 {
			t.Fatal("contacts", e)
		}
	}
	for name, bad := range map[string][2]Solid{
		"same kept side": {tools[1], slabTool(t, false, .5, .25)},
		"empty":          {slabTool(t, true, .5, .25), tools[1]},
		"coincident":     {slabTool(t, true, .375, .25), tools[1]},
		"crossing":       {tools[0], slabTool(t, false, .375, .125)},
	} {
		active := [2]int{4, 5}
		if name == "same kept side" {
			active = [2]int{5, 5}
		}
		if r, e := CombineSplineParallelSlab(ctx, stock, bad, active, SplineRectangleEmbeddingOptions{}); e == nil || len(r.Solid.faces) != 0 {
			t.Fatal(name)
		}
	}
	if r, e := CombineSplineParallelSlab(ctx, stock, tools, [2]int{4, 5}, SplineRectangleEmbeddingOptions{Patches: geom.SurfacePatchOptions{MaxOperations: 1}}); e == nil || len(r.Solid.faces) != 0 {
		t.Fatal("budget")
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := CombineSplineParallelSlab(c, stock, tools, [2]int{4, 5}, SplineRectangleEmbeddingOptions{}); e != context.Canceled {
		t.Fatal("cancel", e)
	}
}

func TestSplineSlabUVHalfplanePremise(t *testing.T) {
	square := Face{Outer: []Use{{UVStart: geom.Vec2{}}, {UVStart: geom.Vec2{X: 1}}, {UVStart: geom.Vec2{X: 1, Y: 1}}, {UVStart: geom.Vec2{Y: 1}}}}
	p := slabFaceUV(square)
	if !slabConvexCCW(p) {
		t.Fatal("square")
	}
	p[1], p[3] = p[3], p[1]
	if slabConvexCCW(p) {
		t.Fatal("clockwise")
	}
	p = slabFaceUV(square)
	p[2] = p[1]
	if slabConvexCCW(p) {
		t.Fatal("duplicate")
	}
	p = slabFaceUV(square)
	p[1], p[2] = p[2], p[1]
	if slabConvexCCW(p) {
		t.Fatal("crossing")
	}
}
