package iges

import (
	"bytes"
	"math/big"
	"os"
	"strings"
	"testing"
)

func FuzzConicalRimFilletBoreIGESArcMatrix(f *testing.F) {
	raw, err := os.ReadFile("testdata/occt-conical-double-rim-eccentric-bore-rounded.igs")
	if err != nil {
		f.Fatal(err)
	}
	doc, err := readTopologyRecordPolicyWithDirectory(bytes.NewReader(raw), Limits{}, true, false, false, false, false, false, true, false, false, igesConicalRepairDirectory)
	if err != nil {
		f.Fatal(err)
	}
	for _, rec := range doc.entities {
		if rec.id != 27 && rec.id != 35 && rec.id != 43 {
			continue
		}
		values := make([]string, len(rec.values))
		for i, v := range rec.values {
			values[i] = v.text
		}
		f.Add(strings.Join(values, ",")+";", rec.id != 27)
	}
	f.Fuzz(func(t *testing.T, raw string, meridian bool) {
		if len(raw) > 1024 {
			return
		}
		budget := 13
		values, err := parseParameters(raw, ',', ';', &budget)
		if err != nil || len(values) != 13 {
			return
		}
		frame, err := intValue(values[0])
		if err != nil || frame != 124 {
			return
		}
		cardinal, phase, err := igesConicalArcMatrix(topologyRecord{values: values}, meridian)
		if err != nil {
			return
		}
		var delta [3][2]*big.Rat
		for axis := range delta {
			for j := range delta[axis] {
				q, err := igesPlanarExact(values[1+4*axis+j])
				if err != nil {
					t.Fatal("accepted non-rational matrix coefficient")
				}
				delta[axis][j] = q.Sub(q, big.NewRat(cardinal[axis][j], 1))
			}
		}
		bound2 := new(big.Rat).Mul(phase, phase)
		if !meridian {
			bound2.Mul(bound2, big.NewRat(4, 1))
		}
		for _, sample := range [][3]int64{{1, 0, 1}, {3, 4, 5}, {0, -1, 1}, {-7, 24, 25}} {
			u, v := big.NewRat(sample[0], sample[2]), big.NewRat(sample[1], sample[2])
			distance2 := new(big.Rat)
			for _, row := range delta {
				x := new(big.Rat).Add(new(big.Rat).Mul(row[0], u), new(big.Rat).Mul(row[1], v))
				distance2.Add(distance2, x.Mul(x, x))
			}
			if distance2.Cmp(bound2) > 0 {
				t.Fatal("stored occurrence displacement exceeds the proved upper bound")
			}
		}
	})
}
