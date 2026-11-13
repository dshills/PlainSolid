package step

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

func checkRevolutionOCCT(t *testing.T, raw []byte, curved bool) {
	t.Helper()
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	found := 0
	for _, id := range f.Order {
		if _, ok := component(f.Entities[id], "RECTANGULAR_TRIMMED_SURFACE"); !ok {
			continue
		}
		s, e := DecodeRevolutionSurface(f, id, 64)
		if e != nil {
			t.Fatal(e)
		}
		g, e := s.Surface()
		if e != nil {
			t.Fatal(e)
		}
		p, e := g.Eval(.5, .5)
		r, z := 2.5, 1.5
		if curved {
			r, z = 3.25, 1.25
		}
		if e != nil || p.Distance(geom.Vec3{X: r * math.Cos(.5), Y: r * math.Sin(.5), Z: z}) > 1e-12 {
			t.Fatal(p, e)
		}
		found++
	}
	if found != 1 {
		t.Fatal(found)
	}
	if _, e = ReadRevolutionSurfaces(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("face topology silently imported")
	}
}
func TestRevolutionSurfaceRetainedOCCT(t *testing.T) {
	for _, curved := range []bool{false, true} {
		name := "revolution-surface-occt.step"
		if curved {
			name = "revolution-curved-occt.step"
		}
		raw, e := os.ReadFile(filepath.Join("testdata", name))
		if e != nil {
			t.Fatal(e)
		}
		checkRevolutionOCCT(t, raw, curved)
	}
}
func TestOCCTRevolutionSurfaces(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_REVOLUTION_SURFACE")
	if oracle == "" {
		t.Skip("set PLAINSOLID_OCCT_REVOLUTION_SURFACE to standalone helper")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, curved := range []bool{false, true} {
		mode := "write"
		if curved {
			mode = "write-curved"
		}
		source := filepath.Join(t.TempDir(), "source.step")
		if b, e := exec.CommandContext(ctx, oracle, mode, source).CombinedOutput(); e != nil {
			t.Fatal(e, string(b))
		}
		raw, e := os.ReadFile(source)
		if e != nil {
			t.Fatal(e)
		}
		checkRevolutionOCCT(t, raw, curved)
		m, e := ReadRevolutionSurfaces(bytes.NewReader(revolutionFixture(t)), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if !curved {
			c, e := geom.NewBSpline(1, []geom.Vec3{{X: 2}, {X: 3, Z: 3}}, []float64{0, 0, 1, 1}, nil)
			if e != nil {
				t.Fatal(e)
			}
			m.Surfaces[0].Profile.Geometry = c
		}
		output := filepath.Join(t.TempDir(), "native.step")
		file, e := os.Create(output)
		if e != nil {
			t.Fatal(e)
		}
		e = WriteRevolutionSurfaces(file, m)
		ce := file.Close()
		if e != nil || ce != nil {
			t.Fatal(e, ce)
		}
		b, e := exec.CommandContext(ctx, oracle, "read", output).CombinedOutput()
		if e != nil {
			t.Fatal(e, string(b))
		}
		var got struct {
			Faces         int
			Area          float64
			Point, Du, Dv [3]float64
		}
		if e = json.Unmarshal(bytes.TrimSpace(b), &got); e != nil {
			t.Fatal(e, string(b))
		}
		want := 2.5 * math.Sqrt(10)
		if curved {
			want = math.Sqrt(10) * (43*math.Sqrt(2) + 55*math.Asinh(1)) / 32
		}
		if got.Faces != 1 || math.Abs(got.Area-want) > 1e-7 {
			t.Fatal(got, want)
		}
		g, e := m.Surfaces[0].Surface()
		if e != nil {
			t.Fatal(e)
		}
		p, _ := g.Eval(.5, .5)
		du, dv, _ := g.Partials(.5, .5)
		for i, v := range []geom.Vec3{p, du, dv} {
			q := [][3]float64{got.Point, got.Du, got.Dv}[i]
			if v.Distance(geom.Vec3{X: q[0], Y: q[1], Z: q[2]}) > 1e-11 {
				t.Fatal("parameter/derivative mismatch", i, v, q)
			}
		}
	}
}
