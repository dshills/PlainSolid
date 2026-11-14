package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"testing"
)

func TestAffineTriangleCyclicFanJSON(t *testing.T) {
	ctx := context.Background()
	for _, count := range []int{5, 8} {
		t.Run(string(rune('0'+count)), func(t *testing.T) {
			mates, pairs := tensorTriangleCyclicFixture(t, count)
			s, e := NewAffineTriangleCyclicFanShell(ctx, mates, pairs, AffineTriangleShellOptions{})
			if e != nil {
				t.Fatal(e)
			}
			var buf bytes.Buffer
			if e = WriteAffineTriangleCyclicFanJSON(ctx, &buf, s, AffineTriangleCyclicFanJSONOptions{}); e != nil {
				t.Fatal(e)
			}
			raw := append([]byte(nil), buf.Bytes()...)
			restored, e := ReadAffineTriangleCyclicFanJSON(ctx, bytes.NewReader(raw), AffineTriangleCyclicFanJSONOptions{})
			if e != nil {
				t.Fatal(e)
			}
			if len(restored.Nonadjacent()) != count*(count-3)/2 || len(restored.Boundary()) != count {
				t.Fatal("complete labelled fan")
			}
			owners, e := restored.VertexSources(restored.CenterVertex())
			if e != nil || len(owners) != count {
				t.Fatal("original central occurrences")
			}
			solid := Solid{vertices: restored.Vertices(), edges: restored.Edges(), faces: restored.Faces(), tolerance: 1e-9}
			r, e := solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
			if e != nil || !r.Complete {
				t.Fatal(r, e)
			}
			var again bytes.Buffer
			if e = WriteAffineTriangleCyclicFanJSON(ctx, &again, restored, AffineTriangleCyclicFanJSONOptions{}); e != nil || !bytes.Equal(raw, again.Bytes()) {
				t.Fatal("canonical table and topology", e)
			}
			for _, mutate := range []func(*affineTriangleCyclicFanRecipe){
				func(n *affineTriangleCyclicFanRecipe) { n.Version = 2 }, func(n *affineTriangleCyclicFanRecipe) { n.Kind = "exact-affine-triangle-cyclic-fan" }, func(n *affineTriangleCyclicFanRecipe) { n.Opposite = n.Opposite[:len(n.Opposite)-1] }, func(n *affineTriangleCyclicFanRecipe) { n.Opposite[1] = n.Opposite[0] }, func(n *affineTriangleCyclicFanRecipe) { n.Opposite[0].Faces = [2]int{0, 1} }, func(n *affineTriangleCyclicFanRecipe) { n.Opposite[0].Contact = n.Opposite[1].Contact }, func(n *affineTriangleCyclicFanRecipe) { n.Opposite[0].Point[0] = "1/4" }, func(n *affineTriangleCyclicFanRecipe) { n.Witness.Vertices[n.Center].Sources[count-1][1] = 1 }, func(n *affineTriangleCyclicFanRecipe) { n.Witness.Edges[0].Sources[0][0] = count - 1 }, func(n *affineTriangleCyclicFanRecipe) { n.Patches[0].Controls[0][0] = "99" },
			} {
				var n affineTriangleCyclicFanRecipe
				if e = json.Unmarshal(raw, &n); e != nil {
					t.Fatal(e)
				}
				mutate(&n)
				for _, p := range n.Patches {
					if _, e := geom.ImportExactTensorSurfaceRecipe(ctx, p, geom.SurfacePatchOptions{}); e != nil {
						t.Fatal("mutated fixture must retain valid local patches", e)
					}
				}
				bad, _ := json.Marshal(n)
				if got, e := ReadAffineTriangleCyclicFanJSON(ctx, bytes.NewReader(bad), AffineTriangleCyclicFanJSONOptions{}); e == nil || got != nil {
					t.Fatal("accepted omitted/stale/altered ownership proof")
				}
			}
			for _, o := range []AffineTriangleCyclicFanJSONOptions{{MaxWork: 1}, {MaxWork: s.ExactOperations() - 1}, {MaxBytes: int64(len(raw) - 1)}, {MaxPoles: 1}, {MaxBits: 63}, {Contact: geom.SurfaceAffineCoordinateOptions{SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: 1}}}, {Shell: AffineTriangleShellOptions{Proof: edgecheck.Options{MaxIntervals: 1}}}} {
				if got, e := ReadAffineTriangleCyclicFanJSON(ctx, bytes.NewReader(raw), o); e == nil || got != nil {
					t.Fatal("read limit")
				}
				var denied bytes.Buffer
				if e := WriteAffineTriangleCyclicFanJSON(ctx, &denied, s, o); e == nil || denied.Len() != 0 {
					t.Fatal("atomic write limit")
				}
			}
			if got, e := ReadAffineTriangleFanJSON(ctx, bytes.NewReader(raw), AffineTriangleFanJSONOptions{}); e == nil || got != nil {
				t.Fatal("large fan in three reader")
			}
			if got, e := ReadAffineTriangleFourFanJSON(ctx, bytes.NewReader(raw), AffineTriangleFanJSONOptions{}); e == nil || got != nil {
				t.Fatal("large fan in four reader")
			}
			if count == 5 {
				three, e := NewAffineTriangleFanShell(ctx, tensorTriangleFanFixture(t, false), AffineTriangleShellOptions{})
				if e != nil {
					t.Fatal(e)
				}
				var sibling bytes.Buffer
				if e = WriteAffineTriangleFanJSON(ctx, &sibling, three, AffineTriangleFanJSONOptions{}); e != nil {
					t.Fatal(e)
				}
				if got, e := ReadAffineTriangleCyclicFanJSON(ctx, &sibling, AffineTriangleCyclicFanJSONOptions{}); e == nil || got != nil {
					t.Fatal("sibling accepted")
				}
			}
			canceled, cancel := context.WithCancel(ctx)
			cancel()
			if got, e := ReadAffineTriangleCyclicFanJSON(canceled, bytes.NewReader(raw), AffineTriangleCyclicFanJSONOptions{}); e == nil || got != nil {
				t.Fatal("cancel")
			}
			s.fan.graph.faces[0].Outer[0].UVEnd.X += .125
			var denied bytes.Buffer
			if e = WriteAffineTriangleCyclicFanJSON(ctx, &denied, s, AffineTriangleCyclicFanJSONOptions{}); e == nil || denied.Len() != 0 {
				t.Fatal("stale actual native use")
			}
		})
	}
}
