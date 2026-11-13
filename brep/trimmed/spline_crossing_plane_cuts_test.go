package trimmed

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestSplineCrossingPlaneCuts(t *testing.T) {
	ctx := context.Background()
	stock, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, .125)
	tools := [2]Solid{slabTool(t, true, .375, .25), slabTool(t, false, .75, 0)}
	// Rotate only the horizontal cutter's original XY parameterization. Its
	// physical rectangle is unchanged, but the two cap intervals are distinct.
	var err error
	tools[1], err = tools[1].Transformed(geom.Affine{{0, -1, 0, 1}, {1, 0, 0, 0}, {0, 0, 1, 0}})
	if err != nil {
		t.Fatal(err)
	}
	for _, swap := range []bool{false, true} {
		inputs, active := tools, [2]int{4, 5}
		if swap {
			inputs = [2]Solid{tools[1], tools[0]}
			active = [2]int{5, 4}
		}
		r, e := CombineSplineCrossingPlaneCuts(ctx, stock, inputs, active, SplineRectangleEmbeddingOptions{})
		if e != nil {
			t.Fatal(e)
		}
		// Integral of max(3/8-(x+y)/4,0) over [0,1]^2; excluded
		// corner triangle has legs1/2 and linearly varying negative height1/8.
		mass, e := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
		center := geom.Vec3{X: 71. / 200, Y: 71. / 200, Z: 521. / 800}
		if e != nil || math.Abs(mass.Volume-25./192) > 1e-10 || mass.Centroid.Sub(center).Length() > 1e-10 || math.Abs(mass.Area-(23./16+21*math.Sqrt(2)/32)) > 1e-10 {
			t.Fatal("integral", mass, e)
		}
		if len(r.CapIntersectionEdges) != 1 || r.CapIntersection == nil || r.CapIntersection.Edge != r.CapIntersectionEdges[0] || r.CapIntersection.Faces != active {
			t.Fatal("shared edge ancestry")
		}
		if r.Solid.Topology().EulerCharacteristic != 2 || len(r.Solid.faces) != 6 || len(r.Solid.edges) != 12 || len(r.Solid.vertices) != 8 {
			t.Fatal("topology")
		}
		different := false
		for end := 0; end < 2; end++ {
			for axis := 0; axis < 2; axis++ {
				different = different || r.CapIntersection.UV[0][end][axis].Cmp(r.CapIntersection.UV[1][end][axis]) != 0
			}
		}
		if !different {
			t.Fatal("fixture lost distinct original charts")
		}
		edge := r.Solid.edges[r.CapIntersection.Edge]
		for k := 0; k < 2; k++ {
			if len(r.SectionEdges[k]) != 4 {
				t.Fatal("stock/cap seam count")
			}
			cap := r.Solid.faces[r.ToolFaces[k][active[k]][0]]
			if cap.Surface != inputs[k].faces[active[k]].Surface || cap.Reversed != inputs[k].faces[active[k]].Reversed {
				t.Fatal("support ownership")
			}
			patch, _, e := inputs[k].splineEmbeddingFace(ctx, inputs[k].faces[active[k]], SplineRectangleEmbeddingOptions{})
			if e != nil {
				t.Fatal(e)
			}
			// Independent exact evaluation at both ends and a nonbinary interior t.
			for _, alpha := range []*big.Rat{new(big.Rat), big.NewRat(1, 3), big.NewRat(1, 1)} {
				var uv [2]*big.Rat
				for axis := range uv {
					a, b := r.CapIntersection.UV[k][0][axis], r.CapIntersection.UV[k][1][axis]
					uv[axis] = new(big.Rat).Add(a, new(big.Rat).Mul(alpha, new(big.Rat).Sub(b, a)))
				}
				j, e := patch.patch.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				for axis := range j.Point {
					a, b := r.CapIntersection.Points[0][axis], r.CapIntersection.Points[1][axis]
					want := new(big.Rat).Add(a, new(big.Rat).Mul(alpha, new(big.Rat).Sub(b, a)))
					if j.Point[axis].Cmp(want) != 0 {
						t.Fatal("independent model correspondence")
					}
				}
			}
		}
		for i, id := range []int{edge.Start, edge.End} {
			v := r.Solid.vertices[id]
			for axis, x := range []float64{v.X, v.Y, v.Z} {
				if r.CapIntersection.Points[i][axis].Cmp(new(big.Rat).SetFloat64(x)) != 0 {
					t.Fatal("edge direction")
				}
			}
		}
		owned := map[int]bool{}
		lists := append(append([][]int{}, r.StockEdges...), r.SectionEdges[:]...)
		lists = append(lists, r.CapIntersectionEdges)
		for _, ids := range lists {
			for _, id := range ids {
				if owned[id] {
					t.Fatal("duplicate provenance")
				}
				owned[id] = true
			}
		}
		if len(owned) != len(r.Solid.edges) {
			t.Fatal("missing provenance")
		}
		var native bytes.Buffer
		if e = WriteJSON(&native, r.Solid); e != nil {
			t.Fatal(e)
		}
		reload, e := ReadJSONContext(ctx, &native, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		rm, e := reload.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(rm.Volume-25./192) > 1e-10 {
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
	for _, upper := range []float64{.625, .875, .9375} { // vertex contact, tangent, separated
		bad := [2]Solid{tools[0], slabTool(t, false, upper, 0)}
		for k, face := range [2]int{4, 5} {
			if _, e := CombineSplinePlaneClip(ctx, stock, bad[k], face, parametric.Intersection, SplineRectangleEmbeddingOptions{}); e != nil {
				t.Fatal("individual proof", upper, e)
			}
		}
		if r, e := CombineSplineCrossingPlaneCuts(ctx, stock, bad, [2]int{4, 5}, SplineRectangleEmbeddingOptions{}); e == nil || len(r.Solid.faces) != 0 || r.CapIntersection != nil {
			t.Fatal("unsupported contact accepted", upper)
		}
	}
	coincident := [2]Solid{tools[0], slabTool(t, false, .375, .25)}
	if r, e := CombineSplineCrossingPlaneCuts(ctx, stock, coincident, [2]int{4, 5}, SplineRectangleEmbeddingOptions{}); e == nil || len(r.Solid.faces) != 0 || r.CapIntersection != nil {
		t.Fatal("coincident cap accepted")
	}
	if r, e := CombineSplineCrossingPlaneCuts(ctx, stock, tools, [2]int{4, 5}, SplineRectangleEmbeddingOptions{Patches: geom.SurfacePatchOptions{MaxOperations: 1}}); e == nil || len(r.Solid.faces) != 0 {
		t.Fatal("budget")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := CombineSplineCrossingPlaneCuts(canceled, stock, tools, [2]int{4, 5}, SplineRectangleEmbeddingOptions{}); e != context.Canceled {
		t.Fatal("cancel", e)
	}
}
