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

func TestReadPeriodicSeamTrimmedFaceWithMixedHoles(t *testing.T) {
	ctx := context.Background()
	for _, example := range []struct {
		name                 string
		order                [2]int
		seamGraph, cellGraph [5]int
	}{
		{"exact-seam-mixed", [2]int{35, 65}, [5]int{35, 37, 51, 39, 53}, [5]int{65, 67, 77, 69, 79}},
		{"exact-seam-mixed-reversed", [2]int{35, 57}, [5]int{57, 59, 73, 61, 75}, [5]int{35, 37, 47, 39, 49}},
	} {
		t.Run(example.name, func(t *testing.T) {
			raw := periodicHoleFixture(t, example.name)
			f, err := ReadPeriodicSeamTrimmedFaceWithMixedHoles(ctx, bytes.NewReader(raw), Limits{MaxEntities: 43, MaxControlPoints: 56})
			if err != nil {
				t.Fatal(err)
			}
			if f.OriginalIGES() != string(raw) || f.HoleContourIDs != example.order || f.CellHoleSpan != 0 ||
				f.ProjectionAxes != [2]int{1, 2} || f.ProjectionOrientation != 1 ||
				periodicTrimIntervalStrings(f.Bounds) != ([2][2]string{{"19/4", "21/4"}, {"1", "3"}}) ||
				periodicTrimIntervalStrings(f.SeamHole.Bounds) != ([2][2]string{{"39/8", "41/8"}, {"3/2", "5/2"}}) ||
				periodicTrimIntervalStrings(f.CellHole.Bounds) != ([2][2]string{{"77/16", "79/16"}, {"9/8", "11/8"}}) {
				t.Fatal("changed the original inner loop order, shared lift or projected orientation")
			}
			if owner := f.Cells[f.CellHoleSpan].KnotOwners[0]; owner.Span != 4 || owner.Cycle.Sign() != 0 ||
				owner.Domain.Lower.Cmp(big.NewRat(4, 1)) != 0 || owner.Domain.Upper.Cmp(big.NewRat(5, 1)) != 0 {
				t.Fatal("lost the original single-span owner", owner)
			}
			s, c := f.SeamHole, f.CellHole
			if [3]int{s.ContourID, s.UVCompositeID, s.ModelCompositeID} != ([3]int(example.seamGraph[:3])) ||
				[3]int{c.ContourID, c.UVCompositeID, c.ModelCompositeID} != ([3]int(example.cellGraph[:3])) {
				t.Fatal("changed original seam or one-span hole graph", s, c)
			}
			checkPeriodicTrimSourceWire(t, f.Edges[:], 9, 23, 0, "1")
			checkPeriodicTrimSourceWire(t, s.Edges[:], example.seamGraph[3], example.seamGraph[4], 20, "-1/4")
			checkPeriodicTrimSourceWire(t, c.Edges[:], example.cellGraph[3], example.cellGraph[4], 40, "-1/32")
			gap := new(big.Rat).Sub(s.Bounds[1].Lower, c.Bounds[1].Upper)
			if gap.Cmp(big.NewRat(1, 8)) != 0 {
				t.Fatal("changed the original strict separation between inner boundaries", gap)
			}
			if old, err := ReadPeriodicSeamTrimmedFaceWithHole(ctx, bytes.NewReader(raw), Limits{}); err == nil || old.Support != nil {
				t.Fatal("single seam-hole reader accepted two holes", err)
			}
		})
	}
}

func TestReadPeriodicSeamTrimmedFaceWithMixedHolesReject(t *testing.T) {
	ctx := context.Background()
	refuse := func(raw []byte, limits Limits, detail string) {
		t.Helper()
		f, err := ReadPeriodicSeamTrimmedFaceWithMixedHoles(ctx, bytes.NewReader(raw), limits)
		if err == nil || !strings.Contains(err.Error(), detail) || f.OriginalIGES() != "" ||
			f.Support != nil || f.SeamHole.ContourID != 0 || f.CellHole.ContourID != 0 {
			t.Fatal("unproved mixed seam source returned partial proof or wrong refusal", err)
		}
	}
	refuse(periodicHoleFixture(t, "exact-seam-mixed-overlap"), Limits{}, "holes #35 and #65 overlap or meet")

	rounded := periodicHoleFixture(t, "exact-seam-mixed-rounded")
	refuse(rounded, Limits{}, "model edge #79 disagrees with UV line #69")
	doc := periodicTrimmedRecords(t, rounded)
	support, err := decodeSurfaceMode(doc.entities[1].values, 100, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	native, err := support.Surface.ExactPeriodicGeometry()
	if err != nil {
		t.Fatal(err)
	}
	point, err := native.EvaluateRational(ctx, big.NewRat(153, 32), big.NewRat(7, 4), geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	rawX, err := igesPlanarExact(doc.entities[(79-1)/2].values[13])
	if err != nil || new(big.Rat).Sub(rawX, point.Point[0]).Cmp(big.NewRat(1, 4000000000)) != 0 {
		t.Fatal("independent rounded source lost its exact off-support gap", rawX, point.Point, err)
	}

	raw := periodicHoleFixture(t, "exact-seam-mixed")
	doc = periodicTrimmedRecords(t, raw)
	for _, edit := range []struct {
		id, index     int
		value, detail string
	}{
		{1, 6, "35", "duplicated or wrong periodic trim owner #35"},
		{65, 2, "7", "mismatched periodic trim contour support"},
		{67, 1, "6", "not representable"},
		{65, 4, "67", "duplicated or wrong periodic trim owner #67"},
		{77, 3, "79", "duplicated or wrong periodic trim owner #79"},
	} {
		refuse(changePeriodicTrimmedRecord(t, doc, edit.id, edit.index, edit.value), Limits{}, edit.detail)
	}
	refuse(raw, Limits{MaxEntities: 42}, "entity limit")
	refuse(raw, Limits{MaxControlPoints: 55}, "model edge #85")
	refuse(periodicHoleFixture(t, "exact-seam-hole"), Limits{}, "invalid explicit periodic trim hole count")
}

func TestReadPeriodicSeamTrimmedFaceWithMixedHolesPinnedOCCT(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_PERIODIC_128_TRIM_IGES")
	if oracle == "" {
		t.Skip("optional pinned OCCT author and reader for two periodic seam-chart holes")
	}
	ctx := context.Background()
	for _, example := range []struct {
		name            string
		nativeEdges     [3]int
		strict          bool
		topology        [4]int
		area, perimeter float64
	}{
		{"exact-seam-mixed", [3]int{6, 6, 4}, true, [4]int{1, 3, 16, 16}, 2.941538669583753, 13.627549535111653},
		{"exact-seam-mixed-reversed", [3]int{6, 4, 6}, true, [4]int{1, 3, 16, 16}, 2.9415386695837524, 13.627549535111653},
		{"exact-seam-mixed-overlap", [3]int{6, 8, 6}, false, [4]int{1, 3, 20, 18}, 2.816067469529198, 14.127549535111763},
		{"exact-seam-mixed-rounded", [3]int{6, 6, 4}, false, [4]int{1, 3, 16, 16}, 2.940078458862369, 13.631505577734265},
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
				t.Fatal("pinned author changed exact original owners and stored coefficients")
			}
			m := periodicTrimmedOracle(t, oracle, "read-wires", path)
			if !m.Valid || !m.PeriodicU || m.Topology != example.topology || m.Bounds != [4]float64{4.75, 5.25, 1, 3} || len(m.Wires) != 3 ||
				math.Abs(m.Area-example.area) > 1e-10 || math.Abs(m.Perimeter-example.perimeter) > 1e-10 {
				t.Fatal("pinned reader changed the physical topology, bounds or measures", m)
			}
			for i, w := range m.Wires {
				if w.Outer != (i == 0) || len(w.Edges) != example.nativeEdges[i] {
					t.Fatal("pinned reader changed original contour order or split loci", m.Wires)
				}
			}
			f, err := ReadPeriodicSeamTrimmedFaceWithMixedHoles(ctx, bytes.NewReader(raw), Limits{})
			if !example.strict {
				if err == nil || f.Support != nil {
					t.Fatal("strict reader admitted a rounded or intersecting original", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			checkPeriodicTrimOracleWire(t, ctx, f.Support, m.Wires[0], f.Edges[:], true, f.ProjectionAxes, f.ProjectionOrientation)
			for i, owner := range f.HoleContourIDs {
				edges := f.CellHole.Edges[:]
				if owner == f.SeamHole.ContourID {
					edges = f.SeamHole.Edges[:]
				}
				checkPeriodicTrimOracleWire(t, ctx, f.Support, m.Wires[i+1], edges, false, f.ProjectionAxes, f.ProjectionOrientation)
			}
		})
	}
}
