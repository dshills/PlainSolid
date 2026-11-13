package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestLayeredDiskBoresSTEP(t *testing.T) {
	for _, height := range []float64{8, -8} {
		m := diskExtrusionFixture(t, height)
		b := &m.Bodies[0]
		b.Holes = nil
		b.Blind = []trimmed.ExtrusionBlindHole{
			{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 0, Y: 2}, Radius: 2}, Depth: 6},
			{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 2, Y: 2}, Radius: 2}, Depth: 5, OppositeCap: true},
		}
		var out bytes.Buffer
		if err := WriteLayeredDiskBores(&out, m); err != nil {
			t.Fatal(height, err)
		}
		data := append([]byte(nil), out.Bytes()...)
		reread, err := ReadLayeredDiskBores(&out, Limits{})
		if err != nil || len(reread.Bodies) != 1 || reread.Bodies[0].Height != 8 || len(reread.Bodies[0].Blind) != 2 {
			t.Fatal(height, reread, err)
		}
		changed := strings.Replace(string(data), ".CURVE_3D.)", ".PCURVE_S1.)", 1)
		if changed == string(data) {
			t.Fatal("mutation fixture missing")
		}
		if _, err := ReadLayeredDiskBores(strings.NewReader(changed), Limits{}); err == nil {
			t.Fatal("accepted a changed edge graph")
		}
		if _, err := ReadLayeredDiskBores(bytes.NewReader(data), Limits{MaxFaces: 6}); err == nil {
			t.Fatal("ignored face limit")
		}
		if os.Getenv("PLAINSOLID_OCCT_STEP") != "" {
			checkLayeredDiskOCCT(t, *b, data)
		}
	}
}

func checkLayeredDiskOCCT(t *testing.T, b DiskHoleExtrusionBody, data []byte) {
	t.Helper()
	path, report := filepath.Join(t.TempDir(), "layered.step"), filepath.Join(t.TempDir(), "mass.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	stepOracle(t, report, "step-mass-precise", path)
	raw, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Volume, Area float64
		Centroid     [3]float64
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	solid, _, err := trimmed.BoreDiskDepthUnion(ctx, b.Frame, b.Outer, b.Holes, b.Blind, b.Height)
	if err != nil {
		t.Fatal(err)
	}
	p, err := solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-10})
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(got.Volume-p.Volume) > 1e-7 || math.Abs(got.Area-p.Area) > 1e-7 ||
		p.Centroid.Distance(geom.Vec3{X: got.Centroid[0], Y: got.Centroid[1], Z: got.Centroid[2]}) > 1e-7 {
		t.Fatal(b.Height, got, p)
	}
}
