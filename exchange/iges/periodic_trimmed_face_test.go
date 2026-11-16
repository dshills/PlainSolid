package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

func TestReadPeriodicTrimmedFace(t *testing.T) {
	ctx := context.Background()
	source := periodicTrimmedFixture(t, false)
	m, err := ReadPeriodicTrimmedFace(ctx, bytes.NewReader(source), Limits{MaxEntities: 13, MaxControlPoints: 24})
	if err != nil {
		t.Fatal(err)
	}
	if m.OriginalIGES() != string(source) || m.LengthUnit != .001 ||
		[5]int{m.FaceID, m.SupportID, m.ContourID, m.UVCompositeID, m.ModelCompositeID} != [5]int{1, 3, 5, 7, 17} ||
		m.ProjectionAxes != [2]int{0, 2} || m.ProjectionOrientation != -1 {
		t.Fatal("lost the source graph, physical units or projected orientation")
	}
	wantBounds := [2][2]string{{"5/4", "7/4"}, {"1", "3"}}
	wantOwners := [2][2]string{{"1", "2"}, {"0", "4"}}
	for i, owner := range m.KnotOwners {
		if [2]string{m.Bounds[i].Lower.RatString(), m.Bounds[i].Upper.RatString()} != wantBounds[i] ||
			[2]string{owner.Domain.Lower.RatString(), owner.Domain.Upper.RatString()} != wantOwners[i] ||
			owner.Span != 1-i || i == 0 && owner.Cycle.Sign() != 0 || i == 1 && owner.Cycle != nil {
			t.Fatal("lost original face bounds or physical knot owners", m.Bounds, m.KnotOwners)
		}
	}
	u, v, _ := m.Support.Data()
	if !u.Periodic || v.Periodic || u.Knots[0].Sign() != 0 || u.Knots[u.Count].Cmp(big.NewRat(5, 1)) != 0 {
		t.Fatal("trim narrowed the original cyclic support")
	}
	wantedUV := [4][2]string{{"5/4", "3"}, {"5/4", "1"}, {"7/4", "1"}, {"7/4", "3"}}
	wantedModelDomains := [4][2]string{{"1", "3"}, {"5/4", "7/4"}, {"1", "3"}, {"13/4", "15/4"}}
	for i, edge := range m.Edges {
		next := wantedUV[(i+1)%4]
		if edge.UVSourceID != 9+2*i || edge.ModelSourceID != 19+2*i || edge.ModelCurve == nil ||
			[2]string{edge.UVStart[0].RatString(), edge.UVStart[1].RatString()} != wantedUV[i] ||
			[2]string{edge.UVEnd[0].RatString(), edge.UVEnd[1].RatString()} != next ||
			[2]string{edge.ModelParameter.Lower.RatString(), edge.ModelParameter.Upper.RatString()} != wantedModelDomains[i] {
			t.Fatal("original directed station owners changed", edge)
		}
	}
	spatial, err := m.Support.EvaluateRational(ctx, big.NewRat(3, 2), big.NewRat(2, 1), geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	patch, err := m.Patch.EvaluateRational(ctx, big.NewRat(1, 2), big.NewRat(1, 2), geom.SurfacePatchEvaluationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for i, x := range spatial.Point {
		if x.Cmp(patch.Point[i]) != 0 || new(big.Rat).Mul(spatial.Du[i], big.NewRat(1, 2)).Cmp(patch.Du[i]) != 0 ||
			new(big.Rat).Mul(spatial.Dv[i], big.NewRat(2, 1)).Cmp(patch.Dv[i]) != 0 {
			t.Fatal("finite native patch changed the exact point or UV normalization")
		}
	}
	if got, err := Read(bytes.NewReader(source), Limits{}); err == nil || len(got.Entities) != 0 {
		t.Fatal("ordinary geometry reader admitted an unproved periodic topology", err)
	}
}

func TestReadPeriodicTrimmedFaceRefusesUnsupportedSource(t *testing.T) {
	ctx := context.Background()
	raw := periodicTrimmedFixture(t, false)
	refuse := func(input []byte, limits Limits, detail string) {
		t.Helper()
		f, err := ReadPeriodicTrimmedFace(ctx, bytes.NewReader(input), limits)
		if err == nil || !strings.Contains(err.Error(), detail) || f.FaceID != 0 || f.Patch != nil || f.OriginalIGES() != "" {
			t.Fatal("invalid source produced a partial face or wrong refusal", err)
		}
	}
	doc := periodicTrimmedRecords(t, raw)
	for _, wrong := range []struct {
		owner, index int
		value        string
		detail       string
	}{
		{1, 4, "3", "wrong periodic trim owner"},
		{3, 8, "0", "requires one U-periodic support"},
		{3, 80, "1.25000000000000001", "does not follow the directed rectangle"},
		{3, 80, "1.", "meets or crosses an original knot station"},
		{5, 2, "7", "contour support"},
		{5, 4, "7", "duplicated"},
		{7, 3, "9", "duplicated"},
		{17, 3, "19", "duplicated"},
		{11, 4, "1.75000000000000001", "does not follow the directed rectangle"},
		{11, 6, "0.0000000000001", "periodic UV line #11"},
		{21, 19, "0.90625000000000001", "model edge #21 disagrees with UV line #11"},
		{25, 25, "3.25000000000000001", "full clamped knot range"},
	} {
		refuse(changePeriodicTrimmedRecord(t, doc, wrong.owner, wrong.index, wrong.value), Limits{}, wrong.detail)
	}
	refuse([]byte(replaceField(string(raw), 'D', 3, 8, "0")), Limits{}, "unsupported topology entity 128")
	refuse(raw, Limits{MaxBytes: int64(len(raw) - 1)}, "byte limit")
	refuse(raw, Limits{MaxEntities: 12}, "entity limit")
	refuse(raw, Limits{MaxControlPoints: 23}, "periodic model edge #25")
	refuse(periodic128Fixture(t, "u"), Limits{}, "unsupported topology entity 128")
	refuse(periodicTrimmedFixture(t, true), Limits{}, "crosses an original knot station")
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if got, err := ReadPeriodicTrimmedFace(cancelled, bytes.NewReader(raw), Limits{}); !errors.Is(err, context.Canceled) || got.Support != nil {
		t.Fatal("cancelled import exposed a face", err)
	}
}

func TestPeriodicTrimmedFaceSeamSourceHasIncompleteModelEdge(t *testing.T) {
	ctx := context.Background()
	source := periodicTrimmedRecords(t, periodicTrimmedFixture(t, true))
	var domain [4]*big.Rat
	e, err := decodeSurfaceMode(source.entities[1].values, 100, true, &domain)
	if err != nil {
		t.Fatal(err)
	}
	g, err := e.Surface.ExactPeriodicGeometry()
	if err != nil {
		t.Fatal(err)
	}
	u, _, _ := g.Data()
	if u.Knots[0].Cmp(big.NewRat(4, 1)) != 0 || u.Knots[u.Count].Cmp(big.NewRat(9, 1)) != 0 ||
		domain[0].Cmp(big.NewRat(19, 4)) != 0 || domain[1].Cmp(big.NewRat(21, 4)) != 0 {
		t.Fatal("independent seam fixture must retain its rephased support and raw face interval")
	}
	// The raw upper UV side starts at 21/4, but the source type-126
	// starts at the old seam 5 and only carries the opposite segment.
	uv, _, err := periodicTrimUVLine(source.entities[7])
	if err != nil || uv[0].Cmp(domain[1]) != 0 {
		t.Fatal("missing original seam-crossing UV owner", err)
	}
	xyz, err := g.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	model, bounds, _, err := periodicTrimModelEdge(ctx, source.entities[12], 8)
	if err != nil {
		t.Fatal(err)
	}
	first, _, err := model.BezierControlNet(ctx, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	h := first.Control[0]
	x := new(big.Rat).Quo(h[0], h[3])
	if bounds.Lower.Sign() != 0 || bounds.Upper.Cmp(big.NewRat(1, 4)) != 0 ||
		x.Cmp(big.NewRat(11, 2)) != 0 || xyz.Point[0].Cmp(big.NewRat(165, 32)) != 0 || x.Cmp(xyz.Point[0]) == 0 {
		t.Fatal("missing exact raw UV/model disagreement", x, xyz.Point)
	}
}

func TestReadPeriodicTrimmedFacePinnedOCCT(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_PERIODIC_128_TRIM_IGES")
	if oracle == "" {
		t.Skip("optional pinned OCCT author and reader for periodic type-144")
	}
	ctx := context.Background()
	for _, seam := range []bool{false, true} {
		mode := "write"
		if seam {
			mode = "write-seam"
		}
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), mode+".iges")
			made := periodicTrimmedOracle(t, oracle, mode, path)
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if one, two := periodicTrimmedRecords(t, raw), periodicTrimmedRecords(t, periodicTrimmedFixture(t, seam)); !reflect.DeepEqual(one.entities, two.entities) {
				t.Fatal("pinned writer changed exact original source owners or values")
			}
			reopened := periodicTrimmedOracle(t, oracle, "read", path)
			for _, hit := range []periodicTrimReport{made, reopened} {
				if !hit.Valid || !hit.PeriodicU || hit.Topology != [4]int{1, 1, 4, 4} || len(hit.Samples) != 5 {
					t.Fatal("OCCT did not transfer one valid periodic rectangular face", hit)
				}
			}
			if seam {
				if m, err := ReadPeriodicTrimmedFace(ctx, bytes.NewReader(raw), Limits{}); err == nil || m.Patch != nil {
					t.Fatal("strict proof accepted the original seam control repaired by OCCT", err)
				}
				return
			}
			f, err := ReadPeriodicTrimmedFace(ctx, bytes.NewReader(raw), Limits{})
			if err != nil {
				t.Fatal(err)
			}
			if math.Abs(reopened.Area-4.63387717461943) > 1e-10 || math.Abs(reopened.Perimeter-8.63387718171492) > 1e-10 ||
				reopened.Bounds != [4]float64{1.25, 1.75, 1, 3} {
				t.Fatal("pinned reopen changed physical bounds, area or perimeter", reopened)
			}
			for _, report := range []periodicTrimReport{made, reopened} {
				for _, sample := range report.Samples {
					jet, err := f.Support.EvaluateRational(ctx, rat(sample.UV[0]), rat(sample.UV[1]), geom.SurfacePatchOptions{})
					if err != nil {
						t.Fatal(err)
					}
					checkPeriodic128Vector(t, jet.Point, sample.Point)
					checkPeriodic128Vector(t, jet.Du, sample.Du)
					checkPeriodic128Vector(t, jet.Dv, sample.Dv)
				}
			}
		})
	}
}

func periodicTrimmedFixture(t *testing.T, seam bool) []byte {
	t.Helper()
	name := "testdata/occt-periodic-trimmed-128"
	if seam {
		name += "-seam"
	}
	raw, err := os.ReadFile(name + ".iges")
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func periodicTrimmedRecords(t *testing.T, raw []byte) topologySource {
	t.Helper()
	doc, err := readTopologyRecordPolicyWithDirectory(bytes.NewReader(raw), Limits{},
		false, false, false, false, false, false, false, false, false, periodicTrimmedFaceDirectory)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func changePeriodicTrimmedRecord(t *testing.T, source topologySource, owner, index int, value string) []byte {
	t.Helper()
	var a topologyAuthor
	root, contour := 0, 10500
	changed := false
	for _, record := range source.entities {
		p := make([]string, len(record.values))
		for i, value := range record.values {
			p[i] = value.text
		}
		if record.id == owner && index < len(p) && value != p[index] {
			p[index] = value
			changed = true
		}
		a.add(record.kind, 0, strings.Join(p, ",")+";")
		emitted := &a.records[len(a.records)-1]
		switch record.kind {
		case 144:
			emitted.status = &root
		case 142:
			emitted.status = &contour
		}
	}
	if !changed {
		t.Fatal("ineffective record mutation", owner, index, value)
	}
	mutant, err := a.file(source.unit)
	if err != nil {
		t.Fatal(err)
	}
	return mutant
}

type periodicTrimReport struct {
	Valid, PeriodicU bool
	Topology         [4]int
	Bounds           [4]float64
	Area, Perimeter  float64
	Edges            []periodicTrimOracleEdge
	Wires            []periodicTrimOracleWire
	Samples          []struct {
		UV            [2]float64
		Point, Du, Dv [3]float64
	}
}

func periodicTrimmedOracle(t *testing.T, oracle, mode, path string) periodicTrimReport {
	t.Helper()
	out, err := exec.Command(oracle, mode, path).CombinedOutput()
	if err != nil {
		t.Fatalf("pinned OCCT %s: %v\n%s", mode, err, out)
	}
	return parsePeriodicTrimOracleReport(t, out)
}

func parsePeriodicTrimOracleReport(t *testing.T, out []byte) periodicTrimReport {
	t.Helper()
	for _, line := range bytes.Split(out, []byte("\n")) {
		var report periodicTrimReport
		if bytes.HasPrefix(line, []byte(`{"valid":`)) && json.Unmarshal(line, &report) == nil {
			return report
		}
	}
	t.Fatalf("missing pinned OCCT trimmed face report: %s", out)
	return periodicTrimReport{}
}
