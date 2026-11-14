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
	"strconv"
	"testing"
	"time"
)

func TestMappedParabolicRoof(t *testing.T) {
	raw, e := os.ReadFile("testdata/mapped-parabolic-roof.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadMappedParabolicRoofSolid(context.Background(), bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if m.Descriptor.Origin != (geom.Vec3{X: .25, Y: -2, Z: 3}) || m.AuthoredFaceOrder[0] != m.SourceFaceIDs[5] || !m.Embedding.Complete || m.SourceSolidID != 1 || m.SourceVertexListID != 115 || m.SourceVertexIndices[7] != 1 || m.SourceEdgeIndices[11] != 1 || m.SourceUseIndices[0][1] != 1 || len(m.Geometry) != 42 {
		t.Fatal(m)
	}
	named := bytes.Clone(raw)
	lines := bytes.Split(named, []byte("\n"))
	for _, line := range lines {
		if len(line) == 80 && line[72] == 'D' {
			seq, _ := field(string(line[73:]))
			if seq == 2 {
				copy(line[56:64], []byte("ORIGINAL"))
				copy(line[64:72], []byte("       7"))
				break
			}
		}
	}
	withName, err := ReadMappedParabolicRoofSolid(context.Background(), bytes.NewReader(bytes.Join(lines, []byte("\n"))), Limits{})
	if err != nil || withName.Metadata[1] != (CylinderDatumMetadata{Label: "ORIGINAL", Subscript: 7}) {
		t.Fatal("source metadata", withName.Metadata[1], err)
	}
	if _, e = ReadParabolicRoofSolid(context.Background(), bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("canonical reader widened")
	}
	var b bytes.Buffer
	if e = WriteParabolicRoofSolid(context.Background(), &b, m.Descriptor, m.LengthUnit); e != nil {
		t.Fatal(e)
	}
	n, e := ReadMappedParabolicRoofSolid(context.Background(), &b, Limits{})
	if e != nil || n.Descriptor != m.Descriptor {
		t.Fatal(e)
	}
}
func TestMappedParabolicRoofRejects(t *testing.T) {
	raw, e := os.ReadFile("testdata/mapped-parabolic-roof.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadMappedParabolicRoofSolid(context.Background(), bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		k, i int
		s    string
	}{{186, 2, "0"}, {514, 3, "0"}, {126, 14, "0.25"}, {128, 22, "0.25"}, {502, 2, "0.5"}, {504, 7, strconv.Itoa(m.SourceModelCurveIDs[11])}, {508, 8, strconv.Itoa(m.SourcePCurveIDs[0][0])}} {
		b := tetraMutate(t, raw, tc.k, tc.i, tc.s)
		r, e := ReadMappedParabolicRoofSolid(context.Background(), bytes.NewReader(b), Limits{})
		if e == nil || r.SourceSolidID != 0 || len(r.Geometry) != 0 {
			t.Fatal(tc, e)
		}
	}
	// Every edge use remains individually valid, but a noncyclic permutation is
	// not the authored directed boundary and must not be normalized away.
	d, e := readTopologySource(bytes.NewReader(raw), Limits{}, true)
	if e != nil {
		t.Fatal(e)
	}
	a := topologyAuthor{}
	zero := geom.Vec3{}
	a.geometry.Entities = []Entity{{Point: &zero}}
	changed := false
	for _, r := range d.entities {
		if r.kind == 508 && !changed {
			first := append([]parameter(nil), r.values[2:9]...)
			copy(r.values[2:9], r.values[9:16])
			copy(r.values[9:16], first)
			changed = true
		}
		p := make([]string, len(r.values))
		for i, v := range r.values {
			p[i] = v.text
		}
		a.add(r.kind, map[int]int{502: 1, 504: 1, 508: 1, 510: 1, 514: 1}[r.kind], joinTopologyValues(p))
	}
	b, e := a.file(d.unit)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadMappedParabolicRoofSolid(context.Background(), bytes.NewReader(b), Limits{}); e == nil {
		t.Fatal("noncyclic loop normalized")
	}
	for _, l := range []Limits{{MaxEntities: 57}, {MaxControlPoints: 103}, {MaxBytes: 100}} {
		if _, e = ReadMappedParabolicRoofSolid(context.Background(), bytes.NewReader(raw), l); e == nil {
			t.Fatal(l)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = ReadMappedParabolicRoofSolid(ctx, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("cancel")
	}
}
func TestOCCTMappedParabolicRoof(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_IGES_SOLID")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	raw, e := os.ReadFile("testdata/mapped-parabolic-roof.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadMappedParabolicRoofSolid(context.Background(), bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, normalized := range []bool{false, true} {
		b := raw
		if normalized {
			var out bytes.Buffer
			if e = WriteParabolicRoofSolid(context.Background(), &out, m.Descriptor, m.LengthUnit); e != nil {
				t.Fatal(e)
			}
			b = out.Bytes()
		}
		path := filepath.Join(t.TempDir(), "mapped.igs")
		if e = os.WriteFile(path, b, 0600); e != nil {
			t.Fatal(e)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		out, e := exec.CommandContext(ctx, bin, path).CombinedOutput()
		cancel()
		if e != nil {
			t.Fatal(e, string(out))
		}
		at := bytes.LastIndex(out, []byte("{\"Faces\""))
		if at < 0 {
			t.Fatal(string(out))
		}
		var got struct {
			Faces, Solids int
			Volume, Area  float64
			Center        [3]float64
		}
		if e = json.Unmarshal(bytes.TrimSpace(out[at:]), &got); e != nil {
			t.Fatal(e)
		}
		area := 20.0/3 + math.Sqrt(5)/2 + math.Asinh(2)/4
		if got.Faces != 6 || got.Solids != 1 || math.Abs(got.Volume-4.0/3) > 1e-10 || math.Abs(got.Area-area) > 1e-8 {
			t.Fatal(got)
		}
		for i, w := range [3]float64{.8125, -1.5, 3.7} {
			if math.Abs(got.Center[i]-w) > 1e-10 {
				t.Fatal(got)
			}
		}
	}
}
func FuzzMappedParabolicRoof(f *testing.F) {
	raw, e := os.ReadFile("testdata/mapped-parabolic-roof.igs")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Add([]byte("invalid"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		m, e := ReadMappedParabolicRoofSolid(ctx, bytes.NewReader(b), Limits{})
		if e != nil && (m.SourceSolidID != 0 || len(m.Geometry) != 0) {
			t.Fatal("partial")
		}
	})
}
