package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func TestWrappedStationLoftRoundtrip(t *testing.T) {
	m := WrappedStationLoftBody{Name: "Station loft", Stations: []trimmed.WrappedLoftStation{{Z: 0, Radius: 2}, {Z: 2, Radius: 1}, {Z: 4, Radius: 2}}, LengthUnit: .001}
	var b bytes.Buffer
	if e := WriteWrappedStationLoft(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadWrappedStationLoft(&b, Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Walls) != 2 || got.Walls[0].SideEdges[2] != got.Walls[1].SideEdges[0] || got.Solid.Topology().Faces != 4 {
		t.Fatal(got)
	}
}

func TestWrappedStationLoftStoredOCCT(t *testing.T) {
	raw, e := os.ReadFile("testdata/wrapped-station-loft.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadWrappedStationLoft(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteWrappedStationLoft(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadWrappedStationLoft(&b, Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for i, w := range m.Walls {
		a, e := w.Geometry.ExportRecipe()
		if e != nil {
			t.Fatal(e)
		}
		c, e := got.Walls[i].Geometry.ExportRecipe()
		if e != nil {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(a.Surface.Data(), c.Surface.Data()) {
			t.Fatal("source support changed")
		}
	}
	native, e := WrappedStationLoftFromSolid(context.Background(), m.Solid, "Native loft", m.LengthUnit, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if e = WriteWrappedStationLoft(&b, native); e != nil {
		t.Fatal(e)
	}
}
func TestWrappedStationLoftOCCT(t *testing.T) {
	source := t.TempDir() + "/source.step"
	stepOracle(t, source, "write-step-wrapped-station-loft")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadWrappedStationLoft(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m.Name = "Circular station loft"
	m.Walls[0].Name = "Descending"
	m.Walls[1].Name = "Ascending"
	m.CapNames = [2]string{"Lower disk", "Upper disk"}
	var b bytes.Buffer
	if e = WriteWrappedStationLoft(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadWrappedStationLoft(bytes.NewReader(b.Bytes()), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if got.Name != m.Name || got.Walls[0].Name != m.Walls[0].Name || got.Walls[1].Name != m.Walls[1].Name || got.CapNames != m.CapNames || got.LengthUnit != m.LengthUnit {
		t.Fatal(got)
	}
	path := t.TempDir() + "/roundtrip.step"
	if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	native, e := m.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if e != nil {
		t.Fatal(e)
	}
	wantV, wantA := 28*math.Pi/3, 6*math.Pi*math.Sqrt(5)+8*math.Pi
	if math.Abs(native.Volume-wantV) > 1e-6 || math.Abs(native.Area-wantA) > 1e-6 || math.Abs(native.Centroid.Z-2) > 1e-7 {
		t.Fatal(native)
	}
	for _, p := range []string{source, path} {
		out := t.TempDir() + "/mass.json"
		stepOracle(t, out, "step-mass-precise", p)
		raw, e := os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		var mass struct {
			Area, Volume float64
			Centroid     [3]float64
		}
		if e = json.Unmarshal(raw, &mass); e != nil {
			t.Fatal(e)
		}
		if math.Abs(mass.Volume-wantV) > 1e-6 || math.Abs(mass.Area-wantA) > 1e-6 || math.Abs(mass.Centroid[2]-2) > 1e-7 {
			t.Fatal(mass)
		}
	}
}

func TestWrappedStationLoftRejectOwnership(t *testing.T) {
	raw, e := os.ReadFile("testdata/wrapped-station-loft.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadWrappedStationLoft(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	shared := m.Walls[0].SideEdges[2]
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	next := int64(1)
	for id := range f.Entities {
		if id >= next {
			next = id + 1
		}
	}
	copyEdge := f.Entities[shared]
	copyEdge.ID = next
	f.Entities[next] = copyEdge
	f.Order = append(f.Order, next)
	changed := false
	for id, ent := range f.Entities {
		if len(ent.Records) != 1 || ent.Records[0].Type != "ORIENTED_EDGE" {
			continue
		}
		r := ent.Records[0]
		edge, _ := ref(r.Args[3])
		sense, _ := boolean(r.Args[4])
		if edge == shared && sense {
			r.Args = append([]Value(nil), r.Args...)
			r.Args[3] = Value{Kind: Reference, Ref: next}
			f.Entities[id] = Entity{ID: id, Records: []Record{r}}
			changed = true
			break
		}
	}
	if !changed {
		t.Fatal("missing shared station use")
	}
	if _, e = ReadWrappedStationLoft(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("accepted unwelded station rim")
	}
	m.Stations[1].Radius += .5
	var rejected bytes.Buffer
	if e = WriteWrappedStationLoft(&rejected, m); e == nil || rejected.Len() != 0 {
		t.Fatal("accepted mismatched cached wall support", e)
	}
	if _, e = ReadWrappedStationLoft(bytes.NewReader(raw), Limits{MaxFaces: 3}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("face limit")
	}
}
func TestWrappedStationLoftThreeBands(t *testing.T) {
	m := WrappedStationLoftBody{Center: geom.Vec2{X: 4, Y: 8}, Stations: []trimmed.WrappedLoftStation{{Z: 0, Radius: 2}, {Z: 2, Radius: 1}, {Z: 4, Radius: 3}, {Z: 6, Radius: 2}}, LengthUnit: .0254}
	var b bytes.Buffer
	if e := WriteWrappedStationLoft(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadWrappedStationLoft(&b, Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(got.Stations, m.Stations) || got.Center != m.Center || got.LengthUnit != m.LengthUnit || len(got.Walls) != 3 {
		t.Fatal(got)
	}
	for i := 1; i < 3; i++ {
		if got.Walls[i-1].SideEdges[2] != got.Walls[i].SideEdges[0] {
			t.Fatal("unshared station")
		}
	}
}
