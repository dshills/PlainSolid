//go:build research

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

func twoCornerClipFixture(t *testing.T) [2]*SplineCornerFan {
	t.Helper()
	ctx := context.Background()
	a, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, .125)
	raw, _ := makeCornerShearBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 0}, 0)
	b, e := raw.Transformed(geom.Affine{{1, 0, 0, 0}, {0, 1, 0, 0}, {5. / 16, 5. / 16, 1, 0}})
	if e != nil {
		t.Fatal(e)
	}
	var fans [2]*SplineCornerFan
	for corner := 0; corner < 2; corner++ {
		var anchors [2]*SplineCornerAnchor
		for i := 0; i < 2; i++ {
			source := geom.ParameterRectangle{UMax: .125, VMax: .125}
			target := geom.ParameterRectangle{UMin: -.125, UMax: .125, VMax: .25}
			if i == 1 {
				source.UMax = .5
				target = geom.ParameterRectangle{UMax: .5, VMin: -.125, VMax: .125}
			}
			val := int64(0)
			if corner == 1 {
				val = 1
				source = geom.ParameterRectangle{UMin: .875, UMax: 1, VMin: .875, VMax: 1}
				target = geom.ParameterRectangle{UMin: .875, UMax: 1.125, VMin: .75, VMax: 1}
				if i == 1 {
					source.UMin = .5
					target = geom.ParameterRectangle{UMin: .5, UMax: 1, VMin: .875, VMax: 1.125}
				}
			}
			hint := [4]*big.Rat{big.NewRat(val, 1), big.NewRat(val, 1), big.NewRat(val, 1), big.NewRat(val, 1)}
			anchors[i], e = CertifySplineFaceCornerAnchor(ctx, a, b, [2]int{2*i + corner, 5}, [2]geom.ParameterRectangle{source, target}, 0, geom.SurfaceTubeBoundaryOptions{SurfacePairTubeOptions: geom.SurfacePairTubeOptions{FreeAxis: i}, Anchor: hint, AtUpper: corner == 1}, SplineRectangleEmbeddingOptions{})
			if e != nil {
				t.Fatal("anchor", corner, i, e)
			}
		}
		point, e := CertifySplineFacePointContact(ctx, a, b, [2]int{4 + corner, 5}, SplineRectangleEmbeddingOptions{})
		if e != nil {
			t.Fatal("point", corner, e)
		}
		fans[corner], e = CertifySplineCornerFan(ctx, anchors, point, SplineRectangleEmbeddingOptions{})
		if e != nil {
			t.Fatal("fan", corner, e)
		}
	}
	return fans
}

func TestSplineTwoCornerPlaneClip(t *testing.T) {
	fans := twoCornerClipFixture(t)
	ctx := context.Background()
	for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Difference} {
		r, e := CombineSplineTwoCornerPlaneClip(ctx, fans, op, SplineRectangleEmbeddingOptions{})
		if e != nil {
			t.Fatal(op, e)
		}
		want := 17. / 32
		center := geom.Vec3{X: 59. / 102, Y: 59. / 102, Z: 1693. / 4896}
		if op == parametric.Difference {
			want = 15. / 32
			center = geom.Vec3{X: 37. / 90, Y: 37. / 90, Z: 3203. / 4320}
		}
		m, e := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(m.Volume-want) > 1e-10 || m.Centroid.Sub(center).Length() > 1e-10 {
			t.Fatal("integral", m, e)
		}
		if r.Solid.Topology().EulerCharacteristic != 2 || len(r.SectionEdges) != 4 {
			t.Fatal("topology")
		}
		for _, fan := range fans {
			v := r.OriginalVertices[fan.vertex]
			if v < 0 || r.Solid.vertices[v] != fan.anchors[0].solids[0].vertices[fan.vertex] {
				t.Fatal("vertex remap")
			}
		}
		cap := r.Solid.faces[r.ToolFaces[5][0]]
		if cap.Reversed != (op == parametric.Difference) {
			t.Fatal("cap orientation")
		}
		var buf bytes.Buffer
		if e = WriteJSON(&buf, r.Solid); e != nil {
			t.Fatal(e)
		}
		reload, e := ReadJSONContext(ctx, &buf, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		rm, e := reload.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(rm.Volume-want) > 1e-10 {
			t.Fatal("reload", e)
		}
		mesh, e := r.Solid.Tessellate(ctx, parametric.TessellationOptions{Deflection: .05})
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
		owned := map[int]bool{}
		for _, ids := range r.StockEdges {
			for _, id := range ids {
				if owned[id] {
					t.Fatal("duplicate source edge")
				}
				owned[id] = true
			}
		}
		for _, id := range r.SectionEdges {
			if owned[id] {
				t.Fatal("source/section overlap")
			}
			owned[id] = true
		}
		if len(owned) != len(r.Solid.edges) {
			t.Fatal("missing edge ancestry")
		}
		swapped, e := CombineSplineTwoCornerPlaneClip(ctx, [2]*SplineCornerFan{fans[1], fans[0]}, op, SplineRectangleEmbeddingOptions{})
		if e != nil {
			t.Fatal(e)
		}
		var first, second bytes.Buffer
		if e = WriteJSON(&first, r.Solid); e != nil {
			t.Fatal(e)
		}
		if e = WriteJSON(&second, swapped.Solid); e != nil || !bytes.Equal(first.Bytes(), second.Bytes()) {
			t.Fatal("fan order nondeterministic", e)
		}
	}
	// A single supplied fan must not hide the opposite point or corner pair.
	stock, tool := fans[0].anchors[0].solids[0], fans[0].anchors[0].solids[1]
	if r, e := combineSplinePlaneClipFans(ctx, stock, tool, 5, []*SplineCornerFan{fans[0]}, parametric.Intersection, SplineRectangleEmbeddingOptions{}); e == nil || len(r.Solid.faces) != 0 {
		t.Fatal("missing second proof")
	}
	for _, bad := range [][2]*SplineCornerFan{{fans[0], nil}, {fans[0], fans[0]}, {fans[0], new(SplineCornerFan)}} {
		if r, e := CombineSplineTwoCornerPlaneClip(ctx, bad, parametric.Intersection, SplineRectangleEmbeddingOptions{}); e == nil || len(r.Solid.faces) != 0 {
			t.Fatal("invalid fan accepted")
		}
	}
	if r, e := CombineSplineTwoCornerPlaneClip(ctx, fans, parametric.Intersection, SplineRectangleEmbeddingOptions{Patches: geom.SurfacePatchOptions{MaxOperations: 1}}); e == nil || len(r.Solid.faces) != 0 {
		t.Fatal("budget")
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := CombineSplineTwoCornerPlaneClip(c, fans, parametric.Intersection, SplineRectangleEmbeddingOptions{}); e != context.Canceled {
		t.Fatal("cancellation", e)
	}
}
