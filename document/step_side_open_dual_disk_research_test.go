//go:build research

package document

import (
	"bytes"
	"context"
	"math"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestDocumentSideOpenDualDiskUnequalRecipe(t *testing.T) {
	ctx := context.Background()
	s := State{LengthUnit: .001, Features: map[string]Feature{
		"stock": {Kind: BoxFeature, Max: geom.Vec3{X: 60, Y: 22, Z: 6}},
		"cut": {Kind: SideOpenDualDiskCutFeature, Input: "stock", SideOpenDualDisk: &SideOpenDualDiskRecipe{
			U: geom.Vec3{X: 1}, Inward: geom.Vec3{Z: 1}, Through: true,
			Discs: [2]trimmed.ExtrusionCircle{{Center: geom.Vec2{X: 20}, Radius: 13}, {Center: geom.Vec2{X: 34}, Radius: 15}},
		}},
	}, Nodes: map[string]Node{"cut": {Asset: "cut", Transform: geom.Identity()}}}
	d, e := New(s, 0)
	if e != nil {
		t.Fatal(e)
	}
	snap, _ := d.Snapshot()
	var b bytes.Buffer
	if e := snap.WriteSideOpenDualDiskSTEP(ctx, &b, "cut"); e != nil {
		t.Fatal(e)
	}
	if _, e := step.ReadSideOpenDualDiskProduct(ctx, bytes.NewReader(b.Bytes()), step.Limits{}, 1e-9); e == nil {
		t.Fatal("unequal feature selected the equal exchange dialect")
	}
	p, e := step.ReadUnequalSideOpenDualDiskProduct(ctx, &b, step.Limits{}, 1e-9)
	if e != nil || p.Descriptor.Discs[0].Radius != 13 || p.Descriptor.Discs[1].Radius != 15 || p.SourceWallFaces[0] == p.SourceWallFaces[1] || p.SourceFloorFace != 0 {
		t.Fatal("unequal cut lost its two owned native circular supports", p.Descriptor, e)
	}
	m, e := p.Native.Measures()
	opening := 84 + (169*(math.Pi/2+math.Asin(5./13))+225*(math.Pi/2+math.Asin(.6)))/2
	if e != nil || math.Abs(m.Volume-(7920-6*opening)) > 1e-7 {
		t.Fatal("wrong native readback volume", m, e)
	}
}
