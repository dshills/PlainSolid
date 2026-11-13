package document

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"strings"
	"testing"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/csg/poly"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"plainsolid/scene"
)

func TestExactAssets(t *testing.T) {
	p, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 2, Z: 3})
	if e != nil {
		t.Fatal(e)
	}
	s, e := parametric.Sphere(geom.Vec3{}, 1)
	if e != nil {
		t.Fatal(e)
	}
	state := State{
		LengthUnit: .001,
		Planar:     map[string]brep.Solid{"box": p},
		Parametric: map[string]parametric.Solid{"ball": s},
		Nodes: map[string]Node{
			"a": {Asset: "ball", Transform: geom.Translation(geom.Vec3{X: 3})},
			"b": {Asset: "ball", Transform: geom.Translation(geom.Vec3{X: -3})},
			"c": {Asset: "box", Transform: geom.Translation(geom.Vec3{Y: 5})},
		},
	}
	d, e := New(state, 2)
	if e != nil {
		t.Fatal(e)
	}
	tx := d.Begin()
	delete(tx.State.Parametric, "ball")
	if e = tx.Commit(); e == nil {
		t.Fatal("accepted missing exact asset")
	}
	var b bytes.Buffer
	if e = d.Write(&b); e != nil {
		t.Fatal(e)
	}
	d, e = Read(&b, 0, 2)
	if e != nil {
		t.Fatal(e)
	}
	snap, _ := d.Snapshot()
	if len(snap.Planar) != 1 || len(snap.Parametric) != 1 || len(snap.Nodes) != 3 || snap.LengthUnit != .001 {
		t.Fatalf("assets lost in persistence: %+v", snap)
	}
	if _, e = snap.Flatten(); e == nil {
		t.Fatal("flatten should require explicit deflection")
	}
	options := parametric.TessellationOptions{Deflection: .005}
	m, e := snap.FlattenGeometry(context.Background(), 100000, 100000, options)
	if e != nil || m.ValidateSolid() != nil {
		t.Fatal(e)
	}
	if math.Abs(m.SignedVolume()-(6+8*math.Pi/3)) > .07 {
		t.Fatalf("wrong assembly volume %g", m.SignedVolume())
	}
	if _, e = snap.FlattenGeometry(context.Background(), len(m.Vertices), len(m.Triangles)-1, options); e == nil {
		t.Fatal("instance expansion exceeded budget")
	}
	tx = d.Begin()
	tx.State.Nodes["d"] = Node{Asset: "box", Transform: geom.Identity()}
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	if e = d.Undo(); e != nil {
		t.Fatal(e)
	}
	snap, _ = d.Snapshot()
	if len(snap.Nodes) != 3 || len(snap.Planar) != 1 || len(snap.Parametric) != 1 {
		t.Fatal("undo lost exact assets")
	}
	snap.Planar["ball"] = p
	if e = snap.Validate(); e == nil {
		t.Fatal("duplicate asset id accepted")
	}
}

func TestFeatureRegenerationAndUndo(t *testing.T) {
	box := Feature{Kind: BoxFeature, Max: geom.Vec3{X: 2, Y: 2, Z: 2}}
	offset := Feature{Kind: PlanarOffsetFeature, Input: "core", Distance: .5, Tolerance: 1e-10}
	final := Feature{Kind: TransformFeature, Input: "offset", Transform: geom.Translation(geom.Vec3{X: 2})}
	s := State{
		LengthUnit: 1,
		Features:   map[string]Feature{"core": box, "offset": offset, "final": final},
		Nodes:      map[string]Node{"instance": {Asset: "final", Transform: geom.Identity()}},
	}
	doc, e := New(s, 2)
	if e != nil {
		t.Fatal(e)
	}
	check := func(s State, v float64) {
		t.Helper()
		x, e := s.EvaluateFeatures(context.Background())
		if e != nil {
			t.Fatal(e)
		}
		p, e := x.Planar["final"].MassProperties()
		if e != nil || math.Abs(p.Volume-v) > 1e-9 {
			t.Fatalf("regenerated volume %g, expected %g: %v", p.Volume, v, e)
		}
	}
	check(s, 27)
	tx := doc.Begin()
	box.Max.X = 3
	tx.State.Features["core"] = box
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	s, _ = doc.Snapshot()
	check(s, 36)
	var b bytes.Buffer
	if e = doc.Write(&b); e != nil {
		t.Fatal(e)
	}
	copy, e := Read(&b, 0, 1)
	if e != nil {
		t.Fatal(e)
	}
	s, _ = copy.Snapshot()
	if len(s.Features) != 3 {
		t.Fatal("feature recipes lost")
	}
	check(s, 36)
	if e = doc.Undo(); e != nil {
		t.Fatal(e)
	}
	s, _ = doc.Snapshot()
	check(s, 27)
	tx = doc.Begin()
	tx.State.Features["core"] = Feature{Kind: TransformFeature, Input: "final", Transform: geom.Identity()}
	if e = tx.Commit(); e == nil {
		t.Fatal("accepted cyclic features")
	}
	s, _ = doc.Snapshot()
	check(s, 27)
}

func TestTrimmedAssetSnapshots(t *testing.T) {
	b, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 2, Z: 3})
	if e != nil {
		t.Fatal(e)
	}
	s, e := trimmed.FromPlanar(b, 1e-10)
	if e != nil {
		t.Fatal(e)
	}
	d, e := New(State{
		LengthUnit: 1,
		Trimmed:    map[string]trimmed.Solid{"base": s},
		Features:   map[string]Feature{"scaled": {Kind: TransformFeature, Input: "base", Transform: geom.Scaling(geom.Vec3{X: 2, Y: 1, Z: -1})}},
		Nodes:      map[string]Node{"instance": {Asset: "scaled", Transform: geom.Identity()}},
	}, 1)
	if e != nil {
		t.Fatal(e)
	}
	var buf bytes.Buffer
	if e = d.Write(&buf); e != nil {
		t.Fatal(e)
	}
	d, e = Read(&buf, 0, 1)
	if e != nil {
		t.Fatal(e)
	}
	state, _ := d.Snapshot()
	m, e := state.FlattenGeometry(context.Background(), 1000, 1000, parametric.TessellationOptions{Deflection: .01})
	if e != nil || math.Abs(m.SignedVolume()-12) > 1e-10 {
		t.Fatalf("mesh volume %g %v", m.SignedVolume(), e)
	}
}

func TestCylinderBoreFeatureRegeneration(t *testing.T) {
	ctx := context.Background()
	box := Feature{Kind: BoxFeature, Max: geom.Vec3{X: 4, Y: 5, Z: 6}}
	bore := Feature{
		Kind: CylinderBoreFeature, Input: "box", EntryRole: "z-min", ExitRole: "x-max",
		Center: geom.Vec3{X: 1, Y: 2}, Direction: geom.Vec3{X: 2, Z: 1},
		Radius: .35, Tolerance: 1e-10,
	}
	d, e := New(State{
		LengthUnit: 1,
		Features:   map[string]Feature{"box": box, "bore": bore},
		Nodes:      map[string]Node{"body": {Asset: "bore", Transform: geom.Identity()}},
	}, 2)
	if e != nil {
		t.Fatal(e)
	}
	check := func(doc *Document, width float64) {
		t.Helper()
		s, _ := doc.Snapshot()
		before, e := s.ResolveBoxFace(FeatureFace{Feature: "box", Role: "x-max"})
		if e != nil {
			t.Fatal(e)
		}
		after, e := s.ResolveBoxFace(FeatureFace{Feature: "bore", Role: "x-max"})
		if e != nil || after != before {
			t.Fatalf("lost semantic face through bore: %d, %d (%v)", before, after, e)
		}
		m, e := s.EvaluateFeatures(ctx)
		if e != nil {
			t.Fatal(e)
		}
		p, e := m.Trimmed["bore"].MassProperties(ctx, parametric.IntegrationOptions{})
		want := width*30 - math.Pi*bore.Radius*bore.Radius*(width-1)*math.Sqrt(5)/2
		if e != nil || math.Abs(p.Volume-want) > 1e-7 {
			t.Fatalf("bore volume=%g, want %g (%v)", p.Volume, want, e)
		}
	}
	check(d, 4)
	tx := d.Begin()
	box.Max.X = 5
	tx.State.Features["box"] = box
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	var stream bytes.Buffer
	if e = d.Write(&stream); e != nil {
		t.Fatal(e)
	}
	roundTrip, e := Read(&stream, 0, 1)
	if e != nil {
		t.Fatal(e)
	}
	check(roundTrip, 5)
	if e = d.Undo(); e != nil {
		t.Fatal(e)
	}
	check(d, 4)
	tx = d.Begin()
	bore.Radius = 2
	tx.State.Features["bore"] = bore
	if e = tx.Commit(); e == nil {
		t.Fatal("accepted bore crossing another face")
	}
}

func TestFilletFeatureAndTrimmedSTEPDocument(t *testing.T) {
	s := State{LengthUnit: .001, Features: map[string]Feature{
		"rounded": {Kind: PrismFilletFeature, Profile: []geom.Vec3{{}, {X: 6}, {X: 6, Y: 4}, {Y: 4}}, Offset: geom.Vec3{Z: 3}, Radius: .6},
	}, Nodes: map[string]Node{"instance": {Asset: "rounded", Transform: geom.Identity()}}}
	r, e := s.EvaluateFeatures(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	doc, e := FromTrimmedSTEP(step.TrimmedModel{LengthUnit: r.LengthUnit, Bodies: []step.TrimmedBody{{Name: "Rounded prism", Solid: r.Trimmed["rounded"]}}}, 1)
	if e != nil {
		t.Fatal(e)
	}
	state, _ := doc.Snapshot()
	if state.Nodes["body-000001"].Name != "Rounded prism" {
		t.Fatal(state)
	}
	props, e := state.Trimmed["body-000001"].MassProperties(context.Background(), parametric.IntegrationOptions{})
	want := (24 - (4-math.Pi)*.36) * 3
	if e != nil || math.Abs(props.Volume-want) > 1e-8 {
		t.Fatal(props, e)
	}
}

func TestDrawingKeepsOccurrenceAndCurvedEdge(t *testing.T) {
	ctx := context.Background()
	b, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 4, Y: 4, Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	r, h, e := trimmed.BoreCylinder(b, 0, 1, geom.Vec3{X: 2, Y: 2}, .5, 1e-10)
	if e != nil {
		t.Fatal(e)
	}
	s := State{LengthUnit: 1, Trimmed: map[string]trimmed.Solid{"part": r}, Nodes: map[string]Node{
		"one": {Asset: "part", Transform: geom.Translation(geom.Vec3{X: -5, Y: -2})},
		"two": {Asset: "part", Transform: geom.Translation(geom.Vec3{X: 1, Y: -2})},
	}}
	c, e := scene.Perspective(geom.Vec3{Z: 12}, geom.Vec3{}, geom.Vec3{Y: 1}, math.Pi/2, 1, 1, 20)
	if e != nil {
		t.Fatal(e)
	}
	o := DrawingOptions{Tessellation: parametric.TessellationOptions{Deflection: .05}, Curves: geom.CurveTessellationOptions{ChordTolerance: .025},
		Occlusion: scene.HiddenLineOptions{DepthTolerance: .01, MaxTests: 2000000}}
	lines, e := s.DrawVisibleEdges(ctx, c, o)
	if e != nil {
		t.Fatal(e)
	}
	m := map[string]int{}
	for _, x := range lines {
		if x.ID == fmt.Sprintf("3:one:%d", h.ExitEdges[0]) || x.ID == fmt.Sprintf("3:two:%d", h.ExitEdges[0]) {
			m[strings.Split(x.ID, ":")[1]]++
		}
	}
	if m["one"] < 2 || m["two"] < 2 {
		t.Fatal(m, len(lines))
	}
	o.MaxSegments = 1
	if _, e = s.DrawVisibleEdges(ctx, c, o); e == nil {
		t.Fatal("drawing exceeded edge budget")
	}
}

func TestPlanarSTEPDocumentPlacement(t *testing.T) {
	p, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 2, Z: 3})
	if e != nil {
		t.Fatal(e)
	}
	doc, e := FromSTEP(step.Model{LengthUnit: .001, Bodies: []step.Body{{Name: "Test box", Solid: p}}}, 1)
	if e != nil {
		t.Fatal(e)
	}
	tx := doc.Begin()
	node := tx.State.Nodes["body-000001"]
	node.Transform = geom.Translation(geom.Vec3{X: 10, Y: -2})
	tx.State.Nodes["body-000001"] = node
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	s, _ := doc.Snapshot()
	var b bytes.Buffer
	if e = s.WritePlanarSTEP(&b, true); e != nil {
		t.Fatal(e)
	}
	m, e := step.Read(&b, step.Limits{})
	if e != nil || len(m.Bodies) != 1 || m.LengthUnit != .001 || m.Bodies[0].Name != "Test box" {
		t.Fatal(m, e)
	}
	properties, e := m.Bodies[0].Solid.MassProperties()
	if e != nil || properties.Volume != 6 || properties.Centroid != (geom.Vec3{X: 10.5, Y: -1, Z: 1.5}) {
		t.Fatal(properties, e)
	}
}

func TestDocumentPresentation(t *testing.T) {
	b, e := brep.Box(geom.Vec3{X: -1, Y: -1, Z: -1}, geom.Vec3{X: 1, Y: 1, Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	s := State{LengthUnit: 1, Planar: map[string]brep.Solid{"cube": b}, Nodes: map[string]Node{
		"front": {Asset: "cube", Transform: geom.Translation(geom.Vec3{Z: 2})},
		"rear":  {Asset: "cube", Transform: geom.Identity()},
	}}
	p, e := s.Presentation(context.Background(), parametric.TessellationOptions{}, 12)
	if e != nil {
		t.Fatal(e)
	}
	c, e := scene.Orthographic(geom.Vec3{Z: 10}, geom.Vec3{}, geom.Vec3{Y: 1}, 4, 4, 1, 20)
	if e != nil {
		t.Fatal(e)
	}
	h, ok, e := p.Pick(c, geom.Vec2{X: .1, Y: .2})
	if e != nil || !ok || h.ID != "front" || h.Point.Z != 3 {
		t.Fatal(h, ok, e)
	}
	if _, e = s.Presentation(context.Background(), parametric.TessellationOptions{}, 11); e == nil {
		t.Fatal("ignored triangle budget")
	}
}

func TestStableBoxFeatureFace(t *testing.T) {
	ref := FeatureFace{Feature: "placed", Role: "x-max"}
	s := State{LengthUnit: 1, Features: map[string]Feature{
		"base":   {Kind: BoxFeature, Max: geom.Vec3{X: 2, Y: 3, Z: 4}},
		"placed": {Kind: TransformFeature, Input: "base", Transform: geom.Scaling(geom.Vec3{X: -1, Y: 1, Z: 1})},
	}}
	id, e := s.ResolveBoxFace(ref)
	if e != nil {
		t.Fatal(e)
	}
	for _, x := range []float64{2, 3} {
		f := s.Features["base"]
		f.Max.X = x
		s.Features["base"] = f
		next, e := s.ResolveBoxFace(ref)
		if e != nil || next != id {
			t.Fatal(next, id, e)
		}
		state, e := s.EvaluateFeatures(context.Background())
		if e != nil {
			t.Fatal(e)
		}
		p, e := state.Planar["placed"].FacePlane(id)
		if e != nil || p.Normal().X != -1 || p.Origin().X != -x {
			t.Fatal(p, e)
		}
	}
}

func TestBooleanFeatureGraph(t *testing.T) {
	s := State{LengthUnit: 1, Features: map[string]Feature{
		"stock": {Kind: BoxFeature, Max: geom.Vec3{X: 3, Y: 4, Z: 5}},
		"tool":  {Kind: BoxFeature, Min: geom.Vec3{X: 1, Y: 1, Z: -1}, Max: geom.Vec3{X: 2, Y: 2, Z: 6}},
		"hole":  {Kind: BooleanFeature, Input: "stock", Tool: "tool", Operation: poly.Difference},
	}, Nodes: map[string]Node{"part": {Asset: "hole", Transform: geom.Identity()}}}
	d, e := New(s, 2)
	if e != nil {
		t.Fatal(e)
	}
	txn := d.Begin()
	p := txn.State.Features["tool"]
	p.Max.Y = 3
	txn.State.Features["tool"] = p
	if e = txn.Commit(); e != nil {
		t.Fatal(e)
	}
	s, _ = d.Snapshot()
	m, e := s.EvaluateFeatures(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	props, e := m.Planar["hole"].MassProperties()
	if e != nil || math.Abs(props.Volume-50) > 1e-9 {
		t.Fatal(props, e)
	}
	txn = d.Begin()
	txn.State.Features["tool"] = Feature{Kind: TransformFeature, Input: "hole", Transform: geom.Identity()}
	if e = txn.Commit(); e == nil {
		t.Fatal("accepted tool dependency cycle")
	}
}
