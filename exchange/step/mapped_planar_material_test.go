package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/geom"
	"testing"
	"time"
)

func mappedPlanarSource(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/occt-mapped-planar-box.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestMappedPlanarMaterial(t *testing.T) {
	t.Parallel()
	raw := mappedPlanarSource(t)
	m, e := ReadMappedPlanarMaterial(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Occurrences) != 2 || m.LengthUnit != .001 || m.SourceBodyID != 15 || m.MapID != 351 || m.Name != "mapped boxes" {
		t.Fatal(m)
	}
	p, e := m.Body.Solid.MassProperties()
	if e != nil || p.Volume != 24 || p.SurfaceArea != 52 || p.Centroid != (geom.Vec3{X: 1, Y: 1.5, Z: 2}) {
		t.Fatal(p, e)
	}
	if m.Occurrences[0].Transform.Apply(p.Centroid) != (geom.Vec3{X: 11, Y: 1.5, Z: 2}) || m.Occurrences[1].Transform.Apply(p.Centroid) != (geom.Vec3{X: 2, Y: 21, Z: 1.5}) {
		t.Fatal(m.Occurrences)
	}
	var out bytes.Buffer
	if e = WriteMappedPlanarMaterialSource(&out, m.Source); e != nil {
		t.Fatal(e)
	}
	n, e := ReadMappedPlanarMaterial(&out, Limits{})
	if e != nil || len(n.Occurrences) != 2 {
		t.Fatal(n, e)
	}
	if _, e = ReadAssembly(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("assembly widened")
	}
}
func TestMappedPlanarMaterialRejects(t *testing.T) {
	t.Parallel()
	raw := mappedPlanarSource(t)
	for _, tc := range []struct {
		id, index int
		v         Value
	}{{356, 6, Value{Kind: Number, Text: "2."}}, {351, 0, Value{Kind: Reference, Ref: 16}}, {359, 4, Value{Kind: Reference, Ref: 353}}, {361, 2, Value{Kind: Reference, Ref: 1}}, {351, 1, Value{Kind: Reference, Ref: 361}}, {356, 0, Value{Kind: String, Text: "unknown function"}}} {
		f, e := Parse(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		en := f.Entities[int64(tc.id)]
		en.Records[0].Args[tc.index] = tc.v
		f.Entities[int64(tc.id)] = en
		m, e := ReadMappedPlanarMaterial(bytes.NewReader(serializeWrappedProofFile(f)), Limits{})
		if e == nil || m.Source != nil {
			t.Fatal(tc)
		}
	}
	for _, l := range []Limits{{MaxBytes: 100}, {MaxFaces: 5}, {MaxVertices: 7}, {MaxEdgeUses: 1}, {MaxEntities: 100}} {
		if _, e := ReadMappedPlanarMaterial(bytes.NewReader(raw), l); e == nil {
			t.Fatal(l)
		}
	}
	// A tiny exactly represented line displacement passes the old relative check
	// but must fail the exact source support proof, without a tolerance fallback.
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, id := range f.Order {
		en := f.Entities[id]
		if len(en.Records) == 1 && en.Records[0].Type == "LINE" {
			pid := en.Records[0].Args[1].Ref
			p := f.Entities[pid]
			coords := p.Records[0].Args[1].Items
			if len(coords) == 3 {
				coords[0] = Value{Kind: Number, Text: "0.000000000931322574615478515625"}
				p.Records[0].Args[1].Items = coords
				f.Entities[pid] = p
				break
			}
		}
	}
	var out bytes.Buffer
	if e = WriteMappedPlanarMaterialSource(&out, f); e == nil || out.Len() != 0 {
		t.Fatal("perturbed line accepted")
	}
}
func TestMappedPlanarPcurveOwnership(t *testing.T) {
	t.Parallel()
	raw := mappedPlanarSource(t)
	for _, mode := range []string{"owner", "offset", "orphan"} {
		f, e := Parse(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if mode == "orphan" {
			id := int64(9999)
			f.Entities[id] = Entity{ID: id, Records: []Record{{Type: "CARTESIAN_POINT", Args: []Value{{Kind: String}, {Kind: Aggregate, Items: []Value{{Kind: Number, Text: "0."}, {Kind: Number, Text: "0."}, {Kind: Number, Text: "0."}}}}}}}
			f.Order = append(f.Order, id)
		} else {
			for _, id := range f.Order {
				en := f.Entities[id]
				if len(en.Records) != 1 || en.Records[0].Type != "SURFACE_CURVE" {
					continue
				}
				pcs := en.Records[0].Args[2].Items
				first, second := f.Entities[pcs[0].Ref], f.Entities[pcs[1].Ref]
				if mode == "owner" {
					first.Records[0].Args[1] = second.Records[0].Args[1]
					f.Entities[first.ID] = first
				} else {
					rep := f.Entities[first.Records[0].Args[2].Ref]
					lineID := rep.Records[0].Args[1].Items[0].Ref
					line := f.Entities[lineID]
					pid := line.Records[0].Args[1].Ref
					p := f.Entities[pid]
					vec := f.Entities[line.Records[0].Args[2].Ref]
					dir := f.Entities[vec.Records[0].Args[1].Ref]
					axis := 0
					v, _ := numeric(dir.Records[0].Args[1].Items[0])
					if v != 0 {
						axis = 1
					}
					p.Records[0].Args[1].Items[axis] = Value{Kind: Number, Text: "0.000000000931322574615478515625"}
					f.Entities[pid] = p
				}
				break
			}
		}
		if v, e := ReadMappedPlanarMaterial(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}); e == nil || v.Source != nil {
			t.Fatal(mode)
		}
	}
}

func TestOCCTMappedPlanarMaterial(t *testing.T) {
	t.Parallel()
	bin := os.Getenv("PLAINSOLID_OCCT_DISK_BOUND_MASS")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	raw := mappedPlanarSource(t)
	m, e := ReadMappedPlanarMaterial(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteMappedPlanarMaterialSource(&out, m.Source); e != nil {
		t.Fatal(e)
	}
	for _, b := range [][]byte{raw, out.Bytes()} {
		p := filepath.Join(t.TempDir(), "mapped.step")
		if e = os.WriteFile(p, b, 0600); e != nil {
			t.Fatal(e)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		result, e := exec.CommandContext(ctx, bin, p).CombinedOutput()
		cancel()
		if e != nil {
			t.Fatal(e, string(result))
		}
		at := bytes.LastIndex(result, []byte(`{"Faces"`))
		if at < 0 {
			t.Fatal(string(result))
		}
		var v struct {
			Faces, Solids int
			Volume, Area  float64
			Center        [3]float64
		}
		if e = json.Unmarshal(result[at:], &v); e != nil {
			t.Fatal(e)
		}
		if v.Faces != 12 || v.Solids != 2 || math.Abs(v.Volume-48) > 1e-9 || math.Abs(v.Area-104) > 1e-9 {
			t.Fatal(v)
		}
		for i, x := range []float64{6.5, 11.25, 1.75} {
			if math.Abs(v.Center[i]-x) > 1e-9 {
				t.Fatal(v)
			}
		}
	}
}
func FuzzMappedPlanarMaterial(f *testing.F) {
	f.Add(mappedPlanarSource(f))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 131072 {
			t.Skip()
		}
		_, _ = ReadMappedPlanarMaterial(bytes.NewReader(b), Limits{MaxBytes: 131072, MaxEntities: 1024, MaxFaces: 16, MaxVertices: 64, MaxEdgeUses: 64})
	})
}
