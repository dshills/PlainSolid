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

func exactPeriodicSeamFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/occt-periodic-exact-seam-trimmed-128.iges")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestReadPeriodicSeamTrimmedFace(t *testing.T) {
	ctx := context.Background()
	raw := exactPeriodicSeamFixture(t)
	m, err := ReadPeriodicSeamTrimmedFace(ctx, bytes.NewReader(raw), Limits{MaxEntities: 17, MaxControlPoints: 30})
	if err != nil {
		t.Fatal(err)
	}
	if m.OriginalIGES() != string(raw) || m.LengthUnit != .001 ||
		[5]int{m.FaceID, m.SupportID, m.ContourID, m.UVCompositeID, m.ModelCompositeID} != [5]int{1, 3, 5, 7, 21} ||
		m.ProjectionAxes != [2]int{1, 2} || m.ProjectionOrientation != 1 || m.SeamU.Cmp(big.NewRat(5, 1)) != 0 {
		t.Fatal("lost the original seam graph, file unit, or projected winding")
	}
	if periodicTrimIntervalStrings(m.CarrierBounds) != ([2][2]string{{"0", "5"}, {"0", "4"}}) ||
		periodicTrimIntervalStrings(m.Bounds) != ([2][2]string{{"19/4", "21/4"}, {"1", "3"}}) {
		t.Fatal("lift changed original support or physical trim intervals", m.CarrierBounds, m.Bounds)
	}
	for i, cell := range m.Cells {
		model := [2][2][2]string{{{"19/4", "5"}, {"1", "3"}}, {{"5", "21/4"}, {"1", "3"}}}
		original := [2][2][2]string{{{"4", "5"}, {"0", "4"}}, {{"5", "6"}, {"0", "4"}}}
		u, v := cell.KnotOwners[0], cell.KnotOwners[1]
		owners := [2]geom.ExactParameterInterval{u.Domain, v.Domain}
		if cell.Patch == nil || periodicTrimIntervalStrings(cell.Bounds) != model[i] ||
			periodicTrimIntervalStrings(owners) != original[i] || u.Cycle.Int64() != int64(i) ||
			u.Span != 4*(1-i) || v.Cycle != nil || v.Span != 0 {
			t.Fatal("lost original one-sided station owner", i, cell)
		}
	}
	corners := [6][2]string{{"19/4", "3"}, {"19/4", "1"}, {"5", "1"}, {"21/4", "1"}, {"21/4", "3"}, {"5", "3"}}
	for i, edge := range m.Edges {
		if edge.UVSourceID != 9+2*i || edge.ModelSourceID != 23+2*i || edge.ModelCurve == nil ||
			[2]string{edge.UVStart[0].RatString(), edge.UVStart[1].RatString()} != corners[i] ||
			[2]string{edge.UVEnd[0].RatString(), edge.UVEnd[1].RatString()} != corners[(i+1)%6] ||
			edge.ModelParameter.Lower.Cmp(big.NewRat(int64(2*i), 1)) != 0 ||
			edge.ModelParameter.Upper.Cmp(big.NewRat(int64(2*i+1), 1)) != 0 {
			t.Fatal("lost a split seam station or its directed model owner", edge)
		}
	}
	for _, station := range []*big.Rat{big.NewRat(0, 1), big.NewRat(1, 3), big.NewRat(1, 1)} {
		left, err := m.Cells[0].Patch.EvaluateRational(ctx, big.NewRat(1, 1), station, geom.SurfacePatchEvaluationOptions{})
		if err != nil {
			t.Fatal(err)
		}
		right, err := m.Cells[1].Patch.EvaluateRational(ctx, big.NewRat(0, 1), station, geom.SurfacePatchEvaluationOptions{})
		if err != nil {
			t.Fatal(err)
		}
		v := new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Mul(station, big.NewRat(2, 1)))
		source, err := m.Support.EvaluateRational(ctx, m.SeamU, v, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		for k, q := range source.Point {
			if q.Cmp(left.Point[k]) != 0 || q.Cmp(right.Point[k]) != 0 ||
				left.Du[k].Cmp(right.Du[k]) != 0 || left.Dv[k].Cmp(right.Dv[k]) != 0 ||
				new(big.Rat).Mul(source.Du[k], big.NewRat(1, 4)).Cmp(right.Du[k]) != 0 ||
				new(big.Rat).Mul(source.Dv[k], big.NewRat(2, 1)).Cmp(right.Dv[k]) != 0 {
				t.Fatal("exact extracted cells disagree on the seam point or fixture derivatives")
			}
		}
	}
	if old, err := ReadPeriodicTrimmedFace(ctx, bytes.NewReader(raw), Limits{}); err == nil || old.Patch != nil {
		t.Fatal("one-cell reader admitted a split seam")
	}
	if ordinary, err := Read(bytes.NewReader(raw), Limits{}); err == nil || len(ordinary.Entities) != 0 {
		t.Fatal("ordinary geometry reader admitted seam topology")
	}
}

func TestReadPeriodicSeamTrimmedFaceReject(t *testing.T) {
	ctx := context.Background()
	raw := exactPeriodicSeamFixture(t)
	check := func(data []byte, l Limits, detail string) {
		t.Helper()
		m, err := ReadPeriodicSeamTrimmedFace(ctx, bytes.NewReader(data), l)
		if err == nil || !strings.Contains(err.Error(), detail) || m.OriginalIGES() != "" || m.Cells[0].Patch != nil || m.Support != nil {
			t.Fatal("unsafe split seam returned a partial proof", err)
		}
	}
	doc := periodicTrimmedRecords(t, raw)
	for _, edit := range []struct {
		id, valueIndex int
		value, detail  string
	}{
		{3, 80, "4.75", "full original carrier intervals"},
		{13, 1, "0.", "expected one lift crossing the upper original U seam"},
		{13, 1, "5.00000000000000001", "does not follow the directed split rectangle"},
		{13, 4, "6.25", "two original knot cells"},
		{21, 4, "25", "duplicated or wrong periodic trim owner"},
		{27, 19, "5.37500000000000001", "model edge #27 disagrees with UV line #13"},
	} {
		check(changePeriodicTrimmedRecord(t, doc, edit.id, edit.valueIndex, edit.value), Limits{}, edit.detail)
	}
	check(raw, Limits{MaxEntities: 16}, "entity limit")
	check(raw, Limits{MaxControlPoints: 29}, "model edge #33")
	check(periodicTrimmedFixture(t, true), Limits{}, "17 source owners")
	check(periodicTrimmedFixture(t, false), Limits{}, "17 source owners")
}

type periodicTrimOracleEdge struct {
	UVStart, UVEnd, UVMid          [2]float64
	ModelStart, ModelEnd, ModelMid [3]float64
	UVRange, ModelRange            [2]float64
}

func TestReadPeriodicSeamTrimmedFacePinnedOCCT(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_PERIODIC_128_TRIM_IGES")
	if oracle == "" {
		t.Skip("optional pinned OCCT exact seam author and reader")
	}
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "exact-seam.iges")
	if out, err := exec.Command(oracle, "write-exact-seam", path).CombinedOutput(); err != nil {
		t.Fatalf("independent seam author: %v\n%s", err, out)
	}
	fresh, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := periodicTrimmedRecords(t, fresh), periodicTrimmedRecords(t, exactPeriodicSeamFixture(t)); !reflect.DeepEqual(got.entities, want.entities) {
		t.Fatal("fresh writer changed original split UV/model ownership or geometry")
	}
	m, err := ReadPeriodicSeamTrimmedFace(ctx, bytes.NewReader(fresh), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	report := periodicTrimmedOracle(t, oracle, "read-edges", path)
	if !report.Valid || !report.PeriodicU || report.Topology != [4]int{1, 1, 6, 6} || len(report.Edges) != 6 ||
		report.Bounds != [4]float64{4.75, 5.25, 1, 3} ||
		math.Abs(report.Area-4.08589030450632) > 1e-12 || math.Abs(report.Perimeter-8.086024487851825) > 1e-12 {
		t.Fatal("pinned reader changed native boundary topology or physical measures", report)
	}
	for i, e := range report.Edges {
		raw := m.Edges[i]
		p := [2]*big.Rat{new(big.Rat).Mul(new(big.Rat).Add(raw.UVStart[0], raw.UVEnd[0]), big.NewRat(1, 2)),
			new(big.Rat).Mul(new(big.Rat).Add(raw.UVStart[1], raw.UVEnd[1]), big.NewRat(1, 2))}
		lo, _ := raw.ModelParameter.Lower.Float64()
		hi, _ := raw.ModelParameter.Upper.Float64()
		// OCCT assigns the paired model parameter interval to the pcurve
		// while retaining the exact directed UV points and split stations.
		if e.ModelRange != [2]float64{lo, hi} || e.UVRange != e.ModelRange {
			t.Fatal("pinned reader changed an original type-126 station range", e)
		}
		for j, uv := range [3][2]*big.Rat{raw.UVStart, p, raw.UVEnd} {
			actual := [3][2]float64{e.UVStart, e.UVMid, e.UVEnd}[j]
			for k := range uv {
				if uv[k].Cmp(rat(actual[k])) != 0 {
					t.Fatal("pinned reader changed an explicit UV vertex or directed edge midpoint", i, j, actual)
				}
			}
			jet, err := m.Support.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchOptions{})
			if err != nil {
				t.Fatal(err)
			}
			checkPeriodic128Vector(t, jet.Point, [3][3]float64{e.ModelStart, e.ModelMid, e.ModelEnd}[j])
		}
	}
	for _, p := range report.Samples {
		jet, err := m.Support.EvaluateRational(ctx, rat(p.UV[0]), rat(p.UV[1]), geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		checkPeriodic128Vector(t, jet.Point, p.Point)
		checkPeriodic128Vector(t, jet.Du, p.Du)
		checkPeriodic128Vector(t, jet.Dv, p.Dv)
	}
}

func periodicTrimIntervalStrings(d [2]geom.ExactParameterInterval) [2][2]string {
	return [2][2]string{{d[0].Lower.RatString(), d[0].Upper.RatString()}, {d[1].Lower.RatString(), d[1].Upper.RatString()}}
}
