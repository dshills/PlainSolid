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
	"strings"
	"testing"
	"time"
)

func placedRoofFixture(t *testing.T) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/leaf-placed-parabolic-roof.igs")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestPlacedParabolicRoof(t *testing.T) {
	b := placedRoofFixture(t)
	ctx := context.Background()
	m, e := ReadPlacedParabolicRoofSolid(ctx, bytes.NewReader(b), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	want := geom.Affine{{0, 0, 1, 7}, {1, 0, 0, -4}, {0, 1, 0, 2}}
	if m.Placement != want || !m.Embedding.Complete || m.SourceTransformID != 117 || m.Local.SourceSolidID != 1 {
		t.Fatal(m.Placement, m.Local.SourceSolidID)
	}
	p, e := m.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-4.0/3) > 1e-9 || p.Centroid.Distance(geom.Vec3{X: 10.7, Y: -3.1875, Z: .5}) > 1e-9 {
		t.Fatal(p, e)
	}
	var out bytes.Buffer
	if e = WritePlacedParabolicRoofSolid(ctx, &out, m.Local.Descriptor, want, m.Local.LengthUnit); e != nil {
		t.Fatal(e)
	}
	n, e := ReadPlacedParabolicRoofSolid(ctx, &out, Limits{})
	if e != nil || n.Local.Descriptor != m.Local.Descriptor || n.Placement != want {
		t.Fatal(n.Local.Descriptor, e)
	}
	if _, e = ReadMappedParabolicRoofSolid(ctx, bytes.NewReader(b), Limits{}); e == nil {
		t.Fatal("old API widened")
	}
}
func placedDir(b []byte, id, field, value int) []byte {
	lines := bytes.Split(append([]byte(nil), b...), []byte("\n"))
	for _, l := range lines {
		if len(l) == 80 && l[72] == 'D' {
			n, _ := fieldInt(l[73:])
			if n == id {
				copy(l[field*8:field*8+8], []byte(padInt(value)))
			}
		}
	}
	return bytes.Join(lines, []byte("\n"))
}
func fieldInt(b []byte) (int, error) { return field(string(b)) }
func padInt(x int) string            { return strings.Repeat(" ", 8-len(number(float64(x)))) + number(float64(x)) }
func TestPlacedParabolicRoofRejects(t *testing.T) {
	b := placedRoofFixture(t)
	ctx := context.Background()
	m, e := ReadPlacedParabolicRoofSolid(ctx, bytes.NewReader(b), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	bad := [][]byte{placedDir(b, 1, 6, 117), placedDir(b, m.Local.SourceSurfaceIDs[0], 6, 0), placedDir(b, m.Local.SourceModelCurveIDs[0], 6, 115), placedDir(b, m.Local.SourcePCurveIDs[0][0], 6, 117), placedDir(b, 117, 6, 117), placedDir(b, 117, 4, 1), bytes.Replace(b, []byte("124,0,0,1,7,1,0,0,-4,0,1,0,2;"), []byte("124,0,0,2,7,1,0,0,-4,0,1,0,2;"), 1)}
	bad = append(bad, bytes.Replace(b, []byte("502,8,12.0"), []byte("502,8,12.5"), 1), bytes.Replace(b, []byte("124,0,0,1,7"), []byte("124,0,0,1,8"), 1), bytes.Replace(b, []byte("124,0,0,1,7"), []byte("123,0,0,1,7"), 1))
	for i, x := range bad {
		if bytes.Equal(x, b) {
			t.Fatal("ineffective mutation", i)
		}
		n, e := ReadPlacedParabolicRoofSolid(ctx, bytes.NewReader(x), Limits{})
		if e == nil || n.SourceTransformID != 0 {
			t.Fatal(i, e)
		}
	}
	for _, l := range []Limits{{MaxEntities: 58}, {MaxControlPoints: 103}, {MaxBytes: 100}, {MaxValues: 20}} {
		if _, e = ReadPlacedParabolicRoofSolid(ctx, bytes.NewReader(b), l); e == nil {
			t.Fatal(l)
		}
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadPlacedParabolicRoofSolid(c, bytes.NewReader(b), Limits{}); e == nil {
		t.Fatal("cancellation")
	}
	for _, pose := range []geom.Affine{geom.Scaling(geom.Vec3{X: -1, Y: 1, Z: 1}), geom.Translation(geom.Vec3{X: math.SmallestNonzeroFloat64}), {{1, 1, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}}} {
		var out bytes.Buffer
		if e = WritePlacedParabolicRoofSolid(ctx, &out, m.Local.Descriptor, pose, .001); e == nil || out.Len() != 0 {
			t.Fatal("invalid writer", pose, e)
		}
	}
}
func TestOCCTPlacedParabolicRoof(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_IGES_SOLID")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	raw := placedRoofFixture(t)
	m, e := ReadPlacedParabolicRoofSolid(context.Background(), bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WritePlacedParabolicRoofSolid(context.Background(), &b, m.Local.Descriptor, m.Placement, m.Local.LengthUnit); e != nil {
		t.Fatal(e)
	}
	for i, data := range [][]byte{raw, b.Bytes()} {
		path := filepath.Join(t.TempDir(), "posed.igs")
		if e = os.WriteFile(path, data, 0600); e != nil {
			t.Fatal(e)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		out, e := exec.CommandContext(ctx, bin, path).CombinedOutput()
		cancel()
		if e != nil {
			t.Fatal(i, e, string(out))
		}
		at := bytes.LastIndex(out, []byte(`{"Faces"`))
		if at < 0 {
			t.Fatal(string(out))
		}
		var p struct {
			Faces, Solids int
			Volume, Area  float64
			Center        [3]float64
		}
		if e = json.Unmarshal(out[at:], &p); e != nil {
			t.Fatal(e)
		}
		area := 20.0/3 + math.Sqrt(5)/2 + math.Asinh(2)/4
		if p.Faces != 6 || p.Solids != 1 || math.Abs(p.Volume-4.0/3) > 1e-9 || math.Abs(p.Area-area) > 1e-8 || math.Abs(p.Center[0]-10.7) > 1e-9 || math.Abs(p.Center[1]+3.1875) > 1e-9 || math.Abs(p.Center[2]-.5) > 1e-9 {
			t.Fatal(i, p)
		}
	}
}
func FuzzPlacedParabolicRoof(f *testing.F) {
	b, e := os.ReadFile("testdata/leaf-placed-parabolic-roof.igs")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Add([]byte("bad"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			t.Skip()
		}
		_, _ = ReadPlacedParabolicRoofSolid(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 59, MaxControlPoints: 104})
	})
}

func TestPlacedParabolicRoofRootTransformRejected(t *testing.T) {
	b, e := os.ReadFile("testdata/placed-parabolic-roof.igs")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadPlacedParabolicRoofSolid(context.Background(), bytes.NewReader(b), Limits{}); e == nil {
		t.Fatal("root transform accepted")
	}
}
func TestPlacedParabolicRoofSignedPoses(t *testing.T) {
	d := ParabolicRoofDescriptor{Width: 2, Depth: 3, Height: .5, Rise: .25}
	for _, m := range []geom.Affine{geom.Identity(), {{0, -1, 0, 8}, {0, 0, -1, 4}, {1, 0, 0, -2}}, {{-1, 0, 0, 3}, {0, 1, 0, 1}, {0, 0, -1, 5}}} {
		var out bytes.Buffer
		if e := WritePlacedParabolicRoofSolid(context.Background(), &out, d, m, 1); e != nil {
			t.Fatal(m, e)
		}
		r, e := ReadPlacedParabolicRoofSolid(context.Background(), &out, Limits{})
		if e != nil || r.Placement != m || r.Local.Descriptor != d || r.Local.LengthUnit != 1 {
			t.Fatal(m, e)
		}
	}
}

func TestOCCTRootOnlyRoofPlacementIsIgnored(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_IGES_SOLID")
	if bin == "" {
		t.Skip("optional pinned OCCT negative evidence")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	out, e := exec.CommandContext(ctx, bin, "testdata/placed-parabolic-roof.igs").CombinedOutput()
	if e != nil {
		t.Fatal(e, string(out))
	}
	at := bytes.LastIndex(out, []byte(`{"Faces"`))
	if at < 0 {
		t.Fatal(string(out))
	}
	var p struct {
		Faces, Solids int
		Center        [3]float64
	}
	if e = json.Unmarshal(out[at:], &p); e != nil {
		t.Fatal(e)
	}
	if p.Faces != 6 || p.Solids != 1 || math.Abs(p.Center[0]-.8125) > 1e-9 || math.Abs(p.Center[1]+1.5) > 1e-9 || math.Abs(p.Center[2]-3.7) > 1e-9 {
		t.Fatal("pinned root-only transform behavior changed", p)
	}
}
