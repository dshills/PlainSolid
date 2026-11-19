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

func obliqueRevolutionFixture(t *testing.T) []byte {
	b := revolutionFixture(t)
	for _, v := range [][2]string{{"(2.,0.,0.)", "(6.,8.,0.)"}, {"(4.,0.,1.)", "(12.,16.,1.)"}, {"(3.,0.,3.)", "(9.,12.,3.)"}} {
		b = bytes.ReplaceAll(b, []byte(v[0]), []byte(v[1]))
	}
	return b
}
func TestRevolutionSurfaceObliqueSource(t *testing.T) {
	t.Parallel()
	b := obliqueRevolutionFixture(t)
	m, e := ReadRevolutionSurfaces(bytes.NewReader(b), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if m.Surfaces[0].Profile.Geometry.Poles()[1].Y != 16 {
		t.Fatal("fixture mutation missed")
	}
	var out bytes.Buffer
	if e = WriteRevolutionSurfaces(&out, m); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadRevolutionSurfaces(&out, Limits{}); e != nil {
		t.Fatal(e)
	}
	for _, v := range []string{"(12.,16.00000000000000000001,1.)", "(-12.,-16.,1.)"} {
		bad := bytes.ReplaceAll(b, []byte("(12.,16.,1.)"), []byte(v))
		if _, e = ReadRevolutionSurfaces(bytes.NewReader(bad), Limits{}); e == nil {
			t.Fatal("accepted", v)
		}
	}
}
func TestOCCTRevolutionOblique(t *testing.T) {
	t.Parallel()
	oracle := os.Getenv("PLAINSOLID_OCCT_REVOLUTION_SURFACE")
	if oracle == "" {
		t.Skip("optional oracle")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "source.step")
	if b, e := exec.CommandContext(ctx, oracle, "write-oblique", path).CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
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
		if s.Profile.Geometry.Poles()[1] != (geom.Vec3{X: 12, Y: 16, Z: 1}) {
			t.Fatal("source phase lost")
		}
		found++
	}
	if found != 1 {
		t.Fatal(found)
	}
	m, e := ReadRevolutionSurfaces(bytes.NewReader(obliqueRevolutionFixture(t)), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var output bytes.Buffer
	if e = WriteRevolutionSurfaces(&output, m); e != nil {
		t.Fatal(e)
	}
	path = filepath.Join(t.TempDir(), "native.step")
	if e = os.WriteFile(path, output.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	b, e := exec.CommandContext(ctx, oracle, "read", path).Output()
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
	// Independent polynomial formula: radius=5(2+4v-3v²), height=2v+v².
	n := 32768
	area := 0.
	for i := 0; i <= n; i++ {
		v := float64(i) / float64(n)
		w := 2.
		if i == 0 || i == n {
			w = 1
		} else if i%2 == 1 {
			w = 4
		}
		area += w * 5 * (2 + 4*v - 3*v*v) * math.Sqrt(25*(4-6*v)*(4-6*v)+(2+2*v)*(2+2*v))
	}
	area /= 3 * float64(n)
	if got.Faces != 1 || math.Abs(got.Area-area) > 1e-7 {
		t.Fatal(got.Area, area)
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

func TestRevolutionSurfaceObliqueRetainedOCCT(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/revolution-oblique-occt.step")
	if e != nil {
		t.Fatal(e)
	}
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
		if s.Profile.Geometry.Poles()[1] != (geom.Vec3{X: 12, Y: 16, Z: 1}) || s.AngleStart != .25 || s.AngleEnd != 1.25 {
			t.Fatal("source phase/range lost")
		}
		found++
	}
	if found != 1 {
		t.Fatal(found)
	}
}
