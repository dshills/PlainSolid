package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/geom"
	"testing"
	"time"
)

func mixedPlacedCircularInputs() []MixedTopologyInput {
	return append(mixedTopologyInputs(), MixedTopologyInput{Family: MixedTopologyPlacedCylinder, Cylinder: CircularCylinderDescriptor{Radius: 2, Height: 3}, Placement: geom.Affine{{0, 0, 1, 7}, {1, 0, 0, -4}, {0, 1, 0, 2}}})
}
func mixedPlacedCircularSource(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/mixed-placed-circular-material.igs")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestMixedPlacedCircularMaterial(t *testing.T) {
	var out bytes.Buffer
	if e := WriteMixedTopologySolids(context.Background(), &out, mixedPlacedCircularInputs(), .001); e != nil {
		t.Fatal(e)
	}
	for _, raw := range [][]byte{mixedPlacedCircularSource(t), out.Bytes()} {
		m, e := ReadMixedTopologySolids(context.Background(), bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if len(m.Bodies) != 3 || m.Bodies[2].Family != MixedTopologyPlacedCylinder {
			t.Fatal(m)
		}
		b := m.Bodies[2]
		if b.PlacedCylinder == nil || len(b.LocalToSource) != 25 || b.LocalToSource[b.PlacedCylinder.Local.SourceSolidID] != b.SourceSolidID || b.PlacedCylinder.Local.Descriptor != mixedPlacedCircularInputs()[2].Cylinder {
			t.Fatal(b)
		}
		seen := map[int]bool{}
		for _, b := range m.Bodies {
			for _, id := range b.LocalToSource {
				if seen[id] {
					t.Fatal("shared source reference")
				}
				seen[id] = true
			}
		}
		if len(seen) != 117 {
			t.Fatal(len(seen))
		}
		if _, e = ReadCircularCylinderSolid(context.Background(), bytes.NewReader(raw), Limits{}); e == nil {
			t.Fatal("standalone widened")
		}
	}
	inputs := mixedPlacedCircularInputs()
	inputs = []MixedTopologyInput{inputs[2], inputs[0], inputs[2]}
	out.Reset()
	if e := WriteMixedTopologySolids(context.Background(), &out, inputs, .001); e != nil {
		t.Fatal(e)
	}
	m, e := ReadMixedTopologySolids(context.Background(), &out, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	m.Bodies[0].PlacedCylinder.Local.Parameters[1][1] = 1000
	m.Bodies[0].LocalToSource[1] = 999
	if m.Bodies[2].PlacedCylinder.Local.Parameters[1][1] != 0 || m.Bodies[2].LocalToSource[1] == 999 {
		t.Fatal("mutable source alias")
	}
}
func TestOCCTMixedPlacedCircularMaterial(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_IGES_SOLID")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	var out bytes.Buffer
	if e := WriteMixedTopologySolids(context.Background(), &out, mixedPlacedCircularInputs(), .001); e != nil {
		t.Fatal(e)
	}
	for _, raw := range [][]byte{mixedPlacedCircularSource(t), out.Bytes()} {
		path := filepath.Join(t.TempDir(), "mixed.igs")
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
		v := 8.0/3 + 12*math.Pi
		area := 6 + 2*math.Sqrt(3) + 20.0/3 + math.Sqrt(5)/2 + math.Asinh(2)/4 + 20*math.Pi
		want := [3]float64{(7.0/4 + 102*math.Pi) / v, (-4.0/3 - 48*math.Pi) / v, (28.0/5 + 24*math.Pi) / v}
		if p.Faces != 13 || p.Solids != 3 || math.Abs(p.Volume-v) > 1e-8 || math.Abs(p.Area-area) > 1e-8 {
			t.Fatal(p)
		}
		for i, x := range want {
			if math.Abs(p.Center[i]-x) > 1e-9 {
				t.Fatal(p, want)
			}
		}
	}
}
func FuzzMixedPlacedCircularMaterial(f *testing.F) {
	f.Add(mixedPlacedCircularSource(f))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 131072 {
			t.Skip()
		}
		_, _ = ReadMixedTopologySolids(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 131072, MaxEntities: 128, MaxControlPoints: 208})
	})
}

func TestMixedPlacedCircularOwnership(t *testing.T) {
	ctx := context.Background()
	in := mixedPlacedCircularInputs()
	in = append(in, in[2])
	var out bytes.Buffer
	if e := WriteMixedTopologySolids(ctx, &out, in, .001); e != nil {
		t.Fatal(e)
	}
	m, e := ReadMixedTopologySolids(ctx, bytes.NewReader(out.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, cross := range []bool{false, true} {
		doc, e := readPlacedTopologyRecords(bytes.NewReader(out.Bytes()), Limits{}, true, false, false, false, false, false, true, true)
		if e != nil {
			t.Fatal(e)
		}
		id := m.Bodies[2].LocalToSource[9]
		target := 0
		if cross {
			target = m.Bodies[3].LocalToSource[49]
		}
		for i := range doc.entities {
			if doc.entities[i].id == id {
				doc.entities[i].transform = target
			}
		}
		raw, e := mixedRoofFile(doc)
		if e != nil {
			t.Fatal(e)
		}
		got, e := ReadMixedTopologySolids(ctx, bytes.NewReader(raw), Limits{})
		if e == nil || len(got.Bodies) != 0 {
			t.Fatal("ownership accepted", cross)
		}
	}
	for _, l := range []Limits{{MaxEntities: 116}, {MaxControlPoints: 155}, {MaxBytes: 100}} {
		got, e := ReadMixedTopologySolids(ctx, bytes.NewReader(mixedPlacedCircularSource(t)), l)
		if e == nil || len(got.Bodies) != 0 {
			t.Fatal(l)
		}
	}
	in = mixedPlacedCircularInputs()
	in[0].Placement = in[2].Placement
	out.Reset()
	if e := WriteMixedTopologySolids(ctx, &out, in, .001); e == nil || out.Len() != 0 {
		t.Fatal("foreign placement payload")
	}
}
