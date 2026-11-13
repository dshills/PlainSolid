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

func rationalRevolutionFixture(t *testing.T) RevolutionSurfaceModel {
	m, e := ReadRevolutionSurfaces(bytes.NewReader(revolutionFixture(t)), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	c := m.Surfaces[0].Profile.Geometry
	c, e = geom.NewBSpline(c.Degree(), c.Poles(), c.Knots(), []float64{1, .5, 1})
	if e != nil {
		t.Fatal(e)
	}
	m.Surfaces[0].Profile.Geometry = c
	return m
}
func TestRevolutionSurfaceRationalSource(t *testing.T) {
	m := rationalRevolutionFixture(t)
	var b bytes.Buffer
	if e := WriteRevolutionSurfaces(&b, m); e != nil {
		t.Fatal(e)
	}
	raw := b.Bytes()
	got, e := ReadRevolutionSurfaces(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if got.Surfaces[0].Profile.Geometry.Weights()[1] != .5 {
		t.Fatal("weight lost")
	}
	scaled := bytes.Replace(raw, []byte("RATIONAL_B_SPLINE_CURVE((1.,0.5,1.))"), []byte("RATIONAL_B_SPLINE_CURVE((2.,1.,2.))"), 1)
	if bytes.Equal(raw, scaled) {
		t.Fatal("mutation missed", string(raw))
	}
	if _, e = ReadRevolutionSurfaces(bytes.NewReader(scaled), Limits{}); e != nil {
		t.Fatal(e)
	}
	bad := bytes.Replace(raw, []byte("0.5,1.))"), []byte("0.50000000000000000001,1.))"), 1)
	if bytes.Equal(raw, bad) {
		t.Fatal("mutation missed")
	}
	if _, e = ReadRevolutionSurfaces(bytes.NewReader(bad), Limits{}); e == nil {
		t.Fatal("rounded raw weight accepted")
	}
}
func TestOCCTRevolutionRational(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_REVOLUTION_SURFACE")
	if oracle == "" {
		t.Skip("optional oracle")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	source := filepath.Join(t.TempDir(), "source.step")
	if b, e := exec.CommandContext(ctx, oracle, "write-rational", source).CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	b, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(b), Limits{})
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
		if s.Profile.Geometry.Weights()[1] != .5 {
			t.Fatal("source weight")
		}
		found++
	}
	if found != 1 {
		t.Fatal(found)
	}
	m := rationalRevolutionFixture(t)
	var out bytes.Buffer
	if e = WriteRevolutionSurfaces(&out, m); e != nil {
		t.Fatal(e)
	}
	target := filepath.Join(t.TempDir(), "output.step")
	if e = os.WriteFile(target, out.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	n := 32768
	area := 0.
	for i := 0; i <= n; i++ {
		v := float64(i) / float64(n)
		w := 1 - v + v*v
		h := v + 2*v*v
		r := (2 + v*v) / w
		dr := (2*v*w - (2+v*v)*(2*v-1)) / (w * w)
		dz := ((1+4*v)*w - h*(2*v-1)) / (w * w)
		factor := 2.
		if i == 0 || i == n {
			factor = 1
		} else if i%2 == 1 {
			factor = 4
		}
		area += factor * r * math.Hypot(dr, dz)
	}
	area /= 3 * float64(n)
	for _, path := range []string{source, target} {
		b, e = exec.CommandContext(ctx, oracle, "read", path).Output()
		if e != nil {
			t.Fatal(e)
		}
		var got struct {
			Faces         int
			Area          float64
			Point, Du, Dv [3]float64
		}
		if e = json.Unmarshal(bytes.TrimSpace(b), &got); e != nil {
			t.Fatal(e, string(b))
		}
		if got.Faces != 1 || math.Abs(got.Area-area) > 1e-9 {
			t.Fatal(got, area)
		}
		g, _ := m.Surfaces[0].Surface()
		p, _ := g.Eval(.5, .5)
		du, dv, _ := g.Partials(.5, .5)
		for i, v := range []geom.Vec3{p, du, dv} {
			q := [][3]float64{got.Point, got.Du, got.Dv}[i]
			if v.Distance(geom.Vec3{X: q[0], Y: q[1], Z: q[2]}) > 1e-11 {
				t.Fatal(v, q)
			}
		}
	}
}

func TestRevolutionSurfaceRationalRetainedOCCT(t *testing.T) {
	b, e := os.ReadFile("testdata/revolution-rational-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(b), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	count := 0
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
		if e != nil || math.Abs(p.Z-4./3) > 1e-14 || s.Profile.Geometry.Weights()[1] != .5 {
			t.Fatal(p, e)
		}
		count++
	}
	if count != 1 {
		t.Fatal(count)
	}
}
