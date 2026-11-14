package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func multipleMappedSource(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/occt-multiple-mapped-planar-boxes.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestMultipleMappedPlanarMaterials(t *testing.T) {
	raw := multipleMappedSource(t)
	m, e := ReadMappedPlanarMaterials(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Materials) != 2 || len(m.MapOrder) != 2 || m.LengthUnit != .001 {
		t.Fatal(m)
	}
	a, b := m.Materials[m.MapOrder[0]], m.Materials[m.MapOrder[1]]
	if a.SourceBodyID == b.SourceBodyID || len(a.Occurrences) != 2 || len(b.Occurrences) != 1 {
		t.Fatal(a, b)
	}
	p, e := a.Body.Solid.MassProperties()
	if e != nil || p.Volume != 24 {
		t.Fatal(p, e)
	}
	q, e := b.Body.Solid.MassProperties()
	if e != nil || q.Volume != 6 {
		t.Fatal(q, e)
	}
	var out bytes.Buffer
	if e = WriteMappedPlanarMaterialsSource(&out, m.Source); e != nil {
		t.Fatal(e)
	}
	again, e := ReadMappedPlanarMaterials(&out, Limits{})
	if e != nil || !reflect.DeepEqual(again.Source.Header, m.Source.Header) || !reflect.DeepEqual(again.MapOrder, m.MapOrder) {
		t.Fatal(e)
	}
	if _, e = ReadMappedPlanarMaterial(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("single map widened")
	}
	if _, e = ReadAssembly(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("assembly widened")
	}
}
func TestMultipleMappedPlanarMaterialsRejects(t *testing.T) {
	raw := multipleMappedSource(t)
	m, e := ReadMappedPlanarMaterials(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, mode := range []string{"same-representation", "unused-map", "shared-line", "radius-orphan"} {
		f, e := Parse(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		first, last := m.MapOrder[0], m.MapOrder[1]
		switch mode {
		case "same-representation":
			en := f.Entities[last]
			en.Records[0].Args[1] = f.Entities[first].Records[0].Args[1]
			f.Entities[last] = en
		case "unused-map":
			for _, id := range f.Order {
				en := f.Entities[id]
				if len(en.Records) == 1 && en.Records[0].Type == "MAPPED_ITEM" && en.Records[0].Args[1].Ref == last {
					en.Records[0].Args[1].Ref = first
					f.Entities[id] = en
				}
			}
		case "shared-line":
			var lines []int64
			for _, mid := range m.MapOrder {
				local := m.Materials[mid]
				for _, id := range local.Source.Order {
					en := local.Source.Entities[id]
					if len(en.Records) == 1 && en.Records[0].Type == "SURFACE_CURVE" {
						lines = append(lines, id)
						break
					}
				}
			}
			if len(lines) != 2 {
				t.Fatal(lines)
			}
			en := f.Entities[lines[1]]
			en.Records[0].Args[1] = f.Entities[lines[0]].Records[0].Args[1]
			f.Entities[lines[1]] = en
		case "radius-orphan":
			id := int64(9999)
			f.Entities[id] = Entity{ID: id, Records: []Record{{Type: "CARTESIAN_POINT", Args: []Value{{Kind: String}, {Kind: Aggregate, Items: []Value{{Kind: Number, Text: "0."}, {Kind: Number, Text: "0."}, {Kind: Number, Text: "0."}}}}}}}
			f.Order = append(f.Order, id)
		}
		var out bytes.Buffer
		if e = WriteMappedPlanarMaterialsSource(&out, f); e == nil || out.Len() != 0 {
			t.Fatal(mode)
		}
	}
	for _, l := range []Limits{{MaxSolids: 1}, {MaxFaces: 11}, {MaxVertices: 15}, {MaxEdgeUses: 47}, {MaxBytes: 100}} {
		if _, e := ReadMappedPlanarMaterials(bytes.NewReader(raw), l); e == nil {
			t.Fatal(l)
		}
	}
}
func TestOCCTMultipleMappedPlanarMaterials(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_DISK_BOUND_MASS")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	raw := multipleMappedSource(t)
	m, e := ReadMappedPlanarMaterials(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteMappedPlanarMaterialsSource(&out, m.Source); e != nil {
		t.Fatal(e)
	}
	for _, b := range [][]byte{raw, out.Bytes()} {
		path := filepath.Join(t.TempDir(), "mapped.step")
		if e = os.WriteFile(path, b, 0600); e != nil {
			t.Fatal(e)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		r, e := exec.CommandContext(ctx, bin, path).CombinedOutput()
		cancel()
		if e != nil {
			t.Fatal(e, string(r))
		}
		at := bytes.LastIndex(r, []byte(`{"Faces"`))
		if at < 0 {
			t.Fatal(string(r))
		}
		var p struct {
			Faces, Solids int
			Volume, Area  float64
			Center        [3]float64
		}
		if e = json.Unmarshal(r[at:], &p); e != nil {
			t.Fatal(e)
		}
		if p.Faces != 18 || p.Solids != 3 || math.Abs(p.Volume-54) > 1e-9 || math.Abs(p.Area-126) > 1e-9 {
			t.Fatal(p)
		}
		for i, x := range []float64{85. / 18, 91. / 9, 31. / 18} {
			if math.Abs(p.Center[i]-x) > 1e-9 {
				t.Fatal(p)
			}
		}
	}
}
func FuzzMultipleMappedPlanarMaterials(f *testing.F) {
	f.Add(multipleMappedSource(f))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 131072 {
			t.Skip()
		}
		_, _ = ReadMappedPlanarMaterials(bytes.NewReader(b), Limits{MaxBytes: 131072, MaxEntities: 2048, MaxSolids: 3, MaxFaces: 24, MaxVertices: 64, MaxEdgeUses: 64})
	})
}
