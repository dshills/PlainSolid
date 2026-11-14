package trimmed

import (
	"context"
	"math"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"testing"
)

func TestMixedBoundaryBudgetRetainsFinalProof(t *testing.T) {
	ctx := context.Background()
	frame, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	discs := [2]ExtrusionCircle{{Center: geom.Vec2{X: 3.5, Y: 4}, Radius: 2.5}, {Center: geom.Vec2{X: 6.5, Y: 4}, Radius: 2.5}}
	cut, e := CutSideOpenDualDisks(ctx, frame, geom.Vec2{Y: 4}, geom.Vec2{X: 10, Y: 10}, discs, 6, 3, 1e-10, trimvalidate.Options{})
	if e != nil {
		t.Fatal(e)
	}
	o := SolidMixedFaceMeshOptions{Face: MixedExactMultiSpanMeshOptions{Face: MixedExactCurvedMeshOptions{Interior: CurvedQuadraticTrimMeshOptions{Deflection: .0035}, Boundary: MixedExactAffineMeshOptions{MaxBoundaryVertices: 2048, Topology: trimvalidate.Options{MaxEdges: 2048, MaxWork: 2000000}}}}}
	original, e := cut.Solid.TessellateMixedFaceCertified(ctx, 0, o)
	if e != nil {
		t.Fatal(e)
	}
	region := original.UVRegion.Region
	if _, e := triangulateCertifiedAffineTrimRegion(ctx, region, 0, 100000); e == nil {
		t.Fatal("legacy256 gate changed")
	}
	if bad, e := triangulateCertifiedAffineTrimRegionBounded(ctx, region, 0, 100000, 2048, 1); e == nil || len(bad.Mesh.Vertices) != 0 {
		t.Fatal("exact triangulation budget", e)
	}
	o.Face.Face.BoundaryModelBudget = .0035 / 4
	refined, e := cut.Solid.TessellateMixedFaceCertified(ctx, 0, o)
	if e != nil {
		t.Fatal(e)
	}
	if len(refined.Mesh.Triangles) >= len(original.Mesh.Triangles) {
		t.Fatal("boundary allocation did not reduce work")
	}
	q, _ := refined.FillSquaredDistance.Float64()
	if q > .0035*.0035 {
		t.Fatal("final distance weakened", q)
	}
	for _, bad := range []float64{-.1, math.NaN(), .0035} {
		o.Face.Face.BoundaryModelBudget = bad
		if r, e := cut.Solid.TessellateMixedFaceCertified(ctx, 0, o); e == nil || len(r.Mesh.Vertices) != 0 {
			t.Fatal("invalid budget accepted", bad, e)
		}
	}
}
