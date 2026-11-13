package document

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"testing"
	"time"
)

func TestColoredWrappedDocument(t *testing.T) {
	b := filletMaterialFixture(t).Bodies[0]
	model := step.ColoredWrappedMaterialAssemblyModel{LengthUnit: .001, Products: []step.ColoredWrappedMaterialAssemblyPart{{ID: "root", Name: "assembly"}, {ID: "part", Name: "colored fillet", Bodies: []step.ColoredWrappedMaterialBody{{Body: b, SurfaceColor: &step.RGB{R: .2, G: .4, B: .6}, FaceColors: map[string]step.RGB{"Bottom": {R: .8, G: .1, B: .3}, "Lower blend": {R: .1, G: .8, B: .3}}}}}}, Occurrences: []step.AssemblyInstance{{ID: "one", Name: "first", Parent: "root", Child: "part", Transform: geom.Identity()}, {ID: "two", Name: "second", Parent: "root", Child: "part", Transform: geom.Translation(geom.Vec3{X: 16})}}}
	var raw bytes.Buffer
	if e := step.WriteColoredWrappedMaterialAssembly(&raw, model); e != nil {
		t.Fatal(e)
	}
	d, e := ReadColoredWrappedMaterialAssemblySTEP(&raw, step.Limits{}, step.WrappedSurfaceOptions{}, STEPAssemblyOptions{}, 1)
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
	if len(s.Trimmed) != 1 {
		t.Fatal("shared asset lost")
	}
	var out bytes.Buffer
	if e = s.WriteColoredWrappedMaterialAssemblySTEP(&out); e != nil {
		t.Fatal(e)
	}
	coloredWrappedDocumentOracle(t, out.Bytes())
	got, e := step.ReadColoredWrappedMaterialAssembly(&out, step.Limits{}, step.WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Assembly.Occurrences) != 2 {
		t.Fatal("occurrences")
	}
	count := 0
	for _, bs := range got.ProductBodies {
		for _, body := range bs {
			count++
			if body.SurfaceColor == nil || len(body.FaceColors) != 2 || body.FaceColors["Bottom"] != (step.RGB{R: .8, G: .1, B: .3}) {
				t.Fatal(body)
			}
		}
	}
	if count != 1 {
		t.Fatal(count)
	}
	for _, kind := range []string{"stale-name", "invalid-style", "stale-id"} {
		t.Run(kind, func(t *testing.T) {
			s, _ := d.Snapshot()
			key := "step:body-000001"
			label := s.Labels[key]
			switch kind {
			case "stale-name":
				label.Attributes["step.cap.lower.name"] = Attribute{Kind: "string", Text: "renamed"}
			case "invalid-style":
				data, _ := json.Marshal(wrappedNamedStyle{FaceColors: map[string]step.RGB{"Bottom": {R: 2}}})
				label.Attributes[wrappedStyleAttribute] = Attribute{Kind: "string", Text: string(data)}
			case "stale-id":
				label.Attributes["step.source.body.id"] = Attribute{Kind: "integer", Integer: 999999}
			}
			s.Labels[key] = label
			var dst bytes.Buffer
			if e := s.WriteColoredWrappedMaterialAssemblySTEP(&dst); e == nil || dst.Len() != 0 {
				t.Fatal("invalid provenance/style emitted")
			}
		})
	}
}

func coloredWrappedDocumentOracle(t *testing.T, raw []byte) {
	t.Helper()
	oracle := os.Getenv("PLAINSOLID_OCCT_ORACLE")
	if oracle == "" || os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
		return
	}
	path := t.TempDir() + "/document.step"
	if e := os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	out := t.TempDir() + "/style.json"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if log, e := exec.CommandContext(ctx, oracle, out, "step-colored-assembly", path).CombinedOutput(); e != nil {
		t.Fatalf("%v %s", e, log)
	}
	data, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var got struct {
		Occurrences            []struct{ Name string }
		BodyColors, FaceColors [][3]float64
	}
	if e = json.Unmarshal(data, &got); e != nil || len(got.Occurrences) != 2 || len(got.BodyColors) != 1 || len(got.FaceColors) != 2 {
		t.Fatal(got, e)
	}
}
