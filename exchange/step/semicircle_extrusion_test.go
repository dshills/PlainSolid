package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"plainsolid/geom"
	"regexp"
	"strings"
	"testing"
)

func TestSemicircleSTEP(t *testing.T) {
	t.Parallel()
	for _, n := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		for _, radius := range []float64{3, 2.5} {
			f, _ := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, n)
			p := SemicircleExtrusionBody{Name: "semicircle", Frame: f, Center: geom.Vec2{X: 2.25, Y: -3.5}, Radius: radius, Height: 2}
			m := SemicircleExtrusionModel{Body: p, LengthUnit: .001}
			var b bytes.Buffer
			if e := WriteSemicircleExtrusion(&b, m); e != nil {
				t.Fatal(n, radius, e)
			}
			got, e := ReadSemicircleExtrusion(bytes.NewReader(b.Bytes()), Limits{})
			if e != nil || got != m {
				t.Fatal(n, got, e)
			}
			var again bytes.Buffer
			if e = WriteSemicircleExtrusion(&again, got); e != nil || !bytes.Equal(b.Bytes(), again.Bytes()) {
				t.Fatal(e, "unstable")
			}
			if os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
				continue
			}
			dir := t.TempDir()
			src, dst := filepath.Join(dir, "semicircle.step"), filepath.Join(dir, "mass.json")
			if e = os.WriteFile(src, b.Bytes(), 0600); e != nil {
				t.Fatal(e)
			}
			stepOracle(t, dst, "step-mass-precise", src)
			raw, e := os.ReadFile(dst)
			if e != nil {
				t.Fatal(e)
			}
			var o struct {
				Volume, Area float64
				Centroid     [3]float64
			}
			if e = json.Unmarshal(raw, &o); e != nil {
				t.Fatal(e)
			}
			v := math.Pi * radius * radius
			a := math.Pi*radius*radius + 2*(math.Pi*radius+2*radius)
			c := f.ToWorld(geom.Vec3{X: p.Center.X, Y: p.Center.Y + 4*radius/(3*math.Pi), Z: 1})
			actual := geom.Vec3{X: o.Centroid[0], Y: o.Centroid[1], Z: o.Centroid[2]}
			if math.Abs(o.Volume-v) > 1e-8 || math.Abs(o.Area-a) > 1e-8 || actual.Distance(c) > 1e-8 {
				t.Fatal(o, v, a, c)
			}
		}
	}
}
func TestSemicircleSTEPRejectsTampering(t *testing.T) {
	t.Parallel()
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	m := SemicircleExtrusionModel{LengthUnit: .001, Body: SemicircleExtrusionBody{Name: "half disk", Frame: f, Radius: 3, Height: 2}}
	var b bytes.Buffer
	if e := WriteSemicircleExtrusion(&b, m); e != nil {
		t.Fatal(e)
	}
	circle := regexp.MustCompile(`CIRCLE\('',#\d+,3\.\)`).FindString(b.String())
	if circle == "" {
		t.Fatal("missing circle")
	}
	changed := strings.Replace(b.String(), circle, strings.Replace(circle, ",3.)", ",2.)", 1), 1)
	if _, e := ReadSemicircleExtrusion(strings.NewReader(changed), Limits{}); e == nil {
		t.Fatal("modified circle accepted")
	}
	m.Body.Height = -2
	b.Reset()
	if e := WriteSemicircleExtrusion(&b, m); e == nil || b.Len() != 0 {
		t.Fatal("negative canonical height accepted")
	}
}
