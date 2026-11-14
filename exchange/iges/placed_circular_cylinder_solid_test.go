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

func placedCircularSourceFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/placed-circular-cylinder-solid.igs")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func circularPose() geom.Affine { return geom.Affine{{0, 0, 1, 7}, {1, 0, 0, -4}, {0, 1, 0, 2}} }
func TestPlacedCircularCylinder(t *testing.T) {
	ctx := context.Background()
	raw := placedCircularSourceFixture(t)
	m, e := ReadPlacedCircularCylinderSolid(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if m.Placement != circularPose() || m.SourceTransformID != 49 || m.Local.Descriptor != (CircularCylinderDescriptor{Radius: 2, Height: 3}) || m.SourceVertices != ([2]geom.Vec3{{X: 10, Y: -2, Z: 2}, {X: 7, Y: -2, Z: 2}}) {
		t.Fatal(m.Placement, m.SourceVertices)
	}
	if m.WorldFrame.Origin() != (geom.Vec3{X: 7, Y: -4, Z: 2}) || m.WorldFrame.ZAxis() != (geom.Vec3{X: 1}) {
		t.Fatal("frame")
	}
	p, e := m.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-12*math.Pi) > 1e-8 || math.Abs(p.Area-20*math.Pi) > 1e-8 || p.Centroid.Distance(geom.Vec3{X: 8.5, Y: -4, Z: 2}) > 1e-9 {
		t.Fatal(p, e)
	}
	for _, pose := range []geom.Affine{circularPose(), geom.Identity(), {{-1, 0, 0, 3}, {0, 1, 0, 1}, {0, 0, -1, 5}}} {
		d := CircularCylinderDescriptor{Origin: geom.Vec3{X: .25, Y: -2, Z: 3}, Radius: 2, Height: 3}
		var b bytes.Buffer
		if e = WritePlacedCircularCylinderSolid(ctx, &b, d, pose, .001); e != nil {
			t.Fatal(e)
		}
		n, e := ReadPlacedCircularCylinderSolid(ctx, bytes.NewReader(b.Bytes()), Limits{})
		if e != nil || n.Local.Descriptor != d || n.Placement != pose {
			t.Fatal(e)
		}
		if _, e = ReadCircularCylinderSolid(ctx, bytes.NewReader(b.Bytes()), Limits{}); e == nil {
			t.Fatal("standalone widened")
		}
	}
}
func TestPlacedCircularCylinderRejects(t *testing.T) {
	ctx := context.Background()
	raw := placedCircularSourceFixture(t)
	for _, tc := range []struct{ id, value int }{{17, 0}, {9, 47}, {25, 49}, {1, 49}, {47, 49}, {49, 49}} {
		m, e := ReadPlacedCircularCylinderSolid(ctx, bytes.NewReader(placedDir(raw, tc.id, 6, tc.value)), Limits{})
		if e == nil || m.SourceTransformID != 0 {
			t.Fatal(tc, e)
		}
	}
	for _, tc := range [][2]string{{"124,0,0,1,7", "124,0,0,1,8"}, {"124,0,0,1,7", "124,0,0,2,7"}, {"502,2,10.0,-2.0", "502,2,10.0,-1.0"}} {
		bad := bytes.Replace(raw, []byte(tc[0]), []byte(tc[1]), 1)
		if bytes.Equal(bad, raw) {
			t.Fatal("ineffective mutation", tc)
		}
		if _, e := ReadPlacedCircularCylinderSolid(ctx, bytes.NewReader(bad), Limits{}); e == nil {
			t.Fatal(tc)
		}
	}
	for _, l := range []Limits{{MaxEntities: 24}, {MaxBytes: 100}, {MaxValues: 20}} {
		if _, e := ReadPlacedCircularCylinderSolid(ctx, bytes.NewReader(raw), l); e == nil {
			t.Fatal(l)
		}
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := ReadPlacedCircularCylinderSolid(c, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("cancel")
	}
	for _, pose := range []geom.Affine{geom.Scaling(geom.Vec3{X: -1, Y: 1, Z: 1}), {{1, 1, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}}, geom.Translation(geom.Vec3{X: math.SmallestNonzeroFloat64})} {
		var out bytes.Buffer
		if e := WritePlacedCircularCylinderSolid(ctx, &out, CircularCylinderDescriptor{Radius: 2, Height: 3}, pose, .001); e == nil || out.Len() != 0 {
			t.Fatal("invalid writer", pose, e)
		}
	}
}
func TestOCCTPlacedCircularCylinder(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_IGES_SOLID")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	for _, d := range []CircularCylinderDescriptor{{Radius: 2, Height: 3}, {Origin: geom.Vec3{X: .25, Y: -2, Z: 3}, Radius: 2, Height: 3}} {
		var out bytes.Buffer
		if e := WritePlacedCircularCylinderSolid(context.Background(), &out, d, circularPose(), .001); e != nil {
			t.Fatal(e)
		}
		inputs := [][]byte{out.Bytes()}
		if d.Origin == (geom.Vec3{}) {
			inputs = append(inputs, placedCircularSourceFixture(t))
		}
		for _, b := range inputs {
			path := filepath.Join(t.TempDir(), "placed.igs")
			if e := os.WriteFile(path, b, 0600); e != nil {
				t.Fatal(e)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			raw, e := exec.CommandContext(ctx, bin, path).CombinedOutput()
			cancel()
			if e != nil {
				t.Fatal(e, string(raw))
			}
			at := bytes.LastIndex(raw, []byte(`{"Faces"`))
			if at < 0 {
				t.Fatal(string(raw))
			}
			var p struct {
				Faces, Solids int
				Volume, Area  float64
				Center        [3]float64
			}
			if e = json.Unmarshal(raw[at:], &p); e != nil {
				t.Fatal(e)
			}
			want := [3]float64{d.Origin.Z + 8.5, d.Origin.X - 4, d.Origin.Y + 2}
			if p.Faces != 3 || p.Solids != 1 || math.Abs(p.Volume-12*math.Pi) > 1e-8 || math.Abs(p.Area-20*math.Pi) > 1e-8 {
				t.Fatal(p)
			}
			for i, x := range want {
				if math.Abs(p.Center[i]-x) > 1e-9 {
					t.Fatal(p, want)
				}
			}
		}
	}
}
func FuzzPlacedCircularCylinder(f *testing.F) {
	f.Add(placedCircularSourceFixture(f))
	f.Add([]byte("bad"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 32768 {
			t.Skip()
		}
		_, _ = ReadPlacedCircularCylinderSolid(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 32768, MaxEntities: 25, MaxValues: 2048})
	})
}
