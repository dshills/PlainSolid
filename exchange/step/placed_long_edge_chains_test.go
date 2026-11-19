package step

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func placedChainFixture() PlacedLongEdgeChainModel {
	return PlacedLongEdgeChainModel{Model: longEdgeChainFixture(), Transform: geom.Affine{{0, -1, 0, 32}, {0, 0, -1, 16}, {1, 0, 0, -4}}}
}
func TestPlacedLongEdgeChain(t *testing.T) {
	t.Parallel()
	for _, transform := range []geom.Affine{geom.Identity(), placedChainFixture().Transform, {{-1, 0, 0, 2}, {0, -1, 0, 4}, {0, 0, 1, 8}}} {
		m := placedChainFixture()
		m.Transform = transform
		var b bytes.Buffer
		if e := WritePlacedLongEdgeChains(&b, m); e != nil {
			t.Fatal(e)
		}
		got, e := ReadPlacedLongEdgeChains(bytes.NewReader(b.Bytes()), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(got, m) {
			t.Fatalf("changed recipe: %+v", got)
		}
		var again bytes.Buffer
		if e = WritePlacedLongEdgeChains(&again, got); e != nil || !bytes.Equal(b.Bytes(), again.Bytes()) {
			t.Fatal("roundtrip", e)
		}
	}
}
func TestPlacedLongEdgeChainRejects(t *testing.T) {
	t.Parallel()
	m := placedChainFixture()
	for _, a := range []geom.Affine{{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, -1, 0}}, {{1, .1, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}}, {{1, 0, 0, math.Inf(1)}, {0, 1, 0, 0}, {0, 0, 1, 0}}} {
		m.Transform = a
		var b bytes.Buffer
		if WritePlacedLongEdgeChains(&b, m) == nil || b.Len() != 0 {
			t.Fatal("accepted invalid placement")
		}
	}
	m = placedChainFixture()
	var b bytes.Buffer
	if e := WritePlacedLongEdgeChains(&b, m); e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, id := range f.Order {
		en := f.Entities[id]
		if len(en.Records) == 1 && en.Records[0].Type == "ITEM_DEFINED_TRANSFORMATION" {
			en.Records[0].Args[2] = en.Records[0].Args[3]
			f.Entities[id] = en
			break
		}
	}
	if _, e = ReadPlacedLongEdgeChains(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}); e == nil {
		t.Fatal("accepted changed source frame")
	}
	if _, e = ReadPlacedLongEdgeChains(bytes.NewReader(b.Bytes()), Limits{MaxSolids: 1}); e == nil {
		t.Fatal("ignored product limit")
	}
}
func TestPlacedLongEdgeSourceIntegrity(t *testing.T) {
	t.Parallel()
	var b bytes.Buffer
	if e := WritePlacedLongEdgeChains(&b, placedChainFixture()); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"CIRCLE", "PCURVE", "FACE_BOUND", "NEXT_ASSEMBLY_USAGE_OCCURRENCE"} {
		f, e := Parse(bytes.NewReader(b.Bytes()), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		changed := false
		for _, id := range f.Order {
			en := f.Entities[id]
			if len(en.Records) != 1 || en.Records[0].Type != kind {
				continue
			}
			r := &en.Records[0]
			switch kind {
			case "CIRCLE":
				r.Args[2] = Value{Kind: Number, Text: "1.00000000000000000001"}
			case "PCURVE":
				r.Args[1] = r.Args[2]
			case "FACE_BOUND":
				r.Args[2] = Value{Kind: Enumeration, Text: "F"}
			case "NEXT_ASSEMBLY_USAGE_OCCURRENCE":
				r.Args[0] = Value{Kind: String, Text: "different"}
			}
			f.Entities[id] = en
			changed = true
			break
		}
		if !changed {
			t.Fatal("missing", kind)
		}
		if _, e = ReadPlacedLongEdgeChains(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}); e == nil {
			t.Fatal("accepted", kind)
		}
	}
	f, e := Parse(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	mapping := map[int64]int64{}
	for _, id := range f.Order {
		mapping[id] = id + 10000
	}
	var renumbered bytes.Buffer
	renumbered.Write(b.Bytes()[:bytes.Index(b.Bytes(), []byte("DATA;"))+5])
	for _, id := range f.Order {
		fmt.Fprintf(&renumbered, "#%d=%s;\n", mapping[id], assemblyEntity(f.Entities[id], mapping))
	}
	renumbered.WriteString("ENDSEC;END-ISO-10303-21;")
	if _, e = ReadPlacedLongEdgeChains(&renumbered, Limits{}); e != nil {
		t.Fatal("renumbering", e)
	}
}

func TestPlacedLongEdgeChainOCCT(t *testing.T) {
	t.Parallel()
	m := placedChainFixture()
	var b bytes.Buffer
	if e := WritePlacedLongEdgeChains(&b, m); e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(t.TempDir(), "placed.step")
	if e := os.WriteFile(p, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	ref := filepath.Join(t.TempDir(), "reference.step")
	stepOracle(t, ref, "write-step-long-edge-chain")
	type mass struct {
		Area, Volume float64
		Centroid     [3]float64
	}
	var got [2]mass
	for i, path := range []string{ref, p} {
		out := filepath.Join(t.TempDir(), "mass.json")
		stepOracle(t, out, "step-mass-precise", path)
		raw, e := os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		if e = json.Unmarshal(raw, &got[i]); e != nil {
			t.Fatal(e)
		}
	}
	expected := m.Transform.Apply(geom.Vec3{X: got[0].Centroid[0], Y: got[0].Centroid[1], Z: got[0].Centroid[2]})
	if math.Abs(got[0].Volume-got[1].Volume) > 1e-6 || math.Abs(got[0].Area-got[1].Area) > 1e-6 {
		t.Fatalf("mass mismatch %+v", got)
	}
	for i, want := range []float64{expected.X, expected.Y, expected.Z} {
		if math.Abs(want-got[1].Centroid[i]) > 1e-7 {
			t.Fatalf("placement mismatch %+v vs %v", got[1], expected)
		}
	}
}
func FuzzPlacedLongEdgeChain(f *testing.F) {
	var b bytes.Buffer
	_ = WritePlacedLongEdgeChains(&b, placedChainFixture())
	f.Add(b.Bytes())
	f.Add([]byte("bad"))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = ReadPlacedLongEdgeChains(bytes.NewReader(data), Limits{MaxBytes: 128 << 10, MaxEntities: 5000, MaxValues: 40000, MaxVertices: 256, MaxFaces: 64, MaxSolids: 2, MaxEdgeUses: 1024})
	})
}
