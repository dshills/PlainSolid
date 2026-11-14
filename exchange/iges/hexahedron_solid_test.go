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

func hexaFixtureVertices() [8]geom.Vec3 {
	var v [8]geom.Vec3
	for i := range v {
		v[i] = geom.Vec3{X: float64(3 - 2*(i&1) + (i >> 2)), Y: float64(4 - (i & 1) + 3*((i>>1)&1)), Z: float64(5 + ((i >> 1) & 1) + 2*(i>>2))}
	}
	return v
}
func TestHexahedronSolid(t *testing.T) {
	raw, e := os.ReadFile("testdata/hexahedron-solid.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadHexahedronSolid(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Solid.Faces()) != 6 || len(m.Geometry) != 42 {
		t.Fatal(m)
	}
	p, e := m.Solid.MassProperties()
	if e != nil || math.Abs(p.Volume-13) > 1e-12 || p.Centroid.Distance(geom.Vec3{X: 2.5, Y: 5, Z: 6.5}) > 1e-12 {
		t.Fatal(p, e)
	}
	var out bytes.Buffer
	if e = WriteHexahedronSolid(&out, hexaFixtureVertices(), .001); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadHexahedronSolid(&out, Limits{}); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadTetrahedronSolid(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("tetra widened")
	}
}
func TestHexahedronSolidRejects(t *testing.T) {
	raw, e := os.ReadFile("testdata/hexahedron-solid.igs")
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		k, i int
		s    string
	}{{186, 2, "0"}, {514, 3, "0"}, {508, 5, "0"}, {126, 14, "0.25"}, {128, 22, "0.25"}, {502, 2, "0.25"}, {502, 0, "503"}, {504, 7, "3"}, {508, 8, "3"}, {510, 3, "0"}} {
		b := tetraMutate(t, raw, tc.k, tc.i, tc.s)
		m, e := ReadHexahedronSolid(bytes.NewReader(b), Limits{})
		if e == nil || m.SourceSolidID != 0 || len(m.Geometry) != 0 {
			t.Fatal(tc, e)
		}
	}
	for _, l := range []Limits{{MaxEntities: 57}, {MaxControlPoints: 95}, {MaxBytes: 100}, {MaxValues: 10}} {
		if _, e = ReadHexahedronSolid(bytes.NewReader(raw), l); e == nil {
			t.Fatal(l)
		}
	}
	v := hexaFixtureVertices()
	v[7].X += .25
	var out bytes.Buffer
	if e = WriteHexahedronSolid(&out, v, .001); e == nil || out.Len() != 0 {
		t.Fatal("warped corner accepted", e)
	}
	v = hexaFixtureVertices()
	v[7] = v[0]
	if e = WriteHexahedronSolid(&out, v, .001); e == nil || out.Len() != 0 {
		t.Fatal("pinch accepted", e)
	}
	// Positive rather than reflected frame remains independently supported.
	v = hexaFixtureVertices()
	for i := range v {
		v[i].X = -v[i].X
	}
	if e = WriteHexahedronSolid(&out, v, .001); e != nil {
		t.Fatal(e)
	}
}
func TestOCCTHexahedronSolid(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_IGES_SOLID")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	raw, e := os.ReadFile("testdata/hexahedron-solid.igs")
	if e != nil {
		t.Fatal(e)
	}
	for _, native := range []bool{false, true} {
		b := raw
		if native {
			var out bytes.Buffer
			if e = WriteHexahedronSolid(&out, hexaFixtureVertices(), .001); e != nil {
				t.Fatal(e)
			}
			b = out.Bytes()
		}
		path := filepath.Join(t.TempDir(), "hexa.igs")
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
		wantArea := 2 * (math.Sqrt(41) + math.Sqrt(46) + math.Sqrt(21))
		if got.Faces != 6 || got.Solids != 1 || math.Abs(got.Volume-13) > 1e-10 || math.Abs(got.Area-wantArea) > 1e-10 {
			t.Fatal(got)
		}
		for i, w := range [3]float64{2.5, 5, 6.5} {
			if math.Abs(got.Center[i]-w) > 1e-10 {
				t.Fatal(got)
			}
		}
	}
}
func FuzzHexahedronSolid(f *testing.F) {
	raw, e := os.ReadFile("testdata/hexahedron-solid.igs")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Add([]byte("invalid"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			return
		}
		m, e := ReadHexahedronSolid(bytes.NewReader(b), Limits{})
		if e != nil && (m.SourceSolidID != 0 || len(m.Geometry) != 0) {
			t.Fatal("partial")
		}
	})
}
