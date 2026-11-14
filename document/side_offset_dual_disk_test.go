package document

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestDocumentOffsetDualDiskRecipe(t *testing.T) {
	ctx := context.Background()
	s := State{LengthUnit: .001, Features: map[string]Feature{
		"stock": {Kind: BoxFeature, Max: geom.Vec3{X: 40, Y: 10, Z: 6}},
		"cut": {Kind: SideOpenDualDiskCutFeature, Input: "stock", SideOpenDualDisk: &SideOpenDualDiskRecipe{
			U: geom.Vec3{X: 1}, Inward: geom.Vec3{Z: 1}, Through: true, OffsetCenters: true,
			Discs: [2]trimmed.ExtrusionCircle{{Center: geom.Vec2{X: 12.5, Y: -7.5}, Radius: 65. / 8}, {Center: geom.Vec2{X: 22, Y: -7.5}, Radius: 87. / 8}},
		}},
	}, Nodes: map[string]Node{"cut": {Asset: "cut", Transform: geom.Affine{{-1, 0, 0, 40}, {0, 1, 0, 2}, {0, 0, 1, 1}}}}}
	d, e := New(s, 2)
	if e != nil {
		t.Fatal(e)
	}
	var data bytes.Buffer
	if e = d.Write(&data); e != nil {
		t.Fatal(e)
	}
	d, e = Read(&data, 0, 2)
	if e != nil {
		t.Fatal(e)
	}
	s, _ = d.Snapshot()
	if p := s.Features["cut"].SideOpenDualDisk; !p.OffsetCenters || !p.Through || p.ExactWalls {
		t.Fatal("persisted feature lost its offset construction policy", p)
	}
	if _, e := s.SideOpenDualDiskPresentation(ctx, []string{"cut"}, trimmed.SideOpenDualDiskMeshOptions{Deflection: .5, AngularTurn: .2}); e != nil {
		t.Fatal("reloaded offset cut did not present from original supports", e)
	}
	if e = s.WriteSideOpenDualDiskSTEP(ctx, &data, "cut"); e != nil || !bytes.Contains(data.Bytes(), []byte("PRODUCT('side-cut-offset-disks'")) {
		t.Fatal("reflected offset occurrence needs its registered STEP class", e)
	}
	p, e := step.ReadOffsetSideOpenDualDiskProduct(ctx, &data, step.Limits{}, 1e-9)
	if e != nil || p.SourceFloorFace != 0 || p.SourceWallFaces[0] == 0 || p.SourceWallFaces[1] == 0 || p.SourceWallFaces[0] == p.SourceWallFaces[1] {
		t.Fatal("reflected through product did not retain the two original wall owners", p.SourceWallFaces, p.SourceFloorFace, e)
	}
	for _, disc := range p.Descriptor.Discs {
		if disc.Center.Y != -7.5 {
			t.Fatal("readback moved a source circle onto the opening side", disc)
		}
	}
	edit := d.Begin()
	edit.State.Features["cut"].SideOpenDualDisk.ExactWalls = true
	if e = edit.Commit(); !errors.Is(e, geom.ErrInvalid) {
		t.Fatal("two distinct exact construction policies were accepted together", e)
	}
}
