package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/geom"
	"testing"
	"time"
)

func mixedEccentricInputs() []MixedTopologyInput {
	b := eccentricBlindDescriptor()
	b.Origin = geom.Vec3{X: -10}
	u := eccentricThroughDescriptor()
	u.Origin = geom.Vec3{X: 10}
	return []MixedTopologyInput{mixedTopologyInputs()[0], {Family: MixedTopologyEccentricBlind, EccentricBlind: b}, {Family: MixedTopologyEccentricThrough, EccentricThrough: u}}
}
func mixedEccentricSource(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/mixed-eccentric-material.igs")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestMixedEccentricMaterial(t *testing.T) {
	ctx := context.Background()
	var out bytes.Buffer
	if e := WriteMixedTopologySolids(ctx, &out, mixedEccentricInputs(), .001); e != nil {
		t.Fatal(e)
	}
	for _, raw := range [][]byte{mixedEccentricSource(t), out.Bytes()} {
		m, e := ReadMixedTopologySolids(ctx, bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if len(m.Bodies) != 3 || m.Bodies[1].Family != MixedTopologyEccentricBlind || m.Bodies[2].Family != MixedTopologyEccentricThrough {
			t.Fatal(m)
		}
		b, u := m.Bodies[1], m.Bodies[2]
		if b.EccentricBlind == nil || u.EccentricThrough == nil || b.EccentricBlind.Descriptor != mixedEccentricInputs()[1].EccentricBlind || u.EccentricThrough.Descriptor != mixedEccentricInputs()[2].EccentricThrough {
			t.Fatal(m)
		}
		if b.LocalToSource[b.EccentricBlind.SourceSolidID] != b.SourceSolidID || u.LocalToSource[u.EccentricThrough.SourceSolidID] != u.SourceSolidID {
			t.Fatal("root provenance")
		}
		seen := map[int]bool{}
		for _, body := range m.Bodies {
			for _, id := range body.LocalToSource {
				if seen[id] {
					t.Fatal("shared source")
				}
				seen[id] = true
			}
		}
		if len(seen) != 110 {
			t.Fatal(len(seen))
		}
	}
	in := mixedEccentricInputs()
	in = append(in, in[1])
	out.Reset()
	if e := WriteMixedTopologySolids(ctx, &out, in, .001); e != nil {
		t.Fatal(e)
	}
	m, e := ReadMixedTopologySolids(ctx, &out, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	m.Bodies[1].EccentricBlind.Parameters[1][1] = 99
	m.Bodies[1].LocalToSource[1] = 999
	if m.Bodies[3].EccentricBlind.Parameters[1][1] != -10 || m.Bodies[3].LocalToSource[1] == 999 {
		t.Fatal("alias")
	}
}
func TestMixedEccentricMaterialRejects(t *testing.T) {
	ctx := context.Background()
	raw := mixedEccentricSource(t)
	m, e := ReadMixedTopologySolids(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		body, id, index int
		value           string
	}{{1, 69, 5, fmt.Sprint(m.Bodies[2].LocalToSource[59])}, {2, 33, 17, fmt.Sprint(m.Bodies[1].LocalToSource[29])}, {1, 75, 5, "1"}, {2, 13, 3, "2"}} {
		doc, e := readPlacedTopologyRecords(bytes.NewReader(raw), Limits{}, true, false, false, false, false, false, true, true)
		if e != nil {
			t.Fatal(e)
		}
		id := m.Bodies[tc.body].LocalToSource[tc.id]
		for i := range doc.entities {
			if doc.entities[i].id == id {
				doc.entities[i].values[tc.index].text = tc.value
			}
		}
		b, e := mixedRoofFile(doc)
		if e != nil {
			t.Fatal(e)
		}
		got, e := ReadMixedTopologySolids(ctx, bytes.NewReader(b), Limits{})
		if e == nil || len(got.Bodies) != 0 {
			t.Fatal(tc, e)
		}
	}
	for _, l := range []Limits{{MaxEntities: 109}, {MaxControlPoints: 51}, {MaxBytes: 100}, {MaxValues: 20}} {
		if _, e := ReadMixedTopologySolids(ctx, bytes.NewReader(raw), l); e == nil {
			t.Fatal(l)
		}
	}
	in := mixedEccentricInputs()
	in[1].EccentricThrough = in[2].EccentricThrough
	var out bytes.Buffer
	if e := WriteMixedTopologySolids(ctx, &out, in, .001); e == nil || out.Len() != 0 {
		t.Fatal("unused payload")
	}
}
func FuzzMixedEccentricMaterial(f *testing.F) {
	f.Add(mixedEccentricSource(f))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 131072 {
			t.Skip()
		}
		_, _ = ReadMixedTopologySolids(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 131072, MaxEntities: 128, MaxControlPoints: 64})
	})
}
func TestOCCTMixedEccentricMaterial(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_IGES_SOLID")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	var out bytes.Buffer
	if e := WriteMixedTopologySolids(context.Background(), &out, mixedEccentricInputs(), .001); e != nil {
		t.Fatal(e)
	}
	for _, raw := range [][]byte{mixedEccentricSource(t), out.Bytes()} {
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
		v := 4.0/3 + 66*math.Pi
		area := 6 + 2*math.Sqrt(3) + 94*math.Pi
		want := [3]float64{(2.0/3 - 26*math.Pi) / v, (2.0/3 - 3*math.Pi) / v, (2.0/3 + 130*math.Pi) / v}
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

func TestMixedEccentricBlindClassifier(t *testing.T) {
	ctx := context.Background()
	raw := mixedEccentricSource(t)
	m, e := ReadMixedTopologySolids(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, rounded := range []bool{false, true} {
		doc, e := readPlacedTopologyRecords(bytes.NewReader(raw), Limits{}, true, false, false, false, false, false, true, true)
		if e != nil {
			t.Fatal(e)
		}
		for i := range doc.entities {
			if doc.entities[i].id == m.Bodies[1].LocalToSource[5] {
				if rounded {
					doc.entities[i].values[1].text = "-9.00000000000000001"
				} else {
					doc.entities[i].values[1].text = "-10"
					doc.entities[i].values[2].text = "0"
				}
			}
		}
		b, e := mixedRoofFile(doc)
		if e != nil {
			t.Fatal(e)
		}
		got, e := ReadMixedTopologySolids(ctx, bytes.NewReader(b), Limits{})
		if e == nil || len(got.Bodies) != 0 {
			t.Fatal("ambiguous/rounded family admitted")
		}
	}
}
