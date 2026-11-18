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

func TestQuarterSectorSTEP(t *testing.T) {
	t.Parallel()
	for _, n := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		for q := 0; q < 4; q++ {
			f, _ := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, n)
			p := QuarterSectorExtrusionBody{Name: "quarter sector", Frame: f, Center: geom.Vec2{X: 2, Y: -3}, Radius: 3, Height: 2, Quarter: q}
			m := QuarterSectorExtrusionModel{Body: p, LengthUnit: .001}
			var b bytes.Buffer
			if e := WriteQuarterSectorExtrusion(&b, m); e != nil {
				t.Fatal(n, q, e)
			}
			got, e := ReadQuarterSectorExtrusion(bytes.NewReader(b.Bytes()), Limits{})
			if e != nil || got != m {
				t.Fatal(n, q, got, e)
			}
			var again bytes.Buffer
			if e = WriteQuarterSectorExtrusion(&again, got); e != nil || !bytes.Equal(b.Bytes(), again.Bytes()) {
				t.Fatal(e, "unstable")
			}
			if os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
				continue
			}
			dir := t.TempDir()
			src, dst := filepath.Join(dir, "sector.step"), filepath.Join(dir, "mass.json")
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
			v := 4.5 * math.Pi
			a := 7.5*math.Pi + 12
			signs := [4][2]float64{{1, 1}, {-1, 1}, {-1, -1}, {1, -1}}
			c := f.ToWorld(geom.Vec3{X: 2 + signs[q][0]*4/math.Pi, Y: -3 + signs[q][1]*4/math.Pi, Z: 1})
			actual := geom.Vec3{X: o.Centroid[0], Y: o.Centroid[1], Z: o.Centroid[2]}
			if math.Abs(o.Volume-v) > 1e-8 || math.Abs(o.Area-a) > 1e-8 || actual.Distance(c) > 1e-8 {
				t.Fatal(o, v, a, c)
			}
		}
	}
}

func TestQuarterSectorSTEPRejectsTampering(t *testing.T) {
	t.Parallel()
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	m := QuarterSectorExtrusionModel{LengthUnit: .001, Body: QuarterSectorExtrusionBody{Name: "sector", Frame: f, Radius: 3, Height: 2}}
	var b bytes.Buffer
	if e := WriteQuarterSectorExtrusion(&b, m); e != nil {
		t.Fatal(e)
	}
	circle := regexp.MustCompile(`CIRCLE\('',#\d+,3\.\)`).FindString(b.String())
	if circle == "" {
		t.Fatal("missing circle")
	}
	changed := strings.Replace(b.String(), circle, strings.Replace(circle, ",3.)", ",2.)", 1), 1)
	if _, e := ReadQuarterSectorExtrusion(strings.NewReader(changed), Limits{}); e == nil {
		t.Fatal("modified circle accepted")
	}
	m.Body.Quarter = 4
	b.Reset()
	if e := WriteQuarterSectorExtrusion(&b, m); e == nil || b.Len() != 0 {
		t.Fatal("unsupported quarter published")
	}
}
