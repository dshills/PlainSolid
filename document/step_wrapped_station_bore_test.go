package document

import (
	"bytes"
	"os"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestStationBoreColoredMixedDocument(t *testing.T) {
	slowtest.Skip(t)
	raw, e := os.ReadFile("../exchange/step/testdata/wrapped-station-bore.step")
	if e != nil {
		t.Fatal(e)
	}
	bore, e := step.ReadWrappedStationBore(bytes.NewReader(raw), step.Limits{}, step.WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	bore.Name = "Bored loft"
	bore.CapNames = [2]string{"lower annulus", "upper annulus"}
	for i := range bore.Outer.Walls {
		bore.Outer.Walls[i].Name = []string{"Outer lower", "Outer upper"}[i]
	}
	for i := range bore.Inner.Walls {
		bore.Inner.Walls[i].Name = []string{"Inner lower", "Inner upper"}[i]
	}
	fillet := filletMaterialFixture(t).Bodies[0]
	model := step.ColoredWrappedMaterialAssemblyModel{LengthUnit: .001, Products: []step.ColoredWrappedMaterialAssemblyPart{{ID: "root", Name: "mixed assembly"}, {ID: "shared", Name: "two materials", Bodies: []step.ColoredWrappedMaterialBody{{Body: fillet, SurfaceColor: &step.RGB{R: .2, G: .4, B: .6}, FaceColors: map[string]step.RGB{"Bottom": {R: .8, G: .1, B: .3}}}, {Body: step.WrappedMaterialBody{StationBore: &bore}, FaceColors: map[string]step.RGB{"Outer upper": {R: .1, G: .8, B: .3}}}}}}, Occurrences: []step.AssemblyInstance{{ID: "one", Name: "first", Parent: "root", Child: "shared", Transform: geom.Identity()}, {ID: "two", Name: "second", Parent: "root", Child: "shared", Transform: geom.Affine{{0, -1, 0, 20}, {1, 0, 0, 0}, {0, 0, 1, 0}}}}}
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
	if len(s.Trimmed) != 2 {
		t.Fatal("shared assets")
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
	if len(got.Assembly.Occurrences) != 2 || got.Assembly.Occurrences[0].Child != got.Assembly.Occurrences[1].Child {
		t.Fatal("shared product")
	}
	found := false
	for _, bs := range got.ProductBodies {
		for _, v := range bs {
			if v.Body.StationBore != nil {
				found = true
				if v.FaceColors["Outer upper"] != (step.RGB{R: .1, G: .8, B: .3}) || v.Body.StationBore.Inner.Walls[1].Name != "Inner upper" {
					t.Fatal(v)
				}
			}
		}
	}
	if !found {
		t.Fatal("family lost")
	}
	// Standalone translations remain exact and preserve all source chart names.
	s.Nodes = map[string]Node{"placed": {Name: "translated bore", Asset: "body-000002", Transform: geom.Translation(geom.Vec3{X: 8, Z: 16})}}
	for key, label := range s.Labels {
		if label.Target == NodeLabel {
			delete(s.Labels, key)
		}
	}
	s.Features = nil
	b.Reset()
	if e = s.WriteWrappedMaterialsSTEP(&b); e != nil {
		t.Fatal(e)
	}
	placed, e := step.ReadWrappedMaterials(&b, step.Limits{}, step.WrappedSurfaceOptions{})
	if e != nil || len(placed.Bodies) != 1 || placed.Bodies[0].StationBore == nil {
		t.Fatal(placed, e)
	}
	if placed.Bodies[0].StationBore.Outer.Center.X != 8 || placed.Bodies[0].StationBore.Inner.Stations[0].Z != 16 {
		t.Fatal("placement")
	}
}
