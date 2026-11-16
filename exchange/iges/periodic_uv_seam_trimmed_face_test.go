package iges

import (
	"bytes"
	"context"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"plainsolid/geom"
)

func TestReadPeriodicSeamTrimmedFaceUV(t *testing.T) {
	ctx := context.Background()
	for _, example := range []struct {
		name       string
		axis       int
		bounds     [2][2]string
		knots      [2][2][2]string
		projection [2]int
		sign       int
	}{
		{"exact-uv-u-seam", 0, [2][2]string{{"19/4", "21/4"}, {"9/4", "11/4"}},
			[2][2][2]string{{{"4", "5"}, {"2", "3"}}, {{"5", "6"}, {"2", "3"}}}, [2]int{0, 1}, 1},
		{"exact-uv-v-seam", 1, [2][2]string{{"5/4", "7/4"}, {"27/4", "29/4"}},
			[2][2][2]string{{{"1", "2"}, {"6", "7"}}, {{"1", "2"}, {"7", "8"}}}, [2]int{0, 2}, -1},
	} {
		t.Run(example.name, func(t *testing.T) {
			raw := periodicHoleFixture(t, example.name)
			f, err := ReadPeriodicSeamTrimmedFace(ctx, bytes.NewReader(raw), Limits{MaxEntities: 17, MaxControlPoints: 67})
			if err != nil {
				t.Fatal(err)
			}
			axis, opposite := example.axis, 1-example.axis
			seams := [2]*big.Rat{f.SeamU, f.SeamV}
			if f.OriginalIGES() != string(raw) || f.LengthUnit != .001 ||
				[5]int{f.FaceID, f.SupportID, f.ContourID, f.UVCompositeID, f.ModelCompositeID} != ([5]int{1, 3, 5, 7, 21}) ||
				seams[opposite] != nil || seams[axis].Cmp(big.NewRat(int64(5+2*axis), 1)) != 0 ||
				f.ProjectionAxes != example.projection || f.ProjectionOrientation != example.sign ||
				periodicTrimIntervalStrings(f.CarrierBounds) != ([2][2]string{{"0", "5"}, {"2", "7"}}) ||
				periodicTrimIntervalStrings(f.Bounds) != example.bounds {
				t.Fatal("lost the original chart, selected seam or directed global projection")
			}
			u, v, _ := f.Support.Data()
			if !u.Periodic || !v.Periodic {
				t.Fatal("lost the original uncut full periodic support")
			}
			checkPeriodicTrimSourceWire(t, f.Edges[:], 9, 23, 48, "1/4")
			for i, c := range f.Cells {
				u, v := c.KnotOwners[0], c.KnotOwners[1]
				if c.Patch == nil || periodicTrimIntervalStrings([2]geom.ExactParameterInterval{u.Domain, v.Domain}) != example.knots[i] ||
					c.KnotOwners[axis].Span != 4*(1-i) || c.KnotOwners[axis].Cycle.Int64() != int64(i) ||
					c.KnotOwners[opposite].Span != axis || c.KnotOwners[opposite].Cycle.Sign() != 0 ||
					c.Bounds[opposite].Lower.Cmp(f.Bounds[opposite].Lower) != 0 || c.Bounds[opposite].Upper.Cmp(f.Bounds[opposite].Upper) != 0 {
					t.Fatal("changed an original span/cycle, or split the untouched axis", i, c)
				}
			}
			checkDualPeriodicSeamJets(t, ctx, f, axis, seams[axis])
		})
	}
}

func checkDualPeriodicSeamJets(t *testing.T, ctx context.Context, f PeriodicSeamTrimmedFace, axis int, seam *big.Rat) {
	t.Helper()
	opposite := 1 - axis
	for _, x := range []*big.Rat{big.NewRat(0, 1), big.NewRat(1, 3), big.NewRat(1, 1)} {
		var low, high, src [2]*big.Rat
		low[axis], high[axis], src[axis] = big.NewRat(1, 1), big.NewRat(0, 1), seam
		low[opposite], high[opposite] = x, x
		src[opposite] = new(big.Rat).Add(f.Bounds[opposite].Lower, new(big.Rat).Mul(x, big.NewRat(1, 2)))
		left, err := f.Cells[0].Patch.EvaluateRational(ctx, low[0], low[1], geom.SurfacePatchEvaluationOptions{})
		if err != nil {
			t.Fatal(err)
		}
		right, err := f.Cells[1].Patch.EvaluateRational(ctx, high[0], high[1], geom.SurfacePatchEvaluationOptions{})
		if err != nil {
			t.Fatal(err)
		}
		physical, err := f.Support.EvaluateRational(ctx, src[0], src[1], geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		scales := [2]*big.Rat{}
		scales[axis], scales[opposite] = big.NewRat(1, 4), big.NewRat(1, 2)
		for k, q := range physical.Point {
			if left.Point[k].Cmp(q) != 0 || right.Point[k].Cmp(q) != 0 ||
				left.Du[k].Cmp(right.Du[k]) != 0 || left.Dv[k].Cmp(right.Dv[k]) != 0 ||
				new(big.Rat).Mul(physical.Du[k], scales[0]).Cmp(left.Du[k]) != 0 ||
				new(big.Rat).Mul(physical.Dv[k], scales[1]).Cmp(left.Dv[k]) != 0 {
				t.Fatal("split source chart changed a physical point or a derivative scale")
			}
		}
	}
}

func TestReadPeriodicSeamTrimmedFaceUVReject(t *testing.T) {
	ctx := context.Background()
	refuse := func(raw []byte, limits Limits, detail string) {
		t.Helper()
		f, err := ReadPeriodicSeamTrimmedFace(ctx, bytes.NewReader(raw), limits)
		if err == nil || !strings.Contains(err.Error(), detail) || f.FaceID != 0 || f.Support != nil ||
			f.Cells[0].Patch != nil || f.OriginalIGES() != "" {
			t.Fatal("unproved doubly periodic source returned partial face or wrong refusal", err)
		}
	}
	raw := periodicHoleFixture(t, "exact-uv-u-seam")
	doc := periodicTrimmedRecords(t, raw)
	for _, edit := range []struct {
		id, index     int
		value, detail string
	}{
		{3, 228, "2.125", "requires the full original carrier intervals"},
		{11, 1, "5.00000000000000001", "does not follow the directed split rectangle"},
		{21, 3, "23", "duplicated or wrong periodic trim owner #23"},
		{25, 19, "22.84375000000000001", "model edge #25 disagrees with UV line #11"},
	} {
		refuse(changePeriodicTrimmedRecord(t, doc, edit.id, edit.index, edit.value), Limits{}, edit.detail)
	}
	refuse(raw, Limits{MaxEntities: 16}, "entity limit")
	refuse(raw, Limits{MaxControlPoints: 66}, "model edge #33")
	refuse(periodicHoleFixture(t, "exact-uv-both-seams"), Limits{}, "17 source owners")

	rewriteV := func(lo, hi string) []byte {
		input := raw
		for id := 9; id <= 19; id += 2 {
			input = editPeriodicTrimWriterSource(t, input, id, func(values []string) []string {
				for _, j := range [2]int{2, 5} {
					if values[j] == "2.25" {
						values[j] = lo
					} else {
						values[j] = hi
					}
				}
				return values
			})
		}
		return input
	}
	// Adding one full V period retains each physical model edge but moves
	// the untouched chart outside its original fundamental cycle.
	refuse(rewriteV("7.25", "7.75"), Limits{}, "opposite axis is outside its original fundamental cycle")
	refuse(rewriteV("6.75", "7.25"), Limits{}, "requires exactly one original upper seam crossing")
	refuse(rewriteV("2.", "2.75"), Limits{}, "touches an additional original knot station")
}

func TestWritePeriodicSeamTrimmedFaceUV(t *testing.T) {
	ctx := context.Background()
	for _, name := range []string{"exact-uv-u-seam", "exact-uv-v-seam"} {
		t.Run(name, func(t *testing.T) {
			raw := periodicHoleFixture(t, name)
			face, err := ReadPeriodicSeamTrimmedFace(ctx, bytes.NewReader(raw), Limits{})
			if err != nil {
				t.Fatal(err)
			}
			var first, second bytes.Buffer
			if err := WritePeriodicSeamTrimmedFace(ctx, &first, face); err != nil {
				t.Fatal(err)
			}
			comparePeriodicTrimRecords(t, raw, first.Bytes())
			again, err := ReadPeriodicSeamTrimmedFace(ctx, bytes.NewReader(first.Bytes()), Limits{})
			if err != nil {
				t.Fatal(err)
			}
			if err := WritePeriodicSeamTrimmedFace(ctx, &second, again); err != nil || !bytes.Equal(first.Bytes(), second.Bytes()) {
				t.Fatal("canonical doubly periodic trim did not repeat byte-for-byte", err)
			}
			again.original = face.original
			if !reflect.DeepEqual(again, face) || face.OriginalIGES() != string(raw) {
				t.Fatal("strict reopen changed a seam, exact physical owner or the original source")
			}
			face.original = string(periodicHoleFixture(t, "exact-uv-both-seams"))
			out := bytes.NewBufferString("untouched")
			if err := WritePeriodicSeamTrimmedFace(ctx, out, face); err == nil || out.String() != "untouched" {
				t.Fatal("forged simultaneous crossing published a partial result", err)
			}
		})
	}
}

func TestReadPeriodicSeamTrimmedFaceUVPinnedOCCT(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_PERIODIC_128_TRIM_IGES")
	if oracle == "" {
		t.Skip("optional pinned OCCT doubly periodic seam author and reader")
	}
	ctx := context.Background()
	for _, example := range []struct {
		name      string
		bounds    [4]float64
		edgeCount int
	}{
		{"exact-uv-u-seam", [4]float64{4.75, 5.25, 2.25, 2.75}, 6},
		{"exact-uv-v-seam", [4]float64{1.25, 1.75, 6.75, 7.25}, 6},
		{"exact-uv-both-seams", [4]float64{4.75, 5.25, 6.75, 7.25}, 8},
	} {
		t.Run(example.name, func(t *testing.T) {
			dir := t.TempDir()
			before, after := filepath.Join(dir, "original.iges"), filepath.Join(dir, "canonical.iges")
			native := periodicTrimmedOracle(t, oracle, "write-"+example.name, before)
			raw, err := os.ReadFile(before)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := periodicTrimmedRecords(t, raw), periodicTrimmedRecords(t, periodicHoleFixture(t, example.name)); !reflect.DeepEqual(got.entities, want.entities) {
				t.Fatal("pinned author changed original periods, path or owners")
			}
			source := periodicTrimmedOracle(t, oracle, "read-wires", before)
			if !source.Valid || !native.Valid || !source.PeriodicU || !source.PeriodicV || source.Bounds != example.bounds ||
				source.Topology != [4]int{1, 1, example.edgeCount, example.edgeCount} ||
				len(source.Wires) != 1 || len(source.Wires[0].Edges) != example.edgeCount ||
				!reflect.DeepEqual(native.Samples, source.Samples) {
				t.Fatal("pinned transfer changed original split edges or physical native jets", source)
			}
			f, err := ReadPeriodicSeamTrimmedFace(ctx, bytes.NewReader(raw), Limits{})
			if example.edgeCount == 8 {
				if err == nil || f.Support != nil {
					t.Fatal("one-seam reader admitted both original seam crossings", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var carrier bytes.Buffer
			if err := WritePeriodicSeamTrimmedFace(ctx, &carrier, f); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(after, carrier.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			reopened := periodicTrimmedOracle(t, oracle, "read-wires", after)
			if !reflect.DeepEqual(source, reopened) {
				t.Fatal("canonical transfer changed source wires, measures or physical jets", reopened)
			}
			checkPeriodicTrimOracleWire(t, ctx, f.Support, reopened.Wires[0], f.Edges[:], true, f.ProjectionAxes, f.ProjectionOrientation)
			for _, sample := range reopened.Samples {
				jet, err := f.Support.EvaluateRational(ctx, rat(sample.UV[0]), rat(sample.UV[1]), geom.SurfacePatchOptions{})
				if err != nil {
					t.Fatal(err)
				}
				for k, p := range sample.Point {
					if jet.Point[k].Cmp(rat(p)) != 0 || jet.Du[k].Cmp(rat(sample.Du[k])) != 0 || jet.Dv[k].Cmp(rat(sample.Dv[k])) != 0 {
						t.Fatal("pinned canonical jet disagrees with the original exact tensor", sample)
					}
				}
			}
		})
	}
}
