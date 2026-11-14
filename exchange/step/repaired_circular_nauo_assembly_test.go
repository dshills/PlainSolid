package step

import (
	"bytes"
	"os"
	"testing"
)

func TestRepairedCircularNAUOAssembly(t *testing.T) {
	for _, tc := range []struct {
		name string
		uses int
	}{{"occt-circular-nauo.step", 3}, {"occt-circular-nauo-dag.step", 7}} {
		t.Run(tc.name, func(t *testing.T) {
			raw, e := os.ReadFile("testdata/" + tc.name)
			if e != nil {
				t.Fatal(e)
			}
			a, e := ReadRepairedCircularNAUOAssembly(bytes.NewReader(raw), Limits{}, CircularNAUORepairOptions{MaxDistance: 1e-9})
			if e != nil {
				t.Fatal(e)
			}
			if len(a.Parts) != 2 || len(a.Repairs) != 1 || len(a.Occurrences) != tc.uses || len(a.Repairs[0].Source.FailedConditions) != 4 {
				t.Fatal("lost local/shared classification")
			}
			uses := 0
			for _, u := range a.Occurrences {
				if u.Path[len(u.Path)-1].ChildID == a.Repairs[0].Source.Product.DefinitionID {
					uses++
				}
			}
			want := 1
			if tc.uses == 7 {
				want = 3
			}
			if uses != want {
				t.Fatal("circular shared use", uses)
			}
			if _, e = ReadExactDAGPlanarNAUO(bytes.NewReader(raw), Limits{}); e == nil {
				t.Fatal("exact reader widened")
			}
		})
	}
}
func TestRepairedCircularNAUOAssemblyReject(t *testing.T) {
	raw, e := os.ReadFile("testdata/occt-circular-nauo-dag.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"tolerance", "faces", "vertices", "uses", "family", "wire", "ownership", "axis", "orphan"} {
		t.Run(kind, func(t *testing.T) {
			a, e := ReadRepairedCircularNAUOAssembly(bytes.NewReader(raw), Limits{}, CircularNAUORepairOptions{MaxDistance: 1e-9})
			if e != nil {
				t.Fatal(e)
			}
			f := a.Source
			l := Limits{}
			o := CircularNAUORepairOptions{MaxDistance: 1e-9}
			switch kind {
			case "tolerance":
				o.MaxDistance = 1e-14
			case "faces":
				l.MaxFaces = 8
			case "vertices":
				l.MaxVertices = 11
			case "uses":
				l.MaxEdgeUses = 35
			case "family":
				en := f.Entities[a.Repairs[0].Source.CylinderID]
				en.Records[0].Type = "SPHERICAL_SURFACE"
				f.Entities[en.ID] = en
			case "wire", "ownership":
				p := a.Parts[0]
				rep := f.Entities[p.Product.Representations[0]]
				for i, v := range rep.Records[0].Args[1].Items {
					if v.Ref == p.SourceBodyID {
						if kind == "ownership" {
							rep.Records[0].Args[1].Items[i].Ref = a.Parts[1].SourceBodyID
						} else {
							edge := f.Entities[a.Repairs[0].Source.SeamEdgeID].Records[0]
							sc := f.Entities[edge.Args[3].Ref].Records[0]
							rep.Records[0].Args[1].Items[i].Ref = sc.Args[1].Ref
						}
					}
				}
				f.Entities[rep.ID] = rep
			case "axis":
				frame := f.Entities[a.Occurrences[0].Path[0].ToPlacementID].Records[0]
				en := f.Entities[frame.Args[3].Ref]
				en.Records[0].Args[1].Items[2].Text = "0.00000000000000000001"
				f.Entities[en.ID] = en
			case "orphan":
				en := f.Entities[a.Parts[0].SourceBodyID]
				en.ID = f.Order[len(f.Order)-1] + 1
				f.Entities[en.ID] = en
				f.Order = append(f.Order, en.ID)
			}
			b, e := mappedPlanarSourceBytes(f)
			if e != nil {
				t.Fatal(e)
			}
			got, e := ReadRepairedCircularNAUOAssembly(bytes.NewReader(b), l, o)
			if e == nil || got.Source != nil || got.Parts != nil || got.Repairs != nil {
				t.Fatal("non-atomic mixed rejection", e)
			}
		})
	}
}
func FuzzRepairedCircularNAUOAssembly(f *testing.F) {
	for _, name := range []string{"occt-circular-nauo.step", "occt-circular-nauo-dag.step", "occt-tube-nauo-dag.step"} {
		b, e := os.ReadFile("testdata/" + name)
		if e != nil {
			f.Fatal(e)
		}
		f.Add(b)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		a, e := ReadRepairedCircularNAUOAssembly(bytes.NewReader(b), Limits{MaxBytes: 1 << 20, MaxEntities: 1000, MaxFaces: 40, MaxVertices: 40, MaxEdgeUses: 160}, CircularNAUORepairOptions{MaxDistance: 1e-9})
		if e != nil && (a.Source != nil || len(a.Parts) != 0 || len(a.Repairs) != 0 || len(a.TubeRepairs) != 0) {
			t.Fatal("partial mixed proof")
		}
	})
}
