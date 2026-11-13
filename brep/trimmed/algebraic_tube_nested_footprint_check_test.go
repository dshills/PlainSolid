package trimmed

import (
	"context"
	"math"
	"plainsolid/brep/trimvalidate"
	"plainsolid/collision"
	"testing"
)

// Invoked by the source qualification test so the expensive original quartic
// coverage is constructed once, independently of this display proof.
func checkNestedTubeFootprint(t *testing.T, r AlgebraicTubeRegions, annulus int) {
	t.Helper()
	o := AlgebraicTubeFootprintMeshOptions{Boundary: AlgebraicTubeRegionsBoundaryOptions{MaxOperations: 256000000, Loop: AlgebraicTubeLensBoundaryOptions{Run: AlgebraicTubeRunMeshOptions{Partition: AlgebraicTubeFacePartitionOptions{}, Display: RationalTubePolylineOptions{Deflection: .04, MaxOperations: 64000000}, MaxPieces: 512}, MaxOwnerOperations: 64000000}}, Polygon: trimvalidate.Options{MaxEdges: 1024, MaxWork: 2000000}}
	o.Boundary.Loop.Run.Partition.MaxOperations = 64000000
	m, e := r.TessellateFootprintCertified(context.Background(), annulus, o)
	if e != nil {
		t.Fatal("nested footprint", e)
	}
	if len(m.BoundaryLoops) != 2 {
		t.Fatal("lost annular hole")
	}
	area, _ := m.PolygonArea.Float64()
	want := math.Pi * (math.Pow(5.0/16, 2) - math.Pow(5.0/32, 2))
	if math.Abs(area-want) > .05*want {
		t.Fatal("independent annulus area", area, want)
	}
	squared, _ := m.SourceFillSquaredDistance.Float64()
	if squared > .04*.04+1e-15 {
		t.Fatal("uncertified fill tolerance", squared)
	}
	for i, p := range m.Mesh.Vertices {
		q := m.Boundary.Vertices[m.GlobalBoundaryVertices[i]]
		if p.X != q.X || p.Y != q.Y || p.Z != 0 {
			t.Fatal("lost global sample identity")
		}
	}
	for _, tri := range m.Mesh.Triangles {
		a, b, c := m.Mesh.Vertices[tri[0]], m.Mesh.Vertices[tri[1]], m.Mesh.Vertices[tri[2]]
		p := a.Add(b).Add(c).Scale(1.0 / 3)
		radius := math.Hypot(p.X-.5, p.Y-.5)
		if radius < 5.0/32-.04 || radius > 5.0/16+.04 {
			t.Fatal("triangle outside certified annular fill", p)
		}
	}
	report, e := collision.SelfIntersectionsBVHContext(context.Background(), m.Mesh, collision.Options{})
	if e != nil || !report.Complete || len(report.Contacts) > 0 {
		t.Fatal("footprint display overlap", report, e)
	}
	t.Log("nested footprint", len(m.Mesh.Vertices), len(m.Mesh.Triangles), area, "source", want, "bound", squared)
}
