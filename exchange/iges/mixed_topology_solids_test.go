package iges

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

func mixedTopologyInputs() []MixedTopologyInput {
	return []MixedTopologyInput{{Family: MixedTopologyTetrahedron, Vertices: [4]geom.Vec3{{}, {X: 2}, {Y: 2}, {Z: 2}}}, {Family: MixedTopologyRoof, Roof: ParabolicRoofDescriptor{Origin: geom.Vec3{X: .25, Y: -2, Z: 3}, Width: 1, Depth: 1, Height: 1, Rise: 1}}}
}
func mixedTopologySource(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/mixed-topology-solids.igs")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestMixedTopologySolids(t *testing.T) {
	ctx := context.Background()
	var out bytes.Buffer
	if e := WriteMixedTopologySolids(ctx, &out, mixedTopologyInputs(), .001); e != nil {
		t.Fatal(e)
	}
	for _, b := range [][]byte{mixedTopologySource(t), out.Bytes()} {
		m, e := ReadMixedTopologySolids(ctx, bytes.NewReader(b), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if len(m.Bodies) != 2 || m.LengthUnit != .001 {
			t.Fatal(m)
		}
		seen := map[int]bool{}
		for _, v := range m.Bodies {
			if v.LocalToSource[func() int {
				if v.Tetrahedron != nil {
					return v.Tetrahedron.SourceSolidID
				}
				return v.Roof.SourceSolidID
			}()] != v.SourceSolidID {
				t.Fatal("root mapping")
			}
			for _, id := range v.LocalToSource {
				if seen[id] {
					t.Fatal("shared source ownership")
				}
				seen[id] = true
			}
			if v.Family == MixedTopologyTetrahedron {
				if v.Tetrahedron == nil || v.Roof != nil || len(v.LocalToSource) != 34 {
					t.Fatal(v)
				}
			} else if v.Roof == nil || v.Tetrahedron != nil || len(v.LocalToSource) != 58 || !v.Roof.Embedding.Complete {
				t.Fatal(v)
			}
		}
		if len(seen) != 92 {
			t.Fatal(len(seen))
		}
		if _, e = ReadMappedParabolicRoofSolid(ctx, bytes.NewReader(b), Limits{}); e == nil {
			t.Fatal("single body accepted multiple")
		}
	}
	reverse := mixedTopologyInputs()
	reverse[0], reverse[1] = reverse[1], reverse[0]
	out.Reset()
	if e := WriteMixedTopologySolids(ctx, &out, reverse, 1); e != nil {
		t.Fatal(e)
	}
	m, e := ReadMixedTopologySolids(ctx, &out, Limits{})
	if e != nil || m.LengthUnit != 1 || m.Bodies[0].Family != MixedTopologyRoof {
		t.Fatal(e)
	}
}
func TestMixedTopologySolidsRejects(t *testing.T) {
	b := mixedTopologySource(t)
	ctx := context.Background()
	for _, l := range []Limits{{MaxEntities: 91}, {MaxControlPoints: 155}, {MaxBytes: 100}, {MaxValues: 40}} {
		m, e := ReadMixedTopologySolids(ctx, bytes.NewReader(b), l)
		if e == nil || len(m.Bodies) != 0 {
			t.Fatal(l, e)
		}
	}
	for _, tc := range []struct {
		k, i int
		s    string
	}{{186, 1, "71"}, {186, 2, "0"}, {504, 2, "181"}, {508, 8, "181"}, {502, 2, "0.5"}} {
		bad := tetraMutate(t, b, tc.k, tc.i, tc.s)
		m, e := ReadMixedTopologySolids(ctx, bytes.NewReader(bad), Limits{})
		if e == nil || len(m.Bodies) != 0 {
			t.Fatal(tc, e)
		}
	}
	bad := placedDir(b, 1, 3, 1)
	if _, e := ReadMixedTopologySolids(ctx, bytes.NewReader(bad), Limits{}); e == nil {
		t.Fatal("metadata erased")
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := ReadMixedTopologySolids(c, bytes.NewReader(b), Limits{}); e == nil {
		t.Fatal("cancel")
	}
	for _, inputs := range [][]MixedTopologyInput{mixedTopologyInputs()[:1], {mixedTopologyInputs()[0], mixedTopologyInputs()[0]}, {{Family: "unknown"}, mixedTopologyInputs()[1]}} {
		var out bytes.Buffer
		if e := WriteMixedTopologySolids(ctx, &out, inputs, .001); e == nil || out.Len() != 0 {
			t.Fatal("atomic writer", e)
		}
	}
}
func TestOCCTMixedTopologySolids(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_IGES_SOLID")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	var out bytes.Buffer
	if e := WriteMixedTopologySolids(context.Background(), &out, mixedTopologyInputs(), .001); e != nil {
		t.Fatal(e)
	}
	for _, b := range [][]byte{mixedTopologySource(t), out.Bytes()} {
		path := filepath.Join(t.TempDir(), "mixed.igs")
		if e := os.WriteFile(path, b, 0600); e != nil {
			t.Fatal(e)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		raw, e := exec.CommandContext(ctx, bin, path).CombinedOutput()
		cancel()
		if e != nil {
			t.Fatal(e, string(raw))
		}
		at := bytes.LastIndex(raw, []byte(`{"Faces"`))
		if at < 0 {
			t.Fatal(string(raw))
		}
		var p struct {
			Faces, Solids int
			Volume, Area  float64
			Center        [3]float64
		}
		if e = json.Unmarshal(raw[at:], &p); e != nil {
			t.Fatal(e)
		}
		area := 6 + 2*math.Sqrt(3) + 20.0/3 + math.Sqrt(5)/2 + math.Asinh(2)/4
		if p.Faces != 10 || p.Solids != 2 || math.Abs(p.Volume-8.0/3) > 1e-9 || math.Abs(p.Area-area) > 1e-8 || math.Abs(p.Center[0]-.65625) > 1e-9 || math.Abs(p.Center[1]+.5) > 1e-9 || math.Abs(p.Center[2]-2.1) > 1e-9 {
			t.Fatal(p)
		}
	}
}
func FuzzMixedTopologySolids(f *testing.F) {
	f.Add(mixedTopologySource(f))
	f.Add([]byte("bad"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			t.Skip()
		}
		_, _ = ReadMixedTopologySolids(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 116, MaxControlPoints: 208})
	})
}

func TestMixedTopologyWriterRejectsUnusedPayload(t *testing.T) {
	for _, which := range []int{0, 1} {
		inputs := mixedTopologyInputs()
		if which == 0 {
			inputs[0].Roof = inputs[1].Roof
		} else {
			inputs[1].Vertices = inputs[0].Vertices
		}
		var b bytes.Buffer
		if e := WriteMixedTopologySolids(context.Background(), &b, inputs, .001); e == nil || b.Len() != 0 {
			t.Fatal(which, e)
		}
	}
	inputs := mixedTopologyInputs()
	for len(inputs) < 9 {
		inputs = append(inputs, inputs[0])
	}
	var b bytes.Buffer
	if e := WriteMixedTopologySolids(context.Background(), &b, inputs, .001); e == nil || b.Len() != 0 {
		t.Fatal("root budget", e)
	}
}

func TestMixedTopologyRejectsOrphanGeometry(t *testing.T) {
	doc, e := readTopologySource(bytes.NewReader(mixedTopologySource(t)), Limits{}, true)
	if e != nil {
		t.Fatal(e)
	}
	var extra topologyRecord
	for _, r := range doc.entities {
		if r.kind == 126 {
			extra = r
			break
		}
	}
	extra.id = 2*len(doc.entities) + 1
	doc.entities = append(doc.entities, extra)
	ids := map[int]int{}
	for _, r := range doc.entities {
		ids[r.id] = r.id
	}
	raw, e := remapTopologyRecords(doc.entities, ids, doc.unit)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadMixedTopologySolids(context.Background(), bytes.NewReader(raw), Limits{})
	if e == nil || len(m.Bodies) != 0 {
		t.Fatal("orphan model curve", e)
	}
}
