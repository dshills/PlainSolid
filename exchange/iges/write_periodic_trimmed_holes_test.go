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

func TestWritePeriodicTrimmedFaceWithHoles(t *testing.T) {
	ctx := context.Background()
	for _, name := range []string{"one-hole", "two-holes"} {
		t.Run(name, func(t *testing.T) {
			raw := periodicHoleFixture(t, name)
			labeled := replaceField(string(raw), 'D', 28, 7, "FIRSTIN")
			labeled = replaceField(labeled, 'D', 28, 8, "17")
			for _, input := range [][]byte{raw, []byte(labeled)} {
				face, err := ReadPeriodicTrimmedFaceWithHoles(ctx, bytes.NewReader(input), Limits{})
				if err != nil {
					t.Fatal(err)
				}
				var first, second bytes.Buffer
				if err := WritePeriodicTrimmedFaceWithHoles(ctx, &first, face); err != nil {
					t.Fatal(err)
				}
				if bytes.Equal(input, first.Bytes()) || face.OriginalIGES() != string(input) {
					t.Fatal("expected a canonical carrier and byte-identical original source")
				}
				comparePeriodicTrimRecords(t, input, first.Bytes())
				again, err := ReadPeriodicTrimmedFaceWithHoles(ctx, bytes.NewReader(first.Bytes()), Limits{})
				if err != nil {
					t.Fatal(err)
				}
				if err := WritePeriodicTrimmedFaceWithHoles(ctx, &second, again); err != nil || !bytes.Equal(first.Bytes(), second.Bytes()) {
					t.Fatal("canonical holed face did not repeat byte-for-byte", err)
				}
				again.original = face.original
				if !reflect.DeepEqual(again, face) {
					t.Fatal("strict reopen changed the common chart or original wire proofs")
				}
			}
		})
	}
}

func TestWritePeriodicTrimmedFaceWithHolesReject(t *testing.T) {
	ctx := context.Background()
	raw := periodicHoleFixture(t, "two-holes")
	read := func(input []byte) PeriodicTrimmedFaceWithHoles {
		t.Helper()
		face, err := ReadPeriodicTrimmedFaceWithHoles(ctx, bytes.NewReader(input), Limits{})
		if err != nil {
			t.Fatal("writer control must first pass exact source proof", err)
		}
		return face
	}
	refuse := func(face PeriodicTrimmedFaceWithHoles, detail string) {
		t.Helper()
		out := bytes.NewBufferString("untouched")
		err := WritePeriodicTrimmedFaceWithHoles(ctx, out, face)
		if !errors.Is(err, geom.ErrUnrepresentable) || !strings.Contains(err.Error(), detail) || out.String() != "untouched" {
			t.Fatal("altered or nonportable face did not refuse atomically", err)
		}
	}
	for _, mutate := range []func(*PeriodicTrimmedFaceWithHoles){
		func(f *PeriodicTrimmedFaceWithHoles) { f.Holes[0], f.Holes[1] = f.Holes[1], f.Holes[0] },
		func(f *PeriodicTrimmedFaceWithHoles) { f.Holes[0].Edges[2].UVSourceID = f.Holes[1].Edges[2].UVSourceID },
		func(f *PeriodicTrimmedFaceWithHoles) {
			f.Holes[0].Bounds[0].Lower.Add(f.Holes[0].Bounds[0].Lower, big.NewRat(1, 1024))
		},
		func(f *PeriodicTrimmedFaceWithHoles) { f.Holes[0].Edges[2].ModelParameter.Lower.SetInt64(30) },
	} {
		face := read(raw)
		mutate(&face)
		refuse(face, "no longer matches its original proof")
	}

	face := read(raw)
	face.original = string(periodicHoleFixture(t, "overlapping-holes"))
	refuse(face, "overlap or meet")

	overlong := editPeriodicTrimWriterSource(t, raw, 43, func(values []string) []string {
		const upper = "23."
		lower := "22." + strings.Repeat("0", 68) + "1"
		for i := range 3 {
			values[7+i], values[10+i] = lower, upper
		}
		values[25], values[26] = lower, upper
		return values
	})
	refuse(read(overlong), "bounded exact decimal")

	closeToOuter := periodicHoleNearOuterSource(t, periodicHoleFixture(t, "one-hole"))
	refuse(read(closeToOuter), "distinct periodic trim stations collapse in binary64 transfer")

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	var out bytes.Buffer
	face = read(raw)
	if err := WritePeriodicTrimmedFaceWithHoles(cancelled, &out, face); !errors.Is(err, context.Canceled) || out.Len() != 0 {
		t.Fatal("cancelled write published a prefix", err)
	}
	if err := WritePeriodicTrimmedFaceWithHoles(ctx, periodicTrimShortWriter{}, face); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal("short write reported as success", err)
	}
}

// Move one boundary to an exact sub-ULP gap and recompute every affected
// model pole from the retained support. Each individual edge still transfers.
func periodicHoleNearOuterSource(t *testing.T, raw []byte) []byte {
	t.Helper()
	ctx := context.Background()
	face, err := ReadPeriodicTrimmedFaceWithHoles(ctx, bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	near := new(big.Rat).Add(face.Bounds[0].Lower, big.NewRat(1, 100000000000000000))
	station := face.Holes[0].Bounds[0].Lower
	token := func(q *big.Rat) string {
		text, err := periodic128Decimal(q)
		if err != nil {
			t.Fatal(err)
		}
		return text
	}
	for _, edge := range face.Holes[0].Edges {
		a, b := edge.UVStart, edge.UVEnd
		if a[0].Cmp(station) == 0 {
			a[0] = near
		}
		if b[0].Cmp(station) == 0 {
			b[0] = near
		}
		raw = editPeriodicTrimWriterSource(t, raw, edge.UVSourceID, func(values []string) []string {
			values[1], values[4] = token(a[0]), token(b[0])
			return values
		})
		var points [3][3]*big.Rat
		mid := [2]*big.Rat{
			new(big.Rat).Quo(new(big.Rat).Add(a[0], b[0]), big.NewRat(2, 1)),
			new(big.Rat).Quo(new(big.Rat).Add(a[1], b[1]), big.NewRat(2, 1)),
		}
		for i, p := range [3][2]*big.Rat{a, mid, b} {
			jet, err := face.Support.EvaluateRational(ctx, p[0], p[1], geom.SurfacePatchOptions{})
			if err != nil {
				t.Fatal(err)
			}
			points[i] = jet.Point
		}
		raw = editPeriodicTrimWriterSource(t, raw, edge.ModelSourceID, func(values []string) []string {
			degree, err := intValue(parameter{text: values[2]})
			if err != nil {
				t.Fatal(err)
			}
			firstPole := 7 + 3*(degree+1)
			for j := range 3 {
				values[firstPole+j] = token(points[0][j])
				values[firstPole+3*degree+j] = token(points[2][j])
				if degree == 2 {
					endpointMean := new(big.Rat).Quo(new(big.Rat).Add(points[0][j], points[2][j]), big.NewRat(2, 1))
					pole := new(big.Rat).Sub(new(big.Rat).Mul(points[1][j], big.NewRat(2, 1)), endpointMean)
					values[firstPole+3+j] = token(pole)
				}
			}
			return values
		})
	}
	return raw
}

func TestWritePeriodicTrimmedFaceWithHolesPinnedOCCT(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_PERIODIC_128_TRIM_IGES")
	if oracle == "" {
		t.Skip("optional pinned OCCT reader for canonical periodic inner trims")
	}
	ctx := context.Background()
	for _, name := range []string{"one-hole", "two-holes"} {
		t.Run(name, func(t *testing.T) {
			raw := periodicHoleFixture(t, name)
			face, err := ReadPeriodicTrimmedFaceWithHoles(ctx, bytes.NewReader(raw), Limits{})
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if err := WritePeriodicTrimmedFaceWithHoles(ctx, &out, face); err != nil {
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
			wires := 1 + len(face.Holes)
			if !got.Valid || !got.PeriodicU || got.Topology != [4]int{1, wires, 4 * wires, 4 * wires} ||
				len(got.Samples) != 5 || len(got.Wires) != wires || !reflect.DeepEqual(got, original) {
				t.Fatal("pinned reader changed the source wires, samples, or measures", got)
			}
			checkPeriodicTrimOracleWire(t, ctx, face.Support, got.Wires[0], face.Edges[:], true, face.ProjectionAxes, face.ProjectionOrientation)
			for i, hole := range face.Holes {
				checkPeriodicTrimOracleWire(t, ctx, face.Support, got.Wires[i+1], hole.Edges[:], false, face.ProjectionAxes, face.ProjectionOrientation)
			}
		})
	}
}
