package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestSideBlindNotchSTEP(t *testing.T) {
	for _, tc := range []struct {
		normal geom.Vec3
		inset  float64
		mixed  bool
	}{
		{geom.Vec3{Z: 1}, 1, false}, {geom.Vec3{Y: -1}, -1, false}, {geom.Vec3{X: 1}, 1, true},
	} {
		f, err := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, tc.normal)
		if err != nil {
			t.Fatal(err)
		}
		p := SideBlindNotchBody{Name: "side-opening bore", Frame: f, Left: 10, Right: 10, Back: 19,
			Inset: tc.inset, Length: 20, Radius: 3, Depth: 6}
		if tc.mixed {
			p.Through = []trimmed.ExtrusionCircle{{Center: geom.Vec2{X: -4, Y: 9}, Radius: 1}}
			p.Blind = []trimmed.ExtrusionBlindHole{
				{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 4, Y: 9}, Radius: 2}, Depth: 4},
				{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{Y: 14}, Radius: 1}, Depth: 6, OppositeCap: true},
			}
		}
		var out bytes.Buffer
		m := SideBlindNotchModel{Body: p, LengthUnit: .001}
		if err := WriteSideBlindNotch(&out, m); err != nil {
			t.Fatal(tc, err)
		}
		got, err := ReadSideBlindNotch(bytes.NewReader(out.Bytes()), Limits{})
		if err != nil || !reflect.DeepEqual(got, m) {
			t.Fatal(tc, got, err)
		}
		if os.Getenv("PLAINSOLID_OCCT_STEP") != "" {
			path, report := filepath.Join(t.TempDir(), "side-bore.step"), filepath.Join(t.TempDir(), "mass.json")
			if err := os.WriteFile(path, out.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			stepOracle(t, report, "step-mass-precise", path)
			raw, err := os.ReadFile(report)
			if err != nil {
				t.Fatal(err)
			}
			var other struct {
				Volume, Area float64
				Centroid     [3]float64
			}
			if err := json.Unmarshal(raw, &other); err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			s, err := trimmed.BoxSideCylinderBlindWithBores(ctx, p.Frame, p.Left, p.Right, p.Back, p.Inset, p.Length, p.Radius, p.Depth, p.Through, p.Blind, 1e-9)
			if err != nil {
				t.Fatal(err)
			}
			m, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
			if err != nil || math.Abs(m.Volume-other.Volume) > 1e-7 || math.Abs(m.Area-other.Area) > 1e-7 ||
				m.Centroid.Distance(geom.Vec3{X: other.Centroid[0], Y: other.Centroid[1], Z: other.Centroid[2]}) > 1e-7 {
				t.Fatal(tc, m, other, err)
			}
		}
	}
}
