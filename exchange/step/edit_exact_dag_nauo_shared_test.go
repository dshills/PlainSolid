package step

import (
	"bytes"
	"math/big"
	"os"
	"reflect"
	"testing"
)

func exactDAGSharedEdit(a ExactDAGPlanarNAUO) ExactNAUOPlacement {
	p := exactDAGRootEdit(a)
	p.NAUOID = a.Occurrences[0].Path[1].NAUOID
	p.Origin[0] = big.NewRat(20, 1)
	return p
}
func TestEditExactDAGNAUOShared(t *testing.T) {
	t.Parallel()
	a := exactDAGEditSource(t)
	p := exactDAGSharedEdit(a)
	before, e := mappedPlanarSourceBytes(a.Source)
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteExactDAGNAUOSharedPlacements(&out, a.Source, []ExactNAUOPlacement{p}, Limits{}); e != nil {
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
	u := a.Occurrences[0].Path[1]
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
	changed := 0
	for i, old := range a.Occurrences {
		got := fresh.Occurrences[i]
		if len(old.Path) != len(got.Path) {
			t.Fatal("path length")
		}
		selected := -1
		for j, path := range old.Path {
			if path.NAUOID == p.NAUOID {
				selected = j
			}
			if path.NAUOID != got.Path[j].NAUOID {
				t.Fatal("path identity")
			}
		}
		var prefix mappedRationalBasis
		if selected >= 0 {
			changed++
			prefix = old.Path[0].pose
			for j := 1; j < selected; j++ {
				prefix = composeMappedPose(prefix, old.Path[j].pose)
			}
		}
		oldM, newM := old.Matrix(), got.Matrix()
		for row := 0; row < 3; row++ {
			for col := 0; col < 4; col++ {
				expected := new(big.Rat).Set(oldM[row][col])
				if selected >= 0 && col == 3 {
					expected.Add(expected, new(big.Rat).Mul(prefix.X[row], big.NewRat(10, 1)))
				}
				if expected.Cmp(newM[row][col]) != 0 {
					t.Fatal("collateral or incorrect pose", i, row, col)
				}
			}
		}
	}
	if changed != 2 {
		t.Fatal("fixture lost shared use", changed)
	}
	p.Origin[0].SetInt64(999)
	if fresh.Occurrences[0].Matrix()[0][3].Cmp(big.NewRat(120, 1)) != 0 {
		t.Fatal("caller rational retained")
	}
	var invalid bytes.Buffer
	if e = WriteExactDAGNAUORootPlacements(&invalid, a.Source, []ExactNAUOPlacement{p}, Limits{}); e == nil || invalid.Len() != 0 {
		t.Fatal("root-only scope widened")
	}
}
func TestEditExactDAGNAUOSharedReject(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"empty", "duplicate", "root", "unknown", "nonterminating", "nonorthogonal", "irrational", "nil", "large", "shared-target", "limit"} {
		t.Run(kind, func(t *testing.T) {
			a := exactDAGEditSource(t)
			p := exactDAGSharedEdit(a)
			ps := []ExactNAUOPlacement{p}
			l := Limits{}
			switch kind {
			case "empty":
				ps = nil
			case "duplicate":
				ps = append(ps, p)
			case "root":
				ps[0].NAUOID = a.Occurrences[0].Path[0].NAUOID
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
				other := a.Occurrences[1].Path[1]
				en := a.Source.Entities[other.TransformationID]
				en.Records[0].Args[3].Ref = a.Occurrences[0].Path[1].ToPlacementID
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
			if e := WriteExactDAGNAUOSharedPlacements(&out, a.Source, ps, l); e == nil || out.Len() != 0 {
				t.Fatal("non-atomic unsupported edit", e)
			}
		})
	}
}
func FuzzExactDAGNAUOSharedEdit(f *testing.F) {
	raw, e := os.ReadFile("testdata/occt-exact-dag-nauo.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(int64(20), int64(1), int64(0), false)
	f.Add(int64(-20), int64(3), int64(4), false)
	f.Add(int64(0), int64(1), int64(1), true)
	f.Fuzz(func(t *testing.T, origin, x, y int64, root bool) {
		a, e := ReadExactDAGPlanarNAUO(bytes.NewReader(raw), Limits{MaxBytes: 1 << 20, MaxEntities: 1000})
		if e != nil {
			t.Fatal(e)
		}
		p := exactDAGSharedEdit(a)
		p.Origin[0] = big.NewRat(origin, 1)
		p.XAxis[0] = big.NewRat(x, 1)
		p.XAxis[1] = big.NewRat(y, 1)
		if root {
			p.NAUOID = a.Occurrences[0].Path[0].NAUOID
		}
		var out bytes.Buffer
		e = WriteExactDAGNAUOSharedPlacements(&out, a.Source, []ExactNAUOPlacement{p}, Limits{MaxBytes: 1 << 20, MaxEntities: 1000})
		if e != nil {
			if out.Len() != 0 {
				t.Fatal("partial output")
			}
			return
		}
		if root {
			t.Fatal("root use admitted")
		}
		if _, e = ReadExactDAGPlanarNAUO(&out, Limits{MaxBytes: 1 << 20, MaxEntities: 1000}); e != nil {
			t.Fatal(e)
		}
	})
}

func TestEditExactDAGNAUOInternalGroup(t *testing.T) {
	t.Parallel()
	a := exactDAGEditSource(t)
	u := a.Occurrences[3].Path[1]
	p := exactDAGSharedEdit(a)
	p.NAUOID = u.NAUOID
	p.Origin = [3]*big.Rat{big.NewRat(0, 1), big.NewRat(60, 1), big.NewRat(0, 1)}
	group := false
	for _, g := range a.Groups {
		if g.DefinitionID == u.ChildID {
			group = true
		}
	}
	if !group {
		t.Fatal("fixture group role")
	}
	var out bytes.Buffer
	if e := WriteExactDAGNAUOSharedPlacements(&out, a.Source, []ExactNAUOPlacement{p}, Limits{}); e != nil {
		t.Fatal(e)
	}
	fresh, e := ReadExactDAGPlanarNAUO(&out, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	changed := 0
	for i, old := range a.Occurrences {
		selected := false
		for _, r := range old.Path {
			if r.NAUOID == u.NAUOID {
				selected = true
			}
		}
		om, nm := old.Matrix(), fresh.Occurrences[i].Matrix()
		if selected {
			changed++
		}
		for row := 0; row < 3; row++ {
			for col := 0; col < 4; col++ {
				expected := new(big.Rat).Set(om[row][col])
				if selected && row == 2 && col == 3 {
					expected.Add(expected, big.NewRat(10, 1))
				}
				if expected.Cmp(nm[row][col]) != 0 {
					t.Fatal("group branch pose", i, row, col)
				}
			}
		}
	}
	if changed != 3 {
		t.Fatal("group descendant count", changed)
	}
}
