package document

import (
	"bytes"
	"os"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"testing"
)

func TestAnnularFrustumColoredDocument(t *testing.T) {
	raw, e := os.ReadFile("../exchange/step/testdata/wrapped-annular-frustum.step")
	if e != nil {
		t.Fatal(e)
	}
	body, e := step.ReadWrappedAnnularFrustum(bytes.NewReader(raw), step.Limits{}, step.WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	body.Name = "annular part"
	body.CapNames = [2]string{"bottom", "top"}
	body.Outer.Walls[0].Name = "outer taper"
	body.Inner.Walls[0].Name = "inner taper"
	model := step.ColoredWrappedMaterialAssemblyModel{LengthUnit: .001, Products: []step.ColoredWrappedMaterialAssemblyPart{{ID: "root", Name: "assembly"}, {ID: "part", Name: "shared frustum", Bodies: []step.ColoredWrappedMaterialBody{{Body: step.WrappedMaterialBody{AnnularFrustum: &body}, SurfaceColor: &step.RGB{R: .2, G: .4, B: .6}, FaceColors: map[string]step.RGB{"bottom": {R: .8, G: .1, B: .3}, "inner taper": {R: .1, G: .8, B: .3}}}}}}, Occurrences: []step.AssemblyInstance{{ID: "one", Name: "first", Parent: "root", Child: "part", Transform: geom.Identity()}, {ID: "two", Name: "second", Parent: "root", Child: "part", Transform: geom.Translation(geom.Vec3{X: 16})}}}
	var b bytes.Buffer
	if e = step.WriteColoredWrappedMaterialAssembly(&b, model); e != nil {
		t.Fatal(e)
	}
	d, e := ReadColoredWrappedMaterialAssemblySTEP(&b, step.Limits{}, step.WrappedSurfaceOptions{}, STEPAssemblyOptions{}, 1)
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
		t.Fatal("shared asset")
	}
	b.Reset()
	if e = s.WriteColoredWrappedMaterialAssemblySTEP(&b); e != nil {
		t.Fatal(e)
	}
	coloredWrappedDocumentOracle(t, b.Bytes())
	got, e := step.ReadColoredWrappedMaterialAssembly(&b, step.Limits{}, step.WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Assembly.Occurrences) != 2 {
		t.Fatal("occurrences")
	}
	count := 0
	for _, bs := range got.ProductBodies {
		for _, v := range bs {
			count++
			if v.Body.AnnularFrustum == nil || v.Body.Tube != nil || v.FaceColors["inner taper"] != (step.RGB{R: .1, G: .8, B: .3}) {
				t.Fatal(v)
			}
		}
	}
	if count != 1 {
		t.Fatal(count)
	}
}
