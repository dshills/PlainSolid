package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"
)

func TestMixedWrappedMaterials(t *testing.T) {
	read := func(name string) []byte {
		raw, e := os.ReadFile("testdata/" + name + ".step")
		if e != nil {
			t.Fatal(e)
		}
		return raw
	}
	a, e := ReadWrappedToroidalBodies(bytes.NewReader(read("wrapped-toroid")), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	b, e := ReadWrappedCylindricalBodies(bytes.NewReader(read("wrapped-cylinder")), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	c, e := ReadWrappedFrusta(bytes.NewReader(read("wrapped-frustum")), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	a.Bodies[0].Name = "Toroid"
	b.Bodies[0].Name = "Cylinder"
	c.Bodies[0].Name = "Frustum"
	m := WrappedMaterialModel{LengthUnit: .0254, Bodies: []WrappedMaterialBody{{Toroid: &a.Bodies[0]}, {Cylinder: &b.Bodies[0]}, {Frustum: &c.Bodies[0]}}}
	var out bytes.Buffer
	if e = WriteWrappedMaterials(&out, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadWrappedMaterials(bytes.NewReader(out.Bytes()), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Bodies) != 3 || got.LengthUnit != m.LengthUnit || got.Bodies[0].Toroid == nil || got.Bodies[1].Cylinder == nil || got.Bodies[2].Frustum == nil || got.Bodies[0].Toroid.Name != "Toroid" || got.Bodies[1].Cylinder.Name != "Cylinder" || got.Bodies[2].Frustum.Name != "Frustum" {
		t.Fatal(got)
	}
	seen := map[int64]bool{}
	for _, b := range got.Bodies {
		if b.SourceBodyID == 0 || seen[b.SourceBodyID] {
			t.Fatal("source ownership")
		}
		seen[b.SourceBodyID] = true
	}
	f, e := Parse(bytes.NewReader(out.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	first, _ := simple(f, got.Bodies[0].SourceBodyID, "MANIFOLD_SOLID_BREP", 2)
	second, _ := simple(f, got.Bodies[1].SourceBodyID, "MANIFOLD_SOLID_BREP", 2)
	second.Args = append([]Value(nil), second.Args...)
	second.Args[1] = first.Args[1]
	id := got.Bodies[1].SourceBodyID
	f.Entities[id] = Entity{ID: id, Records: []Record{second}}
	if _, e = ReadWrappedMaterials(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("accepted shared shell ownership")
	}
	if _, e = ReadWrappedMaterials(bytes.NewReader(out.Bytes()), Limits{MaxSolids: 2}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("body budget")
	}
	m.Bodies[0].Cylinder = &b.Bodies[0]
	var bad bytes.Buffer
	if e = WriteWrappedMaterials(&bad, m); e == nil || bad.Len() != 0 {
		t.Fatal("ambiguous payload", e)
	}
	if _, e = ReadWrappedMaterials(bytes.NewReader(read("conical-sector")), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("guessed unsupported analytic body")
	}
}

func TestMixedWrappedMaterialsOCCT(t *testing.T) {
	m := WrappedMaterialModel{LengthUnit: .001}
	sources := []string{"testdata/wrapped-toroid.step", "testdata/wrapped-cylinder.step", "testdata/wrapped-frustum.step", "testdata/wrapped-tube.step", "testdata/wrapped-blind-bore.step", "testdata/wrapped-shoulder.step", "testdata/wrapped-cavity.step", "testdata/wrapped-toroidal-void.step", "testdata/wrapped-station-loft.step", "testdata/wrapped-rim-fillet.step", "testdata/wrapped-station-bore.step", "testdata/wrapped-annular-frustum.step"}
	for i, path := range sources {
		raw, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		switch i {
		case 0:
			q, e := ReadWrappedToroidalBodies(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
			if e != nil {
				t.Fatal(e)
			}
			m.Bodies = append(m.Bodies, WrappedMaterialBody{Toroid: &q.Bodies[0]})
		case 1:
			q, e := ReadWrappedCylindricalBodies(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
			if e != nil {
				t.Fatal(e)
			}
			m.Bodies = append(m.Bodies, WrappedMaterialBody{Cylinder: &q.Bodies[0]})
		case 2:
			q, e := ReadWrappedFrusta(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
			if e != nil {
				t.Fatal(e)
			}
			m.Bodies = append(m.Bodies, WrappedMaterialBody{Frustum: &q.Bodies[0]})
		case 3:
			q, e := ReadWrappedTube(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
			if e != nil {
				t.Fatal(e)
			}
			m.Bodies = append(m.Bodies, WrappedMaterialBody{Tube: &q})
		case 4:
			q, e := ReadWrappedBlindBore(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
			if e != nil {
				t.Fatal(e)
			}
			m.Bodies = append(m.Bodies, WrappedMaterialBody{BlindBore: &q})
		case 5:
			q, e := ReadWrappedShoulder(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
			if e != nil {
				t.Fatal(e)
			}
			m.Bodies = append(m.Bodies, WrappedMaterialBody{Shoulder: &q})

		case 6:
			q, e := ReadWrappedCavity(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
			if e != nil {
				t.Fatal(e)
			}
			m.Bodies = append(m.Bodies, WrappedMaterialBody{Cavity: &q})
		case 7:
			q, e := ReadWrappedToroidalVoid(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
			if e != nil {
				t.Fatal(e)
			}
			m.Bodies = append(m.Bodies, WrappedMaterialBody{ToroidalVoid: &q})

		case 8:
			q, e := ReadWrappedStationLoft(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
			if e != nil {
				t.Fatal(e)
			}
			m.Bodies = append(m.Bodies, WrappedMaterialBody{StationLoft: &q})
		case 11:
			q, e := ReadWrappedAnnularFrustum(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
			if e != nil {
				t.Fatal(e)
			}
			m.Bodies = append(m.Bodies, WrappedMaterialBody{AnnularFrustum: &q})
		case 10:
			q, e := ReadWrappedStationBore(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
			if e != nil {
				t.Fatal(e)
			}
			m.Bodies = append(m.Bodies, WrappedMaterialBody{StationBore: &q})
		case 9:
			q, e := ReadWrappedRimFillet(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
			if e != nil {
				t.Fatal(e)
			}
			m.Bodies = append(m.Bodies, WrappedMaterialBody{RimFillet: &q})

		}
	}
	var b bytes.Buffer
	if e := WriteWrappedMaterials(&b, m); e != nil {
		t.Fatal(e)
	}
	combined := t.TempDir() + "/mixed.step"
	if e := os.WriteFile(combined, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	sumArea, sumVolume := 0., 0.
	for i, path := range append(sources, combined) {
		out := t.TempDir() + "/mass.json"
		stepOracle(t, out, "step-mass-precise", path)
		raw, e := os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		var p struct{ Area, Volume float64 }
		if e = json.Unmarshal(raw, &p); e != nil {
			t.Fatal(e)
		}
		if i < len(sources) {
			sumArea += p.Area
			sumVolume += p.Volume
		} else if math.Abs(sumArea-p.Area) > 1e-7 || math.Abs(sumVolume-p.Volume) > 1e-7 {
			t.Fatal(sumArea, sumVolume, p)
		}
	}
}

func TestMixedWrappedBoreTags(t *testing.T) {
	read := func(path string) []byte {
		raw, e := os.ReadFile("testdata/" + path + ".step")
		if e != nil {
			t.Fatal(e)
		}
		return raw
	}
	tube, e := ReadWrappedTube(bytes.NewReader(read("wrapped-tube")), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	blind, e := ReadWrappedBlindBore(bytes.NewReader(read("wrapped-blind-bore")), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	shoulder, e := ReadWrappedShoulder(bytes.NewReader(read("wrapped-shoulder")), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	tube.Name = "Tube"
	blind.Name = "Blind bore"
	shoulder.Name = "Shoulder"
	m := WrappedMaterialModel{LengthUnit: .0254, Bodies: []WrappedMaterialBody{{Tube: &tube}, {BlindBore: &blind}, {Shoulder: &shoulder}}}
	var b bytes.Buffer
	if e = WriteWrappedMaterials(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadWrappedMaterials(&b, Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if got.LengthUnit != m.LengthUnit || len(got.Bodies) != 3 || got.Bodies[0].Tube == nil || got.Bodies[0].Tube.Name != "Tube" || got.Bodies[1].BlindBore == nil || got.Bodies[1].BlindBore.Name != "Blind bore" || got.Bodies[2].Shoulder == nil || got.Bodies[2].Shoulder.Name != "Shoulder" {
		t.Fatal(got)
	}
	m.Bodies[0].Shoulder = &shoulder
	if e = WriteWrappedMaterials(&b, m); e == nil || b.Len() != 0 {
		t.Fatal("ambiguous bore payload", e)
	}
}

func TestMixedWrappedSharedVertexReject(t *testing.T) {
	raw, e := os.ReadFile("testdata/wrapped-cylinder.step")
	if e != nil {
		t.Fatal(e)
	}
	q, e := ReadWrappedCylindricalBodies(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m := WrappedMaterialModel{LengthUnit: .001, Bodies: []WrappedMaterialBody{{Cylinder: &q.Bodies[0]}, {Cylinder: &q.Bodies[0]}}}
	var b bytes.Buffer
	if e = WriteWrappedMaterials(&b, m); e != nil {
		t.Fatal(e)
	}
	decoded, e := ReadWrappedMaterials(bytes.NewReader(b.Bytes()), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	first := decoded.Bodies[0].Cylinder.Face.CornerVertices[0]
	second := decoded.Bodies[1].Cylinder.Face.CornerVertices[0]
	f, e := Parse(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var rewrite func(Value) Value
	rewrite = func(v Value) Value {
		if v.Kind == Reference && v.Ref == second {
			v.Ref = first
		}
		v.Items = append([]Value(nil), v.Items...)
		for i := range v.Items {
			v.Items[i] = rewrite(v.Items[i])
		}
		return v
	}
	for id, ent := range f.Entities {
		ent.Records = append([]Record(nil), ent.Records...)
		for i, r := range ent.Records {
			r.Args = append([]Value(nil), r.Args...)
			for j := range r.Args {
				r.Args[j] = rewrite(r.Args[j])
			}
			ent.Records[i] = r
		}
		f.Entities[id] = ent
	}
	if _, e = ReadWrappedMaterials(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("accepted shared source vertex across independent bodies")
	}
}

func TestMixedWrappedVoidTagsAndSchema(t *testing.T) {
	raw, e := os.ReadFile("testdata/wrapped-cavity.step")
	if e != nil {
		t.Fatal(e)
	}
	a, e := ReadWrappedCavity(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	raw, e = os.ReadFile("testdata/wrapped-toroidal-void.step")
	if e != nil {
		t.Fatal(e)
	}
	b, e := ReadWrappedToroidalVoid(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	a.Name = "Cylinder void"
	b.Name = "Toroid void"
	m := WrappedMaterialModel{LengthUnit: .0254, Bodies: []WrappedMaterialBody{{Cavity: &a}, {ToroidalVoid: &b}}}
	var out bytes.Buffer
	if e = WriteWrappedMaterials(&out, m); e != nil {
		t.Fatal(e)
	}
	schema := "AP242_MANAGED_MODEL_BASED_3D_ENGINEERING_MIM_LF"
	ap242 := strings.Replace(out.String(), "CONFIG_CONTROL_DESIGN", schema, 1)
	if ap242 == out.String() {
		t.Fatal("stale schema mutation")
	}
	got, e := ReadWrappedMaterials(strings.NewReader(ap242), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Bodies) != 2 || got.Bodies[0].Cavity == nil || got.Bodies[1].ToroidalVoid == nil || got.Bodies[0].Cavity.Name != a.Name || got.Bodies[1].ToroidalVoid.Name != b.Name || got.LengthUnit != m.LengthUnit || len(got.SourceSchemas) != 1 || got.SourceSchemas[0] != schema {
		t.Fatal(got)
	}
	// Metadata alone does not admit unsupported entity graphs.
	bad := strings.Replace(ap242, "ORIENTED_CLOSED_SHELL", "UNSUPPORTED_VOID_SHELL", 1)
	if _, e = ReadWrappedMaterials(strings.NewReader(bad), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("schema admitted unsupported void graph")
	}
	if _, e = ReadWrappedMaterials(strings.NewReader(ap242), Limits{}, WrappedSurfaceOptions{MaxSurfaces: 3}); e == nil {
		t.Fatal("cumulative surface budget")
	}
	m.Bodies[0].ToroidalVoid = &b
	var rejected bytes.Buffer
	if e = WriteWrappedMaterials(&rejected, m); e == nil || rejected.Len() != 0 {
		t.Fatal("ambiguous void payload", e)
	}
}
