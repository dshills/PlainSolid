package iges

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"plainsolid/geom"
)

func TestWritePeriodicSeamTrimmedFaceWithMixedHoles(t *testing.T) {
	ctx := context.Background()
	for _, name := range []string{"exact-seam-mixed", "exact-seam-mixed-reversed"} {
		t.Run(name, func(t *testing.T) {
			raw := periodicHoleFixture(t, name)
			labeled := replaceField(string(raw), 'D', 36, 7, "INNER")
			labeled = replaceField(labeled, 'D', 36, 8, "8")
			for _, input := range [][]byte{raw, []byte(labeled)} {
				face, err := ReadPeriodicSeamTrimmedFaceWithMixedHoles(ctx, bytes.NewReader(input), Limits{})
				if err != nil {
					t.Fatal(err)
				}
				var first, second bytes.Buffer
				if err := WritePeriodicSeamTrimmedFaceWithMixedHoles(ctx, &first, face); err != nil {
					t.Fatal(err)
				}
				if bytes.Equal(input, first.Bytes()) || face.OriginalIGES() != string(input) {
					t.Fatal("expected preserved original source and separate canonical carrier")
				}
				comparePeriodicTrimRecords(t, input, first.Bytes())
				again, err := ReadPeriodicSeamTrimmedFaceWithMixedHoles(ctx, bytes.NewReader(first.Bytes()), Limits{})
				if err != nil {
					t.Fatal(err)
				}
				if err := WritePeriodicSeamTrimmedFaceWithMixedHoles(ctx, &second, again); err != nil || !bytes.Equal(first.Bytes(), second.Bytes()) {
					t.Fatal("canonical mixed contour order did not repeat byte-for-byte", err)
				}
				again.original = face.original
				if !reflect.DeepEqual(again, face) {
					t.Fatal("strict reopen changed an original contour order, edge pair or chart")
				}
			}
		})
	}
}

func TestWritePeriodicSeamTrimmedFaceWithMixedHolesReject(t *testing.T) {
	ctx := context.Background()
	raw := periodicHoleFixture(t, "exact-seam-mixed")
	read := func(input []byte) PeriodicSeamTrimmedFaceWithMixedHoles {
		t.Helper()
		face, err := ReadPeriodicSeamTrimmedFaceWithMixedHoles(ctx, bytes.NewReader(input), Limits{})
		if err != nil {
			t.Fatal("writer control must first pass exact source proof", err)
		}
		return face
	}
	refuse := func(face PeriodicSeamTrimmedFaceWithMixedHoles, detail string) {
		t.Helper()
		out := bytes.NewBufferString("untouched")
		err := WritePeriodicSeamTrimmedFaceWithMixedHoles(ctx, out, face)
		if !errors.Is(err, geom.ErrUnrepresentable) || !strings.Contains(err.Error(), detail) || out.String() != "untouched" {
			t.Fatal("unqualified mixed face did not refuse atomically", err)
		}
	}
	for _, mutate := range []func(*PeriodicSeamTrimmedFaceWithMixedHoles){
		func(f *PeriodicSeamTrimmedFaceWithMixedHoles) {
			f.HoleContourIDs[0], f.HoleContourIDs[1] = f.HoleContourIDs[1], f.HoleContourIDs[0]
		},
		func(f *PeriodicSeamTrimmedFaceWithMixedHoles) { f.CellHoleSpan = 1 },
		func(f *PeriodicSeamTrimmedFaceWithMixedHoles) {
			f.CellHole.Edges[0].ModelSourceID = f.SeamHole.Edges[0].ModelSourceID
		},
	} {
		face := read(raw)
		mutate(&face)
		refuse(face, "no longer matches its original proof")
	}
	face := read(raw)
	face.original = string(periodicHoleFixture(t, "exact-seam-mixed-overlap"))
	refuse(face, "overlap or meet")
	face.original = string(periodicHoleFixture(t, "exact-seam-mixed-reversed"))
	refuse(face, "no longer matches its original proof")

	// Both exact holes remain separated, but their distinct original V
	// stations round to the same binary64 near the opposite hole.
	narrow := movePeriodicFixtureVStation(t, raw, face.CellHole.Edges[:], "1.375", "1.49999999999999999")
	refuse(read(narrow), "distinct periodic trim stations collapse in binary64 transfer")
}

func TestWritePeriodicSeamTrimmedFaceWithMixedHolesPinnedOCCT(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_PERIODIC_128_TRIM_IGES")
	if oracle == "" {
		t.Skip("optional pinned OCCT reader for canonical mixed seam-chart holes")
	}
	ctx := context.Background()
	for _, name := range []string{"exact-seam-mixed", "exact-seam-mixed-reversed"} {
		t.Run(name, func(t *testing.T) {
			raw := periodicHoleFixture(t, name)
			face, err := ReadPeriodicSeamTrimmedFaceWithMixedHoles(ctx, bytes.NewReader(raw), Limits{})
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if err := WritePeriodicSeamTrimmedFaceWithMixedHoles(ctx, &out, face); err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			before, after := filepath.Join(dir, "original.iges"), filepath.Join(dir, "canonical.iges")
			if err := os.WriteFile(before, raw, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(after, out.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			original := periodicTrimmedOracle(t, oracle, "read-wires", before)
			got := periodicTrimmedOracle(t, oracle, "read-wires", after)
			if !got.Valid || !got.PeriodicU || got.Topology != [4]int{1, 3, 16, 16} || len(got.Samples) != 5 ||
				len(got.Wires) != 3 || !reflect.DeepEqual(got, original) {
				t.Fatal("pinned canonical reopen changed mixed wires, jets or measures", got)
			}
			checkPeriodicTrimOracleWire(t, ctx, face.Support, got.Wires[0], face.Edges[:], true, face.ProjectionAxes, face.ProjectionOrientation)
			for i, id := range face.HoleContourIDs {
				edges := face.CellHole.Edges[:]
				if id == face.SeamHole.ContourID {
					edges = face.SeamHole.Edges[:]
				}
				checkPeriodicTrimOracleWire(t, ctx, face.Support, got.Wires[i+1], edges, false, face.ProjectionAxes, face.ProjectionOrientation)
			}
		})
	}
}
