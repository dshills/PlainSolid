package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/geom"
	"strings"
	"testing"
	"time"
)

func torusFixture() ToroidalSupportModel {
	return ToroidalSupportModel{LengthUnit: .001, Surfaces: []ToroidalSupport{{Center: geom.Vec3{X: 2, Y: 3, Z: 4}, Axis: geom.Vec3{X: 1}, Reference: geom.Vec3{Z: -1}, MajorRadius: 4, MinorRadius: 1.25, Metadata: TorusDatumMetadata{"TORUS", 7}, AxisMetadata: TorusDatumMetadata{"AXIS", 1}}}}
}
func TestToroidalSupportRoundtrip(t *testing.T) {
	for _, sign := range []float64{1, -1} {
		m := torusFixture()
		m.Surfaces[0].Axis.X = sign
		var b bytes.Buffer
		if e := WriteToroidalSupports(&b, m); e != nil {
			t.Fatal(e)
		}
		got, e := ReadToroidalSupports(bytes.NewReader(b.Bytes()), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if got.LengthUnit != m.LengthUnit || len(got.Surfaces) != 1 || got.Surfaces[0] != m.Surfaces[0] {
			t.Fatal(got)
		}
		g, e := got.Surfaces[0].Geometry()
		if e != nil {
			t.Fatal(e)
		}
		p, _ := g.Eval(0, 0)
		if p != (geom.Vec3{X: 2, Y: 3, Z: -1.25}) {
			t.Fatal(p)
		}
		var c bytes.Buffer
		if e = WriteToroidalSupports(&c, got); e != nil || !bytes.Equal(b.Bytes(), c.Bytes()) {
			t.Fatal(e, "bytes")
		}
		if _, e = Read(bytes.NewReader(b.Bytes()), Limits{}); e == nil {
			t.Fatal("generic accepted")
		}
		for _, l := range []Limits{{MaxEntities: 3}, {MaxBytes: 100}, {MaxValues: 4}, {MaxRecords: 2}} {
			if _, e = ReadToroidalSupports(bytes.NewReader(b.Bytes()), l); e == nil {
				t.Fatal("limit")
			}
		}
	}
}
func TestToroidalSupportRejection(t *testing.T) {
	for _, mut := range []func(*ToroidalSupport){func(s *ToroidalSupport) { s.Axis.X = 2 }, func(s *ToroidalSupport) { s.Reference = s.Axis }, func(s *ToroidalSupport) { s.MinorRadius = 0 }, func(s *ToroidalSupport) { s.Center.X = .1 }, func(s *ToroidalSupport) { s.Metadata.Label = " padded" }} {
		m := torusFixture()
		mut(&m.Surfaces[0])
		var b bytes.Buffer
		if e := WriteToroidalSupports(&b, m); e == nil || b.Len() != 0 {
			t.Fatal("accepted or partial")
		}
	}
	m := torusFixture()
	var b bytes.Buffer
	_ = WriteToroidalSupports(&b, m)
	s := b.String()
	for _, change := range [][2]string{{"198,1,3,4.,1.25,5", "198,1,3,4.,1.25,3"}, {"123,1.,0.,0.", "123,2.,0.,0."}} {
		if !strings.Contains(s, change[0]) {
			t.Fatal("stale mutation", change)
		}
		x := strings.Replace(s, change[0], change[1], 1)
		if _, e := ReadToroidalSupports(strings.NewReader(x), Limits{}); e == nil {
			t.Fatal("accepted mutation")
		}
	}
}
func TestToroidalSupportRetained(t *testing.T) {
	b, e := os.ReadFile("testdata/toroidal-support.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadToroidalSupports(bytes.NewReader(b), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	s := m.Surfaces[0]
	if m.LengthUnit != .001 || s.Center != (geom.Vec3{X: 2, Y: 3, Z: 4}) || s.Axis != (geom.Vec3{X: 1}) || s.Reference != (geom.Vec3{Z: -1}) || s.MajorRadius != 4 || s.MinorRadius != 1.25 {
		t.Fatal(m)
	}
}
func TestOCCTToroidalSupport(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_IGES_TORUS")
	if bin == "" {
		t.Skip("optional OCCT")
	}
	run := func(mode, path string) []byte {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		b, e := exec.CommandContext(ctx, bin, mode, path).CombinedOutput()
		if e != nil {
			t.Fatalf("%s: %v %s", mode, e, b)
		}
		return b
	}
	for _, mode := range []string{"write", "write-negative"} {
		p := filepath.Join(t.TempDir(), "source.igs")
		run(mode, p)
		b, _ := os.ReadFile(p)
		m, e := ReadToroidalSupports(bytes.NewReader(b), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		var out bytes.Buffer
		if e = WriteToroidalSupports(&out, m); e != nil {
			t.Fatal(e)
		}
		q := filepath.Join(t.TempDir(), "native.igs")
		_ = os.WriteFile(q, out.Bytes(), 0600)
		g, _ := m.Surfaces[0].Geometry()
		point, _ := g.Eval(.375, -1.25)
		du, dv, _ := g.Partials(.375, -1.25)
		for _, path := range []string{p, q} {
			data := run("read", path)
			i := bytes.LastIndexByte(data, '{')
			if i < 0 {
				t.Fatal(string(data))
			}
			var v struct{ Point, Du, Dv [3]float64 }
			if e = json.Unmarshal(bytes.TrimSpace(data[i:]), &v); e != nil {
				t.Fatal(e)
			}
			for j, a := range [][3]float64{v.Point, v.Du, v.Dv} {
				want := []geom.Vec3{point, du, dv}[j]
				if (geom.Vec3{X: a[0], Y: a[1], Z: a[2]}).Sub(want).Length() > 1e-12 {
					t.Fatal(v, want)
				}
			}
		}
	}
}
func FuzzToroidalSupports(f *testing.F) {
	var b bytes.Buffer
	_ = WriteToroidalSupports(&b, torusFixture())
	f.Add(b.Bytes())
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadToroidalSupports(bytes.NewReader(b), Limits{MaxBytes: 8192, MaxRecords: 128, MaxEntities: 16, MaxValues: 128})
	})
}

func TestToroidalSupportDirectoryAdversaries(t *testing.T) {
	b, e := os.ReadFile("testdata/toroidal-support.igs")
	if e != nil {
		t.Fatal(e)
	}
	for _, mutation := range []struct {
		line, field int
		value       string
	}{{6, 4, "       0"}, {5, 6, "       3"}, {7, 8, "00010300"}, {5, 0, "     192"}} {
		lines := strings.Split(string(b), "\n")
		s := lines[mutation.line]
		a := mutation.field * 8
		lines[mutation.line] = s[:a] + mutation.value + s[a+8:]
		got, e := ReadToroidalSupports(strings.NewReader(strings.Join(lines, "\n")), Limits{})
		if e == nil || len(got.Surfaces) != 0 {
			t.Fatal("invalid directory accepted", mutation, e)
		}
	}
}

type torusShortWriter struct{}

func (torusShortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }
func TestToroidalSupportWriterFailure(t *testing.T) {
	if e := WriteToroidalSupports(torusShortWriter{}, torusFixture()); e != io.ErrShortWrite {
		t.Fatal(e)
	}
	m := torusFixture()
	m.Surfaces = append(m.Surfaces, m.Surfaces[0])
	m.Surfaces[1].Axis = geom.Vec3{}
	var b bytes.Buffer
	if e := WriteToroidalSupports(&b, m); e == nil || b.Len() != 0 {
		t.Fatal("non-atomic validation")
	}
}

func TestToroidalSupportRingAndPeriod(t *testing.T) {
	for _, minor := range []float64{4, 5, -1} {
		m := torusFixture()
		m.Surfaces[0].MinorRadius = minor
		var b bytes.Buffer
		if e := WriteToroidalSupports(&b, m); e == nil || b.Len() != 0 {
			t.Fatal("non-ring accepted")
		}
	}
	s := torusFixture().Surfaces[0]
	g, e := s.Geometry()
	if e != nil {
		t.Fatal(e)
	}
	p, _ := g.Eval(.375, -1.25)
	for _, uv := range [][2]float64{{.375 + 2*math.Pi, -1.25}, {.375, -1.25 + 2*math.Pi}} {
		q, e := g.Eval(uv[0], uv[1])
		if e != nil || q.Sub(p).Length() > 1e-14 {
			t.Fatal(q, p, e)
		}
	}
	m := torusFixture()
	m.LengthUnit = .3048
	var b bytes.Buffer
	if e = WriteToroidalSupports(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadToroidalSupports(&b, Limits{})
	if e != nil || got.LengthUnit != m.LengthUnit || got.Surfaces[0] != m.Surfaces[0] {
		t.Fatal(got, e)
	}
}
