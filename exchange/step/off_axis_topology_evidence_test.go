package step

import (
	"bytes"
	"fmt"
	"os"
	"testing"
)

func TestOffAxisTopologyEvidence(t *testing.T) {
	b, e := os.ReadFile("testdata/off-axis-sphere-bore-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	r, e := ValidateOffAxisBoreTopology(bytes.NewReader(b), Limits{}, 15)
	if e != nil {
		t.Fatal(e)
	}
	if r.Vertices != 3 || r.Edges != 4 || r.EulerCharacteristic != 2 || len(r.Faces) != 3 || len(r.EdgeCurves) != 4 || r.Intersection.EdgeID != 21 {
		t.Fatal(r)
	}
	want := []int{3, 4, 1}
	for i, f := range r.Faces {
		if len(f.Bounds) != 1 || len(f.Bounds[0].Uses) != want[i] {
			t.Fatal(f)
		}
	}
}
func TestOffAxisTopologyEvidenceRejects(t *testing.T) {
	b, e := os.ReadFile("testdata/off-axis-sphere-bore-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, mutate := range []func(*File){
		func(f *File) {
			en := f.Entities[571]
			en.Records[0].Args[1] = Value{Kind: Reference, Ref: 376}
			f.Entities[571] = en
		},
		func(f *File) {
			en := f.Entities[622]
			en.Records[0].Args = en.Records[0].Args[:1]
			f.Entities[622] = en
		},
		func(f *File) {
			en := f.Entities[579]
			en.Records[0].Args[3] = Value{Kind: Enumeration, Text: "T"}
			f.Entities[579] = en
		},

		func(f *File) {
			en := f.Entities[581]
			en.Records[0].Args[1].Items[0], en.Records[0].Args[1].Items[1] = en.Records[0].Args[1].Items[1], en.Records[0].Args[1].Items[0]
			f.Entities[581] = en
		},
		func(f *File) {
			en := f.Entities[578]
			en.Records[0].Args[4] = Value{Kind: Enumeration, Text: "F"}
			f.Entities[578] = en
		},
		func(f *File) {
			en := f.Entities[555]
			en.Records[0].Args[1] = en.Records[0].Args[2]
			f.Entities[555] = en
		},
		func(f *File) {
			en := f.Entities[16]
			en.Records[0].Args[1].Items[2] = en.Records[0].Args[1].Items[0]
			f.Entities[16] = en
		},
		func(f *File) {
			en := f.Entities[637]
			en.ID = 10000
			f.Entities[10000] = en
			f.Order = append(f.Order, 10000)
		},
	} {
		f, e := Parse(bytes.NewReader(b), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		mutate(f)
		if _, e = ValidateOffAxisBoreTopology(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, 15); e == nil {
			t.Fatal("accepted invalid topology")
		}
	}
	if _, e = ValidateOffAxisBoreTopology(bytes.NewReader(b), Limits{MaxEdgeUses: 7}, 15); e == nil {
		t.Fatal("ignored use budget")
	}
}
func FuzzOffAxisTopologyEvidence(f *testing.F) {
	b, _ := os.ReadFile("testdata/off-axis-sphere-bore-occt.step")
	f.Add(b)
	f.Add([]byte("bad"))
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ValidateOffAxisBoreTopology(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 1024, MaxValues: 16384, MaxEdgeUses: 256}, 15)
	})
}

func TestOffAxisBoreEvidenceBundle(t *testing.T) {
	b, e := os.ReadFile("testdata/off-axis-sphere-bore-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	r, e := InspectOffAxisBoreEvidence(bytes.NewReader(b), Limits{}, 15, 1e-6, PolynomialWindingOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if !r.WithinRequestedTolerance || r.Winding.Turns != 1 || r.RimEndpointSquaredGaps[0].Sign() <= 0 || r.RimEndpointSquaredGaps[0].Cmp(r.RimEndpointSquaredGaps[1]) != 0 {
		t.Fatal("missing independent evidence")
	}
}

func TestOffAxisTopologyRenumbered(t *testing.T) {
	raw, e := os.ReadFile("testdata/off-axis-sphere-bore-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	ids := map[int64]int64{}
	for _, id := range f.Order {
		ids[id] = id + 10000
	}
	var b bytes.Buffer
	b.Write(raw[:bytes.Index(raw, []byte("DATA;"))+5])
	for _, id := range f.Order {
		fmt.Fprintf(&b, "#%d=%s;\n", ids[id], assemblyEntity(f.Entities[id], ids))
	}
	b.WriteString("ENDSEC;END-ISO-10303-21;")
	out, e := ValidateOffAxisBoreTopology(&b, Limits{}, 10015)
	if e != nil || out.Intersection.EdgeID != 10021 || out.ShellID != 10016 {
		t.Fatal(out, e)
	}
}
