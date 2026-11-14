package step

import (
	"bytes"
	"os"
	"testing"
)

func TestRepairedTubeNAUOAssembly(t *testing.T) {
	raw, e := os.ReadFile("testdata/occt-tube-nauo-dag.step")
	if e != nil {
		t.Fatal(e)
	}
	a, e := ReadRepairedCircularNAUOAssembly(bytes.NewReader(raw), Limits{}, CircularNAUORepairOptions{1e-9})
	if e != nil {
		t.Fatal(e)
	}
	if len(a.Parts) != 3 || len(a.Repairs) != 1 || len(a.TubeRepairs) != 1 || len(a.Occurrences) != 8 {
		t.Fatal("lost separate families/shared paths")
	}
	tube := a.TubeRepairs[0]
	n := 0
	for _, u := range a.Occurrences {
		if u.Path[len(u.Path)-1].ChildID == tube.Walls[0].Source.Product.DefinitionID {
			n++
		}
	}
	if n != 3 {
		t.Fatal(n)
	}
	for _, p := range a.Parts {
		if p.SourceBodyID == tube.SourceBodyID && p.Family != RepairedNAUOTube {
			t.Fatal("tube disguised as cylinder")
		}
	}
	if _, e = ReadExactDAGPlanarNAUO(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("exact reader widened")
	}
	for _, kind := range []string{"faces", "vertices", "uses", "sense", "owner", "support"} {
		t.Run(kind, func(t *testing.T) {
			f, e := Parse(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			l := Limits{}
			switch kind {
			case "faces":
				l.MaxFaces = 24
			case "vertices":
				l.MaxVertices = 27
			case "uses":
				l.MaxEdgeUses = 99
			case "sense":
				q := f.Entities[tube.Walls[1].Source.SideFaceID]
				q.Records[0].Args[3] = Value{Kind: Enumeration, Text: "T"}
				f.Entities[q.ID] = q
			case "owner":
				q := f.Entities[tube.Walls[1].Source.Rims[0].CylinderPCurveID]
				q.Records[0].Args[1].Ref = a.Repairs[0].Source.CylinderID
				f.Entities[q.ID] = q
			case "support":
				q := f.Entities[tube.Walls[1].Source.CylinderID]
				q.Records[0].Type = "SPHERICAL_SURFACE"
				f.Entities[q.ID] = q
			}
			b, e := mappedPlanarSourceBytes(f)
			if e != nil {
				t.Fatal(e)
			}
			got, e := ReadRepairedCircularNAUOAssembly(bytes.NewReader(b), l, CircularNAUORepairOptions{1e-9})
			if e == nil || got.Source != nil || len(got.Parts) != 0 || len(got.Repairs) != 0 || len(got.TubeRepairs) != 0 {
				t.Fatal("partial mixed acceptance")
			}
		})
	}
}
