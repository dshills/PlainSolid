package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
	"time"
)

func TestParabolicRoof(t *testing.T) {
	raw, e := os.ReadFile("testdata/parabolic-roof.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadParabolicRoofSolid(context.Background(), bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if !m.Embedding.Complete || m.Descriptor != (ParabolicRoofDescriptor{Width: 1, Depth: 1, Height: 1, Rise: 1}) {
		t.Fatal(m.Descriptor)
	}
	p, e := m.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-4.0/3) > 1e-9 || p.Centroid.Distance(geom.Vec3{X: 9.0 / 16, Y: .5, Z: .7}) > 1e-9 {
		t.Fatal(p, e)
	}
	var b bytes.Buffer
	if e = WriteParabolicRoofSolid(context.Background(), &b, m.Descriptor, .001); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadParabolicRoofSolid(context.Background(), &b, Limits{}); e != nil {
		t.Fatal(e)
	}
}
func TestParabolicRoofRejects(t *testing.T) {
	raw, e := os.ReadFile("testdata/parabolic-roof.igs")
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		k, i int
		s    string
	}{{186, 2, "0"}, {514, 3, "0"}, {508, 5, "0"}, {126, 14, "0.25"}, {128, 22, "0.25"}, {502, 2, "0.25"}, {504, 7, "3"}, {508, 8, "3"}} {
		b := tetraMutate(t, raw, tc.k, tc.i, tc.s)
		m, e := ReadParabolicRoofSolid(context.Background(), bytes.NewReader(b), Limits{})
		if e == nil || m.SourceSolidID != 0 || len(m.Geometry) != 0 {
			t.Fatal(tc, e)
		}
	}
	for _, l := range []Limits{{MaxControlPoints: 103}, {MaxEntities: 57}, {MaxBytes: 100}} {
		if _, e = ReadParabolicRoofSolid(context.Background(), bytes.NewReader(raw), l); e == nil {
			t.Fatal(l)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = ReadParabolicRoofSolid(ctx, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("canceled")
	}
	var b bytes.Buffer
	if e = WriteParabolicRoofSolid(context.Background(), &b, ParabolicRoofDescriptor{Width: 1, Depth: 1, Height: 1, Rise: -1}, .001); e == nil || b.Len() != 0 {
		t.Fatal("invalid writer", e)
	}
	d := ParabolicRoofDescriptor{Origin: geom.Vec3{X: .25, Y: -2, Z: 3}, Width: 2, Depth: 3, Height: .5, Rise: .25}
	if e = WriteParabolicRoofSolid(context.Background(), &b, d, .001); e != nil {
		t.Fatal(e)
	}
	m, e := ReadParabolicRoofSolid(context.Background(), &b, Limits{})
	if e != nil || m.Descriptor != d {
		t.Fatal(m.Descriptor, e)
	}
}
func TestOCCTParabolicRoof(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_IGES_SOLID")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	raw, e := os.ReadFile("testdata/parabolic-roof.igs")
	if e != nil {
		t.Fatal(e)
	}
	for _, native := range []bool{false, true} {
		b := raw
		if native {
			var out bytes.Buffer
			if e = WriteParabolicRoofSolid(context.Background(), &out, ParabolicRoofDescriptor{Width: 1, Depth: 1, Height: 1, Rise: 1}, .001); e != nil {
				t.Fatal(e)
			}
			b = out.Bytes()
		}
		path := filepath.Join(t.TempDir(), "roof.igs")
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
			t.Fatal(got, area)
		}
		for i, w := range [3]float64{9.0 / 16, .5, .7} {
			if math.Abs(got.Center[i]-w) > 1e-10 {
				t.Fatal(got)
			}
		}
	}
}
func FuzzParabolicRoof(f *testing.F) {
	raw, e := os.ReadFile("testdata/parabolic-roof.igs")
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
		m, e := ReadParabolicRoofSolid(ctx, bytes.NewReader(b), Limits{})
		if e != nil && (m.SourceSolidID != 0 || len(m.Geometry) != 0) {
			t.Fatal("partial")
		}
	})
}

func TestParabolicRoofWholeCurveMutations(t *testing.T) {
	raw, e := os.ReadFile("testdata/parabolic-roof.igs")
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []int{126, 128} {
		d, e := readTopologySource(bytes.NewReader(raw), Limits{}, true)
		if e != nil {
			t.Fatal(e)
		}
		a := topologyAuthor{}
		zero := geom.Vec3{}
		a.geometry.Entities = []Entity{{Point: &zero}}
		changed := false
		for _, r := range d.entities {
			degreeIndex := 2
			poleIndex := 21
			if kind == 128 {
				degreeIndex = 3
				poleIndex = 31
			}
			if !changed && r.kind == kind {
				degree, e := intValue(r.values[degreeIndex])
				if e != nil {
					t.Fatal(e)
				}
				if degree == 2 {
					r.values[poleIndex].text = "1.25"
					changed = true
				}
			}
			p := make([]string, len(r.values))
			for i, v := range r.values {
				p[i] = v.text
			}
			a.add(r.kind, map[int]int{502: 1, 504: 1, 508: 1, 510: 1, 514: 1}[r.kind], joinTopologyValues(p))
		}
		if !changed {
			t.Fatal("missing quadratic")
		}
		b, e := a.file(d.unit)
		if e != nil {
			t.Fatal(e)
		}
		m, e := ReadParabolicRoofSolid(context.Background(), bytes.NewReader(b), Limits{})
		if e == nil || m.SourceSolidID != 0 {
			t.Fatal("interior coefficient mutation accepted", kind, e)
		}
	}
}
