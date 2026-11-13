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

func cone45Fixture() Conical45SupportModel {
	return Conical45SupportModel{LengthUnit: .001, Surfaces: []Conical45Support{{Center: geom.Vec3{X: 2, Y: 3, Z: 4}, Axis: geom.Vec3{X: 1}, Reference: geom.Vec3{Z: -1}, Radius: 2.5, Metadata: ConeDatumMetadata{"CONE", 7}, AxisMetadata: ConeDatumMetadata{"AXIS", 1}}}}
}
func TestConical45SupportRoundtrip(t *testing.T) {
	for _, sign := range []float64{1, -1} {
		m := cone45Fixture()
		m.Surfaces[0].Axis.X = sign
		var b bytes.Buffer
		if e := WriteConical45Supports(&b, m); e != nil {
			t.Fatal(e)
		}
		got, e := ReadConical45Supports(bytes.NewReader(b.Bytes()), Limits{})
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
		if p != (geom.Vec3{X: 2, Y: 3, Z: 1.5}) {
			t.Fatal(p)
		}
		var c bytes.Buffer
		if e = WriteConical45Supports(&c, got); e != nil || !bytes.Equal(b.Bytes(), c.Bytes()) {
			t.Fatal(e, "bytes")
		}
		if _, e = Read(bytes.NewReader(b.Bytes()), Limits{}); e == nil {
			t.Fatal("generic accepted")
		}
		for _, l := range []Limits{{MaxEntities: 3}, {MaxBytes: 100}, {MaxValues: 4}, {MaxRecords: 2}} {
			if _, e = ReadConical45Supports(bytes.NewReader(b.Bytes()), l); e == nil {
				t.Fatal("limit")
			}
		}
	}
}
func TestConical45SupportRejection(t *testing.T) {
	for _, mut := range []func(*Conical45Support){func(s *Conical45Support) { s.Axis.X = 2 }, func(s *Conical45Support) { s.Reference = s.Axis }, func(s *Conical45Support) { s.Radius = 0 }, func(s *Conical45Support) { s.Center.X = .1 }, func(s *Conical45Support) { s.Metadata.Label = " padded" }} {
		m := cone45Fixture()
		mut(&m.Surfaces[0])
		var b bytes.Buffer
		if e := WriteConical45Supports(&b, m); e == nil || b.Len() != 0 {
			t.Fatal("accepted or partial")
		}
	}
	m := cone45Fixture()
	var b bytes.Buffer
	_ = WriteConical45Supports(&b, m)
	s := b.String()
	for _, change := range [][2]string{{"194,1,3,2.5,45.,5", "194,1,3,2.5,45.,3"}, {"123,1.,0.,0.", "123,2.,0.,0."}} {
		if !strings.Contains(s, change[0]) {
			t.Fatal("stale mutation", change)
		}
		x := strings.Replace(s, change[0], change[1], 1)
		if _, e := ReadConical45Supports(strings.NewReader(x), Limits{}); e == nil {
			t.Fatal("accepted mutation")
		}
	}
}
func TestConical45SupportRetained(t *testing.T) {
	b, e := os.ReadFile("testdata/conical45-support.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadConical45Supports(bytes.NewReader(b), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	s := m.Surfaces[0]
	if m.LengthUnit != .001 || s.Center != (geom.Vec3{X: 2, Y: 3, Z: 4}) || s.Axis != (geom.Vec3{X: 1}) || s.Reference != (geom.Vec3{Z: -1}) || s.Radius != 2.5 {
		t.Fatal(m)
	}
}
func TestOCCTConical45Support(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_IGES_CONE")
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
		m, e := ReadConical45Supports(bytes.NewReader(b), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		var out bytes.Buffer
		if e = WriteConical45Supports(&out, m); e != nil {
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
func FuzzConical45Supports(f *testing.F) {
	var b bytes.Buffer
	_ = WriteConical45Supports(&b, cone45Fixture())
	f.Add(b.Bytes())
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadConical45Supports(bytes.NewReader(b), Limits{MaxBytes: 8192, MaxRecords: 128, MaxEntities: 16, MaxValues: 128})
	})
}

func TestConical45SupportDirectoryAdversaries(t *testing.T) {
	b, e := os.ReadFile("testdata/conical45-support.igs")
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
		got, e := ReadConical45Supports(strings.NewReader(strings.Join(lines, "\n")), Limits{})
		if e == nil || len(got.Surfaces) != 0 {
			t.Fatal("invalid directory accepted", mutation, e)
		}
	}
}

type cone45ShortWriter struct{}

func (cone45ShortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }
func TestConical45SupportWriterFailure(t *testing.T) {
	if e := WriteConical45Supports(cone45ShortWriter{}, cone45Fixture()); e != io.ErrShortWrite {
		t.Fatal(e)
	}
	m := cone45Fixture()
	m.Surfaces = append(m.Surfaces, m.Surfaces[0])
	m.Surfaces[1].Axis = geom.Vec3{}
	var b bytes.Buffer
	if e := WriteConical45Supports(&b, m); e == nil || b.Len() != 0 {
		t.Fatal("non-atomic validation")
	}
}

func TestConical45SupportSignedMapping(t *testing.T) {
	s := cone45Fixture().Surfaces[0]
	g, e := s.Geometry()
	if e != nil {
		t.Fatal(e)
	}
	s.Radius = 99
	for _, v := range []float64{2, -1.25, -5} {
		p, e := g.Eval(0, v)
		if e != nil {
			t.Fatal(e)
		}
		z := v / math.Sqrt(2)
		want := geom.Vec3{X: 2 + z, Y: 3, Z: 4 - (2.5 + z)}
		if p.Sub(want).Length() > 1e-14 {
			t.Fatal(p, want)
		}
		_, dv, e := g.Partials(0, v)
		if e != nil || dv.Sub(geom.Vec3{X: 1 / math.Sqrt(2), Z: -1 / math.Sqrt(2)}).Length() > 1e-14 {
			t.Fatal(dv, e)
		}
	}
	var b bytes.Buffer
	_ = WriteConical45Supports(&b, cone45Fixture())
	text := b.String()
	if !strings.Contains(text, "2.5,45.,5") {
		t.Fatal("stale angle mutation")
	}
	if _, e := ReadConical45Supports(strings.NewReader(strings.Replace(text, "2.5,45.,5", "2.5,30.,5", 1)), Limits{}); e == nil {
		t.Fatal("non45 angle accepted")
	}
}
func TestOCCTConical45OppositeNappe(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_IGES_CONE")
	if bin == "" {
		t.Skip("optional OCCT")
	}
	for _, sign := range []float64{1, -1} {
		m := cone45Fixture()
		m.Surfaces[0].Axis.X = sign
		var b bytes.Buffer
		if e := WriteConical45Supports(&b, m); e != nil {
			t.Fatal(e)
		}
		p := filepath.Join(t.TempDir(), "opposite.igs")
		_ = os.WriteFile(p, b.Bytes(), 0600)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		data, e := exec.CommandContext(ctx, bin, "read-opposite", p).CombinedOutput()
		cancel()
		if e != nil {
			t.Fatal(e, string(data))
		}
		i := bytes.LastIndexByte(data, '{')
		if i < 0 {
			t.Fatal(string(data))
		}
		var got struct{ Point, Du, Dv [3]float64 }
		if e = json.Unmarshal(bytes.TrimSpace(data[i:]), &got); e != nil {
			t.Fatal(e)
		}
		g, _ := m.Surfaces[0].Geometry()
		point, _ := g.Eval(.375, -5)
		du, dv, _ := g.Partials(.375, -5)
		for j, a := range [][3]float64{got.Point, got.Du, got.Dv} {
			want := []geom.Vec3{point, du, dv}[j]
			if (geom.Vec3{X: a[0], Y: a[1], Z: a[2]}).Sub(want).Length() > 1e-12 {
				t.Fatal(got, want)
			}
		}
	}
}
