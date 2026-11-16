package iges

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"plainsolid/geom"
)

func TestReadPeriodicSeamTrimmedFaceV(t *testing.T) {
	ctx := context.Background()
	raw := periodicHoleFixture(t, "exact-v-seam")
	f, err := ReadPeriodicSeamTrimmedFace(ctx, bytes.NewReader(raw), Limits{MaxEntities: 17, MaxControlPoints: 30})
	if err != nil {
		t.Fatal(err)
	}
	if f.OriginalIGES() != string(raw) || f.LengthUnit != .001 || f.SeamU != nil || f.SeamV.Cmp(big.NewRat(5, 1)) != 0 ||
		[5]int{f.FaceID, f.SupportID, f.ContourID, f.UVCompositeID, f.ModelCompositeID} != ([5]int{1, 3, 5, 7, 21}) ||
		f.ProjectionAxes != [2]int{1, 2} || f.ProjectionOrientation != -1 {
		t.Fatal("lost original V-seam topology or continuous projected orientation")
	}
	if periodicTrimIntervalStrings(f.CarrierBounds) != ([2][2]string{{"0", "4"}, {"0", "5"}}) ||
		periodicTrimIntervalStrings(f.Bounds) != ([2][2]string{{"1", "3"}, {"19/4", "21/4"}}) {
		t.Fatal("changed original carrier or lifted V bounds")
	}
	for i, c := range f.Cells {
		box := [2][2][2]string{{{"1", "3"}, {"19/4", "5"}}, {{"1", "3"}, {"5", "21/4"}}}
		owner := [2][2][2]string{{{"0", "4"}, {"4", "5"}}, {{"0", "4"}, {"5", "6"}}}
		u, v := c.KnotOwners[0], c.KnotOwners[1]
		if c.Patch == nil || periodicTrimIntervalStrings(c.Bounds) != box[i] ||
			periodicTrimIntervalStrings([2]geom.ExactParameterInterval{u.Domain, v.Domain}) != owner[i] ||
			u.Span != 0 || u.Cycle != nil || v.Span != 4*(1-i) || v.Cycle.Int64() != int64(i) {
			t.Fatal("lost exact V knot owner or lifted cycle", i, c)
		}
	}
	checkPeriodicTrimSourceWire(t, f.Edges[:], 9, 23, 12, "1")
	stations := [6][2]string{{"1", "19/4"}, {"3", "19/4"}, {"3", "5"}, {"3", "21/4"}, {"1", "21/4"}, {"1", "5"}}
	for i, e := range f.Edges {
		if got := [2]string{e.UVStart[0].RatString(), e.UVStart[1].RatString()}; got != stations[i] {
			t.Fatal("changed an original V seam vertex", i, got)
		}
	}
	for _, u := range []*big.Rat{big.NewRat(0, 1), big.NewRat(1, 3), big.NewRat(1, 1)} {
		lo, err := f.Cells[0].Patch.EvaluateRational(ctx, u, big.NewRat(1, 1), geom.SurfacePatchEvaluationOptions{})
		if err != nil {
			t.Fatal(err)
		}
		hi, err := f.Cells[1].Patch.EvaluateRational(ctx, u, big.NewRat(0, 1), geom.SurfacePatchEvaluationOptions{})
		if err != nil {
			t.Fatal(err)
		}
		physical := new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Mul(u, big.NewRat(2, 1)))
		source, err := f.Support.EvaluateRational(ctx, physical, f.SeamV, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		for k, x := range source.Point {
			if x.Cmp(lo.Point[k]) != 0 || x.Cmp(hi.Point[k]) != 0 || lo.Du[k].Cmp(hi.Du[k]) != 0 || lo.Dv[k].Cmp(hi.Dv[k]) != 0 ||
				new(big.Rat).Mul(source.Du[k], big.NewRat(2, 1)).Cmp(lo.Du[k]) != 0 ||
				new(big.Rat).Mul(source.Dv[k], big.NewRat(1, 4)).Cmp(lo.Dv[k]) != 0 {
				t.Fatal("exact V-seam patches changed a shared point or physical derivative")
			}
		}
	}
	if old, err := ReadPeriodicTrimmedFace(ctx, bytes.NewReader(raw), Limits{}); err == nil || old.Support != nil {
		t.Fatal("one-cell reader accepted a split V seam", err)
	}
}

func TestReadPeriodicSeamTrimmedFaceVReject(t *testing.T) {
	ctx := context.Background()
	refuse := func(raw []byte, limits Limits, detail string) {
		t.Helper()
		f, err := ReadPeriodicSeamTrimmedFace(ctx, bytes.NewReader(raw), limits)
		if err == nil || !strings.Contains(err.Error(), detail) || f.FaceID != 0 || f.Support != nil ||
			f.Cells[0].Patch != nil || f.OriginalIGES() != "" {
			t.Fatal("unsafe V seam returned partial proof or wrong refusal", err)
		}
	}
	raw := periodicHoleFixture(t, "exact-v-seam")
	doc := periodicTrimmedRecords(t, raw)
	for _, edit := range []struct {
		id, index     int
		value, detail string
	}{
		{3, 82, "1.", "full original carrier intervals"},
		{11, 5, "5.00000000000000001", "does not follow the directed split rectangle"},
		{13, 2, "0.", "expected one lift crossing the upper original V seam"},
		{13, 5, "6.25", "two original knot cells"},
		{21, 3, "23", "duplicated or wrong periodic trim owner #23"},
		{25, 19, "5.62500000000000001", "model edge #25 disagrees with UV line #11"},
	} {
		refuse(changePeriodicTrimmedRecord(t, doc, edit.id, edit.index, edit.value), Limits{}, edit.detail)
	}
	refuse(raw, Limits{MaxEntities: 16}, "entity limit")
	refuse(raw, Limits{MaxControlPoints: 29}, "model edge #33")
}

func TestWritePeriodicSeamTrimmedFaceV(t *testing.T) {
	ctx := context.Background()
	raw := periodicHoleFixture(t, "exact-v-seam")
	read := func(data []byte) PeriodicSeamTrimmedFace {
		t.Helper()
		f, err := ReadPeriodicSeamTrimmedFace(ctx, bytes.NewReader(data), Limits{})
		if err != nil {
			t.Fatal(err)
		}
		return f
	}
	f := read(raw)
	var first, second bytes.Buffer
	if err := WritePeriodicSeamTrimmedFace(ctx, &first, f); err != nil {
		t.Fatal(err)
	}
	comparePeriodicTrimRecords(t, raw, first.Bytes())
	again := read(first.Bytes())
	if err := WritePeriodicSeamTrimmedFace(ctx, &second, again); err != nil || !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("canonical V seam did not repeat byte-for-byte", err)
	}
	again.original = f.original
	if !reflect.DeepEqual(again, f) || f.OriginalIGES() != string(raw) {
		t.Fatal("strict canonical reopen changed the original V-seam proof or retained bytes")
	}
	f.SeamV.SetInt64(6)
	out := bytes.NewBufferString("untouched")
	if err := WritePeriodicSeamTrimmedFace(ctx, out, f); err == nil || out.String() != "untouched" {
		t.Fatal("altered native seam candidate published a carrier", err)
	}
}

func TestReadPeriodicSeamTrimmedFaceVPinnedOCCT(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_PERIODIC_128_TRIM_IGES")
	if oracle == "" {
		t.Skip("optional pinned OCCT exact V-seam author and reader")
	}
	ctx := context.Background()
	dir := t.TempDir()
	before, after := filepath.Join(dir, "original.iges"), filepath.Join(dir, "canonical.iges")
	if out, err := exec.Command(oracle, "write-exact-v-seam", before).CombinedOutput(); err != nil {
		t.Fatalf("independent V-seam author: %v\n%s", err, out)
	}
	raw, err := os.ReadFile(before)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := periodicTrimmedRecords(t, raw), periodicTrimmedRecords(t, periodicHoleFixture(t, "exact-v-seam")); !reflect.DeepEqual(got.entities, want.entities) {
		t.Fatal("pinned author changed original V seam records")
	}
	f, err := ReadPeriodicSeamTrimmedFace(ctx, bytes.NewReader(raw), Limits{})
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
	source := periodicTrimmedOracle(t, oracle, "read-wires", before)
	open := periodicTrimmedOracle(t, oracle, "read-wires", after)
	if !reflect.DeepEqual(source, open) || !open.Valid || open.PeriodicU || !open.PeriodicV ||
		open.Topology != [4]int{1, 1, 6, 6} || open.Bounds != [4]float64{1, 3, 4.75, 5.25} ||
		len(open.Wires) != 1 || len(open.Samples) != 5 ||
		math.Abs(open.Area-4.086024488054441) > 1e-10 || math.Abs(open.Perimeter-8.086024487851825) > 1e-10 {
		t.Fatal("pinned V seam reopen changed the six directed pairs or physical measures", open)
	}
	checkPeriodicTrimOracleWire(t, ctx, f.Support, open.Wires[0], f.Edges[:], true, f.ProjectionAxes, f.ProjectionOrientation)
	for _, sample := range open.Samples {
		jet, err := f.Support.EvaluateRational(ctx, rat(sample.UV[0]), rat(sample.UV[1]), geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		checkPeriodic128Vector(t, jet.Point, sample.Point)
		checkPeriodic128Vector(t, jet.Du, sample.Du)
		checkPeriodic128Vector(t, jet.Dv, sample.Dv)
	}
}
