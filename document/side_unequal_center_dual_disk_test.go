package document

import (
	"bytes"
	"context"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestDocumentUnequalCenterDualDiskJoin(t *testing.T) {
	ctx := context.Background()
	join := [2]string{"10", "1"}
	s := State{LengthUnit: .001, Features: map[string]Feature{
		"stock": {Kind: BoxFeature, Max: geom.Vec3{X: 60, Y: 10, Z: 6}},
		"cut": {Kind: SideOpenDualDiskCutFeature, Input: "stock", SideOpenDualDisk: &SideOpenDualDiskRecipe{
			U: geom.Vec3{X: 1}, Inward: geom.Vec3{Z: 1}, Depth: 3, UnequalCenterJoin: &join,
			Discs: [2]trimmed.ExtrusionCircle{{Center: geom.Vec2{X: 7, Y: -3}, Radius: 5}, {Center: geom.Vec2{X: 30, Y: -20}, Radius: 29}},
		}},
	}, Nodes: map[string]Node{"cut": {Asset: "cut", Transform: geom.Affine{{-1, 0, 0, 60}, {0, 1, 0, 2}, {0, 0, 1, 1}}}}}
	d, e := New(s, 2)
	if e != nil {
		t.Fatal(e)
	}
	join[0] = "11" // New must own the recipe, including its join.
	s, _ = d.Snapshot()
	s.Features["cut"].SideOpenDualDisk.UnequalCenterJoin[1] = "2"
	var data bytes.Buffer
	if e = d.Write(&data); e != nil {
		t.Fatal(e)
	}
	d, e = Read(&data, 0, 2)
	if e != nil {
		t.Fatal(e)
	}
	s, _ = d.Snapshot()
	if *s.Features["cut"].SideOpenDualDisk.UnequalCenterJoin != [2]string{"10", "1"} {
		t.Fatal("the document lost exact join ownership")
	}
	if e = s.WriteSideOpenDualDiskSTEP(ctx, &data, "cut"); e != nil || !bytes.Contains(data.Bytes(), []byte("PRODUCT('side-cut-unequal-centers'")) {
		t.Fatal("the reflected occurrence lost its certified source join", e)
	}
	p, e := step.ReadUnequalCenterSideOpenDualDiskProduct(ctx, &data, step.Limits{}, 1e-9)
	if e != nil || p.SourceFloorFace == 0 || p.SourceWallFaces[0] == p.SourceWallFaces[1] || p.RadicalJoin[0].RatString() != "50" || p.RadicalJoin[1].RatString() != "1" {
		t.Fatal("reflected native STEP recipe lost the actual wall join", p.RadicalJoin, e)
	}
	edit := d.Begin()
	edit.State.Features["cut"].SideOpenDualDisk.UnequalCenterJoin[1] = "100000000000000001/100000000000000000"
	if e = edit.Commit(); e == nil {
		t.Fatal("a near join without exact source incidence was accepted")
	}
}
