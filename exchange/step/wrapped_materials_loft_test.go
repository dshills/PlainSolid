package step

import (
	"bytes"
	"os"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestMixedWrappedLoftDisambiguation(t *testing.T) {
	slowtest.Skip(t)
	load := func(name string) []byte {
		t.Helper()
		b, e := os.ReadFile("testdata/" + name + ".step")
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	loft, e := ReadWrappedStationLoft(bytes.NewReader(load("wrapped-station-loft")), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	shoulder, e := ReadWrappedShoulder(bytes.NewReader(load("wrapped-shoulder")), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	frustum, e := ReadWrappedFrusta(bytes.NewReader(load("wrapped-frustum")), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	loft.Name = "station loft"
	shoulder.Name = "shoulder"
	frustum.Bodies[0].Name = "frustum"
	m := WrappedMaterialModel{LengthUnit: .0254, Bodies: []WrappedMaterialBody{{StationLoft: &loft}, {Shoulder: &shoulder}, {Frustum: &frustum.Bodies[0]}}}
	var out bytes.Buffer
	if e = WriteWrappedMaterials(&out, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadWrappedMaterials(bytes.NewReader(out.Bytes()), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Bodies) != 3 || got.LengthUnit != m.LengthUnit || got.Bodies[0].StationLoft == nil || got.Bodies[1].Shoulder == nil || got.Bodies[2].Frustum == nil {
		t.Fatal("family classification", got)
	}
	if got.Bodies[0].StationLoft.Name != loft.Name || got.Bodies[1].Shoulder.Name != shoulder.Name || got.Bodies[2].Frustum.Name != "frustum" {
		t.Fatal("names")
	}
	f, e := Parse(bytes.NewReader(out.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	seen := map[int64]bool{}
	for _, b := range got.Bodies {
		if b.SourceBodyID <= 0 || seen[b.SourceBodyID] {
			t.Fatal("body ownership")
		}
		seen[b.SourceBodyID] = true
		if _, e := simple(f, b.SourceBodyID, "MANIFOLD_SOLID_BREP", 2); e != nil {
			t.Fatal(e)
		}
	}
	for _, wall := range got.Bodies[0].StationLoft.Walls {
		for _, id := range wall.SideEdges {
			if _, e := simple(f, id, "EDGE_CURVE", 5); e != nil {
				t.Fatal("source edge ID", e)
			}
		}
	}
	// A reversed band is neither a positive loft nor an annular shoulder.
	body, _ := simple(f, got.Bodies[0].SourceBodyID, "MANIFOLD_SOLID_BREP", 2)
	sid, _ := ref(body.Args[1])
	shell, _ := simple(f, sid, "CLOSED_SHELL", 2)
	faces, _ := listRefs(shell.Args[1], 1)
	changed := false
	for _, id := range faces {
		face, _ := simple(f, id, "ADVANCED_FACE", 4)
		support, _ := ref(face.Args[2])
		if _, ok := component(f.Entities[support], "B_SPLINE_SURFACE_WITH_KNOTS"); ok {
			face.Args = append([]Value(nil), face.Args...)
			face.Args[3] = Value{Kind: Enumeration, Text: "F"}
			f.Entities[id] = Entity{ID: id, Records: []Record{face}}
			changed = true
			break
		}
	}
	if !changed {
		t.Fatal("missing wall")
	}
	if _, e = ReadWrappedMaterials(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("accepted reversed loft wall")
	}
	m.Bodies[0].Shoulder = &shoulder
	var bad bytes.Buffer
	if e = WriteWrappedMaterials(&bad, m); e == nil || bad.Len() != 0 {
		t.Fatal("ambiguous loft payload", e)
	}
}
