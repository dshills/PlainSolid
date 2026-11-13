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

func TestOCCTIGESTabulated(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_IGES_TABULATED")
	if oracle == "" {
		t.Skip("optional IGES tabulated helper")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, mode := range []string{"write", "write-negative"} {
		source := filepath.Join(t.TempDir(), "source.igs")
		if b, e := exec.CommandContext(ctx, oracle, mode, source).CombinedOutput(); e != nil {
			t.Fatal(e, string(b))
		}
		b, e := os.ReadFile(source)
		if e != nil {
			t.Fatal(e)
		}
		m, e := ReadTabulatedSurfaces(bytes.NewReader(b), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		g, e := m.TabulatedGeometry(0)
		if e != nil {
			t.Fatal(e)
		}
		var out bytes.Buffer
		if e = WriteTabulatedSurfaces(&out, m); e != nil {
			t.Fatal(e)
		}
		target := filepath.Join(t.TempDir(), "native.igs")
		if e = os.WriteFile(target, out.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		n := 32768
		area := 0.
		for i := 0; i <= n; i++ {
			v := float64(i) / float64(n)
			w := 1 - v + v*v
			h := v + 2*v*v
			dr := (2*v*w - (2+v*v)*(2*v-1)) / (w * w)
			dz := ((1+4*v)*w - h*(2*v-1)) / (w * w)
			factor := 2.
			if i == 0 || i == n {
				factor = 1
			} else if i%2 == 1 {
				factor = 4
			}
			area += factor * math.Sqrt(25*dr*dr+9*dz*dz)
		}
		area /= 3 * float64(n)
		for _, path := range []string{source, target} {
			b, e = exec.CommandContext(ctx, oracle, "read", path).Output()
			if e != nil {
				t.Fatal(e, string(b))
			}
			start := bytes.LastIndexByte(b, '{')
			if start < 0 {
				t.Fatal(string(b))
			}
			var got struct {
				Faces         int
				Area, SampleU float64
				Point, Du, Dv [3]float64
			}
			if e = json.Unmarshal(bytes.TrimSpace(b[start:]), &got); e != nil {
				t.Fatal(e, string(b))
			}
			if got.Faces != 1 || math.Abs(got.Area-area) > 1e-9 {
				t.Fatal(got, area)
			}
			p, _ := g.Eval(got.SampleU, .25)
			du, dv, _ := g.Partials(got.SampleU, .25)
			for i, v := range []geom.Vec3{p, du, dv} {
				q := [][3]float64{got.Point, got.Du, got.Dv}[i]
				if v.Distance(geom.Vec3{X: q[0], Y: q[1], Z: q[2]}) > 1e-10 {
					t.Fatal("source parameter/orientation", i, v, q)
				}
			}
		}
	}
}
