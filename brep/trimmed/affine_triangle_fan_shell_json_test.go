//go:build research

package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"testing"
)

func TestAffineTriangleFanJSON(t *testing.T) {
	ctx := context.Background()
	for _, count := range []int{3, 4} {
		t.Run(string(rune('0'+count)), func(t *testing.T) {
			var three *AffineTriangleFanShell
			var four *AffineTriangleFourFanShell
			var e error
			if count == 3 {
				three, e = NewAffineTriangleFanShell(ctx, tensorTriangleFanFixture(t, false), AffineTriangleShellOptions{})
			} else {
				m, o := tensorTriangleFourFanFixture(t)
				four, e = NewAffineTriangleFourFanShell(ctx, m, o, AffineTriangleShellOptions{})
			}
			if e != nil {
				t.Fatal(e)
			}
			write := func(dst *bytes.Buffer, o AffineTriangleFanJSONOptions) error {
				if count == 3 {
					return WriteAffineTriangleFanJSON(ctx, dst, three, o)
				}
				return WriteAffineTriangleFourFanJSON(ctx, dst, four, o)
			}
			read := func(raw []byte, o AffineTriangleFanJSONOptions) (*AffineTriangleFanShell, error) {
				if count == 3 {
					return ReadAffineTriangleFanJSON(ctx, bytes.NewReader(raw), o)
				}
				s, e := ReadAffineTriangleFourFanJSON(ctx, bytes.NewReader(raw), o)
				if e != nil {
					return nil, e
				}
				return s.fan, nil
			}
			var buf bytes.Buffer
			if e = write(&buf, AffineTriangleFanJSONOptions{}); e != nil {
				t.Fatal(e)
			}
			raw := append([]byte(nil), buf.Bytes()...)
			restored, e := read(raw, AffineTriangleFanJSONOptions{})
			if e != nil {
				t.Fatal(e)
			}
			solid := Solid{vertices: restored.Vertices(), edges: restored.Edges(), faces: restored.Faces(), tolerance: 1e-9}
			r, e := solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
			if e != nil || !r.Complete {
				t.Fatal(r, e)
			}
			owners, e := restored.VertexSources(restored.CenterVertex())
			if e != nil || len(owners) != count || len(restored.Boundary()) != count {
				t.Fatal("circular fan/exterior")
			}
			var again bytes.Buffer
			if count == 3 {
				e = WriteAffineTriangleFanJSON(ctx, &again, restored, AffineTriangleFanJSONOptions{})
			} else {
				f, e2 := ReadAffineTriangleFourFanJSON(ctx, bytes.NewReader(raw), AffineTriangleFanJSONOptions{})
				if e2 != nil {
					t.Fatal(e2)
				}
				e = WriteAffineTriangleFourFanJSON(ctx, &again, f, AffineTriangleFanJSONOptions{})
			}
			if e != nil || !bytes.Equal(raw, again.Bytes()) {
				t.Fatal("canonical roundtrip", e)
			}
			for _, mutate := range []func(*affineTriangleFanRecipe){
				func(n *affineTriangleFanRecipe) { n.Version = 2 }, func(n *affineTriangleFanRecipe) { n.Kind = "exact-affine-three-triangle-chain" }, func(n *affineTriangleFanRecipe) { n.Center = (n.Center + 1) % (count + 1) }, func(n *affineTriangleFanRecipe) { n.Mates[count-1] = n.Mates[0] }, func(n *affineTriangleFanRecipe) { n.Witness.Vertices[n.Center].Sources[0][1] = 2 }, func(n *affineTriangleFanRecipe) { n.Witness.Boundary[0][0] = (n.Witness.Boundary[0][0] + 1) % count }, func(n *affineTriangleFanRecipe) { n.Mates[0].Normal[0] = "2" }, func(n *affineTriangleFanRecipe) {
					if count == 4 {
						n.Opposite = n.Opposite[:1]
					} else {
						n.Opposite = append(n.Opposite, affineFanVertexRecipe{})
					}
				}, func(n *affineTriangleFanRecipe) {
					if count == 4 {
						n.Opposite[0].Vertices[0] = 1
					} else {
						n.Mates[0].Triangles[0][0][0] = "1/4"
					}
				},
			} {
				var n affineTriangleFanRecipe
				if e = json.Unmarshal(raw, &n); e != nil {
					t.Fatal(e)
				}
				mutate(&n)
				for _, p := range n.Patches {
					if _, e := geom.ImportExactTensorSurfaceRecipe(ctx, p, geom.SurfacePatchOptions{}); e != nil {
						t.Fatal("invalid local source fixture", e)
					}
				}
				bad, _ := json.Marshal(n)
				if got, e := read(bad, AffineTriangleFanJSONOptions{}); e == nil || got != nil {
					t.Fatal("accepted mutated cycle/ownership")
				}
			}
			for _, o := range []AffineTriangleFanJSONOptions{{MaxWork: 1}, {MaxWork: -1}, {MaxBytes: int64(len(raw) - 1)}, {MaxPoles: 1}, {MaxBits: 63}, {Contact: geom.SurfaceAffineCoordinateOptions{SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: 1}}}, {Shell: AffineTriangleShellOptions{Proof: edgecheck.Options{MaxIntervals: 1}}}} {
				if got, e := read(raw, o); e == nil || got != nil {
					t.Fatal("bounded read")
				}
				var denied bytes.Buffer
				if e := write(&denied, o); e == nil || denied.Len() != 0 {
					t.Fatal("atomic write")
				}
			}
			if count == 3 {
				if got, e := ReadAffineTriangleFourFanJSON(ctx, bytes.NewReader(raw), AffineTriangleFanJSONOptions{}); e == nil || got != nil {
					t.Fatal("three as four")
				}
			} else {
				if got, e := ReadAffineTriangleFanJSON(ctx, bytes.NewReader(raw), AffineTriangleFanJSONOptions{}); e == nil || got != nil {
					t.Fatal("four as three")
				}
			}
			var graph *WrappedAlgebraicFaceChainShell
			if count == 3 {
				graph = three.graph
			} else {
				graph = four.fan.graph
			}
			graph.faces[0].Outer[0].UVStart.X += .125
			var denied bytes.Buffer
			if e = write(&denied, AffineTriangleFanJSONOptions{}); e == nil || denied.Len() != 0 {
				t.Fatal("stale actual pcurve")
			}
		})
	}
}
