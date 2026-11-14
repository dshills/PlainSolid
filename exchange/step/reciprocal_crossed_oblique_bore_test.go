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
	for _, tc := range []struct {
		axis   geom.Vec3
		ratio  geom.Vec2
		height float64
	}{
		{geom.Vec3{Z: 1}, geom.Vec2{X: 3, Y: 4}, 40},
		{geom.Vec3{Z: -1}, geom.Vec2{X: 3, Y: 4}, 40},
		{geom.Vec3{X: 1}, geom.Vec2{X: 3, Y: 4}, 40},
		{geom.Vec3{X: -1}, geom.Vec2{X: 3, Y: 4}, 40},
		{geom.Vec3{Y: 1}, geom.Vec2{X: 3, Y: 4}, 40},
		{geom.Vec3{Y: -1}, geom.Vec2{X: 3, Y: 4}, 40},
		{geom.Vec3{Z: 1}, geom.Vec2{X: 12, Y: 5}, 70},
		{geom.Vec3{Y: -1}, geom.Vec2{X: 12, Y: 5}, 70},
		{geom.Vec3{Z: 1}, geom.Vec2{X: 5, Y: 12}, 70},
		{geom.Vec3{X: -1}, geom.Vec2{X: 5, Y: 12}, 70},
	} {
		n := tc.axis
		f, err := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, n)
		if err != nil {
			t.Fatal(err)
		}
		p := CrossedObliqueBoreBody{Name: "cardinal and oblique crossed bores", Frame: f, Dimensions: geom.Vec3{X: 20, Y: 16, Z: tc.height},
			Crossing: geom.Vec3{X: 10, Y: 8, Z: tc.height / 2}, Radius: 2, AxisRatio: tc.ratio}
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
		stretch := math.Hypot(tc.ratio.X, tc.ratio.Y) / tc.ratio.X
		wallArea := 4 * math.Pi * (tc.height + 20*stretch)
		v := 320*tc.height - wallArea + 128*stretch/3
		a := 2*(320+36*tc.height) + wallArea - 64*stretch - 8*math.Pi*(1+stretch)
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

func TestBlindCrossedObliqueBoreSTEP(t *testing.T) {
	for _, normal := range []geom.Vec3{{Z: 1}, {Y: -1}} {
		f, err := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, normal)
		if err != nil {
			t.Fatal(err)
		}
		for _, opposite := range []bool{false, true} {
			for _, ratio := range []geom.Vec2{{}, {X: 3, Y: 4}, {X: 12, Y: 5}, {X: 5, Y: 12}} {
				m := CrossedObliqueBoreModel{LengthUnit: .001, Body: CrossedObliqueBoreBody{Name: "cardinal blind and oblique through",
					Frame: f, Dimensions: geom.Vec3{X: 20, Y: 16, Z: 70}, Crossing: geom.Vec3{X: 10, Y: 8, Z: 35},
					Radius: 2, AxisRatio: ratio, BlindDepth: 48, OppositeCap: opposite}}
				var out bytes.Buffer
				if err := WriteCrossedObliqueBore(&out, m); err != nil {
					t.Fatal(normal, opposite, ratio, err)
				}
				if strings.Count(out.String(), "ELLIPSE(") != 16 || strings.Contains(out.String(), "B_SPLINE_CURVE_WITH_KNOTS") {
					t.Fatal(normal, opposite, ratio, "unexpected crossing representation")
				}
				got, err := ReadCrossedObliqueBore(bytes.NewReader(out.Bytes()), Limits{})
				if err != nil || got != m {
					t.Fatal(normal, opposite, ratio, got, err)
				}
				var again bytes.Buffer
				if err := WriteCrossedObliqueBore(&again, got); err != nil || !bytes.Equal(out.Bytes(), again.Bytes()) {
					t.Fatal(normal, opposite, ratio, err)
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
				var gotMass struct {
					Volume, Area float64
					Centroid     [3]float64
				}
				if err := json.Unmarshal(raw, &gotMass); err != nil {
					t.Fatal(err)
				}
				stretch := 1.25
				if ratio != (geom.Vec2{}) {
					stretch = math.Hypot(ratio.X, ratio.Y) / ratio.X
				}
				removed := 4*math.Pi*(48+20*stretch) - 128*stretch/3
				v := 22400 - removed
				a := 5680 + 4*math.Pi*(48+20*stretch) - 64*stretch - 8*math.Pi*stretch
				// The retained solid gains the 22 mm of uncut cardinal
				// material at the far cap relative to the symmetric through part.
				z := 70. - 22./2
				if opposite {
					z = 22. / 2
				}
				wantCenter := f.ToWorld(geom.Vec3{X: 10, Y: 8, Z: 35 + (z-35)*22*4*math.Pi/v})
				center := geom.Vec3{X: gotMass.Centroid[0], Y: gotMass.Centroid[1], Z: gotMass.Centroid[2]}
				if math.Abs(gotMass.Volume-v) > 1e-8*v || math.Abs(gotMass.Area-a) > 1e-8*a || center.Distance(wantCenter) > 1e-6 {
					t.Fatal(normal, opposite, ratio, gotMass, v, a, wantCenter)
				}
			}
		}
	}
}
