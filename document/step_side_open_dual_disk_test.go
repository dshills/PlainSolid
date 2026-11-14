package document

import (
	"bytes"
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestDocumentSideOpenDualDiskSTEPPlacedRecipe(t *testing.T) {
	ctx := context.Background()
	box := Feature{Kind: BoxFeature, Min: geom.Vec3{Y: 4}, Max: geom.Vec3{X: 10, Y: 10, Z: 6}}
	cut := Feature{Kind: SideOpenDualDiskCutFeature, Input: "stock", SideOpenDualDisk: &SideOpenDualDiskRecipe{
		Origin: geom.Vec3{Y: 4}, U: geom.Vec3{X: 1}, Inward: geom.Vec3{Z: 1}, Depth: 3,
		Discs: [2]trimmed.ExtrusionCircle{{Center: geom.Vec2{X: 3.5}, Radius: 2.5}, {Center: geom.Vec2{X: 6.5}, Radius: 2.5}},
	}}
	s := State{LengthUnit: .01, Features: map[string]Feature{"stock": box, "cut": cut}, Nodes: map[string]Node{
		"root": {Transform: geom.Translation(geom.Vec3{X: 20})},
		"part": {Name: "Reflected pocket pair", Parent: "root", Asset: "cut", Transform: geom.Affine{{-1, 0, 0, 10}, {0, 1, 0, 2}, {0, 0, 1, -3}}},
	}}
	var b bytes.Buffer
	if e := s.WriteSideOpenDualDiskSTEP(ctx, &b, "part"); e != nil {
		t.Fatal(e)
	}
	p, e := step.ReadSideOpenDualDiskProduct(ctx, &b, step.Limits{}, 1e-10)
	if e != nil || p.Descriptor.Name != "Reflected pocket pair" || p.Descriptor.LengthUnit != .01 {
		t.Fatal("rooted posed source and SI units", p.Descriptor, e)
	}
	m, e := p.Native.Measures()
	vol := 360 - 3*(6.25*(math.Pi-math.Acos(.6))+3)
	if e != nil || math.Abs(m.Volume-vol) > 1e-8 || math.Abs(m.Centroid.X-25) > 1e-8 || m.Centroid.Z <= 0 {
		t.Fatal("placed document measures", m, e)
	}
	for _, v := range p.Native.Solid.Vertices() {
		if v.X < 20 || v.X > 30 || v.Y < 6 || v.Y > 12 || v.Z < -3 || v.Z > 3 {
			t.Fatal("placed native vertex escaped stock", v)
		}
	}
	node := s.Nodes["part"]
	node.Transform[0][0] = 2
	s.Nodes["part"] = node
	b.Reset()
	if e := s.WriteSideOpenDualDiskSTEP(ctx, &b, "part"); !errors.Is(e, geom.ErrDomain) || b.Len() != 0 {
		t.Fatal("nonrigid placement wrote a partial artifact", e, b.Len())
	}
}

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

func TestDocumentSideOpenDualDiskOffCenterAndExactWalls(t *testing.T) {
	ctx := context.Background()
	for _, exact := range []bool{false, true} {
		r0, r1, x1, stockY := 5., 8.5, 24.5, 12.
		if exact {
			r0, r1, x1, stockY = 13, 15, 24, 22
		}
		s := State{LengthUnit: .001, Features: map[string]Feature{
			"stock": {Kind: BoxFeature, Max: geom.Vec3{X: 50, Y: stockY, Z: 6}},
			"cut": {Kind: SideOpenDualDiskCutFeature, Input: "stock", SideOpenDualDisk: &SideOpenDualDiskRecipe{
				U: geom.Vec3{X: 1}, Inward: geom.Vec3{Z: 1}, Depth: 3, ExactWalls: exact,
				Discs: [2]trimmed.ExtrusionCircle{{Center: geom.Vec2{X: 20}, Radius: r0}, {Center: geom.Vec2{X: x1}, Radius: r1}},
			}},
		}, Nodes: map[string]Node{"cut": {Asset: "cut", Transform: geom.Identity()}}}
		d, e := New(s, 2)
		if e != nil {
			t.Fatal(exact, e)
		}
		var state bytes.Buffer
		if e = d.Write(&state); e != nil {
			t.Fatal(e)
		}
		loaded, e := Read(&state, 0, 2)
		if e != nil {
			t.Fatal(e)
		}
		ss, _ := loaded.Snapshot()
		if ss.Features["cut"].SideOpenDualDisk.ExactWalls != exact {
			t.Fatal("document lost explicit nonbinary wall policy")
		}
		built, e := ss.EvaluateFeatures(ctx)
		if e != nil {
			t.Fatal(e)
		}
		walls := 0
		for _, face := range built.Trimmed["cut"].Faces() {
			if _, ok := face.Surface.(*geom.ExactBezierSurface); ok {
				walls++
			}
		}
		if exact && walls != 2 || !exact && walls != 0 {
			t.Fatal("reloaded document changed wall representation", exact, walls)
		}
		var out bytes.Buffer
		if e = ss.WriteSideOpenDualDiskSTEP(ctx, &out, "cut"); e != nil {
			t.Fatal(e)
		}
		if exact {
			if !bytes.Contains(out.Bytes(), []byte("PRODUCT('side-cut-exact-walls'")) {
				t.Fatal("exact wall occurrence did not use the registered STEP product")
			}
			p, e := step.ReadExactWallSideOpenDualDiskProduct(ctx, bytes.NewReader(out.Bytes()), step.Limits{}, 1e-9)
			if e != nil || p.RadicalSide != step.ExternalRadicalLeft || p.SourceWallFaces[0] == 0 || p.SourceWallFaces[1] == 0 || p.SourceFloorFace == 0 {
				t.Fatal("exact wall occurrence did not round trip with original support lineage", p.RadicalSide, p.SourceWallFaces, p.SourceFloorFace, e)
			}
			if _, e = step.ReadGeneralSideOpenDualDiskProduct(ctx, bytes.NewReader(out.Bytes()), step.Limits{}, 1e-9); e == nil {
				t.Fatal("ordinary float-wall reader admitted the exact-wall dialect")
			}
			tx := loaded.Begin()
			tx.State.Features["cut"].SideOpenDualDisk.ExactWalls = false
			if e = tx.Commit(); !errors.Is(e, geom.ErrUnrepresentable) {
				t.Fatal("disabling exact wall must reject nonbinary controls", e)
			}
			continue
		}
		p, e := step.ReadGeneralSideOpenDualDiskProduct(ctx, &out, step.Limits{}, 1e-9)
		if e != nil || p.RadicalSide != step.ExternalRadicalLeft {
			t.Fatal("off-center authored occurrence used wrong STEP domain", p.RadicalSide, e)
		}
	}
}
