package document

import (
	"bytes"
	"context"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestDirectedCircularMultipleInsetsRetainNamedMembers(t *testing.T) {
	ctx := context.Background()
	p := func(x, y float64) geom.Vec2 { return geom.Vec2{X: x, Y: y} }
	disk := func(x, y, r float64) []NamedCircularProfileSegment {
		arc := circularDocumentRing("rim", p(x+r, y), p(x, y+r), p(x-r, y), p(x, y-r))
		for i := range arc {
			arc[i].Curve.Center, arc[i].Curve.Radius = p(x, y), r
		}
		return arc
	}
	tools := []string{"upper", "lower"}
	pose := geom.Affine{{0, 0, -1, 7}, {0, 1, 0, -2}, {1, 0, 0, 4}}
	d, err := New(State{LengthUnit: .001, Features: map[string]Feature{
		"host": {Kind: DirectedCircularExtrusionFeature, DirectedCircularExtrusion: &DirectedCircularExtrusionRecipe{
			Outer: circularDocumentRing("stock", p(0, 0), p(8, 0), p(8, 5), p(0, 5)), Bottom: -2}},
		"upper": {Kind: DirectedCircularExtrusionFeature, DirectedCircularExtrusion: &DirectedCircularExtrusionRecipe{
			Outer: disk(2, 2, 1), Holes: [][]NamedCircularProfileSegment{circularDocumentHole("opening", 2, 2, .25)}, Top: 2}},
		"lower": {Kind: DirectedCircularExtrusionFeature, DirectedCircularExtrusion: &DirectedCircularExtrusionRecipe{
			Outer: disk(5, 2, 1), Bottom: -3, Top: -2}},
		"join":  {Kind: DirectedCircularMultipleInsetJoinFeature, Input: "host", Tools: tools},
		"posed": {Kind: TransformFeature, Input: "join", Transform: pose},
	}, Nodes: map[string]Node{
		"step-part": {Name: "Named inset group", Asset: "posed", Transform: geom.Identity()},
		"iges-part": {Asset: "join", Transform: geom.Identity()},
	}}, 2)
	if err != nil {
		t.Fatal(err)
	}
	tools[0] = "detached"
	check := func(d *Document, upperOffset, lowerOffset int) {
		t.Helper()
		s, _ := d.Snapshot()
		proof, err := s.EvaluateCircularMultipleInsets(ctx, "posed")
		if err != nil {
			t.Fatal(err)
		}
		h, u, l := proof.Host, proof.Guests["upper"], proof.Guests["lower"]
		if h.Feature != "host" || u.Feature != "upper" || l.Feature != "lower" ||
			len(h.Caps[1]) != 2 || len(h.Caps[0]) != 1 || len(u.Caps[0]) != 0 || len(l.Caps[1]) != 0 ||
			u.Edges["rim-0"][0] != upperOffset || l.Edges["rim-0"][1] != lowerOffset+4 {
			t.Fatalf("original members or their consumed cap roles changed: %+v", proof.Guests)
		}
		faces, edges, vertices := proof.Solid.Faces(), proof.Solid.Edges(), proof.Solid.Vertices()
		floor := faces[h.Caps[1][1]]
		rim := u.Edges["opening-0"][0]
		found := false
		for _, use := range floor.Outer {
			found = found || use.Edge == rim
		}
		if !found || faces[h.Caps[1][0]].Surface != floor.Surface ||
			vertices[edges[rim].Start] != pose.Apply(geom.Vec3{X: 2.25, Y: 2}) {
			t.Fatal("upper exposed floor lost the original named opening or placement")
		}
	}
	check(d, 12, 36)
	tx := d.Begin()
	f := tx.State.Features["join"]
	f.Tools[0], f.Tools[1] = f.Tools[1], f.Tools[0]
	tx.State.Features["join"] = f
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	tx = d.Begin()
	f = tx.State.Features["join"]
	f.Tools[0] = f.Tools[1]
	tx.State.Features["join"] = f
	if err := tx.Commit(); err == nil {
		t.Fatal("committed a repeated original guest")
	}
	var data bytes.Buffer
	if err := d.Write(&data); err != nil {
		t.Fatal(err)
	}
	reopened, err := Read(&data, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	check(reopened, 24, 12)
	s, _ := reopened.Snapshot()
	var profiles [3]trimmed.ExactCircularSweep
	for i, id := range [3]string{"host", "lower", "upper"} {
		source := s.Features[id].DirectedCircularExtrusion
		loops, _, err := source.profile()
		if err != nil {
			t.Fatal(err)
		}
		profiles[i] = trimmed.ExactCircularSweep{Outer: loops[0], Holes: loops[1:], Bottom: source.Bottom, Top: source.Top}
	}
	var stepRaw, igesRaw bytes.Buffer
	if err := s.WriteCircularMultipleInsetsSTEP(ctx, &stepRaw, "step-part"); err != nil {
		t.Fatal(err)
	}
	frame, err := geom.NewFrame(geom.Vec3{X: 7, Y: -2, Z: 4}, geom.Vec3{Z: 1}, geom.Vec3{X: -1})
	if err != nil {
		t.Fatal(err)
	}
	stepProof, err := step.ReadExactCircularSweepInsets(ctx, &stepRaw, step.ExactCircularSweepInsets{
		Name: "Named inset group", Frame: frame, Host: profiles[0], Guests: profiles[1:], LengthUnit: .001,
	}, step.Limits{})
	if err != nil || stepProof.GuestFaces[0][1] != 0 || stepProof.GuestFaces[1][0] != 0 {
		t.Fatalf("placed STEP did not retain the reordered guests: %v", err)
	}
	if err := s.WriteCircularMultipleInsetsIGES(ctx, &igesRaw, "iges-part"); err != nil {
		t.Fatal(err)
	}
	igesProof, err := iges.ReadExactCircularSweepInsets(ctx, &igesRaw, iges.ExactCircularSweepInsets{
		Host: profiles[0], Guests: profiles[1:], Unit: .001,
	}, iges.Limits{})
	if err != nil || igesProof.GuestFaces[0][1] != 0 || igesProof.GuestFaces[1][0] != 0 || len(igesProof.HostFaces[1]) != 2 {
		t.Fatalf("IGES did not retain the mapped floor of the reordered guest: %v", err)
	}
}
