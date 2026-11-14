package step

import (
	"bytes"
	"math/big"
	"os"
	"plainsolid/geom"
	"testing"
)

func TestExactDAGPlanarNAUO(t *testing.T) {
	raw, e := os.ReadFile("testdata/occt-exact-dag-nauo.step")
	if e != nil {
		t.Fatal(e)
	}
	a, e := ReadExactDAGPlanarNAUO(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if a.Parts[0].Product.Name != "Exact planar part" || a.Root.Name != "DAG planar NAUO assembly" || a.LengthUnit != .001 || len(a.Occurrences) != 7 {
		t.Fatal(a)
	}
	q, e := a.Occurrences[1].ApplyExact(geom.Vec3{X: 1, Y: 2, Z: 3})
	if e != nil || q[0].Cmp(big.NewRat(103, 1)) != 0 || q[1].Cmp(big.NewRat(21, 1)) != 0 || q[2].Cmp(big.NewRat(2, 1)) != 0 {
		t.Fatal(q, e)
	}
	if len(a.Parts) != 2 || a.Parts[0].SourceBodyID == a.Parts[1].SourceBodyID {
		t.Fatal("local ownership")
	}
	if _, e := ReadExactPlanarNAUO(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("legacy reader widened")
	}
	if len(a.Groups) != 2 || len(a.Relationships) != 7 {
		t.Fatal("group identities")
	}
	for i := 0; i < 3; i++ {
		first, second := a.Occurrences[i].Path, a.Occurrences[i+3].Path
		if len(first) != 2 || len(second) != 3 || first[1].NAUOID != second[2].NAUOID || first[0].NAUOID == second[0].NAUOID {
			t.Fatal("DAG ancestry")
		}
	}
	if a.Occurrences[2].Path[1].ChildID != a.Occurrences[6].Path[1].ChildID {
		t.Fatal("shared leaf")
	}
	if _, e := ReadExactNestedPlanarNAUO(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("single-group reader widened")
	}
	if _, e := ReadExactPlanarNAUOProducts(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("flat reader widened")
	}
	var out bytes.Buffer
	if e = WriteExactDAGPlanarNAUOSource(&out, a.Source, Limits{}); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadExactDAGPlanarNAUO(&out, Limits{}); e != nil {
		t.Fatal(e)
	}
}

func TestExactDAGPlanarNAUOReject(t *testing.T) {
	raw, e := os.ReadFile("testdata/occt-exact-dag-nauo.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"placement-owner", "axis", "unit", "pcurve", "orphan", "limit", "cross-body", "child-owner", "vertex-limit", "use-limit", "cycle", "leaf-parent"} {
		t.Run(kind, func(t *testing.T) {
			a, e := ReadExactDAGPlanarNAUO(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			f := a.Source
			l := Limits{}
			switch kind {
			case "placement-owner":
				id := a.Occurrences[0].Path[0].TransformationID
				en := f.Entities[id]
				en.Records[0].Args[2] = Value{Kind: Reference, Ref: a.Occurrences[0].Path[0].ToPlacementID}
				f.Entities[id] = en
			case "axis":
				frame := f.Entities[a.Occurrences[0].Path[0].ToPlacementID].Records[0]
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
				en := f.Entities[a.Occurrences[2].Path[1].NAUOID]
				en.Records[0].Args[4].Ref = a.Parts[0].Product.DefinitionID
				f.Entities[en.ID] = en
			case "cycle":
				id := a.Occurrences[0].Path[0].NAUOID
				en := f.Entities[id]
				en.Records[0].Args[4].Ref = a.Root.DefinitionID
				f.Entities[id] = en
			case "leaf-parent":
				id := a.Occurrences[0].Path[1].NAUOID
				en := f.Entities[id]
				en.Records[0].Args[3].Ref = a.Parts[0].Product.DefinitionID
				f.Entities[id] = en
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
			if e := WriteExactDAGPlanarNAUOSource(&output, f, l); e == nil || output.Len() != 0 {
				t.Fatal("non-atomic invalid source write", e)
			}
			got, e := ReadExactDAGPlanarNAUO(bytes.NewReader(b), l)
			if e == nil || got.Source != nil || len(got.Occurrences) != 0 {
				t.Fatal("accepted", e)
			}
		})
	}
}
func FuzzExactDAGPlanarNAUO(f *testing.F) {
	b, e := os.ReadFile("testdata/occt-exact-dag-nauo.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		_, _ = ReadExactDAGPlanarNAUO(bytes.NewReader(b), Limits{MaxBytes: 1 << 20, MaxEntities: 1000, MaxFaces: 16, MaxVertices: 32, MaxEdgeUses: 128})
	})
}

// Each duplicate is a complete independent NAUO/CDSR relationship, not an
// invented path. This reaches the expansion cap without hitting direct-use caps.
func TestExactDAGNAUOExpansionBudget(t *testing.T) {
	raw, e := os.ReadFile("testdata/occt-exact-dag-nauo.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, copies := range []int{1, 20} {
		a, e := ReadExactDAGPlanarNAUO(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		f := a.Source
		u := a.Occurrences[0].Path[0]
		pds := f.Entities[u.CDSRID].Records[0].Args[1].Ref
		sourceIDs := []int64{u.NAUOID, pds, u.CDSRID, u.RelationshipID, u.TransformationID}
		next := int64(0)
		for id := range f.Entities {
			if id > next {
				next = id
			}
		}
		for k := 0; k < copies; k++ {
			ids := map[int64]int64{}
			for _, id := range sourceIDs {
				next++
				ids[id] = next
			}
			for _, id := range sourceIDs {
				source := f.Entities[id]
				en := source
				en.ID = ids[id]
				en.Records = append([]Record(nil), source.Records...)
				for i, r := range source.Records {
					en.Records[i].Args = append([]Value(nil), r.Args...)
					for j, v := range en.Records[i].Args {
						if v.Kind == Reference && ids[v.Ref] != 0 {
							en.Records[i].Args[j].Ref = ids[v.Ref]
						}
					}
				}
				f.Entities[en.ID] = en
				f.Order = append(f.Order, en.ID)
			}
		}
		b, e := mappedPlanarSourceBytes(f)
		if e != nil {
			t.Fatal(e)
		}
		got, e := ReadExactDAGPlanarNAUO(bytes.NewReader(b), Limits{})
		if copies == 1 {
			if e != nil || len(got.Occurrences) != 10 {
				t.Fatal("valid extra reuse", e, len(got.Occurrences))
			}
		} else if e == nil || got.Source != nil {
			t.Fatal("expanded budget admitted", e)
		}
	}
}
