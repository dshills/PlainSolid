package document

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"testing"
	"time"
)

func filletMaterialFixture(t *testing.T) step.WrappedMaterialModel {
	t.Helper()
	raw, e := os.ReadFile("../exchange/step/testdata/wrapped-rim-fillet.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := step.ReadWrappedMaterials(bytes.NewReader(raw), step.Limits{}, step.WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Bodies) != 1 || m.Bodies[0].RimFillet == nil {
		t.Fatal("fillet tag")
	}
	m.Bodies[0].RimFillet.Name = "Rim fillet"
	m.Bodies[0].RimFillet.CapNames = [2]string{"Bottom", "Top"}
	for i := range m.Bodies[0].RimFillet.Walls {
		m.Bodies[0].RimFillet.Walls[i].Name = []string{"Lower blend", "Cylinder", "Upper blend"}[i]
	}
	return m
}
func TestWrappedRimFilletDocument(t *testing.T) {
	d, e := FromWrappedMaterialsSTEP(filletMaterialFixture(t), 1)
	if e != nil {
		t.Fatal(e)
	}
	s, _ := d.Snapshot()
	s.Nodes = map[string]Node{"a": {Name: "First", Asset: "body-000001", Transform: geom.Identity()}, "group": {Transform: geom.Translation(geom.Vec3{Y: 16})}, "b": {Name: "Second", Parent: "group", Asset: "body-000001", Transform: geom.Translation(geom.Vec3{Z: 8})}}
	d, e = New(s, 1)
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
	s, _ = d.Snapshot()
	if len(s.Trimmed) != 1 || s.Nodes["a"].Asset != s.Nodes["b"].Asset {
		t.Fatal("shared asset lost")
	}
	var b bytes.Buffer
	if e = s.WriteWrappedMaterialsSTEP(&b); e != nil {
		t.Fatal(e)
	}
	m, e := step.ReadWrappedMaterials(&b, step.Limits{}, step.WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Bodies) != 2 || m.Bodies[1].RimFillet == nil {
		t.Fatal(m)
	}
	q := m.Bodies[1].RimFillet
	if q.Name != "Second" || q.Center != (geom.Vec3{Y: 16, Z: 8}) || q.CapNames != ([2]string{"Bottom", "Top"}) || q.Walls[0].Name != "Lower blend" {
		t.Fatal(q)
	}
	n := s.Nodes["b"]
	n.Transform = geom.Translation(geom.Vec3{X: math.Ldexp(1, -80)})
	s.Nodes["b"] = n
	b.Reset()
	if e = s.WriteWrappedMaterialsSTEP(&b); e == nil || b.Len() != 0 {
		t.Fatal("lossy translation accepted")
	}
}
func TestWrappedRimFilletDocumentAssembly(t *testing.T) {
	model := filletMaterialFixture(t)
	a := step.WrappedMaterialAssemblyModel{LengthUnit: .001, Products: []step.WrappedMaterialAssemblyPart{{ID: "root", Name: "Fillet assembly"}, {ID: "part", Name: "Shared fillet", Bodies: model.Bodies}}, Occurrences: []step.AssemblyInstance{{ID: "one", Name: "First", Parent: "root", Child: "part", Transform: geom.Identity()}, {ID: "two", Name: "Rotated", Parent: "root", Child: "part", Transform: geom.Affine{{0, -1, 0, 16}, {1, 0, 0, 0}, {0, 0, 1, 0}}}}}
	var b bytes.Buffer
	if e := step.WriteWrappedMaterialAssembly(&b, a); e != nil {
		t.Fatal(e)
	}
	d, e := ReadWrappedMaterialAssemblySTEP(bytes.NewReader(b.Bytes()), step.Limits{}, step.WrappedSurfaceOptions{}, STEPAssemblyOptions{}, 1)
	if e != nil {
		t.Fatal(e)
	}
	b.Reset()
	if e = d.Write(&b); e != nil {
		t.Fatal(e)
	}
	d, e = Read(&b, 0, 1)
	if e != nil {
		t.Fatal(e)
	}
	s, _ := d.Snapshot()
	if len(s.Trimmed) != 1 {
		t.Fatal("shared product lost")
	}
	b.Reset()
	if e = s.WriteWrappedMaterialAssemblySTEP(&b); e != nil {
		t.Fatal(e)
	}
	got, e := step.ReadWrappedMaterialAssembly(bytes.NewReader(b.Bytes()), step.Limits{}, step.WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Assembly.Occurrences) != 2 || got.Assembly.Occurrences[0].Child != got.Assembly.Occurrences[1].Child {
		t.Fatal("occurrences")
	}
	bs := got.ProductBodies[got.Assembly.Occurrences[0].Child]
	if len(bs) != 1 || bs[0].RimFillet == nil {
		t.Fatal("family")
	}
	oracle := os.Getenv("PLAINSOLID_OCCT_ORACLE")
	if oracle == "" || os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
		return
	}
	path := t.TempDir() + "/assembly.step"
	if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	out := t.TempDir() + "/mass.json"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	log, e := exec.CommandContext(ctx, oracle, out, "step-assembly", path).CombinedOutput()
	cancel()
	if e != nil {
		t.Fatalf("%v %s", e, log)
	}
	raw, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var p struct {
		Volume      float64
		Occurrences []struct {
			Name      string
			Transform [3][4]float64
		}
	}
	if e = json.Unmarshal(raw, &p); e != nil {
		t.Fatal(e)
	}
	volume := 9 * 3.5 * math.Pi
	for _, r := range []float64{.5, 1} {
		major := 3 - r
		volume += math.Pi * (major*major*r + major*r*r*math.Pi/2 + 2*r*r*r/3)
	}
	if len(p.Occurrences) != 2 || math.Abs(p.Volume-2*volume) > 1e-7 {
		t.Fatal(p)
	}
}
