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

func doubleSeamPositiveFixtures() [2]string {
	return [2]string{"exact-uv-both-seams", "exact-uv-both-seams-unequal"}
}

func TestReadPeriodicDoubleSeamTrimmedFace(t *testing.T) {
	ctx := context.Background()
	for i, name := range doubleSeamPositiveFixtures() {
		t.Run(name, func(t *testing.T) {
			raw := periodicHoleFixture(t, name)
			f, err := ReadPeriodicDoubleSeamTrimmedFace(ctx, bytes.NewReader(raw), Limits{MaxEntities: 21, MaxControlPoints: 73})
			if err != nil {
				t.Fatal(err)
			}
			if f.OriginalIGES() != string(raw) || f.LengthUnit != .001 || f.SeamU.Cmp(big.NewRat(5, 1)) != 0 ||
				f.SeamV.Cmp(big.NewRat(7, 1)) != 0 ||
				[5]int{f.FaceID, f.SupportID, f.ContourID, f.UVCompositeID, f.ModelCompositeID} != ([5]int{1, 3, 5, 7, 25}) ||
				f.ProjectionAxes != [2]int{1, 2} || f.ProjectionOrientation != 1 ||
				periodicTrimIntervalStrings(f.CarrierBounds) != ([2][2]string{{"0", "5"}, {"2", "7"}}) {
				t.Fatal("lost original owners, periods or global lifted projection")
			}
			wantBounds := [2][2][2]string{{{"19/4", "21/4"}, {"27/4", "29/4"}}, {{"9/2", "21/4"}, {"13/2", "29/4"}}}
			if periodicTrimIntervalStrings(f.Bounds) != wantBounds[i] {
				t.Fatal("lost original lifted rectangle", f.Bounds)
			}
			checkPeriodicTrimSourceWire(t, f.Edges[:], 9, 27, 48, [2]string{"1/4", "9/16"}[i])
			wantOwners := [2][2][2]string{{{"4", "5"}, {"5", "6"}}, {{"6", "7"}, {"7", "8"}}}
			for u := range f.Cells {
				for v, cell := range f.Cells[u] {
					if cell.Patch == nil ||
						periodicTrimIntervalStrings([2]geom.ExactParameterInterval{cell.KnotOwners[0].Domain, cell.KnotOwners[1].Domain}) !=
							([2][2]string{wantOwners[0][u], wantOwners[1][v]}) ||
						cell.KnotOwners[0].Cycle.Int64() != int64(u) || cell.KnotOwners[1].Cycle.Int64() != int64(v) ||
						cell.KnotOwners[0].Span != 4*(1-u) || cell.KnotOwners[1].Span != 4*(1-v) {
						t.Fatal("changed an original U/V cycle or span", u, v, cell)
					}
				}
			}
			checkDoubleSeamJets(t, ctx, f)
			if single, err := ReadPeriodicSeamTrimmedFace(ctx, bytes.NewReader(raw), Limits{}); err == nil || single.Support != nil {
				t.Fatal("one-seam reader admitted a double-seam chart", err)
			}
		})
	}
}

func checkDoubleSeamJets(t *testing.T, ctx context.Context, f PeriodicDoubleSeamTrimmedFace) {
	t.Helper()
	for axis, seam := range [2]*big.Rat{f.SeamU, f.SeamV} {
		for segment := range 2 {
			lower, upper := f.Cells[0][segment], f.Cells[1][segment]
			if axis == 1 {
				lower, upper = f.Cells[segment][0], f.Cells[segment][1]
			}
			for _, x := range []*big.Rat{big.NewRat(0, 1), big.NewRat(1, 3), big.NewRat(1, 1)} {
				along := 1 - axis
				from, to := lower.Bounds[along].Lower, lower.Bounds[along].Upper
				var physical [2]*big.Rat
				physical[axis] = seam
				physical[along] = new(big.Rat).Add(from, new(big.Rat).Mul(x, new(big.Rat).Sub(to, from)))
				source, err := f.Support.EvaluateRational(ctx, physical[0], physical[1], geom.SurfacePatchOptions{})
				if err != nil {
					t.Fatal(err)
				}
				for side, cell := range [2]PeriodicSeamTrimCell{lower, upper} {
					var uv [2]*big.Rat
					uv[axis], uv[along] = big.NewRat(int64(1-side), 1), x
					jet, err := cell.Patch.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
					if err != nil {
						t.Fatal(err)
					}
					widthU := new(big.Rat).Sub(cell.Bounds[0].Upper, cell.Bounds[0].Lower)
					widthV := new(big.Rat).Sub(cell.Bounds[1].Upper, cell.Bounds[1].Lower)
					for k, q := range source.Point {
						if q.Cmp(jet.Point[k]) != 0 || new(big.Rat).Quo(jet.Du[k], widthU).Cmp(source.Du[k]) != 0 ||
							new(big.Rat).Quo(jet.Dv[k], widthV).Cmp(source.Dv[k]) != 0 {
							t.Fatal("four-cell lift changed the shared physical jet", axis, segment, side, k)
						}
					}
				}
			}
		}
	}
}

func TestPeriodicTrimPhysicalSeamRejectsNormalDerivative(t *testing.T) {
	ctx := context.Background()
	raw := periodicHoleFixture(t, "exact-uv-both-seams-unequal")
	m, err := ReadPeriodicDoubleSeamTrimmedFace(ctx, bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	cells, _, err := m.Support.ExactBezierCells(ctx, m.Bounds[0], m.Bounds[1], geom.PeriodicSurfaceBezierOptions{MaxCells: 4})
	if err != nil {
		t.Fatal(err)
	}
	for axis, upper := range [2]geom.ExactPeriodicSurfaceBezierCell{cells[2], cells[1]} {
		if err := provePeriodicTrimPhysicalSeam(ctx, cells[0], upper, axis); err != nil {
			t.Fatal(err)
		}
		// Change a single interior control next to the seam, keeping the
		// seam positions and thus the tangential derivative identical.
		upper.Control = append([][4]*big.Rat(nil), upper.Control...)
		row := [2]int{upper.DegreeV + 1, 1}[axis]
		upper.Control[row][0] = new(big.Rat).Add(upper.Control[row][0], big.NewRat(1, 100000000000000000))
		if err := provePeriodicTrimPhysicalSeam(ctx, cells[0], upper, axis); err == nil ||
			!strings.Contains(err.Error(), "physical first partials disagree") {
			t.Fatal("equal seam positions hid unequal physical first partials", axis, err)
		}
	}
}

func TestReadPeriodicDoubleSeamTrimmedFaceReject(t *testing.T) {
	ctx := context.Background()
	refuse := func(raw []byte, limits Limits, detail string) {
		t.Helper()
		m, err := ReadPeriodicDoubleSeamTrimmedFace(ctx, bytes.NewReader(raw), limits)
		if err == nil || !strings.Contains(err.Error(), detail) || m.Support != nil || m.FaceID != 0 ||
			m.Cells[0][0].Patch != nil || m.OriginalIGES() != "" {
			t.Fatal("invalid double-seam original returned a partial proof or wrong refusal", err)
		}
	}
	raw := periodicHoleFixture(t, "exact-uv-both-seams")
	doc := periodicTrimmedRecords(t, raw)
	for _, edit := range []struct {
		id, index     int
		value, detail string
	}{
		{3, 228, "2.125", "requires the full original carrier intervals"},
		{25, 3, "27", "duplicated or wrong periodic trim owner #27"},
		{29, 19, "25.19531250000000001", "model edge #29 disagrees with UV line #11"},
	} {
		refuse(changePeriodicTrimmedRecord(t, doc, edit.id, edit.index, edit.value), Limits{}, edit.detail)
	}
	refuse(periodicHoleFixture(t, "exact-uv-both-seams-clockwise"), Limits{}, "does not follow the directed eight-edge split rectangle")
	refuse(periodicHoleFixture(t, "exact-uv-u-seam"), Limits{}, "21 source owners")
	refuse(raw, Limits{MaxEntities: 20}, "entity limit")
	refuse(raw, Limits{MaxControlPoints: 72}, "model edge #41")

	split := changePeriodicTrimmedRecord(t, doc, 9, 4, "5.125")
	split = changePeriodicTrimmedRecord(t, periodicTrimmedRecords(t, split), 11, 1, "5.125")
	refuse(split, Limits{}, "does not follow the directed eight-edge split rectangle")

	// Lift every U line by a complete period. The original model pieces
	// still pair geometrically with the moved lines, but U cycles 1 and 2
	// omit the original U seam.
	shifted := raw
	next := map[string]string{"4.75": "9.75", "5.": "10.", "5.25": "10.25"}
	for id := 9; id <= 23; id += 2 {
		shifted = editPeriodicTrimWriterSource(t, shifted, id, func(values []string) []string {
			values[1], values[4] = next[values[1]], next[values[4]]
			return values
		})
	}
	refuse(shifted, Limits{}, "must cross both original upper seams in cycles 0 and 1")
}

func TestWritePeriodicDoubleSeamTrimmedFace(t *testing.T) {
	ctx := context.Background()
	for _, name := range doubleSeamPositiveFixtures() {
		t.Run(name, func(t *testing.T) {
			raw := periodicHoleFixture(t, name)
			f, err := ReadPeriodicDoubleSeamTrimmedFace(ctx, bytes.NewReader(raw), Limits{})
			if err != nil {
				t.Fatal(err)
			}
			var first, second bytes.Buffer
			if err := WritePeriodicDoubleSeamTrimmedFace(ctx, &first, f); err != nil {
				t.Fatal(err)
			}
			comparePeriodicTrimRecords(t, raw, first.Bytes())
			again, err := ReadPeriodicDoubleSeamTrimmedFace(ctx, bytes.NewReader(first.Bytes()), Limits{})
			if err != nil {
				t.Fatal(err)
			}
			if err := WritePeriodicDoubleSeamTrimmedFace(ctx, &second, again); err != nil || !bytes.Equal(first.Bytes(), second.Bytes()) {
				t.Fatal("canonical four-cell result did not repeat byte-for-byte", err)
			}
			again.original = f.original
			if !reflect.DeepEqual(again, f) || f.OriginalIGES() != string(raw) {
				t.Fatal("strict reopen changed an original seam, physical owner or source")
			}
			if err := WritePeriodicDoubleSeamTrimmedFace(ctx, periodicTrimShortWriter{}, f); !errors.Is(err, io.ErrShortWrite) {
				t.Fatal("short write reported as success", err)
			}
			f.Cells[1][1].KnotOwners[0].Cycle.SetInt64(0)
			out := bytes.NewBufferString("untouched")
			if err := WritePeriodicDoubleSeamTrimmedFace(ctx, out, f); err == nil || out.String() != "untouched" {
				t.Fatal("altered original cycle published a partial carrier", err)
			}
		})
	}
}

func TestReadPeriodicDoubleSeamTrimmedFacePinnedOCCT(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_PERIODIC_128_TRIM_IGES")
	if oracle == "" {
		t.Skip("optional pinned OCCT doubly periodic double-seam author and reader")
	}
	ctx := context.Background()
	names := doubleSeamPositiveFixtures()
	inputs := []string{names[0], names[1], "exact-uv-both-seams-clockwise"}
	for _, name := range inputs {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			before, after := filepath.Join(dir, "original.iges"), filepath.Join(dir, "canonical.iges")
			native := periodicTrimmedOracle(t, oracle, "write-"+name, before)
			raw, err := os.ReadFile(before)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := periodicTrimmedRecords(t, raw), periodicTrimmedRecords(t, periodicHoleFixture(t, name)); !reflect.DeepEqual(got.entities, want.entities) {
				t.Fatal("pinned author changed original double-seam geometry or owners")
			}
			source := periodicTrimmedOracle(t, oracle, "read-wires", before)
			f, err := ReadPeriodicDoubleSeamTrimmedFace(ctx, bytes.NewReader(raw), Limits{})
			if name == "exact-uv-both-seams-clockwise" {
				if err == nil || f.Support != nil || !source.Valid || source.Topology != [4]int{1, 2, 10, 9} ||
					len(source.Wires) != 2 || source.Wires[0].Outer || !source.Wires[1].Outer {
					t.Fatal("expected strict winding refusal and pinned added outer wire", err, source)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !native.Valid || !source.Valid || !source.PeriodicU || !source.PeriodicV || source.Topology != [4]int{1, 1, 8, 8} ||
				len(source.Wires) != 1 || !reflect.DeepEqual(native.Samples, source.Samples) {
				t.Fatal("pinned double-seam reader changed directed pairs or native jets", source)
			}
			var carrier bytes.Buffer
			if err := WritePeriodicDoubleSeamTrimmedFace(ctx, &carrier, f); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(after, carrier.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			reopened := periodicTrimmedOracle(t, oracle, "read-wires", after)
			if !reflect.DeepEqual(source, reopened) {
				t.Fatal("canonical transfer changed original ranges, jets or topology", reopened)
			}
			checkPeriodicTrimOracleWire(t, ctx, f.Support, reopened.Wires[0], f.Edges[:], true, f.ProjectionAxes, f.ProjectionOrientation)
			for _, sample := range reopened.Samples {
				jet, err := f.Support.EvaluateRational(ctx, rat(sample.UV[0]), rat(sample.UV[1]), geom.SurfacePatchOptions{})
				if err != nil {
					t.Fatal(err)
				}
				for k, p := range sample.Point {
					if jet.Point[k].Cmp(rat(p)) != 0 || jet.Du[k].Cmp(rat(sample.Du[k])) != 0 || jet.Dv[k].Cmp(rat(sample.Dv[k])) != 0 {
						t.Fatal("pinned original or canonical jet differs from the exact tensor", sample)
					}
				}
			}
		})
	}
}
