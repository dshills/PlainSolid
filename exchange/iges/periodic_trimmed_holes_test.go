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

type periodicTrimOracleWire struct {
	Outer bool
	Edges []periodicTrimOracleEdge
}

func periodicHoleFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/occt-periodic-trimmed-128-" + name + ".iges")
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestReadPeriodicTrimmedFaceWithHoles(t *testing.T) {
	ctx := context.Background()
	for i, name := range []string{"one-hole", "two-holes"} {
		t.Run(name, func(t *testing.T) {
			raw := periodicHoleFixture(t, name)
			count := i + 1
			m, err := ReadPeriodicTrimmedFaceWithHoles(ctx, bytes.NewReader(raw), Limits{
				MaxEntities: 13 + 11*count, MaxControlPoints: 24 + 10*count,
			})
			if err != nil {
				t.Fatal(err)
			}
			if m.OriginalIGES() != string(raw) || m.LengthUnit != .001 || m.Patch == nil || len(m.Holes) != count ||
				[5]int{m.FaceID, m.SupportID, m.ContourID, m.UVCompositeID, m.ModelCompositeID} != [5]int{1, 3, 5, 7, 17} ||
				m.ProjectionAxes != [2]int{0, 2} || m.ProjectionOrientation != -1 {
				t.Fatal("lost original face, inner wires, or exact projection")
			}
			u, v := m.KnotOwners[0], m.KnotOwners[1]
			if periodicTrimIntervalStrings(m.Bounds) != ([2][2]string{{"9/8", "15/8"}, {"1/2", "7/2"}}) ||
				periodicTrimIntervalStrings([2]geom.ExactParameterInterval{u.Domain, v.Domain}) != ([2][2]string{{"1", "2"}, {"0", "4"}}) ||
				u.Span != 1 || v.Span != 0 || u.Cycle.Sign() != 0 || v.Cycle != nil {
				t.Fatal("lost the common chart in the original knot cell", m.Bounds, m.KnotOwners)
			}
			uBasis, vBasis, _ := m.Support.Data()
			if !uBasis.Periodic || vBasis.Periodic || uBasis.Knots[0].Sign() != 0 || uBasis.Knots[uBasis.Count].Cmp(big.NewRat(5, 1)) != 0 {
				t.Fatal("trim narrowed the original full cyclic support")
			}
			checkPeriodicTrimSourceWire(t, m.Edges[:], 9, 19, 0, "9/4")
			for k, hole := range m.Holes {
				bounds := [2][2][2]string{{{"5/4", "23/16"}, {"1", "2"}}, {{"25/16", "7/4"}, {"2", "3"}}}
				if [3]int{hole.ContourID, hole.UVCompositeID, hole.ModelCompositeID} != ([3]int{27 + 22*k, 29 + 22*k, 39 + 22*k}) ||
					periodicTrimIntervalStrings(hole.Bounds) != bounds[k] {
					t.Fatal("lost an original inner owner or its bounds", hole)
				}
				checkPeriodicTrimSourceWire(t, hole.Edges[:], 31+22*k, 41+22*k, 20+20*k, "-3/16")
			}
			if old, err := ReadPeriodicTrimmedFace(ctx, bytes.NewReader(raw), Limits{}); err == nil || old.Support != nil {
				t.Fatal("outer-only reader admitted inner topology", err)
			}
			if old, err := ReadPeriodicSeamTrimmedFace(ctx, bytes.NewReader(raw), Limits{}); err == nil || old.Support != nil {
				t.Fatal("seam reader admitted inner topology", err)
			}
			if ordinary, err := Read(bytes.NewReader(raw), Limits{}); err == nil || len(ordinary.Entities) != 0 {
				t.Fatal("ordinary entity reader admitted inner topology", err)
			}
		})
	}
}

func checkPeriodicTrimSourceWire(t *testing.T, pairs []PeriodicTrimEdge, firstUV, firstModel, firstStation int, area string) {
	t.Helper()
	twiceArea := new(big.Rat)
	for i, edge := range pairs {
		next := pairs[(i+1)%len(pairs)]
		if edge.UVSourceID != firstUV+2*i || edge.ModelSourceID != firstModel+2*i || edge.ModelCurve == nil ||
			edge.UVEnd[0].Cmp(next.UVStart[0]) != 0 || edge.UVEnd[1].Cmp(next.UVStart[1]) != 0 ||
			edge.ModelParameter.Lower.Cmp(big.NewRat(int64(firstStation+2*i), 1)) != 0 ||
			edge.ModelParameter.Upper.Cmp(big.NewRat(int64(firstStation+2*i+1), 1)) != 0 {
			t.Fatal("lost an original directed UV/model owner", edge)
		}
		forward := new(big.Rat).Mul(edge.UVStart[0], edge.UVEnd[1])
		backward := new(big.Rat).Mul(edge.UVStart[1], edge.UVEnd[0])
		twiceArea.Add(twiceArea, forward.Sub(forward, backward))
	}
	if twiceArea.Quo(twiceArea, big.NewRat(2, 1)).RatString() != area {
		t.Fatal("wrong exact original winding", twiceArea)
	}
}

func TestReadPeriodicTrimmedFaceWithHolesReject(t *testing.T) {
	ctx := context.Background()
	refuse := func(raw []byte, limits Limits, detail string) {
		t.Helper()
		m, err := ReadPeriodicTrimmedFaceWithHoles(ctx, bytes.NewReader(raw), limits)
		if err == nil || !strings.Contains(err.Error(), detail) || m.OriginalIGES() != "" ||
			m.Support != nil || m.Patch != nil || m.Holes != nil || m.FaceID != 0 {
			t.Fatal("invalid input returned a partial face or wrong refusal", err)
		}
	}
	refuse(periodicHoleFixture(t, "overlapping-holes"), Limits{}, "holes #27 and #49 overlap or meet")
	refuse(periodicHoleFixture(t, "touching-hole"), Limits{}, "hole #27 is not strictly contained")
	refuse(periodicHoleFixture(t, "wrong-hole-winding"), Limits{}, "does not follow the directed rectangle")
	refuse(periodicTrimmedFixture(t, false), Limits{}, "invalid explicit periodic trim hole count")

	raw := periodicHoleFixture(t, "two-holes")
	doc := periodicTrimmedRecords(t, raw)
	for _, edit := range []struct {
		id, index     int
		value, detail string
	}{
		{1, 6, "27", "duplicated or wrong periodic trim owner #27"},
		{49, 2, "7", "mismatched periodic trim contour support"},
		{61, 3, "63", "duplicated or wrong periodic trim owner #63"},
		{43, 19, "1.57031250000000001", "model edge #43 disagrees with UV line #33"},
	} {
		refuse(changePeriodicTrimmedRecord(t, doc, edit.id, edit.index, edit.value), Limits{}, edit.detail)
	}
	refuse(raw, Limits{MaxBytes: int64(len(raw) - 1)}, "byte limit")
	refuse(raw, Limits{MaxEntities: 34}, "entity limit")
	refuse(raw, Limits{MaxControlPoints: 43}, "periodic model edge #69")
}

func TestReadPeriodicTrimmedFaceWithHolesPinnedOCCT(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_PERIODIC_128_TRIM_IGES")
	if oracle == "" {
		t.Skip("optional pinned OCCT author and reader for periodic inner trims")
	}
	ctx := context.Background()
	for _, example := range []struct {
		name            string
		valid, reversed bool
		topology        [4]int
		area, perimeter float64
	}{
		{"one-hole", true, false, [4]int{1, 2, 8, 8}, 9.889664035260374, 16.793505205492284},
		{"two-holes", true, false, [4]int{1, 3, 12, 12}, 8.964677078070961, 20.64347911987115},
		{"overlapping-holes", false, false, [4]int{1, 3, 16, 14}, 0, 0},
		{"touching-hole", false, false, [4]int{1, 2, 10, 8}, 0, 0},
		{"wrong-hole-winding", true, true, [4]int{1, 2, 8, 8}, 9.889664035260374, 16.793505205492284},
	} {
		t.Run(example.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), example.name+".iges")
			if out, err := exec.Command(oracle, "write-"+example.name, path).CombinedOutput(); err != nil {
				t.Fatalf("independent OCCT author: %v\n%s", err, out)
			}
			fresh, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := periodicTrimmedRecords(t, fresh), periodicTrimmedRecords(t, periodicHoleFixture(t, example.name)); !reflect.DeepEqual(got.entities, want.entities) {
				t.Fatal("independent author changed original inner and outer curve owners")
			}
			out, err := exec.Command(oracle, "read-wires", path).CombinedOutput()
			if example.valid {
				if err != nil {
					t.Fatalf("pinned reader: %v\n%s", err, out)
				}
			} else if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 6 {
				t.Fatalf("pinned reader did not detect invalid shape: %v\n%s", err, out)
			}
			report := parsePeriodicTrimOracleReport(t, out)
			if report.Valid != example.valid || !report.PeriodicU || report.Topology != example.topology ||
				report.Bounds != [4]float64{1.125, 1.875, .5, 3.5} || len(report.Wires) != example.topology[1] {
				t.Fatal("pinned reader returned different original-chart topology", report)
			}
			m, err := ReadPeriodicTrimmedFaceWithHoles(ctx, bytes.NewReader(fresh), Limits{})
			if !example.valid || example.reversed {
				if err == nil || m.Support != nil {
					t.Fatal("strict import accepted invalid original topology or rewound a source wire", err)
				}
				if example.reversed && (report.Wires[1].Outer || report.Wires[1].Edges[0].ModelRange != [2]float64{26, 27} ||
					report.Wires[1].Edges[0].UVStart != [2]float64{1.4375, 1} || report.Wires[1].Edges[0].UVEnd != [2]float64{1.25, 1}) {
					t.Fatal("pinned reader did not reverse the original clockwise-required wire", report.Wires[1])
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if math.Abs(report.Area-example.area) > 1e-10 || math.Abs(report.Perimeter-example.perimeter) > 1e-10 {
				t.Fatal("pinned reader changed the physical area or perimeter", report)
			}
			checkPeriodicTrimOracleWire(t, ctx, m.Support, report.Wires[0], m.Edges[:], true, m.ProjectionAxes, m.ProjectionOrientation)
			for i, hole := range m.Holes {
				checkPeriodicTrimOracleWire(t, ctx, m.Support, report.Wires[i+1], hole.Edges[:], false, m.ProjectionAxes, m.ProjectionOrientation)
			}
		})
	}
}

func checkPeriodicTrimOracleWire(t *testing.T, ctx context.Context, support *geom.ExactPeriodicSplineSurface, wire periodicTrimOracleWire, pairs []PeriodicTrimEdge, outer bool, projection [2]int, orientation int) {
	t.Helper()
	if wire.Outer != outer || len(wire.Edges) != len(pairs) {
		t.Fatal("pinned transfer changed the directed wire role or edge count", wire)
	}
	projectedTwiceArea := 0.
	x, y := projection[0], projection[1]
	for i, e := range wire.Edges {
		raw := pairs[i]
		lo, _ := raw.ModelParameter.Lower.Float64()
		hi, _ := raw.ModelParameter.Upper.Float64()
		if e.ModelRange != [2]float64{lo, hi} || e.UVRange != e.ModelRange {
			t.Fatal("pinned reader changed an original model station range", e)
		}
		var mid [2]*big.Rat
		for j := range mid {
			mid[j] = new(big.Rat).Quo(new(big.Rat).Add(raw.UVStart[j], raw.UVEnd[j]), big.NewRat(2, 1))
		}
		for j, uv := range [3][2]*big.Rat{raw.UVStart, mid, raw.UVEnd} {
			actual := [3][2]float64{e.UVStart, e.UVMid, e.UVEnd}[j]
			for k, q := range uv {
				if q.Cmp(rat(actual[k])) != 0 {
					t.Fatal("pinned reader changed an original directed UV sample", i, j, actual)
				}
			}
			jet, err := support.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchOptions{})
			if err != nil {
				t.Fatal(err)
			}
			checkPeriodic128Vector(t, jet.Point, [3][3]float64{e.ModelStart, e.ModelMid, e.ModelEnd}[j])
		}
		projectedTwiceArea += e.ModelStart[x]*e.ModelEnd[y] - e.ModelStart[y]*e.ModelEnd[x]
	}
	oriented := float64(orientation) * projectedTwiceArea
	if (outer && oriented <= 0) || (!outer && oriented >= 0) {
		t.Fatal("pinned reader changed the projected model winding", projectedTwiceArea)
	}
}
