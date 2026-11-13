package document

import (
	"bytes"
	"context"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"plainsolid/brep"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestSTEPAssemblyDocumentRoundTrip(t *testing.T) {
	b, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 2, Z: 3})
	if e != nil {
		t.Fatal(e)
	}
	s := State{
		LengthUnit: .001,
		Planar:     map[string]brep.Solid{"Bracket": b},
		Nodes: map[string]Node{
			"frame": {Name: "Frame", Transform: geom.Translation(geom.Vec3{X: 5})},
			"left":  {Parent: "frame", Name: "Left", Asset: "Bracket", Transform: geom.Translation(geom.Vec3{Y: 10})},
			"right": {Parent: "frame", Name: "Right", Asset: "Bracket", Transform: geom.Translation(geom.Vec3{Y: -10})},
		},
	}
	var output bytes.Buffer
	if e = s.WriteSTEPAssembly(&output, "Fixture"); e != nil {
		t.Fatal(e)
	}
	a, e := step.ReadAssembly(&output, step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(a.Products) != 3 || len(a.Occurrences) != 3 || len(a.Roots) != 1 {
		t.Fatal(a)
	}
	d, e := FromSTEPAssembly(a, STEPAssemblyOptions{}, 1)
	if e != nil {
		t.Fatal(e)
	}
	r, _ := d.Snapshot()
	if len(r.Nodes) != 4 || len(r.Planar) != 1 {
		t.Fatal(r)
	}
	m, e := r.Flatten()
	if e != nil {
		t.Fatal(e)
	}
	bb, e := m.Bounds()
	if e != nil || bb.Min != (geom.Vec3{X: 5, Y: -10}) || bb.Max != (geom.Vec3{X: 6, Y: 12, Z: 3}) || math.Abs(m.SignedVolume()-12) > 1e-9 {
		t.Fatal(bb, m.SignedVolume(), e)
	}
	n := s.Nodes["left"]
	n.Transform = geom.Scaling(geom.Vec3{X: 2, Y: 2, Z: 2})
	s.Nodes["left"] = n
	output.Reset()
	if e = s.WriteSTEPAssembly(&output, "Fixture"); e == nil || output.Len() != 0 {
		t.Fatal("exported unsupported scaled instance", e)
	}
}

func TestColoredSTEPDocumentRoundTrip(t *testing.T) {
	b, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 2, Y: 3, Z: 4})
	if e != nil {
		t.Fatal(e)
	}
	color := step.RGB{R: .2, G: .7, B: .4}
	d, e := FromColoredSTEP(step.ColoredModel{LengthUnit: .001, Bodies: []step.ColoredBody{
		{Body: step.Body{Name: "green", Solid: b}, SurfaceColor: &color},
	}}, 1)
	if e != nil {
		t.Fatal(e)
	}
	s, _ := d.Snapshot()
	var data bytes.Buffer
	if e = s.WriteColoredPlanarSTEP(&data); e != nil {
		t.Fatal(e)
	}
	r, e := step.ReadColoredPlanar(&data, step.Limits{})
	if e != nil || len(r.Bodies) != 1 || r.Bodies[0].SurfaceColor == nil || *r.Bodies[0].SurfaceColor != color {
		t.Fatal(r, e)
	}
	l := s.Labels["step-style:body-000001"]
	delete(l.Attributes, "step.srgb.red")
	if e = s.WriteColoredPlanarSTEP(&data); e == nil {
		t.Fatal("accepted incomplete color")
	}
}

func TestFaceColoredSTEPDocumentRoundTrip(t *testing.T) {
	b, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 2, Y: 3, Z: 4})
	if e != nil {
		t.Fatal(e)
	}
	color := step.RGB{R: .1, G: .4, B: .8}
	d, e := FromFaceColoredSTEP(step.FaceColoredModel{LengthUnit: .001, Bodies: []step.FaceColoredBody{
		{Body: step.Body{Name: "marked", Solid: b}, FaceColors: map[int]step.RGB{0: color, 3: {R: 1}}},
	}}, 1)
	if e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = d.Write(&native); e != nil {
		t.Fatal(e)
	}
	d, e = Read(&native, 0, 1)
	if e != nil {
		t.Fatal(e)
	}
	s, _ := d.Snapshot()
	var stream bytes.Buffer
	if e = s.WriteColoredPlanarSTEP(&stream); e == nil {
		t.Fatal("silently omitted face styles")
	}
	if e = s.WriteFaceColoredPlanarSTEP(&stream); e != nil {
		t.Fatal(e)
	}
	m, e := step.ReadFaceColoredPlanar(&stream, step.Limits{})
	if e != nil || len(m.Bodies) != 1 || m.Bodies[0].FaceColors[0] != color || m.Bodies[0].FaceColors[3] != (step.RGB{R: 1}) {
		t.Fatal(m, e)
	}
}

func TestCavitySTEPDocumentRoundTrip(t *testing.T) {
	outer, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 10, Y: 10, Z: 10})
	if e != nil {
		t.Fatal(e)
	}
	void, e := brep.Box(geom.Vec3{X: 2, Y: 2, Z: 2}, geom.Vec3{X: 5, Y: 5, Z: 5})
	if e != nil {
		t.Fatal(e)
	}
	var data bytes.Buffer
	if e = step.WriteCavities(&data, step.CavityModel{LengthUnit: .0254, Bodies: []step.CavityBody{{Name: "cavity", Outer: outer,
		Voids: []step.OrientedVoidShell{{Shell: void, Reversed: true}}}}}); e != nil {
		t.Fatal(e)
	}
	m, e := step.ReadCavities(&data, step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	d, e := FromCavitySTEP(m, 1)
	if e != nil {
		t.Fatal(e)
	}
	s, _ := d.Snapshot()
	if e = s.WriteCavitySTEP(&data); e != nil {
		t.Fatal(e)
	}
	m, e = step.ReadCavities(&data, step.Limits{})
	if e != nil || len(m.Bodies) != 1 || len(m.Bodies[0].Voids) != 1 || m.LengthUnit != .0254 {
		t.Fatal(m, e)
	}
	if math.Abs(m.Bodies[0].Solid.Tessellate().SignedVolume()-973) > 1e-9 {
		t.Fatal(m)
	}
}

func TestOCCTSTEPAssemblyDocument(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_ORACLE")
	if oracle == "" || os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
		t.Skip("requires optional OCCT STEP oracle")
	}
	path := filepath.Join(t.TempDir(), "assembly.step")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if out, e := exec.CommandContext(ctx, oracle, path, "write-step-assembly").CombinedOutput(); e != nil {
		t.Fatalf("%v: %s", e, out)
	}
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	a, e := step.ReadAssembly(bytes.NewReader(b), step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = FromSTEPAssembly(a, STEPAssemblyOptions{MaxNodes: 2}, 1); e == nil {
		t.Fatal("expanded beyond node limit")
	}
	if _, e = FromSTEPAssembly(a, STEPAssemblyOptions{MaxDepth: 1}, 1); e == nil {
		t.Fatal("expanded beyond depth limit")
	}
	d, e := FromSTEPAssembly(a, STEPAssemblyOptions{}, 1)
	if e != nil {
		t.Fatal(e)
	}
	s, _ := d.Snapshot()
	if len(s.Nodes) != 3 || len(s.Planar) != 1 || s.LengthUnit != .001 {
		t.Fatal(s)
	}
	names := map[string]geom.Affine{}
	for _, n := range s.Nodes {
		names[n.Name] = n.Transform
	}
	if _, ok := names["Machine"]; !ok {
		t.Fatal(names)
	}
	if names["Left bracket"].Apply(geom.Vec3{}).Distance(geom.Vec3{X: 10}) > 1e-10 ||
		names["Right bracket"].Apply(geom.Vec3{X: 1}).Distance(geom.Vec3{Y: 21}) > 1e-10 {
		t.Fatal(names)
	}
	m, e := s.Flatten()
	if e != nil || math.Abs(m.SignedVolume()-12) > 1e-10 {
		t.Fatal(m.SignedVolume(), e)
	}
	var out bytes.Buffer
	if e = d.Write(&out); e != nil {
		t.Fatal(e)
	}
	d, e = Read(&out, 0, 1)
	if e != nil {
		t.Fatal(e)
	}
	r, _ := d.Snapshot()
	if len(r.Planar) != 1 || len(r.Nodes) != 3 {
		t.Fatal(r)
	}
}
