package iges

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/geom"
	"testing"
)

func TestOCCTQuarterEllipseConic(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_IGES_QUARTER_ELLIPSE_CONIC")
	if bin == "" {
		t.Skip("optional pinned OCCT helper")
	}
	verify := func(file string, m QuarterEllipseConic) {
		t.Helper()
		data, e := exec.Command(bin, "read", file).CombinedOutput()
		if e != nil {
			t.Fatal(e, string(data))
		}
		i := bytes.LastIndexByte(data, '{')
		if i < 0 {
			t.Fatal(string(data))
		}
		var v struct {
			First, Last                   float64
			Point, Derivative, Start, End [3]float64
		}
		if e = json.Unmarshal(bytes.TrimSpace(data[i:]), &v); e != nil {
			t.Fatal(e)
		}
		vec := func(a [3]float64) geom.Vec3 { return geom.Vec3{X: a[0], Y: a[1], Z: a[2]} }
		lo, hi, _ := m.AngularRange()
		g, e := m.Curve.Geometry()
		if e != nil {
			t.Fatal(e)
		}
		p, _ := g.Eval(lo + .375)
		d, _ := g.Derivative(lo + .375)
		start, _ := g.Eval(lo)
		end, _ := g.Eval(hi)
		if math.Abs(v.First-lo) > 1e-12 || math.Abs(v.Last-hi) > 1e-12 || vec(v.Point).Distance(p) > 1e-12 || vec(v.Derivative).Distance(d) > 1e-12 || vec(v.Start).Distance(start) > 1e-12 || vec(v.End).Distance(end) > 1e-12 {
			t.Fatal(v, m.StartQuarter)
		}
	}
	for _, file := range []string{"occt-quarter-ellipse-conic.igs", "occt-reverse-quarter-ellipse-conic.igs"} {
		source := "testdata/" + file
		raw, e := os.ReadFile(source)
		if e != nil {
			t.Fatal(e)
		}
		m, e := ReadQuarterEllipseConic(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		verify(source, m)
		for q := 0; q < 4; q++ {
			m.StartQuarter = q
			var out bytes.Buffer
			if e = WriteQuarterEllipseConic(&out, m); e != nil {
				t.Fatal(e)
			}
			path := filepath.Join(t.TempDir(), "quarter.igs")
			if e = os.WriteFile(path, out.Bytes(), 0600); e != nil {
				t.Fatal(e)
			}
			verify(path, m)
		}
	}
}
