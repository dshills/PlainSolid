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

func TestWritePeriodicTrimmedFace(t *testing.T) {
	ctx := context.Background()
	for _, example := range []struct {
		name string
		raw  []byte
	}{
		{"u", periodicTrimmedFixture(t, false)},
		{"v", periodicHoleFixture(t, "exact-v-cell")},
		{"uv", periodicHoleFixture(t, "exact-uv-cell")},
	} {
		t.Run(example.name, func(t *testing.T) {
			labeled := replaceField(string(example.raw), 'D', 6, 7, "CYCLIC")
			labeled = replaceField(labeled, 'D', 10, 8, "17")
			for _, input := range [][]byte{example.raw, []byte(labeled)} {
				face, err := ReadPeriodicTrimmedFace(ctx, bytes.NewReader(input), Limits{})
				if err != nil {
					t.Fatal(err)
				}
				var first, second bytes.Buffer
				if err := WritePeriodicTrimmedFace(ctx, &first, face); err != nil {
					t.Fatal(err)
				}
				if bytes.Equal(input, first.Bytes()) || face.OriginalIGES() != string(input) {
					t.Fatal("expected a separate canonical file and complete original source")
				}
				comparePeriodicTrimRecords(t, input, first.Bytes())
				again, err := ReadPeriodicTrimmedFace(ctx, bytes.NewReader(first.Bytes()), Limits{})
				if err != nil {
					t.Fatal(err)
				}
				if err := WritePeriodicTrimmedFace(ctx, &second, again); err != nil || !bytes.Equal(first.Bytes(), second.Bytes()) {
					t.Fatal("canonical single-cell trim did not repeat byte-for-byte", err)
				}
				again.original = face.original
				if !reflect.DeepEqual(again, face) {
					t.Fatal("strict reopen changed a cyclic owner, direction or original parameter")
				}
			}
		})
	}
}

func TestWritePeriodicTrimmedFaceReject(t *testing.T) {
	ctx := context.Background()
	read := func(input []byte) PeriodicTrimmedFace {
		t.Helper()
		face, err := ReadPeriodicTrimmedFace(ctx, bytes.NewReader(input), Limits{})
		if err != nil {
			t.Fatal("writer control must first pass exact source proof", err)
		}
		return face
	}
	refuse := func(face PeriodicTrimmedFace, detail string) {
		t.Helper()
		out := bytes.NewBufferString("untouched")
		err := WritePeriodicTrimmedFace(ctx, out, face)
		if !errors.Is(err, geom.ErrUnrepresentable) || !strings.Contains(err.Error(), detail) || out.String() != "untouched" {
			t.Fatal("unqualified single-cell face did not refuse atomically", err)
		}
	}
	dual := periodicHoleFixture(t, "exact-uv-cell")
	for _, mutate := range []func(*PeriodicTrimmedFace){
		func(f *PeriodicTrimmedFace) { f.Edges[0].UVSourceID = f.Edges[1].UVSourceID },
		func(f *PeriodicTrimmedFace) { f.KnotOwners[1].Cycle.SetInt64(1) },
		func(f *PeriodicTrimmedFace) { f.Edges[1].ModelParameter.Upper = big.NewRat(36, 1) },
		func(f *PeriodicTrimmedFace) { f.ProjectionOrientation *= -1 },
	} {
		face := read(dual)
		mutate(&face)
		refuse(face, "no longer matches its original proof")
	}
	face := read(dual)
	face.original = string(periodicHoleFixture(t, "exact-uv-cell-clockwise"))
	refuse(face, "does not follow the directed rectangle")

	v := periodicHoleFixture(t, "exact-v-cell")
	for _, example := range []struct{ lo, hi, error string }{
		{"14." + strings.Repeat("0", 67) + "1", "15.", "bounded exact decimal"},
		{"14.", "14.000000000000002", "collapse in binary64 transfer"},
	} {
		raw := editPeriodicTrimWriterSource(t, v, 21, func(values []string) []string {
			for i := range 3 {
				values[7+i], values[10+i] = example.lo, example.hi
			}
			values[25], values[26] = example.lo, example.hi
			return values
		})
		refuse(read(raw), example.error)
	}

	// Each original edge and its associated spline still spans a finite
	// interval, but the final V station is only 1e-17 below the knot at 4.
	const nearV = "3.99999999999999999"
	u := periodicTrimmedFixture(t, false)
	face = read(u)
	near := movePeriodicFixtureVStation(t, u, face.Edges[:], "3.", nearV)
	near = changePeriodicTrimmedRecord(t, periodicTrimmedRecords(t, near), 3, 83, nearV)
	refuse(read(near), "distinct periodic trim stations collapse in binary64 transfer")

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	var out bytes.Buffer
	if err := WritePeriodicTrimmedFace(cancelled, &out, face); !errors.Is(err, context.Canceled) || out.Len() != 0 {
		t.Fatal("cancelled write published a prefix", err)
	}
	if err := WritePeriodicTrimmedFace(ctx, periodicTrimShortWriter{}, face); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal("short write reported as success", err)
	}
}

func TestWritePeriodicTrimmedFacePinnedOCCT(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_PERIODIC_128_TRIM_IGES")
	if oracle == "" {
		t.Skip("optional pinned OCCT reader for periodic single-cell canonical files")
	}
	ctx := context.Background()
	for _, example := range []struct {
		name     string
		raw      []byte
		periodic [2]bool
	}{
		{"u", periodicTrimmedFixture(t, false), [2]bool{true, false}},
		{"v", periodicHoleFixture(t, "exact-v-cell"), [2]bool{false, true}},
		{"uv", periodicHoleFixture(t, "exact-uv-cell"), [2]bool{true, true}},
	} {
		t.Run(example.name, func(t *testing.T) {
			face, err := ReadPeriodicTrimmedFace(ctx, bytes.NewReader(example.raw), Limits{})
			if err != nil {
				t.Fatal(err)
			}
			var carrier bytes.Buffer
			if err := WritePeriodicTrimmedFace(ctx, &carrier, face); err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			before, after := filepath.Join(dir, "original.iges"), filepath.Join(dir, "canonical.iges")
			if err := os.WriteFile(before, example.raw, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(after, carrier.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			source := periodicTrimmedOracle(t, oracle, "read-wires", before)
			reopened := periodicTrimmedOracle(t, oracle, "read-wires", after)
			if !reopened.Valid || [2]bool{reopened.PeriodicU, reopened.PeriodicV} != example.periodic ||
				reopened.Topology != [4]int{1, 1, 4, 4} || len(reopened.Wires) != 1 || len(reopened.Samples) != 5 ||
				!reflect.DeepEqual(reopened, source) {
				t.Fatal("pinned canonical transfer changed directed edge data or physical jets", reopened)
			}
			checkPeriodicTrimOracleWire(t, ctx, face.Support, reopened.Wires[0], face.Edges[:], true, face.ProjectionAxes, face.ProjectionOrientation)
			for _, sample := range reopened.Samples {
				jet, err := face.Support.EvaluateRational(ctx, rat(sample.UV[0]), rat(sample.UV[1]), geom.SurfacePatchOptions{})
				if err != nil {
					t.Fatal(err)
				}
				checkPeriodic128Vector(t, jet.Point, sample.Point)
				checkPeriodic128Vector(t, jet.Du, sample.Du)
				checkPeriodic128Vector(t, jet.Dv, sample.Dv)
			}
		})
	}
}
