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

func throughSource(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/through-circular-cylinder.igs")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func throughDescriptor() ThroughCircularCylinderDescriptor {
	return ThroughCircularCylinderDescriptor{Radius: 3, Height: 4, BoreRadius: 1}
}
func TestThroughCircularCylinder(t *testing.T) {
	ctx := context.Background()
	var out bytes.Buffer
	if e := WriteThroughCircularCylinderSolid(ctx, &out, throughDescriptor(), .001); e != nil {
		t.Fatal(e)
	}
	for _, raw := range [][]byte{throughSource(t), out.Bytes()} {
		v, e := ReadThroughCircularCylinderSolid(ctx, bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if v.Descriptor != throughDescriptor() || v.SourceSolidID != 71 || v.SourceFaceIDs[3] != 67 || len(v.Parameters) != 36 {
			t.Fatal(v)
		}
		p, e := v.Native.Solids[0].MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if math.Abs(p.Volume-32*math.Pi) > 1e-5 || math.Abs(p.Area-48*math.Pi) > 1e-5 || math.Abs(p.Centroid.Z-2) > 1e-6 {
			t.Fatal(p)
		}
		if _, e = ReadCircularCylinderSolid(ctx, bytes.NewReader(raw), Limits{}); e == nil {
			t.Fatal("old codec widened")
		}
	}
	d := throughDescriptor()
	d.Origin = geom.Vec3{X: 2, Y: -3, Z: 4}
	out.Reset()
	if e := WriteThroughCircularCylinderSolid(ctx, &out, d, 1); e != nil {
		t.Fatal(e)
	}
	v, e := ReadThroughCircularCylinderSolid(ctx, &out, Limits{})
	if e != nil || v.Descriptor != d || v.LengthUnit != 1 {
		t.Fatal(v, e)
	}
}
func TestThroughCircularCylinderRejects(t *testing.T) {
	ctx := context.Background()
	raw := throughSource(t)
	for _, tc := range []struct {
		id, index int
		value     string
	}{{11, 3, "2"}, {65, 5, "59"}, {55, 5, "1"}, {69, 5, "1"}, {67, 1, "13"}, {17, 12, "3"}, {49, 4, "359"}, {31, 17, "19"}} {
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
		v, e := ReadThroughCircularCylinderSolid(ctx, bytes.NewReader(b), Limits{})
		if e == nil || v.SourceSolidID != 0 {
			t.Fatal(tc, e)
		}
	}
	for _, l := range []Limits{{MaxBytes: 100}, {MaxEntities: 35}, {MaxValues: 20}} {
		if _, e := ReadThroughCircularCylinderSolid(ctx, bytes.NewReader(raw), l); e == nil {
			t.Fatal(l)
		}
	}
	vendor, e := os.ReadFile("testdata/occt-rounded-through-cylinder.igs")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadThroughCircularCylinderSolid(ctx, bytes.NewReader(vendor), Limits{}); e == nil {
		t.Fatal("rounded source promoted")
	}
	for _, d := range []ThroughCircularCylinderDescriptor{{Radius: 1, BoreRadius: 1, Height: 4}, {Radius: 3, BoreRadius: 1, Height: 0}, {Radius: 3, BoreRadius: 1, Height: -1}} {
		var out bytes.Buffer
		if e := WriteThroughCircularCylinderSolid(ctx, &out, d, .001); e == nil || out.Len() != 0 {
			t.Fatal(d)
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := ReadThroughCircularCylinderSolid(ctx, bytes.NewReader(raw), Limits{}); e != context.Canceled {
		t.Fatal(e)
	}
}
func TestOCCTThroughCircularCylinder(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_IGES_SOLID")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	var out bytes.Buffer
	if e := WriteThroughCircularCylinderSolid(context.Background(), &out, throughDescriptor(), .001); e != nil {
		t.Fatal(e)
	}
	vendor, e := os.ReadFile("testdata/occt-rounded-through-cylinder.igs")
	if e != nil {
		t.Fatal(e)
	}
	for _, raw := range [][]byte{throughSource(t), out.Bytes(), vendor} {
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
		if p.Faces != 4 || p.Solids != 1 || math.Abs(p.Volume-32*math.Pi) > 1e-8 || math.Abs(p.Area-48*math.Pi) > 1e-8 || math.Abs(p.Center[2]-2) > 1e-9 {
			t.Fatal(p)
		}
	}
}
func TestThroughBlindCircularDisjoint(t *testing.T) {
	ctx := context.Background()
	if _, e := ReadThroughCircularCylinderSolid(ctx, bytes.NewReader(blindSource(t)), Limits{}); e == nil {
		t.Fatal("blind admitted as through")
	}
	if _, e := ReadBlindCircularCylinderSolid(ctx, bytes.NewReader(throughSource(t)), Limits{}); e == nil {
		t.Fatal("through admitted as blind")
	}
}

func FuzzThroughCircularCylinder(f *testing.F) {
	f.Add(throughSource(f))
	f.Add([]byte("bad"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			t.Skip()
		}
		_, _ = ReadThroughCircularCylinderSolid(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 36, MaxValues: 4096})
	})
}
