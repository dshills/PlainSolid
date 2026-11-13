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
	for _, mode := range []string{"write", "write-negative"} {
		p := filepath.Join(t.TempDir(), "source.igs")
		run(mode, p)
		b, _ := os.ReadFile(p)
		m, e := ReadAnalyticSupports(bytes.NewReader(b), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		var out bytes.Buffer
		if e = WriteAnalyticSupports(&out, m); e != nil {
			t.Fatal(e)
		}
		q := filepath.Join(t.TempDir(), "native.igs")
		_ = os.WriteFile(q, out.Bytes(), 0600)
		for _, path := range []string{p, q} {
			data := run("read", path)
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
			if len(got.Samples) != 4 {
				t.Fatal(got)
			}
			for i, a := range got.Samples {
				g, e := mixedGeometry(m.Surfaces[i])
				if e != nil {
					t.Fatal(e)
				}
				point, _ := g.Eval(.375, -1.25)
				du, dv, _ := g.Partials(.375, -1.25)
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
