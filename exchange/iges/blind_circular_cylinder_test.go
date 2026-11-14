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

func blindSource(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/blind-circular-cylinder.igs")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func blindDescriptor() BlindCircularCylinderDescriptor {
	return BlindCircularCylinderDescriptor{Radius: 3, Height: 4, BoreRadius: 1, Floor: 2}
}
func TestBlindCircularCylinder(t *testing.T) {
	ctx := context.Background()
	var out bytes.Buffer
	if e := WriteBlindCircularCylinderSolid(ctx, &out, blindDescriptor(), .001); e != nil {
		t.Fatal(e)
	}
	for _, raw := range [][]byte{blindSource(t), out.Bytes()} {
		v, e := ReadBlindCircularCylinderSolid(ctx, bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if v.Descriptor != blindDescriptor() || v.SourceSolidID != 77 || v.SourceFaceIDs[4] != 73 || len(v.Parameters) != 39 {
			t.Fatal(v)
		}
		p, e := v.Native.Solids[0].MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if math.Abs(p.Volume-34*math.Pi) > 1e-5 || math.Abs(p.Area-46*math.Pi) > 1e-5 || math.Abs(p.Centroid.Z-33./17) > 1e-6 {
			t.Fatal(p)
		}
		if _, e = ReadCircularCylinderSolid(ctx, bytes.NewReader(raw), Limits{}); e == nil {
			t.Fatal("old codec widened")
		}
	}
	d := blindDescriptor()
	d.Origin = geom.Vec3{X: 2, Y: -3, Z: 4}
	out.Reset()
	if e := WriteBlindCircularCylinderSolid(ctx, &out, d, 1); e != nil {
		t.Fatal(e)
	}
	v, e := ReadBlindCircularCylinderSolid(ctx, &out, Limits{})
	if e != nil || v.Descriptor != d || v.LengthUnit != 1 {
		t.Fatal(v, e)
	}
}
func TestBlindCircularCylinderRejects(t *testing.T) {
	ctx := context.Background()
	raw := blindSource(t)
	for _, tc := range []struct {
		id, index int
		value     string
	}{{13, 3, "2"}, {69, 5, "61"}, {59, 5, "1"}, {75, 5, "1"}, {73, 1, "15"}, {21, 12, "3"}, {53, 4, "359"}, {35, 17, "23"}} {
		doc, e := readPlacedTopologyRecords(bytes.NewReader(raw), Limits{}, true, false, false, false, false, false, false, true)
		if e != nil {
			t.Fatal(e)
		}
		for i := range doc.entities {
			if doc.entities[i].id == tc.id {
				doc.entities[i].values[tc.index].text = tc.value
			}
		}
		b, e := mixedRoofFile(doc)
		if e != nil {
			t.Fatal(e)
		}
		v, e := ReadBlindCircularCylinderSolid(ctx, bytes.NewReader(b), Limits{})
		if e == nil || v.SourceSolidID != 0 {
			t.Fatal(tc, e)
		}
	}
	for _, l := range []Limits{{MaxBytes: 100}, {MaxEntities: 38}, {MaxValues: 20}} {
		if _, e := ReadBlindCircularCylinderSolid(ctx, bytes.NewReader(raw), l); e == nil {
			t.Fatal(l)
		}
	}
	vendor, e := os.ReadFile("testdata/occt-rounded-blind-cylinder.igs")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadBlindCircularCylinderSolid(ctx, bytes.NewReader(vendor), Limits{}); e == nil {
		t.Fatal("rounded source promoted")
	}
	for _, d := range []BlindCircularCylinderDescriptor{{Radius: 1, BoreRadius: 1, Height: 4, Floor: 2}, {Radius: 3, BoreRadius: 1, Height: 4, Floor: 4}, {Radius: 3, BoreRadius: 1, Height: 4, Floor: 0}} {
		var out bytes.Buffer
		if e := WriteBlindCircularCylinderSolid(ctx, &out, d, .001); e == nil || out.Len() != 0 {
			t.Fatal(d)
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := ReadBlindCircularCylinderSolid(ctx, bytes.NewReader(raw), Limits{}); e != context.Canceled {
		t.Fatal(e)
	}
}
func TestOCCTBlindCircularCylinder(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_IGES_SOLID")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	var out bytes.Buffer
	if e := WriteBlindCircularCylinderSolid(context.Background(), &out, blindDescriptor(), .001); e != nil {
		t.Fatal(e)
	}
	vendor, e := os.ReadFile("testdata/occt-rounded-blind-cylinder.igs")
	if e != nil {
		t.Fatal(e)
	}
	for _, raw := range [][]byte{blindSource(t), out.Bytes(), vendor} {
		path := filepath.Join(t.TempDir(), "blind.igs")
		if e := os.WriteFile(path, raw, 0600); e != nil {
			t.Fatal(e)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		b, e := exec.CommandContext(ctx, bin, path).CombinedOutput()
		cancel()
		if e != nil {
			t.Fatal(e, string(b))
		}
		at := bytes.LastIndex(b, []byte(`{"Faces"`))
		if at < 0 {
			t.Fatal(string(b))
		}
		var p struct {
			Faces, Solids int
			Volume, Area  float64
			Center        [3]float64
		}
		if e = json.Unmarshal(b[at:], &p); e != nil {
			t.Fatal(e)
		}
		if p.Faces != 5 || p.Solids != 1 || math.Abs(p.Volume-34*math.Pi) > 1e-8 || math.Abs(p.Area-46*math.Pi) > 1e-8 || math.Abs(p.Center[2]-33./17) > 1e-9 {
			t.Fatal(p)
		}
	}
}
func FuzzBlindCircularCylinder(f *testing.F) {
	f.Add(blindSource(f))
	f.Add([]byte("bad"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			t.Skip()
		}
		_, _ = ReadBlindCircularCylinderSolid(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 39, MaxValues: 4096})
	})
}
