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

func cylinderFixture() CylindricalSurfaceModel {
	return CylindricalSurfaceModel{LengthUnit: .001, Surfaces: []CylindricalSurface{{Location: geom.Vec3{X: 2, Y: 3, Z: 4}, Axis: geom.Vec3{X: 1}, Reference: geom.Vec3{Z: -1}, Radius: 2.5, Metadata: CylinderDatumMetadata{"CYLINDER", 7}, AxisMetadata: CylinderDatumMetadata{"AXIS", 1}}}}
}
func TestCylindricalSurfaceRoundtrip(t *testing.T) {
	for _, sign := range []float64{1, -1} {
		m := cylinderFixture()
		m.Surfaces[0].Axis.X = sign
		var b bytes.Buffer
		if e := WriteCylindricalSurfaces(&b, m); e != nil {
			t.Fatal(e)
		}
		got, e := ReadCylindricalSurfaces(bytes.NewReader(b.Bytes()), Limits{})
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
		if p != (geom.Vec3{X: 2 - sign*1.25, Y: 3, Z: 1.5}) {
			t.Fatal(p)
		}
		var c bytes.Buffer
		if e = WriteCylindricalSurfaces(&c, got); e != nil || !bytes.Equal(b.Bytes(), c.Bytes()) {
			t.Fatal(e, "bytes")
		}
		if _, e = Read(bytes.NewReader(b.Bytes()), Limits{}); e == nil {
			t.Fatal("generic accepted")
		}
		for _, l := range []Limits{{MaxEntities: 3}, {MaxBytes: 100}, {MaxValues: 4}, {MaxRecords: 2}} {
			if _, e = ReadCylindricalSurfaces(bytes.NewReader(b.Bytes()), l); e == nil {
				t.Fatal("limit")
			}
		}
	}
}
func TestCylindricalSurfaceRejection(t *testing.T) {
	for _, mut := range []func(*CylindricalSurface){func(s *CylindricalSurface) { s.Axis.X = 2 }, func(s *CylindricalSurface) { s.Reference = s.Axis }, func(s *CylindricalSurface) { s.Radius = 0 }, func(s *CylindricalSurface) { s.Location.X = .1 }, func(s *CylindricalSurface) { s.Metadata.Label = " padded" }} {
		m := cylinderFixture()
		mut(&m.Surfaces[0])
		var b bytes.Buffer
		if e := WriteCylindricalSurfaces(&b, m); e == nil || b.Len() != 0 {
			t.Fatal("accepted or partial")
		}
	}
	m := cylinderFixture()
	var b bytes.Buffer
	_ = WriteCylindricalSurfaces(&b, m)
	s := b.String()
	for _, change := range [][2]string{{"192,1,3,2.5,5", "192,1,3,2.5,3"}, {"123,1.,0.,0.", "123,2.,0.,0."}} {
		if !strings.Contains(s, change[0]) {
			t.Fatal("stale mutation", change)
		}
		x := strings.Replace(s, change[0], change[1], 1)
		if _, e := ReadCylindricalSurfaces(strings.NewReader(x), Limits{}); e == nil {
			t.Fatal("accepted mutation")
		}
	}
}
func TestCylindricalSurfaceRetained(t *testing.T) {
	b, e := os.ReadFile("testdata/cylindrical-surface.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadCylindricalSurfaces(bytes.NewReader(b), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	s := m.Surfaces[0]
	if m.LengthUnit != .001 || s.Location != (geom.Vec3{X: 2, Y: 3, Z: 4}) || s.Axis != (geom.Vec3{X: 1}) || s.Reference != (geom.Vec3{Z: -1}) || s.Radius != 2.5 {
		t.Fatal(m)
	}
}
func TestOCCTCylindricalSurface(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_IGES_CYLINDER")
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
		m, e := ReadCylindricalSurfaces(bytes.NewReader(b), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		var out bytes.Buffer
		if e = WriteCylindricalSurfaces(&out, m); e != nil {
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
func FuzzCylindricalSurfaces(f *testing.F) {
	var b bytes.Buffer
	_ = WriteCylindricalSurfaces(&b, cylinderFixture())
	f.Add(b.Bytes())
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadCylindricalSurfaces(bytes.NewReader(b), Limits{MaxBytes: 8192, MaxRecords: 128, MaxEntities: 16, MaxValues: 128})
	})
}

func TestCylindricalSurfaceDirectoryAdversaries(t *testing.T) {
	b, e := os.ReadFile("testdata/cylindrical-surface.igs")
	if e != nil {
		t.Fatal(e)
	}
	for _, mutation := range []struct {
		line, field int
		value       string
	}{{6, 4, "       0"}, {5, 6, "       3"}, {7, 8, "00010300"}, {5, 0, "     190"}} {
		lines := strings.Split(string(b), "\n")
		s := lines[mutation.line]
		a := mutation.field * 8
		lines[mutation.line] = s[:a] + mutation.value + s[a+8:]
		got, e := ReadCylindricalSurfaces(strings.NewReader(strings.Join(lines, "\n")), Limits{})
		if e == nil || len(got.Surfaces) != 0 {
			t.Fatal("invalid directory accepted", mutation, e)
		}
	}
}

type cylinderShortWriter struct{}

func (cylinderShortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }
func TestCylindricalSurfaceWriterFailure(t *testing.T) {
	if e := WriteCylindricalSurfaces(cylinderShortWriter{}, cylinderFixture()); e != io.ErrShortWrite {
		t.Fatal(e)
	}
	m := cylinderFixture()
	m.Surfaces = append(m.Surfaces, m.Surfaces[0])
	m.Surfaces[1].Axis = geom.Vec3{}
	var b bytes.Buffer
	if e := WriteCylindricalSurfaces(&b, m); e == nil || b.Len() != 0 {
		t.Fatal("non-atomic validation")
	}
}
