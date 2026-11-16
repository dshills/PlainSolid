package iges

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"plainsolid/geom"
)

func TestReadPeriodicTrimmedFaceUV(t *testing.T) {
	ctx := context.Background()
	raw := periodicHoleFixture(t, "exact-uv-cell")
	f, err := ReadPeriodicTrimmedFace(ctx, bytes.NewReader(raw), Limits{MaxEntities: 13, MaxControlPoints: 61})
	if err != nil {
		t.Fatal(err)
	}
	if f.OriginalIGES() != string(raw) || f.LengthUnit != .001 || f.Patch == nil ||
		[5]int{f.FaceID, f.SupportID, f.ContourID, f.UVCompositeID, f.ModelCompositeID} != ([5]int{1, 3, 5, 7, 17}) ||
		f.ProjectionAxes != [2]int{0, 1} || f.ProjectionOrientation != 1 {
		t.Fatal("lost the original doubly periodic face, units or projected orientation")
	}
	u, v := f.KnotOwners[0], f.KnotOwners[1]
	if periodicTrimIntervalStrings(f.Bounds) != ([2][2]string{{"5/4", "7/4"}, {"9/4", "11/4"}}) ||
		periodicTrimIntervalStrings([2]geom.ExactParameterInterval{u.Domain, v.Domain}) != ([2][2]string{{"1", "2"}, {"2", "3"}}) ||
		u.Span != 1 || u.Cycle.Sign() != 0 || v.Span != 0 || v.Cycle.Sign() != 0 {
		t.Fatal("lost an original knot rectangle on one of the two periodic axes", f.KnotOwners)
	}
	uBasis, vBasis, _ := f.Support.Data()
	for i, b := range [2]geom.ExactTensorSplineAxis{uBasis, vBasis} {
		if !b.Periodic || b.Degree != 2 || b.Count != 5 || b.Knots[0].Cmp(big.NewRat(int64(2*i), 1)) != 0 ||
			b.Knots[b.Count].Cmp(big.NewRat(int64(5+2*i), 1)) != 0 {
			t.Fatal("trim narrowed the original full cyclic support", i, b)
		}
	}
	checkPeriodicTrimSourceWire(t, f.Edges[:], 9, 19, 32, "1/4")
	want := [4][2]string{{"5/4", "9/4"}, {"7/4", "9/4"}, {"7/4", "11/4"}, {"5/4", "11/4"}}
	for i, e := range f.Edges {
		if got := [2]string{e.UVStart[0].RatString(), e.UVStart[1].RatString()}; got != want[i] {
			t.Fatal("changed a directed source station", i, got)
		}
	}
	source, err := f.Support.EvaluateRational(ctx, big.NewRat(3, 2), big.NewRat(5, 2), geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	wrapped, err := f.Support.EvaluateRational(ctx, big.NewRat(13, 2), big.NewRat(15, 2), geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	patch, err := f.Patch.EvaluateRational(ctx, big.NewRat(1, 2), big.NewRat(1, 2), geom.SurfacePatchEvaluationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if wrapped.U.Cycle.Cmp(big.NewInt(1)) != 0 || wrapped.V.Cycle.Cmp(big.NewInt(1)) != 0 {
		t.Fatal("lost an original cyclic axis")
	}
	for i, x := range source.Point {
		if patch.Point[i].Cmp(x) != 0 || wrapped.Point[i].Cmp(x) != 0 ||
			wrapped.Du[i].Cmp(source.Du[i]) != 0 || wrapped.Dv[i].Cmp(source.Dv[i]) != 0 ||
			new(big.Rat).Mul(source.Du[i], big.NewRat(1, 2)).Cmp(patch.Du[i]) != 0 ||
			new(big.Rat).Mul(source.Dv[i], big.NewRat(1, 2)).Cmp(patch.Dv[i]) != 0 {
			t.Fatal("changed a point or physical derivative in a repeated or normalized chart")
		}
	}
	if m, err := Read(bytes.NewReader(raw), Limits{}); err == nil || len(m.Entities) != 0 {
		t.Fatal("ordinary geometry reader admitted trimmed topology", err)
	}
	if _, err := readPeriodicTrimSource(ctx, bytes.NewReader(raw), Limits{}, 4); err == nil {
		t.Fatal("U-only topology policy admitted a second cyclic axis")
	}
}

func TestReadPeriodicTrimmedFaceUVReject(t *testing.T) {
	ctx := context.Background()
	refuse := func(raw []byte, limits Limits, detail string) {
		t.Helper()
		f, err := ReadPeriodicTrimmedFace(ctx, bytes.NewReader(raw), limits)
		if err == nil || !strings.Contains(err.Error(), detail) || f.FaceID != 0 ||
			f.Support != nil || f.Patch != nil || f.OriginalIGES() != "" {
			t.Fatal("unsafe doubly periodic source returned partial proof or wrong refusal", err)
		}
	}
	refuse(periodicHoleFixture(t, "exact-uv-cell-clockwise"), Limits{}, "does not follow the directed rectangle")

	raw := periodicHoleFixture(t, "exact-uv-cell")
	doc := periodicTrimmedRecords(t, raw)
	for _, edit := range []struct {
		id, index     int
		value, detail string
	}{
		{3, 226, "1.", "meets or crosses an original knot station"},
		{3, 228, "2.", "meets or crosses an original knot station"},
		{3, 94, "30.00000000000000001", "inconsistent repeated periodic surface pole"},
		{3, 184, "30.00000000000000001", "inconsistent repeated periodic surface pole"},
		{17, 3, "19", "duplicated or wrong periodic trim owner #19"},
		{21, 19, "-1.12500000000000001", "model edge #21 disagrees with UV line #11"},
	} {
		refuse(changePeriodicTrimmedRecord(t, doc, edit.id, edit.index, edit.value), Limits{}, edit.detail)
	}
	refuse(raw, Limits{MaxEntities: 12}, "entity limit")
	refuse(raw, Limits{MaxControlPoints: 60}, "model edge #25")
}

func TestReadPeriodicTrimmedFaceUVPinnedOCCT(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_PERIODIC_128_TRIM_IGES")
	if oracle == "" {
		t.Skip("optional pinned OCCT double-periodic type-144 author and reader")
	}
	ctx := context.Background()
	for _, name := range []string{"exact-uv-cell", "exact-uv-cell-clockwise"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), name+".iges")
			native := periodicTrimmedOracle(t, oracle, "write-"+name, path)
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := periodicTrimmedRecords(t, raw), periodicTrimmedRecords(t, periodicHoleFixture(t, name)); !reflect.DeepEqual(got.entities, want.entities) {
				t.Fatal("pinned author changed stored cyclic ownership or directed curves")
			}
			reopen := periodicTrimmedOracle(t, oracle, "read-wires", path)
			if !native.Valid || !native.PeriodicU || !native.PeriodicV || native.Topology != [4]int{1, 1, 4, 4} ||
				!reopen.Valid || !reopen.PeriodicU || !reopen.PeriodicV {
				t.Fatal("second cyclic axis missing in the native author or transferred face", native, reopen)
			}
			m, err := ReadPeriodicTrimmedFace(ctx, bytes.NewReader(raw), Limits{})
			if name == "exact-uv-cell-clockwise" {
				if err == nil || m.Support != nil || reopen.Topology != [4]int{1, 2, 6, 5} || len(reopen.Wires) != 2 ||
					reopen.Wires[0].Outer || !reopen.Wires[1].Outer || reopen.Bounds != [4]float64{3.375, 8.375, 6.875, 11.875} {
					t.Fatal("expected strict original-winding refusal and different pinned transferred topology", err, reopen)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if reopen.Topology != [4]int{1, 1, 4, 4} || reopen.Bounds != [4]float64{1.25, 1.75, 2.25, 2.75} ||
				len(reopen.Wires) != 1 || !reflect.DeepEqual(native.Samples, reopen.Samples) ||
				math.Abs(reopen.Area-28.330004952281637) > 1e-10 || math.Abs(reopen.Perimeter-24.786404260221083) > 1e-10 {
				t.Fatal("pinned transfer changed original physical bounds, jets, area or perimeter", native, reopen)
			}
			checkPeriodicTrimOracleWire(t, ctx, m.Support, reopen.Wires[0], m.Edges[:], true, m.ProjectionAxes, m.ProjectionOrientation)
			for _, sample := range native.Samples {
				jet, err := m.Support.EvaluateRational(ctx, rat(sample.UV[0]), rat(sample.UV[1]), geom.SurfacePatchOptions{})
				if err != nil {
					t.Fatal(err)
				}
				for i := range sample.Point {
					if jet.Point[i].Cmp(rat(sample.Point[i])) != 0 || jet.Du[i].Cmp(rat(sample.Du[i])) != 0 ||
						jet.Dv[i].Cmp(rat(sample.Dv[i])) != 0 {
						t.Fatal("native author or pinned transfer changed a physical jet", sample)
					}
				}
			}
		})
	}
}
