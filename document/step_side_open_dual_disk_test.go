package document

import (
	"bytes"
	"context"
	"errors"
	"math"
	"plainsolid/internal/slowtest"
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

func TestDocumentSideOpenDualDiskOffCenterAndExactWalls(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for _, setting := range []struct{ exact, allow bool }{{false, false}, {false, true}, {true, true}} {
		exact, allow := setting.exact, setting.allow
		r0, r1, x1, stockY := 5., 8.5, 24.5, 12.
		if exact {
			r0, r1, x1, stockY = 13, 15, 24, 22
		}
		s := State{LengthUnit: .001, Features: map[string]Feature{
			"stock": {Kind: BoxFeature, Max: geom.Vec3{X: 50, Y: stockY, Z: 6}},
			"cut": {Kind: SideOpenDualDiskCutFeature, Input: "stock", SideOpenDualDisk: &SideOpenDualDiskRecipe{
				U: geom.Vec3{X: 1}, Inward: geom.Vec3{Z: 1}, Depth: 3, ExactWalls: allow,
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
		if ss.Features["cut"].SideOpenDualDisk.ExactWalls != allow {
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
