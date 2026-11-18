package step

import (
	"bytes"
	"math/big"
	"os"
	"plainsolid/geom"
	"testing"
)

func TestTwoLevelRationalMapped(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/occt-two-level-rational-mapped.step")
	if e != nil {
		t.Fatal(e)
	}
	a, e := ReadTwoLevelRationalMappedPlanarMaterials(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(a.Bodies) != 2 || len(a.Occurrences) != 12 {
		t.Fatal(a)
	}
	first, last := a.Occurrences[0], a.Occurrences[6]
	if first.Path[0] == last.Path[0] || first.Path[1] != last.Path[1] || first.Path[2] != last.Path[2] {
		t.Fatal("ancestry")
	}
	q, e := a.Occurrences[9].Occurrence.ApplyExact(geom.Vec3{X: 2})
	if e != nil || q[0].Cmp(big.NewRat(200, 1)) != 0 || q[1].Cmp(big.NewRat(2636, 25)) != 0 || q[2].Cmp(big.NewRat(248, 25)) != 0 {
		t.Fatal(q, e)
	}
	var out bytes.Buffer
	if e = WriteTwoLevelRationalMappedPlanarSource(&out, a.Source, Limits{}); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadNestedRationalMappedPlanarMaterials(bytes.NewReader(out.Bytes()), Limits{}); e == nil {
		t.Fatal("one-level widened")
	}
	if _, e = ReadRationalMappedPlanarMaterials(bytes.NewReader(out.Bytes()), Limits{}); e == nil {
		t.Fatal("flat widened")
	}
}

func TestTwoLevelRationalMappedReject(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/occt-two-level-rational-mapped.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"cycle", "context", "expansion", "limit"} {
		t.Run(kind, func(t *testing.T) {
			a, e := ReadTwoLevelRationalMappedPlanarMaterials(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			f := a.Source
			p := a.Occurrences[0].Path
			l := Limits{}
			switch kind {
			case "cycle":
				en := f.Entities[p[0].MapID]
				en.Records[0].Args[1] = Value{Kind: Reference, Ref: p[0].MapID}
				f.Entities[p[0].MapID] = en
			case "context":
				en := f.Entities[p[0].RepresentationID]
				en.Records[0].Args[2] = Value{Kind: Reference, Ref: p[0].TargetID}
				f.Entities[p[0].RepresentationID] = en
			case "limit":
				l.MaxEntities = 10
			case "expansion":
				var rootID, next int64
				for _, id := range f.Order {
					if id > next {
						next = id
					}
					en := f.Entities[id]
					if len(en.Records) == 1 && en.Records[0].Type == "SHAPE_DEFINITION_REPRESENTATION" {
						rootID = en.Records[0].Args[1].Ref
					}
				}
				add := func(original int64) int64 {
					next++
					en := f.Entities[original]
					en.ID = next
					f.Entities[next] = en
					f.Order = append(f.Order, next)
					return next
				}
				root := f.Entities[rootID]
				for len(root.Records[0].Args[1].Items) < 16 {
					root.Records[0].Args[1].Items = append(root.Records[0].Args[1].Items, Value{Kind: Reference, Ref: add(p[0].ItemID)})
				}
				f.Entities[rootID] = root
				group := f.Entities[p[0].RepresentationID]
				group.Records[0].Args[1].Items = append(group.Records[0].Args[1].Items, Value{Kind: Reference, Ref: add(p[1].ItemID)})
				f.Entities[p[0].RepresentationID] = group
			}
			data, e := mappedPlanarSourceBytes(f)
			if e != nil {
				t.Fatal(e)
			}
			out, e := ReadTwoLevelRationalMappedPlanarMaterials(bytes.NewReader(data), l)
			if e == nil || out.Source != nil || len(out.Occurrences) != 0 {
				t.Fatal("accepted", e)
			}
		})
	}
}

func FuzzTwoLevelRationalMapped(f *testing.F) {
	b, e := os.ReadFile("testdata/occt-two-level-rational-mapped.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		_, _ = ReadTwoLevelRationalMappedPlanarMaterials(bytes.NewReader(b), Limits{MaxBytes: 1 << 20, MaxEntities: 1500, MaxFaces: 32, MaxVertices: 64, MaxEdgeUses: 256})
	})
}
