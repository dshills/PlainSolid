package document

import (
	"bytes"
	"context"
	"errors"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"plainsolid/scene"
)

func TestSideOpenDualDiskPresentationAfterDocumentReload(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	frame, e := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	discs := [2]trimmed.ExtrusionCircle{{Center: geom.Vec2{X: 3.5, Y: 4}, Radius: 2.5}, {Center: geom.Vec2{X: 6.5, Y: 4}, Radius: 2.5}}
	cut, e := trimmed.CutSideOpenDualDisks(ctx, frame, geom.Vec2{Y: 4}, geom.Vec2{X: 10, Y: 10}, discs, 6, 3, 1e-9, trimvalidate.Options{})
	if e != nil {
		t.Fatal(e)
	}
	s := State{LengthUnit: .001, Trimmed: map[string]trimmed.Solid{"pocket": cut.Solid}, Nodes: map[string]Node{
		"near": {Asset: "pocket", Transform: geom.Identity()}, "far": {Asset: "pocket", Transform: geom.Translation(geom.Vec3{Z: 20})},
	}}
	d, e := New(s, 0)
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e = d.Write(&raw); e != nil {
		t.Fatal(e)
	}
	d, e = Read(&raw, 0, 0)
	if e != nil {
		t.Fatal(e)
	}
	s, _ = d.Snapshot()
	o := trimmed.SideOpenDualDiskMeshOptions{Deflection: .5, AngularTurn: .2, MaxTriangles: 400}
	view, e := s.SideOpenDualDiskPresentation(ctx, []string{"far", "near"}, o)
	if e != nil {
		t.Fatal("reloaded local display", e)
	}
	cam, e := scene.Orthographic(geom.Vec3{X: 5, Y: 5, Z: -3}, geom.Vec3{X: 5, Y: 5}, geom.Vec3{Y: 1}, 10, 10, 1, 40)
	if e != nil {
		t.Fatal(e)
	}
	h, ok, e := view.Pick(cam, geom.Vec2{})
	if e != nil || !ok || h.ID != "near" || !h.Point.IsFinite() || h.Point.Distance(geom.Vec3{X: 5, Y: 5, Z: 3}) > 1e-12 {
		t.Fatal("the retained blind floor should be visible through the opening", h, ok, e)
	}
	// This budget fits one local display only. Reusing the same asset above
	// succeeds; substituting an independent asset consumes the second share.
	s.Trimmed["independent"] = cut.Solid
	f := s.Nodes["far"]
	f.Asset = "independent"
	s.Nodes["far"] = f
	if view, e = s.SideOpenDualDiskPresentation(ctx, []string{"far", "near"}, o); e == nil || view != nil {
		t.Fatal("aggregate triangle budget permitted two independent displays", e)
	}
	if view, e = s.SideOpenDualDiskPresentation(ctx, []string{"absent"}, o); !errors.Is(e, geom.ErrInvalid) || view != nil {
		t.Fatal("missing occurrence did not reject", e)
	}
}
