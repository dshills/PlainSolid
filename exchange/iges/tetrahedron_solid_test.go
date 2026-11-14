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

func TestTetrahedronSolid(t *testing.T) {
	raw, e := os.ReadFile("testdata/tetrahedron-solid.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadTetrahedronSolid(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Solid.Faces()) != 4 || len(m.Geometry) != 22 {
		t.Fatal(m)
	}
	props, e := m.Solid.MassProperties()
	if e != nil || math.Abs(props.Volume-4.0/3) > 1e-12 || math.Abs(props.SurfaceArea-(6+2*math.Sqrt(3))) > 1e-12 || props.Centroid != (geom.Vec3{X: .5, Y: .5, Z: .5}) {
		t.Fatal(props, e)
	}
	var out bytes.Buffer
	verts := [4]geom.Vec3{{}, {X: 2}, {Y: 2}, {Z: 2}}
	if e = WriteTetrahedronSolid(&out, verts, .001); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadTetrahedronSolid(&out, Limits{}); e != nil {
		t.Fatal(e)
	}
}

func tetraMutate(t testing.TB, raw []byte, kind int, index int, value string) []byte {
	t.Helper()
	d, e := readTopologySource(bytes.NewReader(raw), Limits{}, true)
	if e != nil {
		t.Fatal(e)
	}
	a := topologyAuthor{}
	zero := geom.Vec3{}
	a.geometry.Entities = []Entity{{Point: &zero}}
	changed := false
	for _, r := range d.entities {
		if r.kind == kind && !changed {
			r.values[index].text = value
			changed = true
		}
		p := make([]string, len(r.values))
		for i, v := range r.values {
			p[i] = v.text
		}
		a.add(r.kind, map[int]int{502: 1, 504: 1, 508: 1, 510: 1, 514: 1}[r.kind], joinTopologyValues(p))
	}
	if !changed {
		t.Fatal("mutation absent")
	}
	b, e := a.file(d.unit)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestTetrahedronSolidRejects(t *testing.T) {
	raw, e := os.ReadFile("testdata/tetrahedron-solid.igs")
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		k, i int
		s    string
	}{{186, 2, "0"}, {186, 3, "1"}, {514, 3, "0"}, {508, 5, "0"}, {126, 14, "0.25"}, {126, 21, "1"}, {128, 22, "0.25"}, {502, 2, "0.25"}, {502, 0, "503"}, {510, 3, "0"}, {504, 7, "3"}, {508, 8, "3"}, {510, 1, "29"}} {
		b := tetraMutate(t, raw, tc.k, tc.i, tc.s)
		m, e := ReadTetrahedronSolid(bytes.NewReader(b), Limits{})
		if e == nil || m.SourceSolidID != 0 || len(m.Geometry) != 0 {
			t.Fatal(tc, e)
		}
	}
	for _, l := range []Limits{{MaxEntities: 33}, {MaxControlPoints: 51}, {MaxBytes: 100}, {MaxValues: 10}} {
		if _, e = ReadTetrahedronSolid(bytes.NewReader(raw), l); e == nil {
			t.Fatal(l)
		}
	}
	var b bytes.Buffer
	if e = WriteTetrahedronSolid(&b, [4]geom.Vec3{{}, {X: 1}, {Y: 1}, {X: 1, Y: 1}}, .001); e == nil || b.Len() != 0 {
		t.Fatal("non-atomic degenerate writer", e)
	}
	// Reflection reverses global orientation and the writer must reverse faces.
	verts := [4]geom.Vec3{{X: 3, Y: 4, Z: 5}, {X: 1, Y: 4, Z: 5}, {X: 3, Y: 6, Z: 5}, {X: 3, Y: 4, Z: 7}}
	if e = WriteTetrahedronSolid(&b, verts, .001); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadTetrahedronSolid(&b, Limits{}); e != nil {
		t.Fatal(e)
	}
}
func TestOCCTTetrahedronSolid(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_IGES_SOLID")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	raw, e := os.ReadFile("testdata/tetrahedron-solid.igs")
	if e != nil {
		t.Fatal(e)
	}
	for _, native := range []bool{false, true} {
		b := raw
		if native {
			var out bytes.Buffer
			if e = WriteTetrahedronSolid(&out, [4]geom.Vec3{{}, {X: 2}, {Y: 2}, {Z: 2}}, .001); e != nil {
				t.Fatal(e)
			}
			b = out.Bytes()
		}
		path := filepath.Join(t.TempDir(), "tetra.igs")
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
		if got.Faces != 4 || got.Solids != 1 || math.Abs(got.Volume-4.0/3) > 1e-10 || math.Abs(got.Area-(6+2*math.Sqrt(3))) > 1e-10 || got.Center != [3]float64{.5, .5, .5} {
			t.Fatal(got)
		}
	}
}
func FuzzTetrahedronSolid(f *testing.F) {
	raw, e := os.ReadFile("testdata/tetrahedron-solid.igs")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Add([]byte("invalid"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			return
		}
		m, e := ReadTetrahedronSolid(bytes.NewReader(b), Limits{})
		if e != nil && (m.SourceSolidID != 0 || len(m.Geometry) != 0) {
			t.Fatal("partial")
		}
	})
}
