package document

import (
	"bytes"
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

// These files were produced independently by OCCT, rather than by the native
// document serializer or analytic constructors exercised by this regression.
func TestAnalyticSTEPDocumentPersistence(t *testing.T) {
	fixtures := []struct {
		name         string
		volume, area float64
	}{
		{"tube", 32 * math.Pi, 48 * math.Pi},
		{"spherical-zone", 22 * math.Pi / 3, 14 * math.Pi},
		{"cylinder-sector", 4 * math.Pi, 6*math.Pi + 16},
		{"conical-sector", 13 * math.Pi / 3, (2.5+2*math.Sqrt(5))*math.Pi + 16},
		{"torus-elbow", 1.5 * math.Pi * math.Pi, 3*math.Pi*math.Pi + 2*math.Pi},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			raw, e := os.ReadFile(filepath.Join("..", "exchange", "step", "testdata", fixture.name+".step"))
			if e != nil {
				t.Fatal(e)
			}
			model, e := step.ReadAnalyticBodies(bytes.NewReader(raw), step.Limits{})
			if e != nil || len(model.Bodies) != 1 {
				t.Fatal(e, len(model.Bodies))
			}
			model.Bodies[0].Name = "Independent " + fixture.name
			for _, unit := range []float64{.001, .0254} {
				model.LengthUnit = unit
				doc, e := FromAnalyticSTEP(model, 2)
				if e != nil {
					t.Fatal(e)
				}
				state, _ := doc.Snapshot()
				id := "body-000001"
				// Two occurrences retain a shared exact asset. Only the second is nested.
				state.Nodes["parent"] = Node{Name: "Assembly", Transform: geom.Translation(geom.Vec3{X: 10, Y: -4, Z: 2})}
				state.Nodes["second"] = Node{Name: "Repeated part", Parent: "parent", Asset: id, Transform: geom.Affine{{0, -1, 0, 3}, {1, 0, 0, 5}, {0, 0, 1, 7}}}
				doc, e = New(state, 2)
				if e != nil {
					t.Fatal(e)
				}
				var encoded bytes.Buffer
				if e = doc.Write(&encoded); e != nil {
					t.Fatal(e)
				}
				restored, e := Read(bytes.NewReader(encoded.Bytes()), 0, 2)
				if e != nil {
					t.Fatal(e)
				}
				got, _ := restored.Snapshot()
				if got.LengthUnit != unit || len(got.Parametric) != 1 || len(got.Assets) != 0 || len(got.Trimmed) != 0 || len(got.Nodes) != 3 || got.Nodes[id].Name != model.Bodies[0].Name || got.Nodes["second"] != state.Nodes["second"] || got.Nodes["parent"] != state.Nodes["parent"] {
					t.Fatal("asset, names, units or instances changed")
				}
				if len(got.Parametric[id].Faces()) != len(model.Bodies[0].Solid.Faces()) {
					t.Fatal("topology changed")
				}
				p, e := got.Parametric[id].MassProperties(context.Background(), parametric.IntegrationOptions{})
				if e != nil || math.Abs(p.Volume-fixture.volume) > 1e-7 || math.Abs(p.Area-fixture.area) > 1e-7 {
					t.Fatal(p, e)
				}
				transform := state.Nodes["parent"].Transform.Mul(state.Nodes["second"].Transform)
				placed, e := got.Parametric[id].Transformed(transform)
				if e != nil {
					t.Fatal(e)
				}
				pp, e := placed.MassProperties(context.Background(), parametric.IntegrationOptions{})
				if e != nil || math.Abs(pp.Volume-p.Volume) > 1e-7 || pp.Centroid.Distance(transform.Apply(p.Centroid)) > 1e-7 {
					t.Fatal(pp, e)
				}
				var repeated bytes.Buffer
				if e = restored.Write(&repeated); e != nil {
					t.Fatal(e)
				}
				if !bytes.Equal(encoded.Bytes(), repeated.Bytes()) {
					t.Fatal("native representation changed across read/write")
				}
			}
		})
	}
}
