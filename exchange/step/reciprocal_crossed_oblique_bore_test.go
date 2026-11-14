package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"plainsolid/geom"
)

func TestReciprocalCrossedObliqueBoreSTEP(t *testing.T) {
	for _, n := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		f, err := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, n)
		if err != nil {
			t.Fatal(err)
		}
		p := CrossedObliqueBoreBody{Name: "cardinal and oblique crossed bores", Frame: f, Dimensions: geom.Vec3{X: 20, Y: 16, Z: 40},
			Crossing: geom.Vec3{X: 10, Y: 8, Z: 20}, Radius: 2, AxisRatio: geom.Vec2{X: 3, Y: 4}}
		m := CrossedObliqueBoreModel{Body: p, LengthUnit: .001}
		var out bytes.Buffer
		if err := WriteCrossedObliqueBore(&out, m); err != nil {
			t.Fatal(n, err)
		}
		if strings.Contains(out.String(), "B_SPLINE_CURVE_WITH_KNOTS") || strings.Count(out.String(), "ELLIPSE(") != 16 {
			t.Fatal("expected eight opening and eight crossing ellipses")
		}
		got, err := ReadCrossedObliqueBore(bytes.NewReader(out.Bytes()), Limits{})
		if err != nil || got != m {
			t.Fatal(n, got, err)
		}
		var again bytes.Buffer
		if err := WriteCrossedObliqueBore(&again, got); err != nil || !bytes.Equal(out.Bytes(), again.Bytes()) {
			t.Fatal(n, err)
		}
		if os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
			continue
		}
		dir := t.TempDir()
		src, dest := filepath.Join(dir, "crossed-oblique.step"), filepath.Join(dir, "mass.json")
		if err := os.WriteFile(src, out.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		stepOracle(t, dest, "step-mass-precise", src)
		raw, err := os.ReadFile(dest)
		if err != nil {
			t.Fatal(err)
		}
		var o struct {
			Volume, Area float64
			Centroid     [3]float64
		}
		if err := json.Unmarshal(raw, &o); err != nil {
			t.Fatal(err)
		}
		v, a := 12800-(880./3)*math.Pi+640./9, 3520+272*math.Pi-320./3
		if math.Abs(o.Volume-v) > 1e-8*v || math.Abs(o.Area-a) > 1e-8*a ||
			f.ToWorld(p.Crossing).Distance(geom.Vec3{X: o.Centroid[0], Y: o.Centroid[1], Z: o.Centroid[2]}) > 1e-6 {
			t.Fatal(n, o, v, a)
		}
	}
}

func TestReciprocalCrossedObliqueSTEPRejectsUnsupportedRatio(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	for _, ratio := range []geom.Vec2{{X: 4, Y: 3}, {X: 6, Y: 8}, {X: 3, Y: -4}, {X: math.NaN(), Y: 4}} {
		var b bytes.Buffer
		m := CrossedObliqueBoreModel{LengthUnit: .001, Body: CrossedObliqueBoreBody{Frame: f, Dimensions: geom.Vec3{X: 20, Y: 16, Z: 40}, Crossing: geom.Vec3{X: 10, Y: 8, Z: 20}, Radius: 2, AxisRatio: ratio}}
		if e := WriteCrossedObliqueBore(&b, m); e == nil || b.Len() != 0 {
			t.Fatal(ratio, e, b.Len())
		}
	}
}
