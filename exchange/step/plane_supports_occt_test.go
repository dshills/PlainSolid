package step

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

func TestPlaneSupportsRetained(t *testing.T) {
	b, e := os.ReadFile("testdata/plane-support.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadPlaneSupports(bytes.NewReader(b), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if m.LengthUnit != .001 || len(m.Surfaces) != 1 || m.Surfaces[0].Normal != (geom.Vec3{X: 1}) {
		t.Fatal(m)
	}
}
func TestOCCTPlaneSupports(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_PLANE_SUPPORT")
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
		p := filepath.Join(t.TempDir(), "source.step")
		run(mode, p)
		b, _ := os.ReadFile(p)
		m, e := ReadPlaneSupports(bytes.NewReader(b), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		var out bytes.Buffer
		if e = WritePlaneSupports(&out, m); e != nil {
			t.Fatal(e)
		}
		q := filepath.Join(t.TempDir(), "native.step")
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
