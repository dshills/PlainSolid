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

func TestReadPeriodicTrimmedFaceV(t *testing.T) {
	ctx := context.Background()
	raw := periodicHoleFixture(t, "exact-v-cell")
	f, err := ReadPeriodicTrimmedFace(ctx, bytes.NewReader(raw), Limits{MaxEntities: 13, MaxControlPoints: 24})
	if err != nil {
		t.Fatal(err)
	}
	if f.OriginalIGES() != string(raw) || f.LengthUnit != .001 || f.Patch == nil ||
		[5]int{f.FaceID, f.SupportID, f.ContourID, f.UVCompositeID, f.ModelCompositeID} != ([5]int{1, 3, 5, 7, 17}) ||
		f.ProjectionAxes != [2]int{0, 2} || f.ProjectionOrientation != 1 {
		t.Fatal("lost original V-periodic face owners, file units, or source orientation")
	}
	u, v := f.KnotOwners[0], f.KnotOwners[1]
	if periodicTrimIntervalStrings(f.Bounds) != ([2][2]string{{"1", "3"}, {"5/4", "7/4"}}) ||
		periodicTrimIntervalStrings([2]geom.ExactParameterInterval{u.Domain, v.Domain}) != ([2][2]string{{"0", "4"}, {"1", "2"}}) ||
		u.Span != 0 || u.Cycle != nil || v.Span != 1 || v.Cycle.Sign() != 0 {
		t.Fatal("changed original U/V bounds or original V knot owner", f.KnotOwners)
	}
	ub, vb, _ := f.Support.Data()
	if ub.Periodic || !vb.Periodic || vb.Knots[0].Sign() != 0 || vb.Knots[vb.Count].Cmp(big.NewRat(5, 1)) != 0 {
		t.Fatal("lost original axis identity or cyclic V support")
	}
	checkPeriodicTrimSourceWire(t, f.Edges[:], 9, 19, 12, "1")
	vertices := [4][2]string{{"1", "5/4"}, {"3", "5/4"}, {"3", "7/4"}, {"1", "7/4"}}
	for i, e := range f.Edges {
		if got := [2]string{e.UVStart[0].RatString(), e.UVStart[1].RatString()}; got != vertices[i] {
			t.Fatal("changed original directed edge order", i, got)
		}
	}
	source, err := f.Support.EvaluateRational(ctx, big.NewRat(2, 1), big.NewRat(3, 2), geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	patch, err := f.Patch.EvaluateRational(ctx, big.NewRat(1, 2), big.NewRat(1, 2), geom.SurfacePatchEvaluationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for i, x := range source.Point {
		if patch.Point[i].Cmp(x) != 0 || new(big.Rat).Mul(source.Du[i], big.NewRat(2, 1)).Cmp(patch.Du[i]) != 0 ||
			new(big.Rat).Mul(source.Dv[i], big.NewRat(1, 2)).Cmp(patch.Dv[i]) != 0 {
			t.Fatal("normalized patch changed source orientation or exact mapped derivatives")
		}
	}
	if ordinary, err := Read(bytes.NewReader(raw), Limits{}); err == nil || len(ordinary.Entities) != 0 {
		t.Fatal("ordinary geometry reader admitted trimmed topology", err)
	}
}

func TestReadPeriodicTrimmedFaceVReject(t *testing.T) {
	ctx := context.Background()
	refuse := func(raw []byte, limits Limits, detail string) {
		t.Helper()
		f, err := ReadPeriodicTrimmedFace(ctx, bytes.NewReader(raw), limits)
		if err == nil || !strings.Contains(err.Error(), detail) || f.FaceID != 0 ||
			f.Support != nil || f.Patch != nil || f.OriginalIGES() != "" {
			t.Fatal("unproved V source returned partial face or wrong refusal", err)
		}
	}
	raw := periodicHoleFixture(t, "exact-v-cell")
	refuse(periodicHoleFixture(t, "exact-v-cell-clockwise"), Limits{}, "does not follow the directed rectangle")
	doc := periodicTrimmedRecords(t, raw)
	for _, edit := range []struct {
		id, index     int
		value, detail string
	}{
		{3, 9, "0", "one V-periodic support"},
		{3, 82, "1.", "meets or crosses an original knot station"},
		{5, 2, "7", "mismatched periodic trim contour support"},
		{17, 3, "19", "duplicated or wrong periodic trim owner #19"},
		{21, 19, "0.90625000000000001", "model edge #21 disagrees with UV line #11"},
	} {
		refuse(changePeriodicTrimmedRecord(t, doc, edit.id, edit.index, edit.value), Limits{}, edit.detail)
	}
	refuse(raw, Limits{MaxEntities: 12}, "entity limit")
	refuse(raw, Limits{MaxControlPoints: 23}, "model edge #25")
}

func TestReadPeriodicTrimmedFaceVPinnedOCCT(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_PERIODIC_128_TRIM_IGES")
	if oracle == "" {
		t.Skip("optional pinned OCCT V-periodic type-144 author and reader")
	}
	ctx := context.Background()
	for _, name := range []string{"exact-v-cell", "exact-v-cell-clockwise"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), name+".iges")
			if out, err := exec.Command(oracle, "write-"+name, path).CombinedOutput(); err != nil {
				t.Fatalf("independent author: %v\n%s", err, out)
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := periodicTrimmedRecords(t, raw), periodicTrimmedRecords(t, periodicHoleFixture(t, name)); !reflect.DeepEqual(got.entities, want.entities) {
				t.Fatal("pinned author changed the stored V support or directed owners")
			}
			report := periodicTrimmedOracle(t, oracle, "read-wires", path)
			if !report.Valid || report.PeriodicU || !report.PeriodicV || report.Topology != [4]int{1, 1, 4, 4} ||
				report.Bounds != [4]float64{1, 3, 1.25, 1.75} || len(report.Wires) != 1 || len(report.Wires[0].Edges) != 4 ||
				math.Abs(report.Area-4.633877174619431) > 1e-10 || math.Abs(report.Perimeter-8.63387718171492) > 1e-10 {
				t.Fatal("pinned reader changed the original V-periodic support or physical trim", report)
			}
			m, err := ReadPeriodicTrimmedFace(ctx, bytes.NewReader(raw), Limits{})
			if name == "exact-v-cell-clockwise" {
				e := report.Wires[0].Edges[0]
				if err == nil || m.Support != nil || e.ModelRange != [2]float64{18, 19} ||
					e.UVStart != [2]float64{1, 1.75} || e.UVEnd != [2]float64{1, 1.25} {
					t.Fatal("expected strict original winding refusal and pinned rewinding", err, report)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			checkPeriodicTrimOracleWire(t, ctx, m.Support, report.Wires[0], m.Edges[:], true, m.ProjectionAxes, m.ProjectionOrientation)
			for _, sample := range report.Samples {
				jet, err := m.Support.EvaluateRational(ctx, rat(sample.UV[0]), rat(sample.UV[1]), geom.SurfacePatchOptions{})
				if err != nil {
					t.Fatal(err)
				}
				checkPeriodic128Vector(t, jet.Point, sample.Point)
				checkPeriodic128Vector(t, jet.Du, sample.Du)
				checkPeriodic128Vector(t, jet.Dv, sample.Dv)
			}
		})
	}
}
