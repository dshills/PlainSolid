package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"testing"
)

func polygonFootprintNetwork(loops [][]geom.Vec2) CertifiedAlgebraicTubeRegionsBoundary {
	var n CertifiedAlgebraicTubeRegionsBoundary
	for _, loop := range loops {
		var ids, path []int
		for i, p := range loop {
			ids = append(ids, len(n.Vertices))
			path = append(path, i)
			n.Vertices = append(n.Vertices, geom.Vec3{X: p.X, Y: p.Y, Z: 7})
		}
		path = append(path, 0)
		n.LoopVertices = append(n.LoopVertices, ids)
		n.Loops = append(n.Loops, CertifiedAlgebraicTubeLensBoundary{Run: CertifiedAlgebraicTubeRun{PieceVertices: [][]int{path}, SquaredErrors: [][]*big.Rat{{new(big.Rat)}}}})
	}
	return n
}
func TestTubeRegionHoledFootprintSeed(t *testing.T) {
	outer := []geom.Vec2{{X: -2, Y: -2}, {X: 2, Y: -2}, {X: 2, Y: 2}, {X: -2, Y: 2}}
	inner := []geom.Vec2{{X: -1, Y: -1}, {X: 1, Y: -1}, {X: 1, Y: 1}, {X: -1, Y: 1}}
	n := polygonFootprintNetwork([][]geom.Vec2{outer, inner})
	o := AlgebraicTubeFootprintMeshOptions{}
	m, e := triangulateTubeFootprintNetwork(context.Background(), n, []int{0, 1}, o)
	if e != nil {
		t.Fatal(e)
	}
	if m.PolygonArea.Cmp(big.NewRat(12, 1)) != 0 || len(m.BoundaryLoops) != 2 {
		t.Fatal("annular coverage", m.PolygonArea)
	}
	for i, p := range m.Mesh.Vertices {
		q := n.Vertices[m.GlobalBoundaryVertices[i]]
		if p.X != q.X || p.Y != q.Y || p.Z != 0 {
			t.Fatal("common boundary mapping")
		}
	}
	for _, tri := range m.Mesh.Triangles {
		p := m.Mesh.Vertices[tri[0]].Add(m.Mesh.Vertices[tri[1]]).Add(m.Mesh.Vertices[tri[2]]).Scale(1.0 / 3)
		if math.Abs(p.X) < 1 && math.Abs(p.Y) < 1 {
			t.Fatal("filled the hole")
		}
	}
	touching := []geom.Vec2{{X: 1, Y: -1}, {X: 2, Y: -1}, {X: 2, Y: 1}, {X: 1, Y: 1}}
	if _, e := triangulateTubeFootprintNetwork(context.Background(), polygonFootprintNetwork([][]geom.Vec2{outer, touching}), []int{0, 1}, o); e == nil {
		t.Fatal("touching hole accepted")
	}
	for _, limited := range []AlgebraicTubeFootprintMeshOptions{{MaxTriangles: 1}, {Polygon: trimvalidate.Options{MaxEdges: 7}}} {
		m, e := triangulateTubeFootprintNetwork(context.Background(), n, []int{0, 1}, limited)
		if e == nil || len(m.Mesh.Vertices) > 0 {
			t.Fatal("partial footprint", e)
		}
	}
}
func TestTubeRegionSourceFootprint(t *testing.T) {
	r := disjointRegionMeshFixture(t)
	o := AlgebraicTubeFootprintMeshOptions{Boundary: AlgebraicTubeRegionsBoundaryOptions{Loop: AlgebraicTubeLensBoundaryOptions{Run: AlgebraicTubeRunMeshOptions{Display: RationalTubePolylineOptions{Deflection: .025}}}}}
	m, e := r.TessellateFootprintCertified(context.Background(), 0, o)
	if e != nil {
		t.Fatal(e)
	}
	area, _ := m.PolygonArea.Float64()
	want := math.Pi * math.Pow(5.0/32, 2)
	if math.Abs(area-want) > .05*want {
		t.Fatal(area, want)
	}
	if m.SourceFillSquaredDistance.Sign() <= 0 || m.PolygonProof.Status != trimvalidate.Certified {
		t.Fatal("missing proofs")
	}
	for i, p := range m.Mesh.Vertices {
		q := m.Boundary.Vertices[m.GlobalBoundaryVertices[i]]
		if p.X != q.X || p.Y != q.Y || p.Z != 0 {
			t.Fatal("global network mapping")
		}
	}
	t.Log(len(m.Mesh.Triangles), area, want)
}

func TestExactMultipleHoleFootprintBridges(t *testing.T) {
	outer := []geom.Vec2{{X: -4, Y: -4}, {Y: -4}, {X: 4, Y: -4}, {X: 4, Y: 4}, {Y: 4}, {X: -4, Y: 4}}
	left := []geom.Vec2{{X: -3, Y: -1}, {X: -1, Y: -1}, {X: -1, Y: 1}, {X: -3, Y: 1}}
	right := []geom.Vec2{{X: 1, Y: -1}, {X: 3, Y: -1}, {X: 3, Y: 1}, {X: 1, Y: 1}}
	n := polygonFootprintNetwork([][]geom.Vec2{outer, left, right})
	m, e := triangulateTubeFootprintNetwork(context.Background(), n, []int{0, 1, 2}, AlgebraicTubeFootprintMeshOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if m.PolygonArea.Cmp(big.NewRat(56, 1)) != 0 || len(m.Mesh.Vertices) != 14 || len(m.Mesh.Triangles) != 16 {
		t.Fatal("lost boundary subdivisions", m.PolygonArea, len(m.Mesh.Vertices), len(m.Mesh.Triangles))
	}
	if _, e := triangulateTubeFootprintNetwork(context.Background(), n, []int{0, 1, 2}, AlgebraicTubeFootprintMeshOptions{MaxPredicates: 1}); e == nil {
		t.Fatal("predicate budget")
	}
}
