package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/geom"
	"strings"
	"testing"
	"time"
)

func planeFixture() PlanarSupportModel {
	return PlanarSupportModel{LengthUnit: .001, Surfaces: []PlanarSupport{{Location: geom.Vec3{X: 2, Y: 3, Z: 4}, Normal: geom.Vec3{X: 1}, Reference: geom.Vec3{Z: -1}, Metadata: PlaneDatumMetadata{"PLANE", 7}, NormalMetadata: PlaneDatumMetadata{"AXIS", 1}}}}
}
func TestPlanarSupportRoundtrip(t *testing.T) {
	for _, sign := range []float64{1, -1} {
		m := planeFixture()
		m.Surfaces[0].Normal.X = sign
		if sign < 0 {
			m.LengthUnit = .3048
		}
		var b bytes.Buffer
		if e := WritePlanarSupports(&b, m); e != nil {
			t.Fatal(e)
		}
		got, e := ReadPlanarSupports(bytes.NewReader(b.Bytes()), Limits{})
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
		p, _ := g.Eval(0, -1.25)
		if p != (geom.Vec3{X: 2, Y: 3 - sign*1.25, Z: 4}) {
			t.Fatal(p)
		}
		var c bytes.Buffer
		if e = WritePlanarSupports(&c, got); e != nil || !bytes.Equal(b.Bytes(), c.Bytes()) {
			t.Fatal(e, "bytes")
		}
		if _, e = Read(bytes.NewReader(b.Bytes()), Limits{}); e == nil {
			t.Fatal("generic accepted")
		}
		for _, l := range []Limits{{MaxEntities: 3}, {MaxBytes: 100}, {MaxValues: 4}, {MaxRecords: 2}} {
			if _, e = ReadPlanarSupports(bytes.NewReader(b.Bytes()), l); e == nil {
				t.Fatal("limit")
			}
		}
	}
}
func TestPlanarSupportRejection(t *testing.T) {
	for _, mut := range []func(*PlanarSupport){func(s *PlanarSupport) { s.Normal.X = 2 }, func(s *PlanarSupport) { s.Reference = s.Normal }, func(s *PlanarSupport) { s.Location.X = .1 }, func(s *PlanarSupport) { s.Metadata.Label = " padded" }} {
		m := planeFixture()
		mut(&m.Surfaces[0])
		var b bytes.Buffer
		if e := WritePlanarSupports(&b, m); e == nil || b.Len() != 0 {
			t.Fatal("accepted or partial")
		}
	}
	m := planeFixture()
	var b bytes.Buffer
	_ = WritePlanarSupports(&b, m)
	s := b.String()
	for _, change := range [][2]string{{"190,1,3,5", "190,1,3,3"}, {"123,1.,0.,0.", "123,2.,0.,0."}} {
		if !strings.Contains(s, change[0]) {
			t.Fatal("stale mutation", change)
		}
		x := strings.Replace(s, change[0], change[1], 1)
		if _, e := ReadPlanarSupports(strings.NewReader(x), Limits{}); e == nil {
			t.Fatal("accepted mutation")
		}
	}
}
func TestPlanarSupportRetained(t *testing.T) {
	b, e := os.ReadFile("testdata/planar-support.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadPlanarSupports(bytes.NewReader(b), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	s := m.Surfaces[0]
	if m.LengthUnit != .001 || s.Location != (geom.Vec3{X: 2, Y: 3, Z: 4}) || s.Normal != (geom.Vec3{X: 1}) || s.Reference != (geom.Vec3{Z: -1}) {
		t.Fatal(m)
	}
}
func TestOCCTPlanarSupport(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_IGES_PLANE")
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
		m, e := ReadPlanarSupports(bytes.NewReader(b), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		var out bytes.Buffer
		if e = WritePlanarSupports(&out, m); e != nil {
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
func FuzzPlanarSupports(f *testing.F) {
	var b bytes.Buffer
	_ = WritePlanarSupports(&b, planeFixture())
	f.Add(b.Bytes())
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadPlanarSupports(bytes.NewReader(b), Limits{MaxBytes: 8192, MaxRecords: 128, MaxEntities: 16, MaxValues: 128})
	})
}

func TestPlanarSupportDirectoryAdversaries(t *testing.T) {
	b, e := os.ReadFile("testdata/planar-support.igs")
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
		got, e := ReadPlanarSupports(strings.NewReader(strings.Join(lines, "\n")), Limits{})
		if e == nil || len(got.Surfaces) != 0 {
			t.Fatal("invalid directory accepted", mutation, e)
		}
	}
}

type planeShortWriter struct{}

func (planeShortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }
func TestPlanarSupportWriterFailure(t *testing.T) {
	if e := WritePlanarSupports(planeShortWriter{}, planeFixture()); e != io.ErrShortWrite {
		t.Fatal(e)
	}
	m := planeFixture()
	m.Surfaces = append(m.Surfaces, m.Surfaces[0])
	m.Surfaces[1].Normal = geom.Vec3{}
	var b bytes.Buffer
	if e := WritePlanarSupports(&b, m); e == nil || b.Len() != 0 {
		t.Fatal("non-atomic validation")
	}
}
