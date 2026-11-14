package step

import (
	"bytes"
	"os"
	"reflect"
	"testing"
)

func nestedOuterEdits(t *testing.T) (NestedRationalMappedPlanarMaterials, []RationalMappedPlacement) {
	t.Helper()
	raw, e := os.ReadFile("testdata/occt-nested-rational-mapped.step")
	if e != nil {
		t.Fatal(e)
	}
	a, e := ReadNestedRationalMappedPlanarMaterials(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	ps := []RationalMappedPlacement{}
	seen := map[int64]bool{}
	for _, u := range a.Occurrences {
		p := u.Path[0]
		if seen[p.ItemID] {
			continue
		}
		seen[p.ItemID] = true
		b, e := mappedRationalOperator(a.Source, p.TargetID)
		if e != nil {
			t.Fatal(e)
		}
		ps = append(ps, RationalMappedPlacement{ItemID: p.ItemID, Origin: b.Origin, XAxis: b.X, ZAxis: b.Z})
	}
	ps[0].Origin[2].SetInt64(5)
	ps[1].Origin[0].SetInt64(200)
	return a, ps
}
func TestEditedNestedRationalMapped(t *testing.T) {
	a, ps := nestedOuterEdits(t)
	var before, out, after bytes.Buffer
	if e := WriteNestedRationalMappedPlanarSource(&before, a.Source, Limits{}); e != nil {
		t.Fatal(e)
	}
	if e := WriteNestedRationalMappedOuterPlacements(&out, a.Source, ps, Limits{}); e != nil {
		t.Fatal(e)
	}
	b, e := ReadNestedRationalMappedPlanarMaterials(bytes.NewReader(out.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(b.Occurrences) != 6 {
		t.Fatal(b)
	}
	for i, u := range a.Occurrences {
		v := b.Occurrences[i]
		if u.Path[1] != v.Path[1] || u.Path[0].ItemID != v.Path[0].ItemID || u.Path[0].MapID != v.Path[0].MapID || u.Path[0].TargetID == v.Path[0].TargetID {
			t.Fatal("ancestry", u, v)
		}
	}
	todo := []int64{a.Occurrences[0].Path[0].RepresentationID}
	seen := map[int64]bool{}
	for len(todo) > 0 {
		id := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if seen[id] {
			continue
		}
		seen[id] = true
		en := a.Source.Entities[id]
		if !reflect.DeepEqual(en, b.Source.Entities[id]) {
			t.Fatal("inner closure edited", id)
		}
		var walk func(Value)
		walk = func(v Value) {
			if v.Kind == Reference {
				todo = append(todo, v.Ref)
			}
			for _, x := range v.Items {
				walk(x)
			}
		}
		for _, r := range en.Records {
			for _, v := range r.Args {
				walk(v)
			}
		}
	}
	if e = WriteNestedRationalMappedPlanarSource(&after, a.Source, Limits{}); e != nil || !bytes.Equal(before.Bytes(), after.Bytes()) {
		t.Fatal("source changed", e)
	}
	if e = WriteRationalMappedPlanarPlacements(&after, a.Source, ps, Limits{}); e == nil {
		t.Fatal("flat writer accepted nested source")
	}
}
func TestEditedNestedRationalReject(t *testing.T) {
	for _, kind := range []string{"inner", "missing", "duplicate", "irrational", "limit"} {
		t.Run(kind, func(t *testing.T) {
			a, ps := nestedOuterEdits(t)
			l := Limits{}
			switch kind {
			case "inner":
				ps[0].ItemID = a.Occurrences[0].Path[1].ItemID
			case "missing":
				ps = ps[:1]
			case "duplicate":
				ps[1] = ps[0]
			case "irrational":
				ps[0].XAxis = rat3(1, 1, 0)
			case "limit":
				l.MaxEntities = 10
			}
			var out bytes.Buffer
			if e := WriteNestedRationalMappedOuterPlacements(&out, a.Source, ps, l); e == nil || out.Len() != 0 {
				t.Fatal(e)
			}
		})
	}
}
