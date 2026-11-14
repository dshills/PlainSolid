package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/geom"
	"testing"
	"time"
)

func rat3(x, y, z int64) [3]*big.Rat {
	return [3]*big.Rat{big.NewRat(x, 1), big.NewRat(y, 1), big.NewRat(z, 1)}
}
func rationalMappedFixture(t *testing.T) (*File, []RationalMappedPlacement) {
	t.Helper()
	raw, e := os.ReadFile("testdata/occt-multiple-mapped-planar-boxes.step")
	if e != nil {
		t.Fatal(e)
	}
	a, e := ReadRationalMappedPlanarMaterials(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	ps := []RationalMappedPlacement{}
	for _, u := range a.Occurrences {
		m := u.Matrix()
		ps = append(ps, RationalMappedPlacement{ItemID: u.ItemID, Origin: [3]*big.Rat{m[0][3], m[1][3], m[2][3]}, XAxis: [3]*big.Rat{m[0][0], m[1][0], m[2][0]}, ZAxis: [3]*big.Rat{m[0][2], m[1][2], m[2][2]}})
	}
	ps[0].XAxis = rat3(3, 4, 0)
	return a.Source, ps
}
func TestRationalMappedOccurrences(t *testing.T) {
	source, ps := rationalMappedFixture(t)
	var out bytes.Buffer
	if e := WriteRationalMappedPlanarPlacements(&out, source, ps, Limits{}); e != nil {
		t.Fatal(e)
	}
	a, e := ReadRationalMappedPlanarMaterials(bytes.NewReader(out.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	u := a.Occurrences[0]
	q, e := u.ApplyExact(geom.Vec3{X: 2})
	if e != nil || q[0].Cmp(big.NewRat(56, 5)) != 0 || q[1].Cmp(big.NewRat(8, 5)) != 0 {
		t.Fatal(q, e)
	}
	if _, e = u.ApplyRepresented(geom.Vec3{X: 2}); e == nil {
		t.Fatal("rounded world point")
	}
	p, e := u.ApplyRepresented(geom.Vec3{X: 5})
	if e != nil || p != (geom.Vec3{X: 13, Y: 4}) {
		t.Fatal(p, e)
	}
	m := u.Matrix()
	m[0][0].SetInt64(99)
	ps[0].XAxis[0].SetInt64(88)
	q, e = u.ApplyExact(geom.Vec3{X: 5})
	if e != nil || q[0].Cmp(big.NewRat(13, 1)) != 0 {
		t.Fatal("mutable decoded pose", q, e)
	}
	if _, e = ReadPythagoreanMappedPlanarMaterials(bytes.NewReader(out.Bytes()), Limits{}); e == nil {
		t.Fatal("old occurrence contract widened")
	}
	var again bytes.Buffer
	if e = WriteRationalMappedPlanarSource(&again, a.Source, Limits{}); e != nil || !bytes.Equal(again.Bytes(), out.Bytes()) {
		t.Fatal(e)
	}
}
func TestRationalMappedReject(t *testing.T) {
	for _, kind := range []string{"missing", "duplicate", "origin", "nonorthogonal", "irrational", "nil", "limit"} {
		t.Run(kind, func(t *testing.T) {
			source, ps := rationalMappedFixture(t)
			l := Limits{}
			switch kind {
			case "missing":
				ps = ps[1:]
			case "duplicate":
				ps[1] = ps[0]
			case "origin":
				ps[0].Origin[0] = big.NewRat(1, 3)
			case "nonorthogonal":
				ps[0].ZAxis = rat3(1, 0, 1)
			case "irrational":
				ps[0].XAxis = rat3(1, 1, 0)
			case "nil":
				ps[0].XAxis[0] = nil
			case "limit":
				l.MaxEntities = 10
			}
			var out bytes.Buffer
			if e := WriteRationalMappedPlanarPlacements(&out, source, ps, l); e == nil || out.Len() != 0 {
				t.Fatal(e)
			}
		})
	}
}
func TestOCCTRationalMappedBounds(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_MAPPED_BOUNDS")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	source, ps := rationalMappedFixture(t)
	var out bytes.Buffer
	if e := WriteRationalMappedPlanarPlacements(&out, source, ps, Limits{}); e != nil {
		t.Fatal(e)
	}
	sourceRaw, err := os.ReadFile("testdata/occt-rational-mapped-occurrence.step")
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range [][]byte{out.Bytes(), sourceRaw} {
		if _, err = ReadRationalMappedPlanarMaterials(bytes.NewReader(fixture), Limits{}); err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(t.TempDir(), "rational.step")
		if e := os.WriteFile(p, fixture, 0600); e != nil {
			t.Fatal(e)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		raw, e := exec.CommandContext(ctx, bin, p).CombinedOutput()
		if e != nil {
			t.Fatal(e, string(raw))
		}
		at := bytes.LastIndex(raw, []byte(`{"Min"`))
		if at < 0 {
			t.Fatal(string(raw))
		}
		var b struct{ Min, Max [3]float64 }
		if e = json.Unmarshal(raw[at:], &b); e != nil {
			t.Fatal(e)
		}
		for i, v := range []float64{-10, 0, 0} {
			if math.Abs(b.Min[i]-v) > 1e-6 {
				t.Fatal(b)
			}
		}
		for i, v := range []float64{11.2, 22, 4} {
			if math.Abs(b.Max[i]-v) > 1e-6 {
				t.Fatal(b)
			}
		}
	}

}

func TestRationalMappedOriginalOwnership(t *testing.T) {
	raw, e := os.ReadFile("testdata/occt-rational-mapped-occurrence.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"orphan", "axis2", "scale", "wrong-map"} {
		t.Run(kind, func(t *testing.T) {
			f, e := Parse(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			var item int64
			for _, id := range f.Order {
				en := f.Entities[id]
				if len(en.Records) == 1 && en.Records[0].Type == "MAPPED_ITEM" {
					item = id
					break
				}
			}
			target := f.Entities[item].Records[0].Args[2].Ref
			en := f.Entities[target]
			switch kind {
			case "orphan":
				id := f.Order[len(f.Order)-1] + 1
				f.Order = append(f.Order, id)
				f.Entities[id] = Entity{ID: id, Records: []Record{{Type: "CARTESIAN_POINT", Args: []Value{{Kind: String}, {Kind: Aggregate, Items: []Value{{Kind: Number, Text: "0"}, {Kind: Number, Text: "0"}, {Kind: Number, Text: "0"}}}}}}}
			case "axis2":
				en.Records[0].Args[4] = en.Records[0].Args[3]
				f.Entities[target] = en
			case "scale":
				en.Records[0].Args[6] = Value{Kind: Number, Text: "2"}
				f.Entities[target] = en
			case "wrong-map":
				it := f.Entities[item]
				it.Records[0].Args[1] = Value{Kind: Reference, Ref: target}
				f.Entities[item] = it
			}
			b, e := mappedPlanarSourceBytes(f)
			if e != nil {
				t.Fatal(e)
			}
			a, e := ReadRationalMappedPlanarMaterials(bytes.NewReader(b), Limits{})
			if e == nil || a.Source != nil || len(a.Occurrences) != 0 {
				t.Fatal("accepted invalid original graph", e)
			}
		})
	}
}

func FuzzRationalMappedOccurrences(f *testing.F) {
	b, e := os.ReadFile("testdata/occt-rational-mapped-occurrence.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		_, _ = ReadRationalMappedPlanarMaterials(bytes.NewReader(b), Limits{MaxBytes: 1 << 20, MaxEntities: 1500, MaxVertices: 64, MaxFaces: 32, MaxEdgeUses: 256})
	})
}
