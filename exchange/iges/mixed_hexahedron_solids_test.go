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
	"reflect"
	"testing"
	"time"
)

func threeFamilyInputs() []MixedTopologyInput {
	t := mixedTopologyInputs()[0]
	second := t
	for i, p := range second.Vertices {
		second.Vertices[i] = p.Add(geom.Vec3{X: 10})
	}
	h := MixedTopologyInput{Family: MixedTopologyHexahedron}
	for i := range h.HexVertices {
		p := geom.Vec3{X: 3, Y: 4, Z: 5}
		if i&1 != 0 {
			p = p.Add(geom.Vec3{X: -2, Y: -1})
		}
		if i&2 != 0 {
			p = p.Add(geom.Vec3{Y: 3, Z: 1})
		}
		if i&4 != 0 {
			p = p.Add(geom.Vec3{X: 1, Z: 2})
		}
		h.HexVertices[i] = p
	}
	return []MixedTopologyInput{t, second, h, mixedTopologyInputs()[1]}
}
func threeFamilySource(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/three-family-topology-solids.igs")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestMixedHexahedronSolids(t *testing.T) {
	var out bytes.Buffer
	if e := WriteMixedTopologySolids(context.Background(), &out, threeFamilyInputs(), .001); e != nil {
		t.Fatal(e)
	}
	for _, raw := range [][]byte{threeFamilySource(t), out.Bytes()} {
		a, e := ReadMixedTopologySolids(context.Background(), bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		again, e := ReadMixedTopologySolids(context.Background(), bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if len(a.Bodies) != 4 {
			t.Fatal(a)
		}
		want := []MixedTopologyFamily{MixedTopologyTetrahedron, MixedTopologyTetrahedron, MixedTopologyHexahedron, MixedTopologyRoof}
		seen := map[int]bool{}
		for i, b := range a.Bodies {
			if b.Family != want[i] || !reflect.DeepEqual(b.LocalToSource, again.Bodies[i].LocalToSource) || b.SourceSolidID != again.Bodies[i].SourceSolidID {
				t.Fatal("deterministic provenance", i)
			}
			for _, id := range b.LocalToSource {
				if seen[id] {
					t.Fatal("shared source ID")
				}
				seen[id] = true
			}
			if b.Family == MixedTopologyHexahedron {
				if b.Hexahedron == nil || b.Roof != nil || b.Tetrahedron != nil || len(b.LocalToSource) != 58 {
					t.Fatal(b)
				}
			}
		}
		if len(seen) != 184 {
			t.Fatal(len(seen))
		}
	}
	pair := threeFamilyInputs()
	out.Reset()
	if e := WriteMixedTopologySolids(context.Background(), &out, []MixedTopologyInput{pair[2], pair[0]}, 1); e != nil {
		t.Fatal(e)
	}
	a, e := ReadMixedTopologySolids(context.Background(), &out, Limits{})
	if e != nil || a.Bodies[0].Family != MixedTopologyHexahedron || a.LengthUnit != 1 {
		t.Fatal(e)
	}
}
func TestMixedHexahedronRejects(t *testing.T) {
	b := threeFamilySource(t)
	for _, l := range []Limits{{MaxEntities: 183}, {MaxControlPoints: 303}, {MaxBytes: 100}} {
		a, e := ReadMixedTopologySolids(context.Background(), bytes.NewReader(b), l)
		if e == nil || len(a.Bodies) != 0 {
			t.Fatal(l, e)
		}
	}
	// Find an actual hexahedral support and corrupt one corner pole. Bilinear
	// family classification still succeeds, but the exact affine/material gate fails.
	doc, e := readTopologySource(bytes.NewReader(b), Limits{}, true)
	if e != nil {
		t.Fatal(e)
	}
	changed := false
	for i, r := range doc.entities {
		if r.kind == 128 && r.id > 136 && r.id < 253 {
			doc.entities[i].values[22].text = "3.25"
			changed = true
			break
		}
	}
	if !changed {
		t.Fatal("no hexahedral support")
	}
	ids := map[int]int{}
	for _, r := range doc.entities {
		ids[r.id] = r.id
	}
	raw, e := remapTopologyRecords(doc.entities, ids, doc.unit)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadMixedTopologySolids(context.Background(), bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("nonaffine bilinear accepted")
	}
	inputs := threeFamilyInputs()
	inputs[2].Roof = inputs[3].Roof
	var out bytes.Buffer
	if e = WriteMixedTopologySolids(context.Background(), &out, inputs, .001); e == nil || out.Len() != 0 {
		t.Fatal("unused payload", e)
	}
}
func TestOCCTThreeFamilyTopology(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_IGES_SOLID")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	var b bytes.Buffer
	if e := WriteMixedTopologySolids(context.Background(), &b, threeFamilyInputs(), .001); e != nil {
		t.Fatal(e)
	}
	for _, raw := range [][]byte{threeFamilySource(t), b.Bytes()} {
		path := filepath.Join(t.TempDir(), "three.igs")
		if e := os.WriteFile(path, raw, 0600); e != nil {
			t.Fatal(e)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		out, e := exec.CommandContext(ctx, bin, path).CombinedOutput()
		cancel()
		if e != nil {
			t.Fatal(e, string(out))
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
		area := 2*(6+2*math.Sqrt(3)) + 20.0/3 + math.Sqrt(5)/2 + math.Asinh(2)/4 + 2*(math.Sqrt(41)+math.Sqrt(46)+math.Sqrt(21))
		if p.Faces != 20 || p.Solids != 4 || math.Abs(p.Volume-17) > 1e-9 || math.Abs(p.Area-area) > 1e-8 || math.Abs(p.Center[0]-193.0/68) > 1e-9 || math.Abs(p.Center[1]-193.0/51) > 1e-9 || math.Abs(p.Center[2]-2723.0/510) > 1e-9 {
			t.Fatal(p)
		}
	}
}
func FuzzThreeFamilyTopology(f *testing.F) {
	f.Add(threeFamilySource(f))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 131072 {
			t.Skip()
		}
		_, _ = ReadMixedTopologySolids(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 131072, MaxEntities: 232, MaxControlPoints: 416})
	})
}
