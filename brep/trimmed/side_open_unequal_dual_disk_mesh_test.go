package trimmed

import (
	"context"
	"errors"
	"math"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestUnequalSideOpenDualDiskCertifiedDisplay(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	frame, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	discs := [2]ExtrusionCircle{{Center: geom.Vec2{X: 2.5}, Radius: 13. / 8}, {Center: geom.Vec2{X: 34. / 8}, Radius: 15. / 8}}
	removed := (84 + (169*(math.Pi/2+math.Asin(5./13))+225*(math.Pi/2+math.Asin(9./15)))/2) / 64
	for _, depth := range []float64{.375, .75} {
		s, e := CutUnequalSideOpenDualDisks(ctx, frame, geom.Vec2{}, geom.Vec2{X: 7.5, Y: 2.75}, discs, .75, depth, 1e-10, trimvalidate.Options{})
		if e != nil {
			t.Fatal(e)
		}
		r, e := s.TessellateCertified(ctx, SideOpenDualDiskMeshOptions{Deflection: .05, AngularTurn: .2})
		if e != nil {
			t.Fatal(depth, e)
		}
		v, _ := r.DisplayVolume.Float64()
		want := 7.5*2.75*.75 - removed*depth
		if math.Abs(v-want) > .025 {
			t.Fatal(v, want)
		}
		t.Log(depth, len(r.Mesh.Triangles), v, want)
		if e = r.Mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		bad := s
		bad.WallFaces[0] = bad.WallFaces[1]
		if _, e = bad.TessellateCertified(ctx, SideOpenDualDiskMeshOptions{Deflection: .05, AngularTurn: .2}); e == nil {
			t.Fatal("duplicate wall accepted")
		}
	}
}

func TestUnequalSideOpenDualDiskBridgeDisplay(t *testing.T) {
	ctx := context.Background()
	frame, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	discs := [2]ExtrusionCircle{{Radius: 13}, {Center: geom.Vec2{X: 14}, Radius: 15}}
	s, e := CutUnequalSideOpenDualDisks(ctx, frame, geom.Vec2{X: -20}, geom.Vec2{X: 40, Y: 22}, discs, 6, 3, 1e-10, trimvalidate.Options{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Solid.TessellateMixedFaceCertified(ctx, 0, SolidMixedFaceMeshOptions{Face: MixedExactMultiSpanMeshOptions{Face: MixedExactCurvedMeshOptions{Interior: CurvedQuadraticTrimMeshOptions{Deflection: .5}}}}); !errors.Is(e, geom.ErrDomain) {
		t.Fatal("default control hull gate", e)
	}
	r, e := s.TessellateCertified(ctx, SideOpenDualDiskMeshOptions{Deflection: .5, AngularTurn: .2})
	if e != nil {
		t.Fatal(e)
	}
	if r.Network.Faces[0].UVRegion.DomainSubdivisions == 0 {
		t.Fatal("missing exact overhang proof")
	}
	t.Log(len(r.Mesh.Triangles), r.Network.Faces[0].UVRegion.DomainSubdivisions)
}
