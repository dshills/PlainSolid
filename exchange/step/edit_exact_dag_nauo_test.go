package step

import (
	"bytes"
	"math/big"
	"os"
	"reflect"
	"testing"
)

func exactDAGEditSource(t *testing.T) ExactDAGPlanarNAUO {
	t.Helper()
	raw, e := os.ReadFile("testdata/occt-exact-dag-nauo.step")
	if e != nil {
		t.Fatal(e)
	}
	a, e := ReadExactDAGPlanarNAUO(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	return a
}
func exactDAGRootEdit(a ExactDAGPlanarNAUO) ExactNAUOPlacement {
	return ExactNAUOPlacement{NAUOID: a.Occurrences[0].Path[0].NAUOID, Origin: [3]*big.Rat{big.NewRat(200, 1), big.NewRat(0, 1), big.NewRat(0, 1)}, XAxis: [3]*big.Rat{big.NewRat(1, 1), big.NewRat(0, 1), big.NewRat(0, 1)}, ZAxis: [3]*big.Rat{big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(1, 1)}}
}
func TestEditExactDAGNAUORoot(t *testing.T) {
	a := exactDAGEditSource(t)
	p := exactDAGRootEdit(a)
	before, e := mappedPlanarSourceBytes(a.Source)
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteExactDAGNAUORootPlacements(&out, a.Source, []ExactNAUOPlacement{p}, Limits{}); e != nil {
		t.Fatal(e)
	}
	fresh, e := ReadExactDAGPlanarNAUO(&out, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	after, _ := mappedPlanarSourceBytes(a.Source)
	if !bytes.Equal(before, after) {
		t.Fatal("caller source mutated")
	}
	u := a.Occurrences[0].Path[0]
	frame := a.Source.Entities[u.ToPlacementID].Records[0]
	allowed := map[int64]bool{}
	for _, v := range frame.Args[1:] {
		allowed[v.Ref] = true
	}
	if !reflect.DeepEqual(a.Source.Order, fresh.Source.Order) || !reflect.DeepEqual(a.Source.Header, fresh.Source.Header) {
		t.Fatal("source identities changed")
	}
	for id, en := range a.Source.Entities {
		if !allowed[id] && !reflect.DeepEqual(en, fresh.Source.Entities[id]) {
			t.Fatalf("unselected record #%d changed", id)
		}
	}
	if !reflect.DeepEqual(a.Parts, fresh.Parts) || !reflect.DeepEqual(a.Groups, fresh.Groups) {
		t.Fatal("local definitions changed")
	}
	for i, old := range a.Occurrences {
		got := fresh.Occurrences[i]
		// Pose is private; every exposed source relationship identifier must remain.
		for j, path := range old.Path {
			n := got.Path[j]
			if path.NAUOID != n.NAUOID || path.CDSRID != n.CDSRID || path.RelationshipID != n.RelationshipID || path.TransformationID != n.TransformationID || path.FromPlacementID != n.FromPlacementID || path.ToPlacementID != n.ToPlacementID || path.ParentID != n.ParentID || path.ChildID != n.ChildID {
				t.Fatal("path identity changed")
			}
		}
		oldM, newM := old.Matrix(), got.Matrix()
		for row := 0; row < 3; row++ {
			for col := 0; col < 4; col++ {
				expected := new(big.Rat).Set(oldM[row][col])
				if old.Path[0].NAUOID == p.NAUOID && row == 0 && col == 3 {
					expected.Add(expected, big.NewRat(100, 1))
				}
				if expected.Cmp(newM[row][col]) != 0 {
					t.Fatal("collateral or incorrect pose", i, row, col)
				}
			}
		}
	}
	p.Origin[0].SetInt64(999)
	if fresh.Occurrences[0].Matrix()[0][3].Cmp(big.NewRat(210, 1)) != 0 {
		t.Fatal("caller rational retained")
	}
}
func TestEditExactDAGNAUOFromAndRational(t *testing.T) {
	a := exactDAGEditSource(t)
	p := exactDAGRootEdit(a)
	from := a.Source.Entities[a.Occurrences[0].Path[0].FromPlacementID].Records[0]
	point := a.Source.Entities[from.Args[1].Ref]
	point.Records[0].Args[1].Items = []Value{{Kind: Number, Text: "1"}, {Kind: Number, Text: "2"}, {Kind: Number, Text: "3"}}
	a.Source.Entities[point.ID] = point
	p.Origin[0] = big.NewRat(1, 10)
	p.XAxis = [3]*big.Rat{big.NewRat(3, 1), big.NewRat(4, 1), big.NewRat(0, 1)}
	var out bytes.Buffer
	if e := WriteExactDAGNAUORootPlacements(&out, a.Source, []ExactNAUOPlacement{p}, Limits{}); e != nil {
		t.Fatal(e)
	}
	fresh, e := ReadExactDAGPlanarNAUO(&out, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, u := range fresh.Relationships {
		if u.NAUOID == p.NAUOID {
			m := u.Matrix()
			if m[0][0].Cmp(big.NewRat(3, 5)) != 0 || m[1][0].Cmp(big.NewRat(4, 5)) != 0 || m[0][3].Cmp(big.NewRat(1, 10)) != 0 || m[1][3].Sign() != 0 || m[2][3].Sign() != 0 {
				t.Fatal("from frame composition", m)
			}
		}
	}
}
func TestEditExactDAGNAUOReject(t *testing.T) {
	for _, kind := range []string{"empty", "duplicate", "internal", "unknown", "nonterminating", "nonorthogonal", "irrational", "nil", "large", "shared-target", "limit"} {
		t.Run(kind, func(t *testing.T) {
			a := exactDAGEditSource(t)
			p := exactDAGRootEdit(a)
			ps := []ExactNAUOPlacement{p}
			l := Limits{}
			switch kind {
			case "empty":
				ps = nil
			case "duplicate":
				ps = append(ps, p)
			case "internal":
				ps[0].NAUOID = a.Occurrences[0].Path[1].NAUOID
			case "unknown":
				ps[0].NAUOID = -1
			case "nonterminating":
				ps[0].Origin[0] = big.NewRat(1, 3)
			case "nonorthogonal":
				ps[0].XAxis[2] = big.NewRat(1, 1)
			case "irrational":
				ps[0].XAxis[1] = big.NewRat(1, 1)
			case "nil":
				ps[0].Origin[0] = nil
			case "large":
				ps[0].Origin[0] = new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 5000))
			case "shared-target":
				outer := a.Occurrences[3].Path[0]
				en := a.Source.Entities[outer.TransformationID]
				en.Records[0].Args[3].Ref = a.Occurrences[0].Path[0].ToPlacementID
				a.Source.Entities[en.ID] = en
				raw, e := mappedPlanarSourceBytes(a.Source)
				if e != nil {
					t.Fatal(e)
				}
				if _, e = ReadExactDAGPlanarNAUO(bytes.NewReader(raw), Limits{}); e != nil {
					t.Fatal("shared target fixture invalid", e)
				}
			case "limit":
				l.MaxEntities = 100
			}
			var out bytes.Buffer
			if e := WriteExactDAGNAUORootPlacements(&out, a.Source, ps, l); e == nil || out.Len() != 0 {
				t.Fatal("non-atomic unsupported edit", e)
			}
		})
	}
}

func FuzzExactDAGNAUORootEdit(f *testing.F) {
	raw, e := os.ReadFile("testdata/occt-exact-dag-nauo.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(int64(200), int64(1), int64(0), false)
	f.Add(int64(-20), int64(3), int64(4), false)
	f.Add(int64(0), int64(1), int64(1), true)
	f.Fuzz(func(t *testing.T, origin, x, y int64, internal bool) {
		a, e := ReadExactDAGPlanarNAUO(bytes.NewReader(raw), Limits{MaxBytes: 1 << 20, MaxEntities: 1000})
		if e != nil {
			t.Fatal(e)
		}
		p := exactDAGRootEdit(a)
		p.Origin[0] = big.NewRat(origin, 1)
		p.XAxis[0] = big.NewRat(x, 1)
		p.XAxis[1] = big.NewRat(y, 1)
		if internal {
			p.NAUOID = a.Occurrences[0].Path[1].NAUOID
		}
		var out bytes.Buffer
		e = WriteExactDAGNAUORootPlacements(&out, a.Source, []ExactNAUOPlacement{p}, Limits{MaxBytes: 1 << 20, MaxEntities: 1000})
		if e != nil {
			if out.Len() != 0 {
				t.Fatal("partial output")
			}
			return
		}
		if internal {
			t.Fatal("internal use admitted")
		}
		if _, e = ReadExactDAGPlanarNAUO(&out, Limits{MaxBytes: 1 << 20, MaxEntities: 1000}); e != nil {
			t.Fatal(e)
		}
	})
}
