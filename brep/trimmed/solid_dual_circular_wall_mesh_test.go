package trimmed

import (
	"bytes"
	"context"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func TestStoredDualCircularWallCertifiedDisplay(t *testing.T) {
	ctx := context.Background()
	frame, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	discs := [2]ExtrusionCircle{{Center: geom.Vec2{X: 2.5}, Radius: 13. / 8}, {Center: geom.Vec2{X: 34. / 8}, Radius: 15. / 8}}
	cut, e := CutUnequalSideOpenDualDisks(ctx, frame, geom.Vec2{}, geom.Vec2{X: 7.5, Y: 2.75}, discs, .75, .375, 1e-10, trimvalidate.Options{})
	if e != nil {
		t.Fatal(e)
	}
	var data bytes.Buffer
	if e = WriteJSON(&data, cut.Solid); e != nil {
		t.Fatal(e)
	}
	s, e := ReadJSONContext(ctx, &data, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	options := SideOpenDualDiskMeshOptions{Deflection: .05, AngularTurn: .2}
	a, e := cut.TessellateCertified(ctx, options)
	if e != nil {
		t.Fatal(e)
	}
	b, e := s.TessellateDualCircularWallCertified(ctx, options)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(a.Mesh, b.Mesh) || a.SquaredDistance.Cmp(b.SquaredDistance) != 0 || a.SquaredFacetTangent.Cmp(b.SquaredFacetTangent) != 0 {
		t.Fatal("recovery changed exact display")
	}
	bad := s
	bad.faces = append([]Face(nil), s.faces...)
	bad.faces[cut.WallFaces[0][0]].Surface = bad.faces[0].Surface
	if r, e := bad.TessellateDualCircularWallCertified(ctx, options); e == nil || len(r.Mesh.Vertices) != 0 {
		t.Fatal("missing wall accepted")
	}
}
