package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/geom"
	"testing"
	"time"
)

func mixedGeometry(s AnalyticSupport) (geom.Surface, error) {
	switch {
	case s.Plane != nil:
		return s.Plane.Geometry()
	case s.Cylinder != nil:
		return s.Cylinder.Geometry()
	case s.Sphere != nil:
		return s.Sphere.Geometry()
	case s.Cone45 != nil:
		return s.Cone45.Geometry()
	default:
		return s.Torus.Geometry()
	}
}
func TestAnalyticSupportsRetained(t *testing.T) {
	b, e := os.ReadFile("testdata/analytic-supports.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticSupports(bytes.NewReader(b), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if m.LengthUnit != .001 || len(m.Surfaces) != 4 || m.Surfaces[0].Torus == nil || m.Surfaces[1].Plane == nil || m.Surfaces[2].Sphere == nil || m.Surfaces[3].Cylinder == nil {
		t.Fatal(m)
	}
}
func TestOCCTAnalyticSupports(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_IGES_ANALYTIC_SUPPORTS")
	if bin == "" {
		t.Skip("optional independent OCCT")
	}
	run := func(mode, path string) []byte {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		b, e := exec.CommandContext(ctx, bin, mode, path).CombinedOutput()
		if e != nil {
			t.Fatal(e, string(b))
		}
		return b
	}
	for _, mode := range []string{"write", "write-negative", "write-with-cone", "write-with-cone-negative"} {
		read, write := ReadAnalyticSupports, WriteAnalyticSupports
		modes, count := []string{"read"}, 4
		if mode == "write-with-cone" || mode == "write-with-cone-negative" {
			read, write = ReadAnalyticSupportsWithCones, WriteAnalyticSupportsWithCones
			modes, count = []string{"read-with-cone", "read-with-cone-opposite"}, 5
		}
		p := filepath.Join(t.TempDir(), "source.igs")
		run(mode, p)
		b, _ := os.ReadFile(p)
		m, e := read(bytes.NewReader(b), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		var out bytes.Buffer
		if e = write(&out, m); e != nil {
			t.Fatal(e)
		}
		q := filepath.Join(t.TempDir(), "native.igs")
		_ = os.WriteFile(q, out.Bytes(), 0600)
		for _, path := range []string{p, q} {
			for _, readMode := range modes {
				data := run(readMode, path)
				i := bytes.Index(data, []byte("{\"Samples\""))
				if i < 0 {
					t.Fatal(string(data))
				}
				var got struct {
					Samples []struct{ Point, Du, Dv [3]float64 }
				}
				if e = json.Unmarshal(bytes.TrimSpace(data[i:]), &got); e != nil {
					t.Fatal(e)
				}
				if len(got.Samples) != count {
					t.Fatal(got)
				}
				for i, a := range got.Samples {
					g, e := mixedGeometry(m.Surfaces[i])
					if e != nil {
						t.Fatal(e)
					}
					v := -1.25
					if m.Surfaces[i].Cone45 != nil && readMode == "read-with-cone-opposite" {
						v = -5
					}
					point, _ := g.Eval(.375, v)
					du, dv, _ := g.Partials(.375, v)
					for j, x := range [][3]float64{a.Point, a.Du, a.Dv} {
						want := []geom.Vec3{point, du, dv}[j]
						if (geom.Vec3{X: x[0], Y: x[1], Z: x[2]}).Sub(want).Length() > 1e-12 {
							t.Fatal(i, a, want)
						}
					}
				}
			}
		}
	}
}

func TestAnalyticSupportsWithCones(t *testing.T) {
	source, err := os.ReadFile("testdata/occt-analytic-supports-with-cone.igs")
	if err != nil {
		t.Fatal(err)
	}
	if old, err := ReadAnalyticSupports(bytes.NewReader(source), Limits{}); err == nil || len(old.Surfaces) != 0 {
		t.Fatal("legacy IGES mode accepted the cone", err)
	}
	m, err := ReadAnalyticSupportsWithCones(bytes.NewReader(source), Limits{})
	if err != nil || len(m.Surfaces) != 5 || m.Surfaces[4].Cone45 == nil || m.LengthUnit != .001 {
		t.Fatal("mixed original source order and units", m, err)
	}
	cone := m.Surfaces[4].Cone45
	if cone.Center != (geom.Vec3{X: 7, Y: -2, Z: 3}) || cone.Radius != 2.5 {
		t.Fatal("original cone parameters", cone)
	}
	cone.Metadata = ConeDatumMetadata{"cone", 7}
	cone.AxisMetadata = ConeDatumMetadata{"axis", 2}
	var staged bytes.Buffer
	if err := WriteAnalyticSupports(&staged, m); err == nil || staged.Len() != 0 {
		t.Fatal("legacy writer failure must be atomic", err)
	}
	if err := WriteAnalyticSupportsWithCones(&staged, m); err != nil {
		t.Fatal(err)
	}
	out, err := ReadAnalyticSupportsWithCones(bytes.NewReader(staged.Bytes()), Limits{})
	if err != nil || len(out.Surfaces) != 5 || *out.Surfaces[4].Cone45 != *cone {
		t.Fatal("mixed cone coefficient and metadata round trip", out, err)
	}
	bad := bytes.Replace(source, []byte(",45.,"), []byte(",44.,"), 1)
	if bytes.Equal(bad, source) {
		t.Fatal("negative case did not change the angle field")
	}
	if out, err := ReadAnalyticSupportsWithCones(bytes.NewReader(bad), Limits{}); err == nil || len(out.Surfaces) != 0 {
		t.Fatal("unqualified angle must reject atomically", err)
	}
	m.Surfaces[4].Plane = m.Surfaces[1].Plane
	staged.Reset()
	if err := WriteAnalyticSupportsWithCones(&staged, m); err == nil || staged.Len() != 0 {
		t.Fatal("ambiguous mixed variant must reject atomically", err)
	}
}
