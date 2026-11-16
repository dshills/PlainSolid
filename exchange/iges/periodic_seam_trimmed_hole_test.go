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

func TestReadPeriodicSeamTrimmedFaceWithHole(t *testing.T) {
	ctx := context.Background()
	raw := periodicHoleFixture(t, "exact-seam-hole")
	m, err := ReadPeriodicSeamTrimmedFaceWithHole(ctx, bytes.NewReader(raw), Limits{MaxEntities: 32, MaxControlPoints: 46})
	if err != nil {
		t.Fatal(err)
	}
	if m.OriginalIGES() != string(raw) || m.LengthUnit != .001 || m.SeamU.Cmp(big.NewRat(5, 1)) != 0 ||
		m.ProjectionAxes != [2]int{1, 2} || m.ProjectionOrientation != 1 ||
		[5]int{m.FaceID, m.SupportID, m.ContourID, m.UVCompositeID, m.ModelCompositeID} != ([5]int{1, 3, 5, 7, 21}) ||
		[3]int{m.Hole.ContourID, m.Hole.UVCompositeID, m.Hole.ModelCompositeID} != ([3]int{35, 37, 51}) {
		t.Fatal("lost original graph, common projection or upper U seam")
	}
	if periodicTrimIntervalStrings(m.CarrierBounds) != ([2][2]string{{"0", "5"}, {"0", "4"}}) ||
		periodicTrimIntervalStrings(m.Bounds) != ([2][2]string{{"19/4", "21/4"}, {"1", "3"}}) ||
		periodicTrimIntervalStrings(m.Hole.Bounds) != ([2][2]string{{"39/8", "41/8"}, {"3/2", "5/2"}}) {
		t.Fatal("changed original carrier and lifted loop bounds")
	}
	checkPeriodicTrimSourceWire(t, m.Edges[:], 9, 23, 0, "1")
	checkPeriodicTrimSourceWire(t, m.Hole.Edges[:], 39, 53, 20, "-1/4")
	stations := [6][2]string{{"39/8", "3/2"}, {"39/8", "5/2"}, {"5", "5/2"}, {"41/8", "5/2"}, {"41/8", "3/2"}, {"5", "3/2"}}
	for i, edge := range m.Hole.Edges {
		if got := [2]string{edge.UVStart[0].RatString(), edge.UVStart[1].RatString()}; got != stations[i] {
			t.Fatal("changed a directed inner seam station", i, got)
		}
	}
	for i, cell := range m.Hole.Cells {
		want := [2][2][2]string{{{"39/8", "5"}, {"3/2", "5/2"}}, {{"5", "41/8"}, {"3/2", "5/2"}}}
		if cell.Patch == nil || periodicTrimIntervalStrings(cell.Bounds) != want[i] ||
			!reflect.DeepEqual(cell.KnotOwners, m.Cells[i].KnotOwners) {
			t.Fatal("inner wire failed to use the same original physical knot owners as the outer lift", i, cell)
		}
	}
	for _, station := range []*big.Rat{big.NewRat(0, 1), big.NewRat(1, 3), big.NewRat(1, 1)} {
		left, err := m.Hole.Cells[0].Patch.EvaluateRational(ctx, big.NewRat(1, 1), station, geom.SurfacePatchEvaluationOptions{})
		if err != nil {
			t.Fatal(err)
		}
		right, err := m.Hole.Cells[1].Patch.EvaluateRational(ctx, big.NewRat(0, 1), station, geom.SurfacePatchEvaluationOptions{})
		if err != nil {
			t.Fatal(err)
		}
		v := new(big.Rat).Add(big.NewRat(3, 2), station)
		native, err := m.Support.EvaluateRational(ctx, m.SeamU, v, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		for k, q := range native.Point {
			if left.Point[k].Cmp(q) != 0 || right.Point[k].Cmp(q) != 0 || left.Du[k].Cmp(right.Du[k]) != 0 ||
				new(big.Rat).Mul(native.Du[k], big.NewRat(1, 8)).Cmp(right.Du[k]) != 0 ||
				left.Dv[k].Cmp(native.Dv[k]) != 0 || right.Dv[k].Cmp(native.Dv[k]) != 0 {
				t.Fatal("exact shared inner seam changed the fixture surface jets")
			}
		}
	}
	if plain, err := ReadPeriodicTrimmedFaceWithHoles(ctx, bytes.NewReader(raw), Limits{}); err == nil || plain.Support != nil {
		t.Fatal("single-cell reader admitted a lifted inner wire", err)
	}
	if simple, err := ReadPeriodicSeamTrimmedFace(ctx, bytes.NewReader(raw), Limits{}); err == nil || simple.Support != nil {
		t.Fatal("outer-only seam reader admitted inner topology", err)
	}
}

func TestReadPeriodicSeamTrimmedFaceWithHoleReject(t *testing.T) {
	ctx := context.Background()
	refuse := func(raw []byte, limits Limits, detail string) {
		t.Helper()
		f, err := ReadPeriodicSeamTrimmedFaceWithHole(ctx, bytes.NewReader(raw), limits)
		if err == nil || !strings.Contains(err.Error(), detail) || f.OriginalIGES() != "" || f.Support != nil ||
			f.Cells[0].Patch != nil || f.Hole.Cells[0].Patch != nil || f.Hole.ContourID != 0 {
			t.Fatal("unsupported seam hole returned a partial face or wrong refusal", err)
		}
	}
	raw := periodicHoleFixture(t, "exact-seam-hole")
	doc := periodicTrimmedRecords(t, raw)
	for _, edit := range []struct {
		id, index     int
		value, detail string
	}{
		{1, 5, "5", "duplicated or wrong periodic trim owner #5"},
		{35, 2, "7", "mismatched periodic trim contour support"},
		{41, 4, "5.00000000000000001", "does not follow the directed split rectangle"},
		{51, 3, "53", "duplicated or wrong periodic trim owner #53"},
		{55, 19, "5.56250000000000001", "model edge #55 disagrees with UV line #41"},
	} {
		refuse(changePeriodicTrimmedRecord(t, doc, edit.id, edit.index, edit.value), Limits{}, edit.detail)
	}

	// Move only the right UV records into the previous period. They still
	// trace the individual model curves, but no longer form one native lift.
	mixed := raw
	for _, edit := range []struct {
		id, index int
		value     string
	}{{43, 1, "0."}, {43, 4, "0.125"}, {45, 1, "0.125"}, {45, 4, "0.125"}, {47, 1, "0.125"}, {47, 4, "0."}} {
		mixed = changePeriodicTrimmedRecord(t, periodicTrimmedRecords(t, mixed), edit.id, edit.index, edit.value)
	}
	refuse(mixed, Limits{}, "split lifted trim at two original knot cells")
	refuse(periodicHoleFixture(t, "exact-seam-touching-hole"), Limits{}, "hole #35 is not strictly contained")
	refuse(exactPeriodicSeamFixture(t), Limits{}, "invalid explicit periodic trim hole count")
	refuse(raw, Limits{MaxEntities: 31}, "entity limit")
	refuse(raw, Limits{MaxControlPoints: 45}, "periodic model edge #63")
}

func TestReadPeriodicSeamTrimmedFaceWithHolePinnedOCCT(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_PERIODIC_128_TRIM_IGES")
	if oracle == "" {
		t.Skip("optional pinned OCCT author and reader for a periodic seam and inner trim")
	}
	ctx := context.Background()
	for _, example := range []struct {
		name           string
		topology       [4]int
		outerEdgeCount int
	}{
		{"exact-seam-hole", [4]int{1, 2, 12, 12}, 6},
		{"exact-seam-touching-hole", [4]int{1, 2, 14, 11}, 8},
	} {
		t.Run(example.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), example.name+".iges")
			if out, err := exec.Command(oracle, "write-"+example.name, path).CombinedOutput(); err != nil {
				t.Fatalf("independent author: %v\n%s", err, out)
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := periodicTrimmedRecords(t, raw), periodicTrimmedRecords(t, periodicHoleFixture(t, example.name)); !reflect.DeepEqual(got.entities, want.entities) {
				t.Fatal("pinned author changed the original seam and curve owners")
			}
			report := periodicTrimmedOracle(t, oracle, "read-wires", path)
			if !report.Valid || !report.PeriodicU || report.Topology != example.topology ||
				report.Bounds != [4]float64{4.75, 5.25, 1, 3} || len(report.Wires) != 2 ||
				len(report.Wires[0].Edges) != example.outerEdgeCount || len(report.Wires[1].Edges) != 6 {
				t.Fatal("pinned reader changed its seam transfer", report)
			}
			m, err := ReadPeriodicSeamTrimmedFaceWithHole(ctx, bytes.NewReader(raw), Limits{})
			if example.outerEdgeCount > 6 {
				if err == nil || !strings.Contains(err.Error(), "not strictly contained") || m.Support != nil ||
					report.Wires[0].Edges[1].ModelRange != [2]float64{2, 2.5} ||
					report.Wires[0].Edges[2].ModelRange != [2]float64{2.5, 3} {
					t.Fatal("expected original touch refusal and a split pinned repair", err, report)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if math.Abs(report.Area-3.067009869638307) > 1e-10 || math.Abs(report.Perimeter-12.123779934675332) > 1e-10 {
				t.Fatal("pinned reader changed physical area or perimeter", report)
			}
			checkPeriodicTrimOracleWire(t, ctx, m.Support, report.Wires[0], m.Edges[:], true, m.ProjectionAxes, m.ProjectionOrientation)
			checkPeriodicTrimOracleWire(t, ctx, m.Support, report.Wires[1], m.Hole.Edges[:], false, m.ProjectionAxes, m.ProjectionOrientation)
		})
	}
}
