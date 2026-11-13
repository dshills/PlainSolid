package trimmed

import (
	"context"
	"math"
	"os"
	"plainsolid/brep/trimvalidate"
	"plainsolid/collision"
	"testing"
)

func checkNestedTubeRegionShell(t *testing.T, r AlgebraicTubeRegions, annulus int) {
	t.Helper()
	o := AlgebraicTubeRegionMeshOptions{Footprint: AlgebraicTubeFootprintMeshOptions{Boundary: AlgebraicTubeRegionsBoundaryOptions{MaxOperations: 256000000, Loop: AlgebraicTubeLensBoundaryOptions{Run: AlgebraicTubeRunMeshOptions{Display: RationalTubePolylineOptions{Deflection: .03, MaxOperations: 64000000}, MaxPieces: 512}, MaxOwnerOperations: 64000000}}, Polygon: trimvalidate.Options{MaxEdges: 1024, MaxWork: 2000000}}, Surface: MultiSpanTubeLensMeshOptions{Closed: AlgebraicTubeLensMeshOptions{Collision: collision.Options{MaxPairs: 16000000}, Interior: CurvedQuadraticTrimMeshOptions{Deflection: .004}}, Span: MultiSpanTriangleBoundOptions{MaxDegree: 4}}}
	o.Footprint.Boundary.Loop.Run.Partition.MaxOperations = 64000000
	m, e := r.TessellateRegionClosedCertified(context.Background(), annulus, o)
	if e != nil {
		t.Fatal("quartic annular shell", e)
	}
	if !m.Collision.Complete || len(m.Collision.Contacts) > 0 {
		t.Fatal("shell contacts")
	}
	if e := m.Mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	edges := map[[2]int]bool{}
	for _, tri := range m.Mesh.Triangles {
		for i, a := range tri {
			b := tri[(i+1)%3]
			if a > b {
				a, b = b, a
			}
			edges[[2]int{a, b}] = true
		}
	}
	if len(m.Mesh.Vertices)-len(edges)+len(m.Mesh.Triangles) != 0 {
		t.Fatal("annular genus")
	}
	for id, local := range m.GlobalBoundaryVertices {
		if m.Mesh.Vertices[local] != m.Footprint.Boundary.Vertices[id] {
			t.Fatal("changed inner/outer shared sample")
		}
	}
	// Z=(9/4)*(q-ri)*(q-ro), dA=pi dq.
	delta := 25.0/256 - 25.0/1024
	want := math.Pi * (9.0 / 4) * delta * delta * delta / 6
	v, _ := m.SignedDisplayVolume.Float64()
	if math.Abs(v-want) > .2*want {
		t.Fatal("independent annulus mass", v, want)
	}
	t.Log("quartic annular shell", len(m.Mesh.Vertices), len(m.Mesh.Triangles), m.ResidualOperations, "volume", v, "source", want)
}
func TestQualifyNestedQuarticShell(t *testing.T) {
	if os.Getenv("PLAINSOLID_TEST_NESTED_SHELL") != "1" {
		t.Skip("independent expensive nested source qualification")
	}
	r := nestedTubeMaterialFixture(t)
	for i, x := range r.Regions() {
		if len(x.Holes) > 0 {
			checkNestedTubeRegionShell(t, r, i)
			return
		}
	}
	t.Fatal("missing annulus")
}
