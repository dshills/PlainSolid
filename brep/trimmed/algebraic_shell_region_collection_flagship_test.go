package trimmed

import (
	"context"
	"math"
	"math/big"
	"os"
	"testing"

	"plainsolid/brep/trimvalidate"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/internal/testgraphfixture"
	"plainsolid/intersect"
)

func tripleRadialAffineCoverage() *intersect.RadialOverlapCoverageOptions {
	return &intersect.RadialOverlapCoverageOptions{
		AffineCharts: true,
		Center:       [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)},
		Radii:        testgraphfixture.TripleRadialRadii(),
		Proof:        geom.RadialTubeCoverageOptions{MaxOperations: 128000000, MaxTubes: 512},
	}
}

// Invoked by the original material fixture only under the explicit environment
// gate PLAINSOLID_TRIPLE_RADIAL_MESH=1. It reuses the genuine nominal-degree8
// original native material, never the independent exact degree6 test chart.
func qualifyTripleRadialNativeMesh(t *testing.T, s AlgebraicShellBoolean) {
	t.Helper()
	o := AlgebraicShellRegionCollectionMeshOptions{Section: nestedTubeSectionOptions(), Regions: nestedTubeRegionOptions()}
	o.Section.MaxSegments = 512
	o.Section.Trace.Coverage.OverlappingAffineXYFallback = true
	if os.Getenv("PLAINSOLID_TRIPLE_RADIAL_AFFINE_COVERAGE") == "1" {
		o.Section.Trace.Coverage.RadialOverlap = tripleRadialAffineCoverage()
	}
	o.Mesh.Component = AlgebraicTubeRegionMeshOptions{
		Footprint: AlgebraicTubeFootprintMeshOptions{
			Boundary: AlgebraicTubeRegionsBoundaryOptions{MaxOperations: 512000000, Loop: AlgebraicTubeLensBoundaryOptions{Run: AlgebraicTubeRunMeshOptions{Display: RationalTubePolylineOptions{Deflection: .03, MaxOperations: 64000000}, MaxPieces: 512}, MaxOwnerOperations: 64000000}},
			Polygon:  trimvalidate.Options{MaxEdges: 2048, MaxWork: 8000000},
		},
		Surface: MultiSpanTubeLensMeshOptions{AffineCharts: true, MaxResidualOperations: 512000000, Closed: AlgebraicTubeLensMeshOptions{Collision: collision.Options{MaxPairs: 32000000}, Interior: CurvedQuadraticTrimMeshOptions{Deflection: .001}}, Span: MultiSpanTriangleBoundOptions{MaxDegree: 8, MaxResidualOperations: 16000000}},
	}
	o.Mesh.Component.Footprint.Boundary.Loop.Run.Partition.MaxOperations = 64000000
	t.Log("starting complete original degree8 native collection reproof and fill")
	m, e := s.TessellateTubeRegionComponentsCertified(context.Background(), o)
	if e != nil {
		t.Fatal("original degree8 native mesh", e)
	}
	if len(m.Components) != 2 || len(m.ComponentFaces) != 2 || len(m.ComponentBranches) != 2 || len(m.BoundaryEdges) != len(s.index) || len(m.TopologyVertices) != len(s.vertices) || !m.Collision.Complete || len(m.Collision.Contacts) != 0 {
		t.Fatal("complete original native mesh ownership")
	}
	if e = m.Mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	seenFaces := map[int]bool{}
	for _, face := range m.TriangleFaces {
		if face < 0 || face >= len(s.faces) {
			t.Fatal("invalid original face")
		}
		seenFaces[face] = true
	}
	if len(seenFaces) != 4 {
		t.Fatal("missing original face")
	}
	for edge, path := range m.BoundaryEdges {
		if len(path) < 2 || path[0] != m.TopologyVertices[s.index[edge].Start] || path[len(path)-1] != m.TopologyVertices[s.index[edge].End] {
			t.Fatal("lost original endpoint", edge)
		}
	}
	euler := 0
	for _, c := range m.Components {
		edges := map[[2]int]bool{}
		for _, tri := range c.Mesh.Triangles {
			for i, a := range tri {
				b := tri[(i+1)%3]
				if a > b {
					a, b = b, a
				}
				edges[[2]int{a, b}] = true
			}
		}
		chi := len(c.Mesh.Vertices) - len(edges) + len(c.Mesh.Triangles)
		if chi != 0 && chi != 2 {
			t.Fatal("component genus", chi)
		}
		euler += chi
	}
	if euler != 2 {
		t.Fatal("disk plus annulus incidence", euler)
	}
	volume, _ := m.SignedDisplayVolume.Float64()
	analytic := tripleRadialAnalyticVolume()
	if math.Abs(volume-analytic) > .25*analytic {
		t.Fatal("independent volume comparison", volume, analytic)
	}
	t.Log("original degree8 native disk/annulus mesh", len(m.Mesh.Vertices), len(m.Mesh.Triangles), "display volume", volume, "analytic", analytic, "residual work", m.ResidualOperations, "collision pairs", m.CollisionPairs, "endpoint work", m.EndpointOperations)
}
