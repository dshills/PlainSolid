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

const concavePeriodicTwoHoleFixture = "exact-concave-polygon-two-holes"

func TestReadPeriodicConcavePolygonTrimmedFaceWithHoles(t *testing.T) {
	ctx := context.Background()
	raw := periodicHoleFixture(t, concavePeriodicTwoHoleFixture)
	f, err := ReadPeriodicConcavePolygonTrimmedFaceWithHoles(ctx, bytes.NewReader(raw), Limits{MaxEntities: 33, MaxControlPoints: 58})
	if err != nil {
		t.Fatal(err)
	}
	if [5]int{f.FaceID, f.SupportID, f.ContourID, f.UVCompositeID, f.ModelCompositeID} != ([5]int{1, 3, 5, 7, 19}) ||
		f.OriginalIGES() != string(raw) || f.LengthUnit != .001 || f.Patch == nil || len(f.Edges) != 5 ||
		f.ProjectionAxes != [2]int{0, 1} || f.ProjectionOrientation != -1 ||
		periodicTrimIntervalStrings(f.Bounds) != ([2][2]string{{"5/4", "7/4"}, {"1", "3"}}) {
		t.Fatal("lost original face owners or chart")
	}
	checkPeriodicTrimSourceWire(t, f.Edges, 9, 21, 144, "75/128")
	first, second := f.Holes[0], f.Holes[1]
	checkPeriodicPolygonFixtureLoop(t, first, periodicPolygonFixtureLoop{3, 31, 33, 41, 35, 43, 160, "-7/512"})
	checkPeriodicPolygonFixtureLoop(t, second, periodicPolygonFixtureLoop{3, 49, 51, 59, 53, 61, 176, "-3/512"})
	outer := periodicAffineWireVertices(f.Edges)
	for _, h := range f.Holes {
		p := h.Edges[0].UVStart
		if periodicUVWindingNumber(outer, p) != 1 || periodicUVTurn(outer[1], outer[2], p) >= 0 {
			t.Fatal("both holes must be inside the actual concave domain and outside the second edge's half-plane")
		}
	}
	for axis := range 2 {
		a, b := first.Bounds[axis], second.Bounds[axis]
		if a.Upper.Cmp(b.Lower) <= 0 || b.Upper.Cmp(a.Lower) <= 0 {
			t.Fatal("positive source must overlap UV bounding boxes in both axes")
		}
	}
	normal := [2]*big.Rat{big.NewRat(-1, 1), big.NewRat(2, 1)}
	a := periodicUVProjection(periodicAffineWireVertices(first.Edges), normal)
	b := periodicUVProjection(periodicAffineWireVertices(second.Edges), normal)
	if a.Lower.Cmp(big.NewRat(49, 16)) != 0 || a.Upper.Cmp(big.NewRat(7, 2)) != 0 ||
		b.Lower.Cmp(big.NewRat(45, 16)) != 0 || b.Upper.Cmp(big.NewRat(3, 1)) != 0 ||
		new(big.Rat).Sub(a.Lower, b.Upper).Cmp(big.NewRat(1, 16)) != 0 {
		t.Fatal("lost exact diagonal separating witness", a, b)
	}
	outerOnly, err := ReadPeriodicConcavePolygonTrimmedFace(ctx, bytes.NewReader(raw), Limits{})
	if err == nil || outerOnly.Support != nil {
		t.Fatal("outer-only reader admitted two holes", err)
	}
	oneHole, err := ReadPeriodicConcavePolygonTrimmedFaceWithHole(ctx, bytes.NewReader(raw), Limits{})
	if err == nil || oneHole.Support != nil {
		t.Fatal("one-hole reader admitted two holes", err)
	}
}

func TestReadPeriodicConcavePolygonTrimmedFaceWithHolesReject(t *testing.T) {
	ctx := context.Background()
	refuse := func(raw []byte, limits Limits, detail string) {
		t.Helper()
		f, err := ReadPeriodicConcavePolygonTrimmedFaceWithHoles(ctx, bytes.NewReader(raw), limits)
		if err == nil || !strings.Contains(err.Error(), detail) || !reflect.DeepEqual(f, PeriodicConcavePolygonTrimmedFaceWithHoles{}) {
			t.Fatal("unsafe source returned a partial proof or wrong refusal", err)
		}
	}
	touching := periodicHoleFixture(t, concavePeriodicTwoHoleFixture+"-touching")
	refuse(touching, Limits{}, "concave trim holes #31 and #49 overlap or meet")
	doc := periodicTrimmedRecords(t, touching)
	a, b := periodicConcaveFixtureUV(t, doc, 39)
	c, d := periodicConcaveFixtureUV(t, doc, 53)
	if a[0].Cmp(d[0]) != 0 || a[1].Cmp(d[1]) != 0 || b[0].Cmp(c[0]) != 0 || b[1].Cmp(c[1]) != 0 {
		t.Fatal("independent contact control must share a complete reversed edge")
	}
	overlap := periodicHoleFixture(t, concavePeriodicTwoHoleFixture+"-overlap")
	refuse(overlap, Limits{}, "concave trim holes #31 and #49 overlap or meet")
	notch := periodicHoleFixture(t, concavePeriodicTwoHoleFixture+"-notch")
	refuse(notch, Limits{}, "concave trim hole #49: iges: hole lies outside the actual concave UV domain")
	refuse(periodicHoleFixture(t, concavePeriodicHoleFixture), Limits{}, "requires exactly two holes")

	raw := periodicHoleFixture(t, concavePeriodicTwoHoleFixture)
	doc = periodicTrimmedRecords(t, raw)
	refuse(changePeriodicTrimmedRecord(t, doc, 1, 6, "31"), Limits{}, "duplicated or wrong periodic trim owner #31")
	refuse(changePeriodicTrimmedRecord(t, doc, 61, 22, "297493.00000000000000001"), Limits{}, "model edge #61 disagrees with UV line #53")
	refuse(raw, Limits{MaxEntities: 32}, "entity limit")
	refuse(raw, Limits{MaxControlPoints: 57}, "model edge #65")
}

func TestWritePeriodicConcavePolygonTrimmedFaceWithHoles(t *testing.T) {
	ctx := context.Background()
	raw := periodicHoleFixture(t, concavePeriodicTwoHoleFixture)
	f, err := ReadPeriodicConcavePolygonTrimmedFaceWithHoles(ctx, bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	var first, second bytes.Buffer
	if err := WritePeriodicConcavePolygonTrimmedFaceWithHoles(ctx, &first, f); err != nil {
		t.Fatal(err)
	}
	comparePeriodicTrimRecords(t, raw, first.Bytes())
	reopened, err := ReadPeriodicConcavePolygonTrimmedFaceWithHoles(ctx, bytes.NewReader(first.Bytes()), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if err := WritePeriodicConcavePolygonTrimmedFaceWithHoles(ctx, &second, reopened); err != nil || !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("canonical three-wire trim changed on repeat", err)
	}
	reopened.original = f.original
	if !reflect.DeepEqual(reopened, f) {
		t.Fatal("canonical reopen changed the original owner ledger")
	}
	out := bytes.NewBufferString("untouched")
	f.Holes[0], f.Holes[1] = f.Holes[1], f.Holes[0]
	if err := WritePeriodicConcavePolygonTrimmedFaceWithHoles(ctx, out, f); err == nil || out.String() != "untouched" {
		t.Fatal("reordered inner contours published a carrier", err)
	}
	f.original = string(periodicHoleFixture(t, concavePeriodicTwoHoleFixture+"-touching"))
	if err := WritePeriodicConcavePolygonTrimmedFaceWithHoles(ctx, out, f); err == nil || out.String() != "untouched" {
		t.Fatal("forged mutual contact published a carrier", err)
	}
}

func TestWritePeriodicConcavePolygonTrimmedFaceWithHolesRejectsRoundedContact(t *testing.T) {
	ctx := context.Background()
	raw := periodicHoleFixture(t, concavePeriodicTwoHoleFixture)
	f, err := ReadPeriodicConcavePolygonTrimmedFaceWithHoles(ctx, bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	doc := periodicTrimmedRecords(t, periodicHoleFixture(t, concavePeriodicTwoHoleFixture+"-touching"))
	for i := range f.Holes[1].Edges {
		start, _ := periodicConcaveFixtureUV(t, doc, 53+2*i)
		// Move along the potential contact edge, then down by 10^-17.
		// The resulting UV stations are distinct from the first hole.
		start[0].Add(start[0], big.NewRat(1, 64))
		start[1].Add(start[1], big.NewRat(1, 128)).Sub(start[1], big.NewRat(1, 100000000000000000))
		f.Holes[1].Edges[i].UVStart = start
	}
	outer := periodicAffineWireVertices(f.Edges)
	a, b := periodicAffineWireVertices(f.Holes[0].Edges), periodicAffineWireVertices(f.Holes[1].Edges)
	if err := provePeriodicConcaveHoleContainment(outer, b); err != nil || !periodicUVStrictlyConvex(b, -1) ||
		!periodicConvexWiresStrictlyDisjoint(a, b) {
		t.Fatal("precision control must have two contained, separated holes", err)
	}
	rounded, err := provePeriodicRoundedConcaveHoles(f.PeriodicConcavePolygonTrimmedFace, f.Holes[:])
	if err != nil || periodicConvexWiresStrictlyDisjoint(rounded[0], rounded[1]) {
		t.Fatal("rounding must close the mutual gap without changing station order, convexity or outer containment", err)
	}
	if err := checkPeriodicConcaveHolesTransfer(f); err == nil || !strings.Contains(err.Error(), "lose strict separation in binary64 transfer") {
		t.Fatal("binary64 transfer admitted mutual contact", err)
	}
}

func TestWritePeriodicConcavePolygonTrimmedFaceWithHolesPinnedOCCT(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_PERIODIC_128_TRIM_IGES")
	if oracle == "" {
		t.Skip("optional pinned OCCT author and reader for the concave polygon with two holes")
	}
	ctx := context.Background()
	for _, example := range []struct {
		suffix   string
		exitCode int
		topology [4]int
	}{
		{"", 0, [4]int{1, 3, 11, 11}},
		{"-touching", 0, [4]int{1, 3, 11, 9}},
		{"-overlap", 0, [4]int{1, 3, 23, 17}},
		{"-notch", 6, [4]int{}},
	} {
		name := concavePeriodicTwoHoleFixture + example.suffix
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
			f, err := ReadPeriodicConcavePolygonTrimmedFaceWithHoles(ctx, bytes.NewReader(raw), Limits{})
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
			if err := WritePeriodicConcavePolygonTrimmedFaceWithHoles(ctx, &carrier, f); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(after, carrier.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			reopened := periodicTrimmedOracle(t, oracle, "read-wires", after)
			if !native.Valid || !source.PeriodicU || source.PeriodicV || source.Bounds != native.Bounds ||
				math.Abs(source.Area/264964700498.77664-1) > 1e-10 || math.Abs(source.Perimeter/3350866.1905677416-1) > 1e-10 ||
				!reflect.DeepEqual(source.Samples, native.Samples) || !reflect.DeepEqual(source, reopened) || len(reopened.Wires) != 3 {
				t.Fatal("pinned reopen changed original topology, metrics, clocks or jets", source, reopened)
			}
			checkPeriodicPolygonPinnedWire(t, ctx, f.Support, reopened.Wires[0], f.Edges, true, f.ProjectionAxes, f.ProjectionOrientation)
			for i, hole := range f.Holes {
				checkPeriodicPolygonPinnedWire(t, ctx, f.Support, reopened.Wires[i+1], hole.Edges, false, f.ProjectionAxes, f.ProjectionOrientation)
			}
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
