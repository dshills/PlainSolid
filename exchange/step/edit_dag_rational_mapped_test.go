package step

import (
	"bytes"
	"os"
	"reflect"
	"testing"
)

func dagSharedEdit(t *testing.T) (DAGRationalMappedPlanarMaterials, RationalMappedPlacement) {
	t.Helper()
	raw, e := os.ReadFile("testdata/occt-dag-rational-mapped.step")
	if e != nil {
		t.Fatal(e)
	}
	a, e := ReadDAGRationalMappedPlanarMaterials(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	p := a.Occurrences[0].Path
	id := p[len(p)-1].ItemID
	return a, RationalMappedPlacement{ItemID: id, Origin: rat3(30, 0, 0), XAxis: rat3(3, 4, 0), ZAxis: rat3(0, 0, 1)}
}
func TestEditedDAGRationalShared(t *testing.T) {
	a, p := dagSharedEdit(t)
	var before, out, after bytes.Buffer
	if e := WriteDAGRationalMappedPlanarSource(&before, a.Source, Limits{}); e != nil {
		t.Fatal(e)
	}
	if e := WriteDAGRationalMappedSharedPlacements(&out, a.Source, []RationalMappedPlacement{p}, Limits{}); e != nil {
		t.Fatal(e)
	}
	b, e := ReadDAGRationalMappedPlanarMaterials(bytes.NewReader(out.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	changed := 0
	for i, u := range a.Occurrences {
		v := b.Occurrences[i]
		affected := false
		for _, s := range u.Path {
			if s.ItemID == p.ItemID {
				affected = true
			}
		}
		same := reflect.DeepEqual(u.Occurrence.Matrix(), v.Occurrence.Matrix())
		if affected {
			changed++
			if same {
				t.Fatal("missed shared use")
			}
		} else if !same || !reflect.DeepEqual(u.Path, v.Path) {
			t.Fatal("collateral pose edit")
		}
	}
	if changed != 4 {
		t.Fatal(changed)
	}
	for id, en := range a.Source.Entities {
		if id == p.ItemID {
			continue
		}
		if got, ok := b.Source.Entities[id]; ok && !reflect.DeepEqual(en, got) {
			t.Fatal("collateral record edit", id)
		}
	}
	if e = WriteDAGRationalMappedPlanarSource(&after, a.Source, Limits{}); e != nil || !bytes.Equal(before.Bytes(), after.Bytes()) {
		t.Fatal("source mutated", e)
	}
}
func TestEditedDAGRationalReject(t *testing.T) {
	for _, kind := range []string{"root-internal", "shared-root", "duplicate", "empty", "foreign", "limit"} {
		t.Run(kind, func(t *testing.T) {
			a, p := dagSharedEdit(t)
			ps := []RationalMappedPlacement{p}
			write := WriteDAGRationalMappedSharedPlacements
			l := Limits{}
			switch kind {
			case "root-internal":
				write = WriteDAGRationalMappedRootPlacements
			case "shared-root":
				ps[0].ItemID = a.Occurrences[0].Path[0].ItemID
			case "duplicate":
				ps = append(ps, p)
			case "empty":
				ps = nil
			case "foreign":
				ps[0].ItemID = -1
			case "limit":
				l.MaxEntities = 10
			}
			var out bytes.Buffer
			if e := write(&out, a.Source, ps, l); e == nil || out.Len() != 0 {
				t.Fatal(e)
			}
		})
	}
}
func TestEditedDAGRationalSharedRootAlias(t *testing.T) {
	a, _ := dagSharedEdit(t)
	shared := a.Occurrences[0].Path[1]
	f := a.Source
	var rootID int64
	for _, en := range f.Entities {
		if len(en.Records) == 1 && en.Records[0].Type == "SHAPE_DEFINITION_REPRESENTATION" {
			rootID = en.Records[0].Args[1].Ref
		}
	}
	root := f.Entities[rootID]
	root.Records[0].Args[1].Items = append(root.Records[0].Args[1].Items, Value{Kind: Reference, Ref: shared.ItemID})
	f.Entities[rootID] = root
	b, e := mappedPlanarSourceBytes(f)
	if e != nil {
		t.Fatal(e)
	}
	a, e = ReadDAGRationalMappedPlanarMaterials(bytes.NewReader(b), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	p := RationalMappedPlacement{ItemID: shared.ItemID, Origin: rat3(0, 0, 7), XAxis: rat3(1, 0, 0), ZAxis: rat3(0, 0, 1)}
	var out bytes.Buffer
	if e = WriteDAGRationalMappedRootPlacements(&out, a.Source, []RationalMappedPlacement{p}, Limits{}); e == nil || out.Len() != 0 {
		t.Fatal("root API edited shared internal definition")
	}
	if e = WriteDAGRationalMappedSharedPlacements(&out, a.Source, []RationalMappedPlacement{p}, Limits{}); e != nil {
		t.Fatal(e)
	}
}

func FuzzDAGSharedPlacements(f *testing.F) {
	b, e := os.ReadFile("testdata/occt-dag-rational-mapped.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b, int64(30))
	f.Add([]byte("ISO-10303-21;"), int64(0))
	f.Fuzz(func(t *testing.T, b []byte, x int64) {
		if len(b) > 1<<20 || x < -1000 || x > 1000 {
			t.Skip()
		}
		l := Limits{MaxBytes: 1 << 20, MaxEntities: 1500, MaxFaces: 32, MaxVertices: 64, MaxEdgeUses: 256}
		a, e := ReadDAGRationalMappedPlanarMaterials(bytes.NewReader(b), l)
		if e != nil {
			return
		}
		path := a.Occurrences[0].Path
		p := RationalMappedPlacement{ItemID: path[len(path)-1].ItemID, Origin: rat3(x, 0, 0), XAxis: rat3(3, 4, 0), ZAxis: rat3(0, 0, 1)}
		var out bytes.Buffer
		e = WriteDAGRationalMappedSharedPlacements(&out, a.Source, []RationalMappedPlacement{p}, l)
		if e != nil {
			if out.Len() != 0 {
				t.Fatal("partial output")
			}
			return
		}
		n, e := ReadDAGRationalMappedPlanarMaterials(&out, l)
		if e != nil {
			t.Fatal(e)
		}
		if len(n.Bodies) != len(a.Bodies) {
			t.Fatal("asset mutation")
		}
		for id, b := range a.Bodies {
			if !reflect.DeepEqual(b.Solid.Vertices(), n.Bodies[id].Solid.Vertices()) {
				t.Fatal("geometry mutation")
			}
		}
	})
}
