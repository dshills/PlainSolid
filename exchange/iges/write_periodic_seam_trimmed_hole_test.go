package iges

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"plainsolid/geom"
)

func TestWritePeriodicSeamTrimmedFaceWithHole(t *testing.T) {
	ctx := context.Background()
	raw := periodicHoleFixture(t, "exact-seam-hole")
	labeled := replaceField(string(raw), 'D', 36, 7, "SEAMHOLE")
	labeled = replaceField(labeled, 'D', 36, 8, "29")
	for _, input := range [][]byte{raw, []byte(labeled)} {
		face, err := ReadPeriodicSeamTrimmedFaceWithHole(ctx, bytes.NewReader(input), Limits{})
		if err != nil {
			t.Fatal(err)
		}
		var first, second bytes.Buffer
		if err := WritePeriodicSeamTrimmedFaceWithHole(ctx, &first, face); err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(input, first.Bytes()) || face.OriginalIGES() != string(input) {
			t.Fatal("expected separate canonical carrier and complete original source")
		}
		comparePeriodicTrimRecords(t, input, first.Bytes())
		again, err := ReadPeriodicSeamTrimmedFaceWithHole(ctx, bytes.NewReader(first.Bytes()), Limits{})
		if err != nil {
			t.Fatal(err)
		}
		if err := WritePeriodicSeamTrimmedFaceWithHole(ctx, &second, again); err != nil || !bytes.Equal(first.Bytes(), second.Bytes()) {
			t.Fatal("canonical seam trim with hole did not repeat byte-for-byte", err)
		}
		again.original = face.original
		if !reflect.DeepEqual(again, face) {
			t.Fatal("strict reopen changed an original seam split, inner chart or owner")
		}
	}
}

func TestWritePeriodicSeamTrimmedFaceWithHoleReject(t *testing.T) {
	ctx := context.Background()
	raw := periodicHoleFixture(t, "exact-seam-hole")
	read := func(input []byte) PeriodicSeamTrimmedFaceWithHole {
		t.Helper()
		face, err := ReadPeriodicSeamTrimmedFaceWithHole(ctx, bytes.NewReader(input), Limits{})
		if err != nil {
			t.Fatal("writer control must first pass exact source proof", err)
		}
		return face
	}
	refuse := func(face PeriodicSeamTrimmedFaceWithHole, detail string) {
		t.Helper()
		out := bytes.NewBufferString("untouched")
		err := WritePeriodicSeamTrimmedFaceWithHole(ctx, out, face)
		if !errors.Is(err, geom.ErrUnrepresentable) || !strings.Contains(err.Error(), detail) || out.String() != "untouched" {
			t.Fatal("altered or nonportable seam hole did not refuse atomically", err)
		}
	}
	for _, mutate := range []func(*PeriodicSeamTrimmedFaceWithHole){
		func(f *PeriodicSeamTrimmedFaceWithHole) { f.Hole.Edges[1].UVSourceID = f.Edges[1].UVSourceID },
		func(f *PeriodicSeamTrimmedFaceWithHole) { f.Hole.Edges[1].UVEnd[0] = big.NewRat(39, 8) },
		func(f *PeriodicSeamTrimmedFaceWithHole) { f.Hole.Edges[2].ModelParameter.Upper.SetInt64(24) },
		func(f *PeriodicSeamTrimmedFaceWithHole) { f.Hole.Cells[0].Patch = f.Cells[0].Patch },
	} {
		face := read(raw)
		mutate(&face)
		refuse(face, "no longer matches its original proof")
	}
	face := read(raw)
	face.original = string(periodicHoleFixture(t, "exact-seam-touching-hole"))
	refuse(face, "not strictly contained")

	// Z is exactly V in the independent reference support. Move the inner
	// lower station and its model controls together, retaining exact pairing.
	original := read(raw)
	narrow := movePeriodicFixtureVStation(t, raw, original.Hole.Edges[:], "1.5", "1.00000000000000001")
	refuse(read(narrow), "distinct periodic trim stations collapse in binary64 transfer")

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	var out bytes.Buffer
	face = read(raw)
	if err := WritePeriodicSeamTrimmedFaceWithHole(cancelled, &out, face); !errors.Is(err, context.Canceled) || out.Len() != 0 {
		t.Fatal("cancelled write published a prefix", err)
	}
	if err := WritePeriodicSeamTrimmedFaceWithHole(ctx, periodicTrimShortWriter{}, face); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal("short write reported as success", err)
	}
}

func movePeriodicFixtureVStation(t *testing.T, raw []byte, edges []PeriodicTrimEdge, original, corrected string) []byte {
	t.Helper()
	for _, e := range edges {
		raw = editPeriodicTrimWriterSource(t, raw, e.UVSourceID, func(values []string) []string {
			for _, i := range [2]int{2, 5} {
				if values[i] == original {
					values[i] = corrected
				}
			}
			return values
		})
		raw = editPeriodicTrimWriterSource(t, raw, e.ModelSourceID, func(values []string) []string {
			degree, err := intValue(parameter{text: values[2]})
			if err != nil {
				t.Fatal(err)
			}
			poles := degree + 1
			for i := range poles {
				z := 7 + 3*poles + 3*i + 2
				if values[z] == original {
					values[z] = corrected
				}
			}
			return values
		})
	}
	return raw
}

func TestWritePeriodicSeamTrimmedFaceWithHolePinnedOCCT(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_PERIODIC_128_TRIM_IGES")
	if oracle == "" {
		t.Skip("optional pinned OCCT reader for the canonical periodic seam hole")
	}
	ctx := context.Background()
	raw := periodicHoleFixture(t, "exact-seam-hole")
	face, err := ReadPeriodicSeamTrimmedFaceWithHole(ctx, bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := WritePeriodicSeamTrimmedFaceWithHole(ctx, &out, face); err != nil {
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
	if !got.Valid || !got.PeriodicU || got.Topology != [4]int{1, 2, 12, 12} || len(got.Samples) != 5 ||
		len(got.Wires) != 2 || !reflect.DeepEqual(got, original) {
		t.Fatal("pinned canonical reopen changed source seam, wires, jets or measures", got)
	}
	checkPeriodicTrimOracleWire(t, ctx, face.Support, got.Wires[0], face.Edges[:], true, face.ProjectionAxes, face.ProjectionOrientation)
	checkPeriodicTrimOracleWire(t, ctx, face.Support, got.Wires[1], face.Hole.Edges[:], false, face.ProjectionAxes, face.ProjectionOrientation)
}
