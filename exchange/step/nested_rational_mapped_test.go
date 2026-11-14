package step

import (
	"bytes"
	"math/big"
	"os"
	"plainsolid/geom"
	"testing"
)

func TestNestedRationalMapped(t *testing.T) {
	raw, e := os.ReadFile("testdata/occt-nested-rational-mapped.step")
	if e != nil {
		t.Fatal(e)
	}
	a, e := ReadNestedRationalMappedPlanarMaterials(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(a.Bodies) != 2 || len(a.Occurrences) != 6 {
		t.Fatal(a)
	}
	first, last := a.Occurrences[0], a.Occurrences[3]
	if first.Path[1] != last.Path[1] || first.Path[0] == last.Path[0] || first.Occurrence.MapID != last.Occurrence.MapID {
		t.Fatal("lost source ancestry")
	}
	q, e := last.Occurrence.ApplyExact(geom.Vec3{X: 2})
	if e != nil || q[0].Cmp(big.NewRat(2636, 25)) != 0 || q[1].Cmp(big.NewRat(248, 25)) != 0 {
		t.Fatal(q, e)
	}
	var out bytes.Buffer
	if e = WriteNestedRationalMappedPlanarSource(&out, a.Source, Limits{}); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadNestedRationalMappedPlanarMaterials(&out, Limits{}); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadRationalMappedPlanarMaterials(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("flat reader widened")
	}
}

func TestNestedRationalMappedReject(t *testing.T) {
	raw, e := os.ReadFile("testdata/occt-nested-rational-mapped.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"cycle", "duplicate", "context", "orphan", "scale", "limits"} {
		t.Run(kind, func(t *testing.T) {
			a, e := ReadNestedRationalMappedPlanarMaterials(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			f := a.Source
			path := a.Occurrences[0].Path[0]
			l := Limits{}
			switch kind {
			case "cycle":
				en := f.Entities[path.MapID]
				en.Records[0].Args[1] = Value{Kind: Reference, Ref: path.MapID}
				f.Entities[path.MapID] = en
			case "duplicate":
				for _, id := range f.Order {
					en := f.Entities[id]
					if len(en.Records) == 1 && en.Records[0].Type == "SHAPE_DEFINITION_REPRESENTATION" {
						rootID := en.Records[0].Args[1].Ref
						root := f.Entities[rootID]
						root.Records[0].Args[1].Items[1] = root.Records[0].Args[1].Items[0]
						f.Entities[rootID] = root
						break
					}
				}
			case "context":
				en := f.Entities[path.RepresentationID]
				en.Records[0].Args[2] = Value{Kind: Reference, Ref: path.TargetID}
				f.Entities[path.RepresentationID] = en
			case "orphan":
				id := f.Order[len(f.Order)-1] + 1
				f.Order = append(f.Order, id)
				f.Entities[id] = Entity{ID: id, Records: []Record{{Type: "DIRECTION", Args: []Value{{Kind: String}, {Kind: Aggregate, Items: []Value{{Kind: Number, Text: "1"}, {Kind: Number, Text: "0"}, {Kind: Number, Text: "0"}}}}}}}
			case "scale":
				en := f.Entities[path.TargetID]
				en.Records[0].Args[6] = Value{Kind: Number, Text: "2"}
				f.Entities[path.TargetID] = en
			case "limits":
				l.MaxEntities = 10
			}
			data, e := mappedPlanarSourceBytes(f)
			if e != nil {
				t.Fatal(e)
			}
			got, e := ReadNestedRationalMappedPlanarMaterials(bytes.NewReader(data), l)
			if e == nil || got.Source != nil || len(got.Occurrences) != 0 {
				t.Fatal("accepted", e)
			}
		})
	}
}

func FuzzNestedRationalMapped(f *testing.F) {
	b, e := os.ReadFile("testdata/occt-nested-rational-mapped.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		_, _ = ReadNestedRationalMappedPlanarMaterials(bytes.NewReader(b), Limits{MaxBytes: 1 << 20, MaxEntities: 1500, MaxFaces: 32, MaxVertices: 64, MaxEdgeUses: 256})
	})
}
