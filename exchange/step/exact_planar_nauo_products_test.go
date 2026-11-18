package step

import (
	"bytes"
	"math/big"
	"os"
	"plainsolid/geom"
	"testing"
)

func TestExactPlanarNAUOProducts(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/occt-exact-nauo-products.step")
	if e != nil {
		t.Fatal(e)
	}
	a, e := ReadExactPlanarNAUOProducts(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if a.Parts[0].Product.Name != "Exact planar part" || a.Root.Name != "Mixed planar NAUO assembly" || a.LengthUnit != .001 || len(a.Occurrences) != 3 {
		t.Fatal(a)
	}
	q, e := a.Occurrences[1].ApplyExact(geom.Vec3{X: 1, Y: 2, Z: 3})
	if e != nil || q[0].Cmp(big.NewRat(3, 1)) != 0 || q[1].Cmp(big.NewRat(21, 1)) != 0 || q[2].Cmp(big.NewRat(2, 1)) != 0 {
		t.Fatal(q, e)
	}
	if len(a.Parts) != 2 || a.Parts[0].SourceBodyID == a.Parts[1].SourceBodyID {
		t.Fatal("local ownership")
	}
	if _, e := ReadExactPlanarNAUO(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("legacy reader widened")
	}
	var out bytes.Buffer
	if e = WriteExactPlanarNAUOProductsSource(&out, a.Source, Limits{}); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadExactPlanarNAUOProducts(&out, Limits{}); e != nil {
		t.Fatal(e)
	}
}

func TestExactPlanarNAUOProductsReject(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/occt-exact-nauo-products.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"placement-owner", "axis", "unit", "pcurve", "orphan", "limit", "cross-body", "child-owner", "vertex-limit", "use-limit"} {
		t.Run(kind, func(t *testing.T) {
			a, e := ReadExactPlanarNAUOProducts(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			f := a.Source
			l := Limits{}
			switch kind {
			case "placement-owner":
				id := a.Occurrences[0].TransformationID
				en := f.Entities[id]
				en.Records[0].Args[2] = Value{Kind: Reference, Ref: a.Occurrences[0].ToPlacementID}
				f.Entities[id] = en
			case "axis":
				frame := f.Entities[a.Occurrences[0].ToPlacementID].Records[0]
				id := frame.Args[3].Ref
				en := f.Entities[id]
				en.Records[0].Args[1].Items[2] = Value{Kind: Number, Text: "0.0000000000000001"}
				f.Entities[id] = en
			case "unit":
				rep := f.Entities[a.Parts[0].Product.Representations[0]].Records[0]
				for _, r := range f.Entities[rep.Args[2].Ref].Records {
					if r.Type == "GLOBAL_UNIT_ASSIGNED_CONTEXT" {
						for _, v := range r.Args[0].Items {
							en := f.Entities[v.Ref]
							length := false
							for _, u := range en.Records {
								if u.Type == "LENGTH_UNIT" {
									length = true
								}
							}
							if length {
								for i := range en.Records {
									if en.Records[i].Type == "SI_UNIT" {
										en.Records[i].Args[0] = Value{Kind: Enumeration, Text: "KILO"}
									}
								}
								f.Entities[v.Ref] = en
							}
						}
					}
				}
			case "pcurve":
				for _, id := range f.Order {
					en := f.Entities[id]
					if len(en.Records) == 1 && en.Records[0].Type == "PCURVE" {
						rep := f.Entities[en.Records[0].Args[2].Ref].Records[0]
						line := f.Entities[rep.Args[1].Items[0].Ref].Records[0]
						point := f.Entities[line.Args[1].Ref]
						point.Records[0].Args[1].Items[0] = Value{Kind: Number, Text: "99"}
						point.Records[0].Args[1].Items[1] = Value{Kind: Number, Text: "99"}
						f.Entities[point.ID] = point
						break
					}
				}
			case "orphan":
				id := f.Order[len(f.Order)-1] + 1
				en := f.Entities[a.Parts[0].SourceBodyID]
				en.ID = id
				f.Entities[id] = en
				f.Order = append(f.Order, id)
			case "cross-body":
				id := a.Parts[1].Product.Representations[0]
				en := f.Entities[id]
				for i, v := range en.Records[0].Args[1].Items {
					if v.Ref == a.Parts[1].SourceBodyID {
						en.Records[0].Args[1].Items[i].Ref = a.Parts[0].SourceBodyID
					}
				}
				f.Entities[id] = en
			case "child-owner":
				en := f.Entities[a.Occurrences[2].NAUOID]
				en.Records[0].Args[4].Ref = a.Parts[0].Product.DefinitionID
				f.Entities[en.ID] = en
			case "vertex-limit":
				l.MaxVertices = 15
			case "use-limit":
				l.MaxEdgeUses = 47
			case "limit":
				l.MaxFaces = 11
			}
			b, e := mappedPlanarSourceBytes(f)
			if e != nil {
				t.Fatal(e)
			}
			var output bytes.Buffer
			if e := WriteExactPlanarNAUOProductsSource(&output, f, l); e == nil || output.Len() != 0 {
				t.Fatal("non-atomic invalid source write", e)
			}
			got, e := ReadExactPlanarNAUOProducts(bytes.NewReader(b), l)
			if e == nil || got.Source != nil || len(got.Occurrences) != 0 {
				t.Fatal("accepted", e)
			}
		})
	}
}
func FuzzExactPlanarNAUOProducts(f *testing.F) {
	b, e := os.ReadFile("testdata/occt-exact-nauo-products.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		_, _ = ReadExactPlanarNAUOProducts(bytes.NewReader(b), Limits{MaxBytes: 1 << 20, MaxEntities: 1000, MaxFaces: 16, MaxVertices: 32, MaxEdgeUses: 128})
	})
}
