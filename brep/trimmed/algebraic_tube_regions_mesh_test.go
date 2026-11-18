//go:build research

package trimmed

import (
	"context"
	"math"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestCertifiedDisjointTubeRegionsFilled(t *testing.T) {
	slowtest.Skip(t)
	r := disjointRegionMeshFixture(t)
	o := AlgebraicTubeRegionsMeshOptions{Boundary: AlgebraicTubeRegionsBoundaryOptions{Loop: AlgebraicTubeLensBoundaryOptions{Run: AlgebraicTubeRunMeshOptions{Display: RationalTubePolylineOptions{Deflection: .025}}}}, Component: AlgebraicTubeLensMeshOptions{Interior: CurvedQuadraticTrimMeshOptions{Deflection: .01}}}
	m, e := r.TessellateDisjointClosedCertified(context.Background(), o)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Components) != 2 || !m.Collision.Complete || len(m.Collision.Contacts) > 0 {
		t.Fatal("component/contact")
	}
	for loop, ids := range m.Boundary.LoopVertices {
		for local, id := range ids {
			if m.Mesh.Vertices[id] != m.Boundary.Loops[loop].Run.Vertices[local] {
				t.Fatal("boundary changed")
			}
		}
	}
	for i, component := range m.Components {
		for j, p := range component.Mesh.Vertices {
			if m.Mesh.Vertices[m.ComponentVertices[i][j]] != p {
				t.Fatal("shared component index")
			}
		}
	}
	want := math.Pi * math.Pow(5.0/32, 4)
	v, _ := m.SignedDisplayVolume.Float64()
	if math.Abs(v-want) > .15*want {
		t.Fatal("independent two disk integral", v, want)
	}
	if len(m.OriginalRegion) != len(m.Mesh.Triangles) || len(m.OriginalChart) != len(m.Mesh.Triangles) {
		t.Fatal("ancestry")
	}
	t.Log(len(m.Mesh.Vertices), len(m.Mesh.Triangles), v, want)
	o.Component.Interior.MaxTriangles = 1
	if m, e := r.TessellateDisjointClosedCertified(context.Background(), o); e == nil || len(m.Mesh.Vertices) > 0 {
		t.Fatal("partial material", e)
	}
}
