package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestSolidCertifiedRationalSplitCorner(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	source := solidNativeEllipseFixture(t)
	split, history, err := source.SplitRationalEndpointQuadraticTrimEdge(ctx, 0, big.NewRat(1, 3), edgecheck.Options{})
	if err != nil {
		t.Fatal(err)
	}
	exact, err := split.RationalVertices(ctx)
	if err != nil {
		t.Fatal(err)
	}
	nonbinary := false
	for _, v := range exact {
		if v.Vertex == history.Vertex {
			for _, q := range v.Point {
				_, representable := q.Float64()
				nonbinary = nonbinary || !representable
			}
		}
	}
	if !nonbinary {
		t.Fatal("fixture needs genuinely nonbinary model corner")
	}
	opts := SolidExactQuadraticMeshOptions{Shell: ExactQuadraticShellMeshOptions{Join: ExactQuadraticFaceJoinOptions{Face: ExactQuadraticRegionMeshOptions{Face: MultiSpanCurvedTrimMeshOptions{Mesh: CurvedQuadraticTrimMeshOptions{Deflection: .02}}}}}}
	got, err := split.TessellateExactQuadraticShellCertified(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.RationalCorners) != len(exact) || got.ModelEdgeUsesCertified != 10 {
		t.Fatal("rational ancestry")
	}
	for _, v := range exact {
		display := got.Mesh.Vertices[got.JunctionVertices[v.Vertex]]
		if display != v.Display {
			t.Fatal("shared corner display changed")
		}
		incident := 0
		for _, face := range got.Faces {
			for i, p := range face.ExactPoints {
				if sameRationalVertex(p, v.Point) {
					incident++
					if face.Mesh.Vertices[i] != display {
						t.Fatal("incident rounding mismatch")
					}
				}
			}
		}
		if incident < 2 {
			t.Fatal("missing incident exact point")
		}
	}
	volume, _ := got.SignedDisplayVolume.Float64()
	want := math.Pi / 1024
	if math.Abs(volume-want) > .15*want {
		t.Fatal("source volume reference", volume, want)
	}
	if !got.DisplayCollision.Complete || len(got.DisplayCollision.Contacts) != 0 {
		t.Fatal("display embedding")
	}
	damaged := split
	damaged.vertices = split.Vertices()
	damaged.vertices[history.Vertex].X = math.Nextafter(damaged.vertices[history.Vertex].X, math.Inf(1))
	if mesh, e := damaged.TessellateExactQuadraticShellCertified(ctx, opts); e == nil || len(mesh.Mesh.Triangles) != 0 {
		t.Fatal("incorrect stored display accepted")
	}
	duplicated := split
	duplicated.vertices = split.Vertices()
	duplicated.vertices[history.Vertex] = duplicated.vertices[0]
	if mesh, e := duplicated.TessellateExactQuadraticShellCertified(ctx, opts); e == nil || len(mesh.Mesh.Triangles) != 0 {
		t.Fatal("display-collapsed corners accepted")
	}
	t.Log("triangles", len(got.Mesh.Triangles), "volume", volume, "exact corners", len(exact))
}
