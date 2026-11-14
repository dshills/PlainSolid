package trimmed

import (
	"context"
	"math"
	"plainsolid/brep/trimvalidate"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestCertifiedSelectedTubeRegionCollection(t *testing.T) {
	r := disjointRegionMeshFixture(t)
	ctx := context.Background()
	o := AlgebraicTubeRegionCollectionMeshOptions{Component: AlgebraicTubeRegionMeshOptions{Footprint: AlgebraicTubeFootprintMeshOptions{Boundary: AlgebraicTubeRegionsBoundaryOptions{Loop: AlgebraicTubeLensBoundaryOptions{Run: AlgebraicTubeRunMeshOptions{Display: RationalTubePolylineOptions{Deflection: .025}}}}}, Surface: MultiSpanTubeLensMeshOptions{Closed: AlgebraicTubeLensMeshOptions{Interior: CurvedQuadraticTrimMeshOptions{Deflection: .01}, Collision: collision.Options{MaxPairs: 4000000}}}}}
	m, e := r.TessellateSelectedRegionsClosedCertified(ctx, []int{0, 1}, o)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Components) != 2 || !m.Collision.Complete || len(m.Collision.Contacts) != 0 || len(m.DisplayForest) != 2 {
		t.Fatal("complete disjoint collection", m.Collision)
	}
	if e = m.Mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	for i, c := range m.Components {
		for j, p := range c.Mesh.Vertices {
			if m.Mesh.Vertices[m.ComponentVertices[i][j]] != p {
				t.Fatal("component vertex source")
			}
		}
		for b, local := range c.GlobalBoundaryVertices {
			if m.GlobalBoundaryVertices[b] != m.ComponentVertices[i][local] || m.Mesh.Vertices[m.ComponentVertices[i][local]] != m.Boundary.Vertices[b] {
				t.Fatal("original boundary identity")
			}
		}
	}
	if len(m.OriginalRegion) != len(m.Mesh.Triangles) || len(m.OriginalChart) != len(m.Mesh.Triangles) {
		t.Fatal("triangle ancestry")
	}
	volume, _ := m.SignedDisplayVolume.Float64()
	want := math.Pi * math.Pow(5.0/32, 4)
	if math.Abs(volume-want) > .15*want {
		t.Fatal("independent two-disk display volume", volume, want)
	}
	for _, ids := range [][]int{nil, {1, 0}, {0, 0}, {2}} {
		if x, e := r.TessellateSelectedRegionsClosedCertified(ctx, ids, o); e == nil || len(x.Mesh.Vertices) != 0 {
			t.Fatal("bad selection", ids, e)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if x, e := r.TessellateSelectedRegionsClosedCertified(cancelled, []int{0, 1}, o); e != context.Canceled || len(x.Mesh.Vertices) != 0 {
		t.Fatal("cancel", e)
	}
	bad := o
	bad.Component.Footprint.Boundary.MaxOperations = 1
	if x, e := r.TessellateSelectedRegionsClosedCertified(ctx, []int{0, 1}, bad); e == nil || len(x.Mesh.Vertices) != 0 {
		t.Fatal("atomic boundary budget", e)
	}
	// A nested display without triangle crossings must still reject if it
	// changes the original disjoint containment forest.
	shifted := m.Boundary
	shifted.Vertices = append([]geom.Vec3(nil), m.Boundary.Vertices...)
	for _, id := range shifted.LoopVertices[1] {
		p := shifted.Vertices[id]
		shifted.Vertices[id] = geom.Vec3{X: .25 + (p.X-.75)/4, Y: .5 + (p.Y-.5)/4, Z: p.Z}
	}
	if _, proof, e := certifyTubeDisplayForest(ctx, r, shifted, o.Component.Footprint.Polygon); e == nil || proof.Status != trimvalidate.Certified {
		t.Fatal("changed displayed containment accepted")
	}
	limited := o
	limited.Component.Surface.Closed.Collision.MaxPairs = m.CollisionPairs - 1
	if x, e := r.TessellateSelectedRegionsClosedCertified(ctx, []int{0, 1}, limited); e == nil || len(x.Mesh.Vertices) != 0 || len(x.Components) != 0 {
		t.Fatal("late cumulative collision quota", e)
	}
	single, e := r.TessellateSelectedRegionsClosedCertified(ctx, []int{1}, o)
	if e != nil {
		t.Fatal("selected subset", e)
	}
	used := make([]bool, len(single.Mesh.Vertices))
	for _, tri := range single.Mesh.Triangles {
		for _, i := range tri {
			used[i] = true
		}
	}
	for _, u := range used {
		if !u {
			t.Fatal("unselected loop leaked isolated display vertices")
		}
	}
	for _, id := range single.OriginalRegion {
		if id != 1 {
			t.Fatal("lost selected region ID")
		}
	}
	t.Log("shared network collection", len(m.Mesh.Vertices), len(m.Mesh.Triangles), "collision pairs", m.CollisionPairs, "residual work", m.ResidualOperations)
}
