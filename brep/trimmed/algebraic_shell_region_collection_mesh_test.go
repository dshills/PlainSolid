//go:build research

package trimmed

import (
	"context"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestNativeTubeRegionCollectionMesh(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	s := radialMaterialForBounds(t)
	t.Log("original native radial material proved")
	o := AlgebraicShellRegionCollectionMeshOptions{}
	o.Section.Trace.Coverage.Isolation.SeparatingProjections = true
	o.Section.Trace.Coverage.Isolation.ParameterTolerance = .25
	o.Section.Trace.Coverage.Cell.MaxOperations = 16000000
	o.Mesh.Component.Footprint.Boundary.Loop.Run.Display.Deflection = .05
	o.Mesh.Component.Surface.Closed.Interior.Deflection = .04
	o.Mesh.Component.Surface.Closed.Collision = collision.Options{MaxPairs: 4000000}
	// Diagnostic copies are not authoritative construction recipes.
	s.Faces = nil
	s.Edges = nil
	s.Vertices = nil
	m, e := s.TessellateTubeRegionComponentsCertified(ctx, o)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Components) != 1 || len(m.ComponentFaces) != 1 || len(m.ComponentBranches) != 1 || len(m.BoundaryEdges) != len(s.index) || len(m.TopologyVertices) != len(s.vertices) {
		t.Fatal("complete original topology")
	}
	if e = m.Mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	if !m.Collision.Complete || len(m.Collision.Contacts) != 0 {
		t.Fatal(m.Collision)
	}
	for i, face := range m.TriangleFaces {
		if face != m.ComponentFaces[0][m.OriginalChart[i]] {
			t.Fatal("original face ownership")
		}
	}
	for edge, path := range m.BoundaryEdges {
		if len(path) < 2 || path[0] != m.TopologyVertices[s.index[edge].Start] || path[len(path)-1] != m.TopologyVertices[s.index[edge].End] {
			t.Fatal("canonical native edge", edge)
		}
	}
	t.Log("native collection positive completed")
	bad := o
	bad.Endpoints.MaxOperations = 1
	if x, e := s.TessellateTubeRegionComponentsCertified(ctx, bad); e == nil || len(x.Mesh.Vertices) != 0 || len(x.BoundaryEdges) != 0 {
		t.Fatal("late atomic endpoint quota", e)
	}
	broken := s
	broken.faces = append([]AlgebraicShellBooleanFace(nil), s.faces...)
	broken.faces[0].Reversed = !broken.faces[0].Reversed
	if x, e := broken.TessellateTubeRegionComponentsCertified(ctx, o); e == nil || len(x.Mesh.Vertices) != 0 {
		t.Fatal("material orientation", e)
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if x, e := s.TessellateTubeRegionComponentsCertified(cancelCtx, o); e != context.Canceled || len(x.Mesh.Vertices) != 0 {
		t.Fatal("cancel", e)
	}
	if x, e := s.TessellateTubeRegionComponentsCertified(ctx, AlgebraicShellRegionCollectionMeshOptions{Endpoints: geom.ExactCurvePlaneEventOptions{MaxOperations: -1}}); e == nil || len(x.Mesh.Vertices) != 0 {
		t.Fatal("invalid quota", e)
	}
	t.Log("native vertices/triangles/endpoint operations", len(m.Mesh.Vertices), len(m.Mesh.Triangles), m.EndpointOperations)
}
