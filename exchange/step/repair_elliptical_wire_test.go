package step

import (
	"bytes"
	"math"
	"math/big"
	"os"
	"reflect"
	"testing"
)

func roundedEllipseFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/occt-rounded-ellipse-wire.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestRepairRoundedEllipticalWire(t *testing.T) {
	raw := roundedEllipseFixture(t)
	m, e := RepairRoundedEllipticalWire(bytes.NewReader(raw), Limits{}, EllipticalWireRepairOptions{MaxDistance: 1e-11})
	if e != nil {
		t.Fatal(e)
	}
	if m.TrimID != 16 || m.CartesianSelectorIDs != [2]int64{22, 23} || m.Natural.EllipseID != 17 || m.Natural.MajorRadius != 4 || m.Natural.MinorRadius != 2 || m.Natural.LengthUnit != .001 {
		t.Fatal("provenance", m)
	}
	b := m.Bounds
	end, _ := new(big.Rat).SetString("6.28318530718")
	if b.SourceStart.Sign() != 0 || b.SourceEnd.Cmp(end) != 0 || b.SignedEndResidual[0].Sign() <= 0 || b.DisplacementUpper.Cmp(b.Tolerance) > 0 {
		t.Fatal("bounds", b)
	}
	if b.SignedEndResidual[0].Cmp(big.NewRat(4, 10000000000000)) <= 0 || b.SignedEndResidual[1].Cmp(big.NewRat(5, 10000000000000)) >= 0 {
		t.Fatal("pi residual enclosure")
	}
	want := new(big.Rat).Mul(big.NewRat(4, 1), b.SignedEndResidual[1])
	if b.DisplacementUpper.Cmp(want) != 0 {
		t.Fatal("physical scale")
	}
	// Sample validation is supplementary; the continuous proof uses the exact
	// analytic derivative bound majorRadius and the rational pi enclosure.
	physical, _ := b.DisplacementUpper.Float64()
	for i := 0; i <= 100; i++ {
		s := float64(i) / 100
		p, _ := m.Natural.Geometry.Eval(s * 6.28318530718)
		q, _ := m.Natural.Geometry.Eval(s * 2 * math.Pi)
		if p.Distance(q) > physical+1e-14 {
			t.Fatal(p, q, physical)
		}
	}
	var out bytes.Buffer
	if e = WriteEllipticalWireSource(&out, m.Natural.Source, Limits{}); e != nil {
		t.Fatal(e)
	}
	n, e := ReadEllipticalWire(&out, Limits{})
	if e != nil || n.Geometry != m.Natural.Geometry {
		t.Fatal(e)
	}
	if _, e = ReadEllipticalWire(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("strict default widened")
	}
	original, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil || !reflect.DeepEqual(original, m.Original) {
		t.Fatal("original changed", e)
	}
	// Separate ownership of both complete graphs and rational evidence.
	x := m.Natural.Source.Entities[7]
	x.Records[0].Args[1].Text = "changed"
	m.Natural.Source.Entities[7] = x
	if reflect.DeepEqual(m.Natural.Source.Entities[7], m.Original.Entities[7]) {
		t.Fatal("source aliases")
	}
	b.SourceEnd.SetInt64(0)
	if b.SignedEndResidual[1].Sign() <= 0 {
		t.Fatal("bound aliases")
	}
}
func TestRepairRoundedEllipticalWireReject(t *testing.T) {
	raw := roundedEllipseFixture(t)
	changes := []func(*File){
		func(f *File) {
			x := f.Entities[21]
			x.Records[0].Args[1].Items = []Value{{Kind: Number, Text: "1"}, {Kind: Number, Text: "0"}, {Kind: Number, Text: "0"}}
			f.Entities[21] = x
		},

		func(f *File) { x := f.Entities[16]; x.Records[0].Args[4].Text = "F"; f.Entities[16] = x },
		func(f *File) { x := f.Entities[16]; x.Records[0].Args[5].Text = "CARTESIAN"; f.Entities[16] = x },
		func(f *File) {
			x := f.Entities[16]
			x.Records[0].Args[2].Items[1].Items[0].Text = "0.1"
			f.Entities[16] = x
		},
		func(f *File) {
			x := f.Entities[16]
			x.Records[0].Args[3].Items[1].Items[0].Text = "12.56637061436"
			f.Entities[16] = x
		},
		func(f *File) { x := f.Entities[16]; x.Records[0].Args[3].Items[0].Ref = 22; f.Entities[16] = x },
		func(f *File) {
			x := f.Entities[23]
			x.Records[0].Args[1].Items[0].Text = "1.0000000000000000001"
			f.Entities[23] = x
		},
		func(f *File) {
			for _, id := range []int64{22, 23} {
				x := f.Entities[id]
				x.Records[0].Args[1].Items[2].Text = "7"
				f.Entities[id] = x
			}
		},
		func(f *File) {
			x := f.Entities[17]
			x.Records[0].Args[2].Text = "4.00000000000000001"
			f.Entities[17] = x
		},
		func(f *File) {
			x := f.Entities[15]
			x.Records[0].Args[1].Items = append(x.Records[0].Args[1].Items, Value{Kind: Reference, Ref: 16})
			f.Entities[15] = x
		},
		func(f *File) { x := f.Entities[18]; x.Records[0].Args[1].Ref = 22; f.Entities[18] = x },
	}
	for i, change := range changes {
		f, e := Parse(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		change(f)
		b, e := mappedPlanarSourceBytes(f)
		if e != nil {
			t.Fatal(e)
		}
		m, e := RepairRoundedEllipticalWire(bytes.NewReader(b), Limits{}, EllipticalWireRepairOptions{MaxDistance: 1e-11})
		if e == nil || m.Original != nil || m.Natural.Source != nil || m.Bounds.SourceEnd != nil {
			t.Fatal("accepted mutation", i, e)
		}
	}
	for _, tol := range []float64{0, -1, math.NaN(), math.Inf(1), 1e-13, 1e-3} {
		if m, e := RepairRoundedEllipticalWire(bytes.NewReader(raw), Limits{}, EllipticalWireRepairOptions{MaxDistance: tol}); e == nil || m.Original != nil {
			t.Fatal("tolerance", tol, e)
		}
	}
	for _, l := range []Limits{{MaxBytes: 100}, {MaxEntities: 28}, {MaxValues: 20}} {
		if _, e := RepairRoundedEllipticalWire(bytes.NewReader(raw), l, EllipticalWireRepairOptions{MaxDistance: 1e-11}); e == nil {
			t.Fatal("limit")
		}
	}
	if _, e := RepairRoundedEllipticalWire(bytes.NewReader(ellipticalWireFixture(t)), Limits{}, EllipticalWireRepairOptions{MaxDistance: 1e-11}); e == nil {
		t.Fatal("not rounded source")
	}
}
func TestRepairRoundedEllipticalWireUnderTurn(t *testing.T) {
	raw := bytes.Replace(roundedEllipseFixture(t), []byte("6.28318530718"), []byte("6.283185307179"), 1)
	m, e := RepairRoundedEllipticalWire(bytes.NewReader(raw), Limits{}, EllipticalWireRepairOptions{MaxDistance: 1e-11})
	if e != nil || m.Bounds.SignedEndResidual[1].Sign() >= 0 {
		t.Fatal(e)
	}
	want := new(big.Rat).Mul(big.NewRat(-4, 1), m.Bounds.SignedEndResidual[0])
	if m.Bounds.DisplacementUpper.Cmp(want) != 0 {
		t.Fatal("gap bound")
	}
}
func FuzzRepairRoundedEllipticalWire(f *testing.F) {
	f.Add(roundedEllipseFixture(f))
	f.Add([]byte("bad"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			t.Skip()
		}
		m, e := RepairRoundedEllipticalWire(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 29, MaxValues: 2048, MaxDepth: 16}, EllipticalWireRepairOptions{MaxDistance: 1e-11})
		if e != nil && (m.Original != nil || m.Natural.Source != nil) {
			t.Fatal("partial")
		}
	})
}
