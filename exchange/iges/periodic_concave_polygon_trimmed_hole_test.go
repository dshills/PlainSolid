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

const concavePeriodicHoleFixture = "exact-concave-polygon-hole"

func TestReadPeriodicConcavePolygonTrimmedFaceWithHole(t *testing.T) {
	ctx := context.Background()
	raw := periodicHoleFixture(t, concavePeriodicHoleFixture)
	f, err := ReadPeriodicConcavePolygonTrimmedFaceWithHole(ctx, bytes.NewReader(raw), Limits{MaxEntities: 24, MaxControlPoints: 46})
	if err != nil {
		t.Fatal(err)
	}
	if [5]int{f.FaceID, f.SupportID, f.ContourID, f.UVCompositeID, f.ModelCompositeID} != ([5]int{1, 3, 5, 7, 19}) ||
		f.OriginalIGES() != string(raw) || f.LengthUnit != .001 || f.Patch == nil || len(f.Edges) != 5 ||
		f.ProjectionAxes != [2]int{0, 1} || f.ProjectionOrientation != -1 ||
		periodicTrimIntervalStrings(f.Bounds) != ([2][2]string{{"5/4", "7/4"}, {"1", "3"}}) ||
		periodicTrimIntervalStrings(f.Hole.Bounds) != ([2][2]string{{"23/16", "25/16"}, {"9/4", "5/2"}}) {
		t.Fatal("lost the original loop owners, support or chart")
	}
	checkPeriodicTrimSourceWire(t, f.Edges, 9, 21, 144, "75/128")
	checkPeriodicPolygonFixtureLoop(t, f.Hole, periodicPolygonFixtureLoop{3, 31, 33, 41, 35, 43, 160, "-7/512"})
	p := f.Hole.Edges[0].UVStart
	outer := periodicAffineWireVertices(f.Edges)
	if periodicUVWindingNumber(outer, p) != 1 || periodicUVTurn(outer[1], outer[2], p) >= 0 {
		t.Fatal("positive hole must be inside the concave domain and outside an outer-edge half-plane")
	}
	old, err := ReadPeriodicConcavePolygonTrimmedFace(ctx, bytes.NewReader(raw), Limits{})
	if err == nil || old.Support != nil {
		t.Fatal("outer-only reader admitted an original hole", err)
	}
}

func TestReadPeriodicConcavePolygonTrimmedFaceWithHoleReject(t *testing.T) {
	ctx := context.Background()
	refuse := func(raw []byte, limits Limits, detail string) {
		t.Helper()
		f, err := ReadPeriodicConcavePolygonTrimmedFaceWithHole(ctx, bytes.NewReader(raw), limits)
		if err == nil || !strings.Contains(err.Error(), detail) || !reflect.DeepEqual(f, PeriodicConcavePolygonTrimmedFaceWithHole{}) {
			t.Fatal("unsafe hole returned a partial proof or wrong refusal", err)
		}
	}
	notch := periodicHoleFixture(t, concavePeriodicHoleFixture+"-notch")
	refuse(notch, Limits{}, "hole lies outside the actual concave UV domain")
	nOuter, nInner := periodicConcaveHoleFixturePoints(t, notch)
	for _, p := range nInner {
		if periodicUVWindingNumber(nOuter, p) != 0 || p[0].Cmp(big.NewRat(5, 4)) <= 0 || p[0].Cmp(big.NewRat(7, 4)) >= 0 ||
			p[1].Cmp(big.NewRat(1, 1)) <= 0 || p[1].Cmp(big.NewRat(3, 1)) >= 0 {
			t.Fatal("independent notch hole must be strictly within the bounding rectangle and outside the actual face")
		}
	}
	crossing := periodicHoleFixture(t, concavePeriodicHoleFixture+"-crossing")
	refuse(crossing, Limits{}, "outer edge 2 and hole edge 3 meet")
	cOuter, cInner := periodicConcaveHoleFixturePoints(t, crossing)
	for _, p := range cInner {
		if periodicUVWindingNumber(cOuter, p) != 1 {
			t.Fatal("the crossing hole must have all vertices inside the concave outer domain")
		}
	}
	for _, i := range []int{1, 2} {
		a, b, c, d := cOuter[i], cOuter[i+1], cInner[2], cInner[0]
		if periodicUVTurn(a, b, c)*periodicUVTurn(a, b, d) != -1 || periodicUVTurn(c, d, a)*periodicUVTurn(c, d, b) != -1 {
			t.Fatal("the third hole edge must cross both sides of the notch", i)
		}
	}
	touch := periodicHoleFixture(t, concavePeriodicHoleFixture+"-touching")
	refuse(touch, Limits{}, "outer edge 2 and hole edge 2 meet")
	tOuter, tInner := periodicConcaveHoleFixturePoints(t, touch)
	if tOuter[2][0].Cmp(tInner[2][0]) != 0 || tOuter[2][1].Cmp(tInner[2][1]) != 0 {
		t.Fatal("the touching control must coincide at the outer reflex vertex")
	}

	raw := periodicHoleFixture(t, concavePeriodicHoleFixture)
	doc := periodicTrimmedRecords(t, raw)
	refuse(changePeriodicTrimmedRecord(t, doc, 41, 3, "43"), Limits{}, "duplicated or wrong periodic trim owner #43")
	refuse(changePeriodicTrimmedRecord(t, doc, 43, 22, "330928.00000000000000001"), Limits{}, "model edge #43 disagrees with UV line #35")
	refuse(raw, Limits{MaxEntities: 23}, "entity limit")
	refuse(raw, Limits{MaxControlPoints: 45}, "model edge #47")
	refuse(periodicHoleFixture(t, concavePeriodicFixture), Limits{}, "requires exactly one hole")
	refuse(periodicHoleFixture(t, "exact-convex-affine-two-holes"), Limits{}, "requires exactly one hole")
}

func periodicConcaveHoleFixturePoints(t *testing.T, raw []byte) ([][2]*big.Rat, [][2]*big.Rat) {
	t.Helper()
	doc := periodicTrimmedRecords(t, raw)
	outer, inner := make([][2]*big.Rat, 5), make([][2]*big.Rat, 3)
	for i := range outer {
		outer[i], _ = periodicConcaveFixtureUV(t, doc, 9+2*i)
	}
	for i := range inner {
		inner[i], _ = periodicConcaveFixtureUV(t, doc, 35+2*i)
	}
	return outer, inner
}

func TestWritePeriodicConcavePolygonTrimmedFaceWithHole(t *testing.T) {
	ctx := context.Background()
	raw := periodicHoleFixture(t, concavePeriodicHoleFixture)
	f, err := ReadPeriodicConcavePolygonTrimmedFaceWithHole(ctx, bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	var first, second bytes.Buffer
	if err := WritePeriodicConcavePolygonTrimmedFaceWithHole(ctx, &first, f); err != nil {
		t.Fatal(err)
	}
	comparePeriodicTrimRecords(t, raw, first.Bytes())
	reopened, err := ReadPeriodicConcavePolygonTrimmedFaceWithHole(ctx, bytes.NewReader(first.Bytes()), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if err := WritePeriodicConcavePolygonTrimmedFaceWithHole(ctx, &second, reopened); err != nil || !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("canonical concave trim and hole changed on repeat", err)
	}
	reopened.original = f.original
	if !reflect.DeepEqual(reopened, f) {
		t.Fatal("canonical reopen changed original owner proof")
	}
	out := bytes.NewBufferString("untouched")
	f.Hole.Edges[0].ModelParameter = f.Hole.Edges[1].ModelParameter
	if err := WritePeriodicConcavePolygonTrimmedFaceWithHole(ctx, out, f); err == nil || out.String() != "untouched" {
		t.Fatal("changed inner range published a partial carrier", err)
	}
	f.original = string(periodicHoleFixture(t, concavePeriodicHoleFixture+"-notch"))
	if err := WritePeriodicConcavePolygonTrimmedFaceWithHole(ctx, out, f); err == nil || out.String() != "untouched" {
		t.Fatal("forged notch source published a partial carrier", err)
	}
}

func TestWritePeriodicConcavePolygonTrimmedFaceWithHoleRejectsRoundedContact(t *testing.T) {
	ctx := context.Background()
	raw := periodicHoleFixture(t, concavePeriodicHoleFixture)
	f, err := ReadPeriodicConcavePolygonTrimmedFaceWithHole(ctx, bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	// The third outer edge contains (103/64, 33/16). Place the
	// third hole vertex just inside it at a distinct U and V station.
	f.Hole.Edges[2].UVStart = [2]*big.Rat{
		new(big.Rat).Sub(big.NewRat(103, 64), big.NewRat(1, 100000000000000000)), big.NewRat(33, 16),
	}
	outer, inner := periodicAffineWireVertices(f.Edges), periodicAffineWireVertices(f.Hole.Edges)
	if err := provePeriodicConcaveHoleContainment(outer, inner); err != nil || !periodicUVStrictlyConvex(inner, -1) {
		t.Fatal("precision control must retain an originally contained clockwise convex hole", err)
	}
	rounded, err := periodicRoundedAffineWires(f.KnotOwners, [][]PeriodicTrimEdge{f.Edges, f.Hole.Edges})
	if err != nil || !periodicUVStrictlyConvex(rounded[1], -1) {
		t.Fatal("rounding control must preserve station order and clockwise turns", err)
	}
	if err := checkPeriodicConcaveHoleTransfer(f); err == nil || !strings.Contains(err.Error(), "outer edge 3 and hole edge 2 meet") {
		t.Fatal("binary64 transfer admitted a new hole contact", err)
	}
}

func TestWritePeriodicConcavePolygonTrimmedFaceWithHolePinnedOCCT(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_PERIODIC_128_TRIM_IGES")
	if oracle == "" {
		t.Skip("optional pinned OCCT author and reader for the concave polygon and hole")
	}
	ctx := context.Background()
	for _, example := range []struct {
		suffix   string
		exitCode int
		topology [4]int
	}{
		{"", 0, [4]int{1, 2, 8, 8}},
		{"-notch", 6, [4]int{}},
		{"-crossing", 0, [4]int{1, 2, 12, 10}},
		{"-touching", 0, [4]int{1, 2, 8, 7}},
	} {
		name := concavePeriodicHoleFixture + example.suffix
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
			var source periodicTrimReport
			if example.exitCode == 0 {
				if err != nil {
					t.Fatalf("pinned read: %v\n%s", err, output)
				}
				source = parsePeriodicTrimOracleReport(t, output)
				if !source.Valid || source.Topology != example.topology {
					t.Fatal("pinned reader changed the expected topology", source)
				}
			} else if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != example.exitCode {
				t.Fatalf("pinned one-face control: %v\n%s", err, output)
			}
			f, err := ReadPeriodicConcavePolygonTrimmedFaceWithHole(ctx, bytes.NewReader(raw), Limits{})
			if example.suffix != "" {
				if err == nil || f.Support != nil {
					t.Fatal("strict reader admitted independently authored negative", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var carrier bytes.Buffer
			if err := WritePeriodicConcavePolygonTrimmedFaceWithHole(ctx, &carrier, f); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(after, carrier.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			reopened := periodicTrimmedOracle(t, oracle, "read-wires", after)
			if !native.Valid || !source.PeriodicU || source.PeriodicV || source.Bounds != native.Bounds ||
				math.Abs(source.Area/267837324695.45642-1) > 1e-10 || math.Abs(source.Perimeter/2998908.9170183865-1) > 1e-10 ||
				!reflect.DeepEqual(source.Samples, native.Samples) || !reflect.DeepEqual(source, reopened) || len(reopened.Wires) != 2 {
				t.Fatal("pinned reopen changed the original geometry, topology, measures or jets", source, reopened)
			}
			checkPeriodicPolygonPinnedWire(t, ctx, f.Support, reopened.Wires[0], f.Edges, true, f.ProjectionAxes, f.ProjectionOrientation)
			checkPeriodicPolygonPinnedWire(t, ctx, f.Support, reopened.Wires[1], f.Hole.Edges, false, f.ProjectionAxes, f.ProjectionOrientation)
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
