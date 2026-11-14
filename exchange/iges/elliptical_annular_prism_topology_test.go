package iges

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

func TestEllipticalAnnularPrismTopology(t *testing.T) {
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/elliptical-annular-prism-topology.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadEllipticalAnnularPrismTopology(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if m.SourceFaceIDs != ([4]int{45, 47, 49, 51}) || m.SourceEdgeListID != 31 {
		t.Fatal(m)
	}
	if _, e = ReadEllipticalPrismTopology(ctx, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("solid-disk reader widened")
	}
	for _, pair := range [][2]string{{"122,29,2,0,7;", "122,23,2,0,7;"}, {"514,4,45,1,47,0,49,1,51,0;", "514,4,45,1,47,1,49,1,51,0;"}, {"508,1,0,31,4,0,0;", "508,1,0,31,4,1,0;"}, {"510,13,2,1,37,39;", "510,13,2,1,37,37;"}} {
		bad := append([]byte(nil), raw...)
		off := bytes.Index(bad, []byte(pair[0]))
		if off < 0 {
			t.Fatal(pair)
		}
		copy(bad[off:off+64], []byte(pair[1]+strings.Repeat(" ", 64-len(pair[1]))))
		if _, e = ReadEllipticalAnnularPrismTopology(ctx, bytes.NewReader(bad), Limits{}); e == nil {
			t.Fatal("owner/sense accepted")
		}
	}
	for _, coeff := range [][6]float64{m.Descriptor.OuterCoefficients, {1, 0, 4, 0, 0, -16}, {1, 0, 4, 0, 0, -36}} {
		d := m.Descriptor
		d.InnerCoefficients = coeff
		var out bytes.Buffer
		if WriteEllipticalAnnularPrismTopology(ctx, &out, d, .001) == nil || out.Len() != 0 {
			t.Fatal("non-nesting accepted")
		}
	}
	for _, l := range []Limits{{MaxEntities: 27}, {MaxBytes: 80}, {MaxRecords: 2}, {MaxValues: 4}} {
		if _, e = ReadEllipticalAnnularPrismTopology(ctx, bytes.NewReader(raw), l); e == nil {
			t.Fatal("limit")
		}
	}
}
func FuzzEllipticalAnnularPrismTopology(f *testing.F) {
	b, e := os.ReadFile("testdata/elliptical-annular-prism-topology.igs")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 32768 {
			return
		}
		m, e := ReadEllipticalAnnularPrismTopology(context.Background(), bytes.NewReader(b), Limits{})
		if e == nil {
			var out bytes.Buffer
			if e = WriteEllipticalAnnularPrismTopology(context.Background(), &out, m.Descriptor, m.LengthUnit); e != nil {
				t.Fatal(e)
			}
		}
	})
}
