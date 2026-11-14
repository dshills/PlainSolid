package trimmed

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"testing"
)

func BenchmarkSideOpenDualDiskCloseDisplay(b *testing.B) {
	ctx := context.Background()
	frame, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	discs := [2]ExtrusionCircle{{Center: geom.Vec2{X: 3.5, Y: 4}, Radius: 2.5}, {Center: geom.Vec2{X: 6.5, Y: 4}, Radius: 2.5}}
	s, e := CutSideOpenDualDisks(ctx, frame, geom.Vec2{Y: 4}, geom.Vec2{X: 10, Y: 10}, discs, 6, 3, 1e-10, trimvalidate.Options{})
	if e != nil {
		b.Fatal(e)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r, e := s.TessellateCertified(ctx, SideOpenDualDiskMeshOptions{Deflection: .0035, AngularTurn: .08})
		if e != nil {
			b.Fatal(e)
		}
		encoded, _ := json.Marshal(r.Mesh)
		b.Logf("mesh sha256 %x; distance %s; turn %s; facet %s", sha256.Sum256(encoded), r.SquaredDistance.RatString(), r.SquaredTurnTangent.RatString(), r.SquaredFacetTangent.RatString())
		if len(r.Mesh.Triangles) == 0 {
			b.Fatal(len(r.Mesh.Triangles))
		}
	}
}
