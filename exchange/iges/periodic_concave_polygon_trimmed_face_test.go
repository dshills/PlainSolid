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

const concavePeriodicFixture = "exact-concave-polygon"

func TestReadPeriodicConcavePolygonTrimmedFace(t *testing.T) {
	ctx := context.Background()
	raw := periodicHoleFixture(t, concavePeriodicFixture)
	f, err := ReadPeriodicConcavePolygonTrimmedFace(ctx, bytes.NewReader(raw), Limits{MaxEntities: 15, MaxControlPoints: 34})
	if err != nil {
		t.Fatal(err)
	}
	if [5]int{f.FaceID, f.SupportID, f.ContourID, f.UVCompositeID, f.ModelCompositeID} != ([5]int{1, 3, 5, 7, 19}) ||
		f.OriginalIGES() != string(raw) || f.LengthUnit != .001 || f.Patch == nil || len(f.Edges) != 5 ||
		f.ProjectionAxes != [2]int{0, 1} || f.ProjectionOrientation != -1 ||
		periodicTrimIntervalStrings(f.Bounds) != ([2][2]string{{"5/4", "7/4"}, {"1", "3"}}) {
		t.Fatal("lost original concave trim owner, geometry or chart")
	}
	u, v := f.KnotOwners[0], f.KnotOwners[1]
	if u.Span != 1 || v.Span != 0 || u.Cycle.Sign() != 0 || v.Cycle != nil ||
		periodicTrimIntervalStrings([2]geom.ExactParameterInterval{u.Domain, v.Domain}) != ([2][2]string{{"1", "2"}, {"0", "4"}}) {
		t.Fatal("lost original cell owners")
	}
	checkPeriodicTrimSourceWire(t, f.Edges, 9, 21, 144, "75/128")
	points := periodicAffineWireVertices(f.Edges)
	wantTurns := [5]int{1, -1, 1, 1, 1}
	for i, a := range points {
		if periodicUVTurn(a, points[(i+1)%5], points[(i+2)%5]) != wantTurns[i] {
			t.Fatal("changed original reflex vertex", i)
		}
	}
	old, err := ReadPeriodicConvexPolygonTrimmedFace(ctx, bytes.NewReader(raw), Limits{})
	if err == nil || old.Support != nil {
		t.Fatal("convex reader admitted a reflex vertex", err)
	}
}

func TestReadPeriodicConcavePolygonTrimmedFaceReject(t *testing.T) {
	ctx := context.Background()
	refuse := func(raw []byte, limits Limits, detail string) {
		t.Helper()
		f, err := ReadPeriodicConcavePolygonTrimmedFace(ctx, bytes.NewReader(raw), limits)
		if err == nil || !strings.Contains(err.Error(), detail) || !reflect.DeepEqual(f, PeriodicConcavePolygonTrimmedFace{}) {
			t.Fatal("unsafe concave trim returned a partial proof or wrong refusal", err)
		}
	}
	crossing := periodicHoleFixture(t, concavePeriodicFixture+"-crossing")
	refuse(crossing, Limits{}, "nonadjacent concave trim edges 3 and 5 meet")
	doc := periodicTrimmedRecords(t, crossing)
	a, b := periodicConcaveFixtureUV(t, doc, 13)
	c, d := periodicConcaveFixtureUV(t, doc, 17)
	if periodicUVTurn(a, b, c)*periodicUVTurn(a, b, d) != -1 ||
		periodicUVTurn(c, d, a)*periodicUVTurn(c, d, b) != -1 {
		t.Fatal("independent crossing control lacks a proper exact intersection")
	}
	touch := periodicHoleFixture(t, concavePeriodicFixture+"-touching")
	refuse(touch, Limits{}, "nonadjacent concave trim edges 1 and 3 meet")
	doc = periodicTrimmedRecords(t, touch)
	a, b = periodicConcaveFixtureUV(t, doc, 9)
	_, d = periodicConcaveFixtureUV(t, doc, 13)
	if periodicUVTurn(a, b, d) != 0 || d[0].Cmp(a[0]) <= 0 || d[0].Cmp(b[0]) >= 0 {
		t.Fatal("independent touching control lacks an exact contact in the first edge's interior")
	}
	refuse(periodicHoleFixture(t, concavePeriodicFixture+"-clockwise"), Limits{}, "requires counterclockwise UV winding")
	refuse(periodicHoleFixture(t, "exact-convex-polygon-triangle"), Limits{}, "requires a reflex UV vertex")
	refuse(periodicHoleFixture(t, "exact-convex-polygon-pentagon-triangle"), Limits{}, "requires one outer loop")

	raw := periodicHoleFixture(t, concavePeriodicFixture)
	doc = periodicTrimmedRecords(t, raw)
	refuse(changePeriodicTrimmedRecord(t, doc, 19, 3, "21"), Limits{}, "duplicated or wrong periodic trim owner #21")
	refuse(changePeriodicTrimmedRecord(t, doc, 21, 22, "317824.00000000000000001"), Limits{}, "model edge #21 disagrees with UV line #9")
	refuse(raw, Limits{MaxEntities: 14}, "entity limit")
	refuse(raw, Limits{MaxControlPoints: 33}, "model edge #29")
}

func periodicConcaveFixtureUV(t *testing.T, doc topologySource, id int) ([2]*big.Rat, [2]*big.Rat) {
	t.Helper()
	a, b, err := periodicTrimUVLine(doc.entities[(id-1)/2])
	if err != nil {
		t.Fatal(err)
	}
	return a, b
}

func TestWritePeriodicConcavePolygonTrimmedFace(t *testing.T) {
	ctx := context.Background()
	raw := periodicHoleFixture(t, concavePeriodicFixture)
	f, err := ReadPeriodicConcavePolygonTrimmedFace(ctx, bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	var first, second bytes.Buffer
	if err := WritePeriodicConcavePolygonTrimmedFace(ctx, &first, f); err != nil {
		t.Fatal(err)
	}
	comparePeriodicTrimRecords(t, raw, first.Bytes())
	reopened, err := ReadPeriodicConcavePolygonTrimmedFace(ctx, bytes.NewReader(first.Bytes()), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if err := WritePeriodicConcavePolygonTrimmedFace(ctx, &second, reopened); err != nil || !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("canonical concave trim changed on repeat", err)
	}
	reopened.original = f.original
	if !reflect.DeepEqual(reopened, f) {
		t.Fatal("canonical reopen changed original owner proof")
	}
	out := bytes.NewBufferString("untouched")
	f.Edges[2], f.Edges[3] = f.Edges[3], f.Edges[2]
	if err := WritePeriodicConcavePolygonTrimmedFace(ctx, out, f); err == nil || out.String() != "untouched" {
		t.Fatal("changed owner order published a partial carrier", err)
	}
	f.original = string(periodicHoleFixture(t, concavePeriodicFixture+"-crossing"))
	if err := WritePeriodicConcavePolygonTrimmedFace(ctx, out, f); err == nil || out.String() != "untouched" {
		t.Fatal("forged crossing original published a partial carrier", err)
	}
}

func TestWritePeriodicConcavePolygonTrimmedFaceRejectsRoundedContact(t *testing.T) {
	ctx := context.Background()
	raw := periodicHoleFixture(t, concavePeriodicFixture)
	f, err := ReadPeriodicConcavePolygonTrimmedFace(ctx, bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	// V=17/16 lies on the nonadjacent first edge at U=11/8.
	// Keep a positive exact gap small enough to round to contact.
	f.Edges[3].UVStart = [2]*big.Rat{
		big.NewRat(11, 8), new(big.Rat).Add(big.NewRat(17, 16), big.NewRat(1, 100000000000000000)),
	}
	points := periodicAffineWireVertices(f.Edges)
	if err := provePeriodicUVConcavity(points); err != nil {
		t.Fatal("precision control must have an exact simple concave loop", err)
	}
	rounded, err := periodicRoundedAffineWires(f.KnotOwners, [][]PeriodicTrimEdge{f.Edges})
	if err != nil || !periodicUVSegmentsMeet(rounded[0][0], rounded[0][1], rounded[0][2], rounded[0][3]) {
		t.Fatal("precision control must retain distinct stations but round to edge contact", err)
	}
	if err := checkPeriodicConcaveTransfer(f); err == nil || !strings.Contains(err.Error(), "nonadjacent concave trim edges 1 and 3 meet") {
		t.Fatal("binary64 transfer admitted edge contact", err)
	}
}

func TestWritePeriodicConcavePolygonTrimmedFacePinnedOCCT(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_PERIODIC_128_TRIM_IGES")
	if oracle == "" {
		t.Skip("optional pinned OCCT author and reader for the concave polygon")
	}
	ctx := context.Background()
	for _, example := range []struct {
		suffix   string
		exitCode int
	}{
		{"", 0},
		{"-crossing", 6},
		{"-touching", 6},
		{"-clockwise", 0},
	} {
		name := concavePeriodicFixture + example.suffix
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			before, after := filepath.Join(dir, "original.iges"), filepath.Join(dir, "canonical.iges")
			native := periodicTrimmedOracle(t, oracle, "write-"+name, before)
			raw, err := os.ReadFile(before)
			if err != nil {
				t.Fatal(err)
			}
			checkPeriodicPolygonAuthoredRecords(t, name, raw)
			output, err := exec.Command(oracle, "read-wires", before).CombinedOutput()
			if example.exitCode == 0 {
				if err != nil {
					t.Fatalf("pinned read: %v\n%s", err, output)
				}
			} else if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != example.exitCode {
				t.Fatalf("pinned invalid transfer control: %v\n%s", err, output)
			}
			f, err := ReadPeriodicConcavePolygonTrimmedFace(ctx, bytes.NewReader(raw), Limits{})
			if example.suffix != "" {
				if err == nil || f.Support != nil {
					t.Fatal("strict reader admitted independently authored negative", err)
				}
				if example.exitCode == 0 {
					source := parsePeriodicTrimOracleReport(t, output)
					if !source.Valid || source.Topology != [4]int{1, 1, 5, 5} {
						t.Fatal("pinned reader did not transfer the clockwise control", source)
					}
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var carrier bytes.Buffer
			if err := WritePeriodicConcavePolygonTrimmedFace(ctx, &carrier, f); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(after, carrier.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			source := parsePeriodicTrimOracleReport(t, output)
			reopened := periodicTrimmedOracle(t, oracle, "read-wires", after)
			if !source.Valid || !native.Valid || !source.PeriodicU || source.PeriodicV ||
				source.Topology != [4]int{1, 1, 5, 5} || source.Bounds != native.Bounds ||
				math.Abs(source.Area/274686048004.52005-1) > 1e-10 ||
				math.Abs(source.Perimeter/2588852.3507343554-1) > 1e-10 ||
				!reflect.DeepEqual(source.Samples, native.Samples) || !reflect.DeepEqual(source, reopened) {
				t.Fatal("pinned transfer changed topology, measures, source directions or physical jets", source, reopened)
			}
			if len(reopened.Wires) != 1 {
				t.Fatal("pinned transfer changed outer wire count", reopened)
			}
			checkPeriodicPolygonPinnedWire(t, ctx, f.Support, reopened.Wires[0], f.Edges, true, f.ProjectionAxes, f.ProjectionOrientation)
			for _, sample := range source.Samples {
				jet, err := f.Support.EvaluateRational(ctx, rat(sample.UV[0]), rat(sample.UV[1]), geom.SurfacePatchOptions{})
				if err != nil {
					t.Fatal(err)
				}
				for axis, p := range sample.Point {
					if jet.Point[axis].Cmp(rat(p)) != 0 || jet.Du[axis].Cmp(rat(sample.Du[axis])) != 0 || jet.Dv[axis].Cmp(rat(sample.Dv[axis])) != 0 {
						t.Fatal("physical jet disagrees with exact source support", sample)
					}
				}
			}
		})
	}
}
