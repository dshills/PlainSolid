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
	"strings"
	"testing"
	"time"
)

func mixedCircularInputs() []MixedTopologyInput {
	return append(mixedTopologyInputs(), MixedTopologyInput{Family: MixedTopologyCylinder, Cylinder: CircularCylinderDescriptor{Origin: geom.Vec3{X: 8}, Radius: 2, Height: 3}})
}
func mixedCircularSource(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/mixed-circular-material.igs")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestMixedCircularMaterial(t *testing.T) {
	var out bytes.Buffer
	if e := WriteMixedTopologySolids(context.Background(), &out, mixedCircularInputs(), .001); e != nil {
		t.Fatal(e)
	}
	for _, raw := range [][]byte{mixedCircularSource(t), out.Bytes()} {
		m, e := ReadMixedTopologySolids(context.Background(), bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if len(m.Bodies) != 3 || m.Bodies[2].Family != MixedTopologyCylinder {
			t.Fatal(m)
		}
		b := m.Bodies[2]
		if b.Cylinder == nil || len(b.LocalToSource) != 24 || b.LocalToSource[b.Cylinder.SourceSolidID] != b.SourceSolidID || b.Cylinder.Descriptor != mixedCircularInputs()[2].Cylinder {
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
		if len(seen) != 116 {
			t.Fatal(len(seen))
		}
		if _, e = ReadCircularCylinderSolid(context.Background(), bytes.NewReader(raw), Limits{}); e == nil {
			t.Fatal("standalone widened")
		}
	}
	inputs := mixedCircularInputs()
	inputs = []MixedTopologyInput{inputs[2], inputs[0], inputs[2]}
	out.Reset()
	if e := WriteMixedTopologySolids(context.Background(), &out, inputs, .001); e != nil {
		t.Fatal(e)
	}
	m, e := ReadMixedTopologySolids(context.Background(), &out, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	m.Bodies[0].Cylinder.Parameters[1][1] = 1000
	m.Bodies[0].LocalToSource[1] = 999
	if m.Bodies[2].Cylinder.Parameters[1][1] != 8 || m.Bodies[2].LocalToSource[1] == 999 {
		t.Fatal("mutable source alias")
	}
}
func TestMixedCircularMaterialRejects(t *testing.T) {
	raw := mixedCircularSource(t)
	ctx := context.Background()
	m, e := ReadMixedTopologySolids(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, l := range []Limits{{MaxEntities: 115}, {MaxControlPoints: 155}, {MaxValues: 40}, {MaxBytes: 100}} {
		v, e := ReadMixedTopologySolids(ctx, bytes.NewReader(raw), l)
		if e == nil || len(v.Bodies) != 0 {
			t.Fatal(l, e)
		}
	}
	for _, tc := range []struct {
		local, index int
		value        string
	}{{23, 2, "3"}, {35, 6, "1"}, {9, 3, "3"}, {25, 1, "359"}} {
		doc, e := readPlacedTopologyRecords(bytes.NewReader(raw), Limits{}, true, false, false, false, false, false, false, true)
		if e != nil {
			t.Fatal(e)
		}
		id := m.Bodies[2].LocalToSource[tc.local]
		changed := false
		for i, r := range doc.entities {
			if r.id == id {
				if r.values[tc.index].text == tc.value {
					t.Fatal("ineffective mutation")
				}
				doc.entities[i].values[tc.index].text = tc.value
				changed = true
			}
		}
		if !changed {
			t.Fatal("no target")
		}
		a := topologyAuthor{}
		a.geometry.Entities = []Entity{{Point: &geom.Vec3{}}}
		for _, r := range doc.entities {
			v := make([]string, len(r.values))
			for i, p := range r.values {
				v[i] = p.text
			}
			form := 0
			if r.kind >= 500 || r.kind == 190 || r.kind == 192 {
				form = 1
			}
			a.add(r.kind, form, strings.Join(v, ",")+";")
		}
		b, e := a.file(doc.unit)
		if e != nil {
			t.Fatal(e)
		}
		v, e := ReadMixedTopologySolids(ctx, bytes.NewReader(b), Limits{})
		if e == nil || len(v.Bodies) != 0 {
			t.Fatal(tc, e)
		}
	}
	inputs := mixedCircularInputs()
	inputs[2].Roof = inputs[1].Roof
	var out bytes.Buffer
	if e = WriteMixedTopologySolids(ctx, &out, inputs, .001); e == nil || out.Len() != 0 {
		t.Fatal("invalid payload", e)
	}
}
func TestOCCTMixedCircularMaterial(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_IGES_SOLID")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	var out bytes.Buffer
	if e := WriteMixedTopologySolids(context.Background(), &out, mixedCircularInputs(), .001); e != nil {
		t.Fatal(e)
	}
	for _, raw := range [][]byte{mixedCircularSource(t), out.Bytes()} {
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
		want := [3]float64{(7.0/4 + 96*math.Pi) / v, (-4.0 / 3) / v, (28.0/5 + 18*math.Pi) / v}
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
func FuzzMixedCircularMaterial(f *testing.F) {
	f.Add(mixedCircularSource(f))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 131072 {
			t.Skip()
		}
		_, _ = ReadMixedTopologySolids(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 131072, MaxEntities: 128, MaxControlPoints: 208})
	})
}
