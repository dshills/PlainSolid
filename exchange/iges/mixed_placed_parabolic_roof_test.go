package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
	"time"
)

func mixedRoofFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/mixed-placed-parabolic-roof.igs")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestMixedPlacedParabolicRoof(t *testing.T) {
	ctx := context.Background()
	b := mixedRoofFixture(t)
	m, e := ReadMixedPlacedParabolicRoofSolid(ctx, bytes.NewReader(b), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.OriginalGeometry) != 18 || len(m.SourceLeafTransforms) != 18 || !m.Embedding.Complete {
		t.Fatal("provenance")
	}
	p, e := m.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-4.0/3) > 1e-9 || p.Centroid.Distance(geom.Vec3{X: 10.7, Y: -3.1875, Z: .5}) > 1e-9 {
		t.Fatal(p, e)
	}
	for id, ptr := range m.SourceLeafTransforms {
		if ptr != 0 {
			continue
		}
		if src := m.OriginalGeometry[id]; src.Surface != nil {
			p := src.Surface.Geometry.Data().Poles[0]
			q, e := roofMapPoint(m.Placement, p, true)
			if e != nil || q != m.Local.Geometry[id].Surface.Geometry.Data().Poles[0] {
				t.Fatal("source basis lost", id, e)
			}
		}
	}
	for _, bake := range []RoofLeafBake{{Surfaces: [6]bool{true, false, true}, ModelCurves: [12]bool{false, true, false, true}}, {Surfaces: [6]bool{true}}, {ModelCurves: [12]bool{false, false, false, true}}} {
		var out bytes.Buffer
		if e = WriteMixedPlacedParabolicRoofSolid(ctx, &out, m.Local.Descriptor, m.Placement, m.Local.LengthUnit, bake); e != nil {
			t.Fatal(e)
		}
		n, e := ReadMixedPlacedParabolicRoofSolid(ctx, bytes.NewReader(out.Bytes()), Limits{})
		if e != nil || n.Placement != m.Placement || n.Local.Descriptor != m.Local.Descriptor {
			t.Fatal(e)
		}
		if _, e = ReadPlacedParabolicRoofSolid(ctx, bytes.NewReader(out.Bytes()), Limits{}); e == nil {
			t.Fatal("uniform API widened")
		}
	}
}
func TestMixedPlacedParabolicRoofRejects(t *testing.T) {
	ctx := context.Background()
	b := mixedRoofFixture(t)
	m, e := ReadMixedPlacedParabolicRoofSolid(ctx, bytes.NewReader(b), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var baked, placed int
	for id, p := range m.SourceLeafTransforms {
		if p == 0 {
			baked = id
		} else {
			placed = id
		}
	}
	for i, bad := range [][]byte{placedDir(b, baked, 6, 117), placedDir(b, placed, 6, 0), placedDir(b, m.Local.SourcePCurveIDs[0][0], 6, 117), placedDir(b, baked, 3, 1), placedDir(b, 117, 4, 1)} {
		n, e := ReadMixedPlacedParabolicRoofSolid(ctx, bytes.NewReader(bad), Limits{})
		if e == nil || len(n.OriginalGeometry) != 0 {
			t.Fatal(i, e)
		}
	}
	if _, e = ReadMixedPlacedParabolicRoofSolid(ctx, bytes.NewReader(placedRoofFixture(t)), Limits{}); e == nil {
		t.Fatal("uniform input admitted")
	}
	for _, l := range []Limits{{MaxBytes: 100}, {MaxEntities: 58}, {MaxControlPoints: 103}, {MaxValues: 20}} {
		if _, e = ReadMixedPlacedParabolicRoofSolid(ctx, bytes.NewReader(b), l); e == nil {
			t.Fatal(l)
		}
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadMixedPlacedParabolicRoofSolid(c, bytes.NewReader(b), Limits{}); e == nil {
		t.Fatal("canceled")
	}
	var out bytes.Buffer
	if e = WriteMixedPlacedParabolicRoofSolid(ctx, &out, m.Local.Descriptor, m.Placement, 1, RoofLeafBake{}); e == nil || out.Len() != 0 {
		t.Fatal("atomic empty selection", e)
	}
}
func TestOCCTMixedPlacedParabolicRoof(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_IGES_SOLID")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	b := mixedRoofFixture(t)
	m, e := ReadMixedPlacedParabolicRoofSolid(context.Background(), bytes.NewReader(b), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteMixedPlacedParabolicRoofSolid(context.Background(), &out, m.Local.Descriptor, m.Placement, m.Local.LengthUnit, RoofLeafBake{Surfaces: [6]bool{true, false, true}, ModelCurves: [12]bool{true, false, true}}); e != nil {
		t.Fatal(e)
	}
	for i, data := range [][]byte{b, out.Bytes()} {
		path := filepath.Join(t.TempDir(), "mixed.igs")
		if e = os.WriteFile(path, data, 0600); e != nil {
			t.Fatal(e)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		result, e := exec.CommandContext(ctx, bin, path).CombinedOutput()
		cancel()
		if e != nil {
			t.Fatal(e, string(result))
		}
		at := bytes.LastIndex(result, []byte(`{"Faces"`))
		if at < 0 {
			t.Fatal(string(result))
		}
		var p struct {
			Faces, Solids int
			Volume, Area  float64
			Center        [3]float64
		}
		if e = json.Unmarshal(result[at:], &p); e != nil {
			t.Fatal(e)
		}
		if p.Faces != 6 || p.Solids != 1 || math.Abs(p.Volume-4.0/3) > 1e-9 || math.Abs(p.Area-(20.0/3+math.Sqrt(5)/2+math.Asinh(2)/4)) > 1e-8 || math.Abs(p.Center[0]-10.7) > 1e-9 || math.Abs(p.Center[1]+3.1875) > 1e-9 || math.Abs(p.Center[2]-.5) > 1e-9 {
			t.Fatal(i, p)
		}
	}
}
func FuzzMixedPlacedParabolicRoof(f *testing.F) {
	f.Add(mixedRoofFixture(f))
	f.Add([]byte("bad"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			t.Skip()
		}
		_, _ = ReadMixedPlacedParabolicRoofSolid(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 59, MaxControlPoints: 104})
	})
}
