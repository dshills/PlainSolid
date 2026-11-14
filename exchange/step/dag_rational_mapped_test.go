package step

import (
	"bytes"
	"os"
	"testing"
)

func TestDAGRationalMapped(t *testing.T) {
	raw, e := os.ReadFile("testdata/occt-dag-rational-mapped.step")
	if e != nil {
		t.Fatal(e)
	}
	a, e := ReadDAGRationalMappedPlanarMaterials(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(a.Bodies) != 2 || len(a.Occurrences) != 11 {
		t.Fatal(a)
	}
	counts := map[int]int{}
	for _, u := range a.Occurrences {
		counts[len(u.Path)]++
	}
	if counts[3] != 6 || counts[2] != 5 {
		t.Fatal(counts)
	}
	if a.Occurrences[0].Path[2] != a.Occurrences[6].Path[1] || a.Occurrences[0].Path[2] != a.Occurrences[8].Path[1] {
		t.Fatal("shared leaf ancestry")
	}
	var out bytes.Buffer
	if e = WriteDAGRationalMappedPlanarSource(&out, a.Source, Limits{}); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadTwoLevelRationalMappedPlanarMaterials(bytes.NewReader(out.Bytes()), Limits{}); e == nil {
		t.Fatal("fixed hierarchy widened")
	}
}

func TestDAGRationalMappedRejectAndDepth(t *testing.T) {
	raw, e := os.ReadFile("testdata/occt-dag-rational-mapped.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"cycle", "orphan", "duplicate", "four-steps", "five-steps", "limit"} {
		t.Run(kind, func(t *testing.T) {
			a, e := ReadDAGRationalMappedPlanarMaterials(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			f := a.Source
			l := Limits{}
			var sdrID, rootID, next int64
			for _, id := range f.Order {
				if id > next {
					next = id
				}
				en := f.Entities[id]
				if len(en.Records) == 1 && en.Records[0].Type == "SHAPE_DEFINITION_REPRESENTATION" {
					sdrID = id
					rootID = en.Records[0].Args[1].Ref
				}
			}
			add := func(kind string, args ...Value) int64 {
				next++
				f.Order = append(f.Order, next)
				f.Entities[next] = Entity{ID: next, Records: []Record{{Type: kind, Args: args}}}
				return next
			}
			rv := func(id int64) Value { return Value{Kind: Reference, Ref: id} }
			switch kind {
			case "cycle":
				id := a.Occurrences[0].Path[0].MapID
				en := f.Entities[id]
				en.Records[0].Args[1] = rv(rootID)
				f.Entities[id] = en
			case "orphan":
				add("DIRECTION", Value{Kind: String}, Value{Kind: Aggregate, Items: []Value{{Kind: Number, Text: "1"}, {Kind: Number, Text: "0"}, {Kind: Number, Text: "0"}}})
			case "duplicate":
				en := f.Entities[rootID]
				en.Records[0].Args[1].Items[1] = en.Records[0].Args[1].Items[0]
				f.Entities[rootID] = en
			case "limit":
				l.MaxEntities = 10
			default:
				n := 1
				if kind == "five-steps" {
					n = 2
				}
				path := a.Occurrences[0].Path[0]
				origin := f.Entities[path.MapID].Records[0].Args[0]
				ctx := f.Entities[rootID].Records[0].Args[2]
				for i := 0; i < n; i++ {
					mid := add("REPRESENTATION_MAP", origin, rv(rootID))
					item := add("MAPPED_ITEM", Value{Kind: String, Text: "extra"}, rv(mid), rv(path.TargetID))
					rootID = add("SHAPE_REPRESENTATION", Value{Kind: String, Text: "wrapped"}, Value{Kind: Aggregate, Items: []Value{rv(item)}}, ctx)
				}
				sd := f.Entities[sdrID]
				sd.Records[0].Args[1] = rv(rootID)
				f.Entities[sdrID] = sd
			}
			b, e := mappedPlanarSourceBytes(f)
			if e != nil {
				t.Fatal(e)
			}
			out, e := ReadDAGRationalMappedPlanarMaterials(bytes.NewReader(b), l)
			if kind == "four-steps" {
				if e != nil || len(out.Occurrences) != 11 {
					t.Fatal(e)
				}
				return
			}
			if e == nil || out.Source != nil || len(out.Occurrences) != 0 {
				t.Fatal("accepted", e)
			}
		})
	}
}
func FuzzDAGRationalMapped(f *testing.F) {
	b, e := os.ReadFile("testdata/occt-dag-rational-mapped.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		_, _ = ReadDAGRationalMappedPlanarMaterials(bytes.NewReader(b), Limits{MaxBytes: 1 << 20, MaxEntities: 1500, MaxFaces: 32, MaxVertices: 64, MaxEdgeUses: 256})
	})
}
