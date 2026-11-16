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

func TestReadPeriodicConvexTrimmedFace(t *testing.T) {
	ctx := context.Background()
	raw := periodicHoleFixture(t, "exact-convex-affine")
	f, err := ReadPeriodicConvexTrimmedFace(ctx, bytes.NewReader(raw), Limits{MaxEntities: 13, MaxControlPoints: 30})
	if err != nil {
		t.Fatal(err)
	}
	if f.OriginalIGES() != string(raw) || f.LengthUnit != .001 ||
		[5]int{f.FaceID, f.SupportID, f.ContourID, f.UVCompositeID, f.ModelCompositeID} != ([5]int{1, 3, 5, 7, 17}) ||
		f.ProjectionAxes != [2]int{0, 1} || f.ProjectionOrientation != -1 ||
		periodicTrimIntervalStrings(f.Bounds) != ([2][2]string{{"5/4", "7/4"}, {"1", "3"}}) {
		t.Fatal("lost the original directed convex face or its enclosing chart")
	}
	u, v := f.KnotOwners[0], f.KnotOwners[1]
	if periodicTrimIntervalStrings([2]geom.ExactParameterInterval{u.Domain, v.Domain}) != ([2][2]string{{"1", "2"}, {"0", "4"}}) ||
		u.Span != 1 || u.Cycle.Sign() != 0 || v.Span != 0 || v.Cycle != nil {
		t.Fatal("lost the original knot owners", f.KnotOwners)
	}
	checkPeriodicTrimSourceWire(t, f.Edges[:], 9, 19, 64, "1/2")
	vertices := [4][2]string{{"3/2", "1"}, {"7/4", "2"}, {"3/2", "3"}, {"5/4", "2"}}
	for i, edge := range f.Edges {
		if got := [2]string{edge.UVStart[0].RatString(), edge.UVStart[1].RatString()}; got != vertices[i] ||
			edge.UVStart[0].Cmp(edge.UVEnd[0]) == 0 || edge.UVStart[1].Cmp(edge.UVEnd[1]) == 0 {
			t.Fatal("original edge lost a directed station or an affine coordinate", i, got)
		}
		net, _, err := edge.ModelCurve.BezierControlNet(ctx, geom.SurfacePatchOptions{})
		if err != nil || len(net.Control) != 4 {
			t.Fatal("lost the original cubic model curve", i, err)
		}
		a, b, c, d := net.Control[0][0], net.Control[1][0], net.Control[2][0], net.Control[3][0]
		third := new(big.Rat).Add(new(big.Rat).Sub(d, a), new(big.Rat).Mul(big.NewRat(3, 1), new(big.Rat).Sub(b, c)))
		if third.Sign() == 0 {
			t.Fatal("native fixture lacks its genuine cubic composition term", i)
		}
	}
	m, err := f.Support.EvaluateRational(ctx, big.NewRat(3, 2), big.NewRat(2, 1), geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	patch, err := f.Patch.EvaluateRational(ctx, big.NewRat(1, 2), big.NewRat(1, 2), geom.SurfacePatchEvaluationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for i, q := range m.Point {
		if patch.Point[i].Cmp(q) != 0 || patch.Du[i].Cmp(new(big.Rat).Mul(m.Du[i], big.NewRat(1, 2))) != 0 ||
			patch.Dv[i].Cmp(new(big.Rat).Mul(m.Dv[i], big.NewRat(2, 1))) != 0 {
			t.Fatal("source patch or physical derivative normalization changed")
		}
	}
	if simple, err := ReadPeriodicTrimmedFace(ctx, bytes.NewReader(raw), Limits{}); err == nil || simple.Support != nil {
		t.Fatal("rectangle reader admitted the diagonal loop", err)
	}
	if ordinary, err := Read(bytes.NewReader(raw), Limits{}); err == nil || len(ordinary.Entities) != 0 {
		t.Fatal("ordinary geometry reader admitted trimmed topology", err)
	}
}

func TestReadPeriodicConvexTrimmedFaceReject(t *testing.T) {
	ctx := context.Background()
	refuse := func(raw []byte, limits Limits, detail string) {
		t.Helper()
		f, err := ReadPeriodicConvexTrimmedFace(ctx, bytes.NewReader(raw), limits)
		if err == nil || !strings.Contains(err.Error(), detail) || f.Support != nil || f.FaceID != 0 ||
			f.Patch != nil || f.OriginalIGES() != "" {
			t.Fatal("unqualified affine loop produced partial proof or wrong refusal", err)
		}
	}
	raw := periodicHoleFixture(t, "exact-convex-affine")
	doc := periodicTrimmedRecords(t, raw)
	for _, edit := range []struct {
		id, index     int
		value, detail string
	}{
		{3, 80, "1.", "meets or crosses an original knot station"},
		{3, 81, "1.875", "convex trim bounds disagree"},
		{9, 4, "1.75000000000000001", "convex trim bounds disagree"},
		{9, 5, "2.00000000000000001", "do not form one directed cycle"},
		{17, 3, "19", "duplicated or wrong periodic trim owner #19"},
		{19, 22, "2.09375000000000001", "model edge #19 disagrees with UV line #9"},
	} {
		refuse(changePeriodicTrimmedRecord(t, doc, edit.id, edit.index, edit.value), Limits{}, edit.detail)
	}
	refuse(periodicHoleFixture(t, "exact-convex-affine-clockwise"), Limits{}, "strictly convex counterclockwise UV loop")
	refuse(periodicTrimmedFixture(t, false), Limits{}, "requires a nonisoparametric UV line")
	refuse(raw, Limits{MaxEntities: 12}, "entity limit")
	refuse(raw, Limits{MaxControlPoints: 29}, "model edge #25")

	crossed := raw
	vertices := [4][2]string{{"1.5", "1."}, {"1.5", "3."}, {"1.75", "2."}, {"1.25", "2."}}
	for i, start := range vertices {
		end := vertices[(i+1)%len(vertices)]
		crossed = editPeriodicTrimWriterSource(t, crossed, 9+2*i, func(values []string) []string {
			values[1], values[2], values[4], values[5] = start[0], start[1], end[0], end[1]
			return values
		})
	}
	refuse(crossed, Limits{}, "strictly convex counterclockwise UV loop")
}

func TestWritePeriodicConvexTrimmedFace(t *testing.T) {
	ctx := context.Background()
	raw := periodicHoleFixture(t, "exact-convex-affine")
	labeled := []byte(replaceField(string(raw), 'D', 6, 7, "AFFINE"))
	for _, input := range [][]byte{raw, labeled} {
		f, err := ReadPeriodicConvexTrimmedFace(ctx, bytes.NewReader(input), Limits{})
		if err != nil {
			t.Fatal(err)
		}
		var first, second bytes.Buffer
		if err := WritePeriodicConvexTrimmedFace(ctx, &first, f); err != nil {
			t.Fatal(err)
		}
		comparePeriodicTrimRecords(t, input, first.Bytes())
		again, err := ReadPeriodicConvexTrimmedFace(ctx, bytes.NewReader(first.Bytes()), Limits{})
		if err != nil {
			t.Fatal(err)
		}
		if err := WritePeriodicConvexTrimmedFace(ctx, &second, again); err != nil || !bytes.Equal(first.Bytes(), second.Bytes()) {
			t.Fatal("canonical affine face did not repeat byte-for-byte", err)
		}
		again.original = f.original
		if !reflect.DeepEqual(again, f) || f.OriginalIGES() != string(input) {
			t.Fatal("strict reopen changed the physical chart, original sources or model ownership")
		}
		if err := WritePeriodicConvexTrimmedFace(ctx, periodicTrimShortWriter{}, f); !errors.Is(err, io.ErrShortWrite) {
			t.Fatal("short write reported as success", err)
		}
		f.Edges[0].ModelParameter.Lower.SetInt64(63)
		out := bytes.NewBufferString("untouched")
		if err := WritePeriodicConvexTrimmedFace(ctx, out, f); err == nil || out.String() != "untouched" {
			t.Fatal("altered model interval published partial carrier", err)
		}
	}
}

func TestPeriodicConvexTransferRejectsRoundedTurn(t *testing.T) {
	q := func(n int64) *big.Rat { return big.NewRat(n, 1) }
	almost := new(big.Rat).Add(q(3), big.NewRat(1, 100000000000000000))
	points := [4][2]*big.Rat{{q(1), q(1)}, {q(2), q(2)}, {q(3), almost}, {q(1), q(4)}}
	if !periodicUVStrictlyConvex(points, 1) {
		t.Fatal("precision control must have strict exact turns")
	}
	var f PeriodicConvexTrimmedFace
	domain := geom.ExactParameterInterval{Lower: q(0), Upper: q(5)}
	f.KnotOwners = [2]geom.ExactSurfaceKnotSpan{{Domain: domain}, {Domain: domain}}
	for i, p := range points {
		f.Edges[i].UVStart = p
	}
	if err := checkPeriodicConvexTransfer(f, nil); err == nil || !strings.Contains(err.Error(), "loses its original winding or convexity") {
		t.Fatal("binary64-collinear UV triplet passed the convex transfer guard", err)
	}
}

func TestReadPeriodicConvexTrimmedFacePinnedOCCT(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_PERIODIC_128_TRIM_IGES")
	if oracle == "" {
		t.Skip("optional pinned OCCT convex periodic face author and reader")
	}
	ctx := context.Background()
	for _, name := range []string{"exact-convex-affine", "exact-convex-affine-clockwise"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			before, after := filepath.Join(dir, "original.iges"), filepath.Join(dir, "canonical.iges")
			native := periodicTrimmedOracle(t, oracle, "write-"+name, before)
			raw, err := os.ReadFile(before)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := periodicTrimmedRecords(t, raw), periodicTrimmedRecords(t, periodicHoleFixture(t, name)); !reflect.DeepEqual(got.entities, want.entities) {
				t.Fatal("pinned author changed original affine UV/model coefficients")
			}
			source := periodicTrimmedOracle(t, oracle, "read-wires", before)
			if !native.Valid || !source.Valid || !source.PeriodicU || source.PeriodicV ||
				source.Topology != [4]int{1, 1, 4, 4} || source.Bounds != [4]float64{1.25, 1.75, 1, 3} ||
				len(source.Wires) != 1 || len(source.Wires[0].Edges) != 4 || !reflect.DeepEqual(native.Samples, source.Samples) {
				t.Fatal("pinned reader changed support flags, affine pairing or native jet samples", source)
			}
			f, err := ReadPeriodicConvexTrimmedFace(ctx, bytes.NewReader(raw), Limits{})
			if name == "exact-convex-affine-clockwise" {
				if err == nil || f.Support != nil || source.Wires[0].Edges[0].ModelRange != [2]float64{70, 71} ||
					source.Wires[0].Edges[0].UVStart != [2]float64{1.25, 2} || source.Wires[0].Edges[0].UVEnd != [2]float64{1.5, 1} {
					t.Fatal("expected original winding refusal and pinned rewinding", err, source)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var carrier bytes.Buffer
			if err := WritePeriodicConvexTrimmedFace(ctx, &carrier, f); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(after, carrier.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			reopened := periodicTrimmedOracle(t, oracle, "read-wires", after)
			if !reflect.DeepEqual(source, reopened) {
				t.Fatal("canonical transfer changed original directions, intervals, jets or measures", reopened)
			}
			checkPeriodicTrimOracleWire(t, ctx, f.Support, reopened.Wires[0], f.Edges[:], true, f.ProjectionAxes, f.ProjectionOrientation)
			for _, sample := range reopened.Samples {
				jet, err := f.Support.EvaluateRational(ctx, rat(sample.UV[0]), rat(sample.UV[1]), geom.SurfacePatchOptions{})
				if err != nil {
					t.Fatal(err)
				}
				for k, p := range sample.Point {
					if jet.Point[k].Cmp(rat(p)) != 0 || jet.Du[k].Cmp(rat(sample.Du[k])) != 0 || jet.Dv[k].Cmp(rat(sample.Dv[k])) != 0 {
						t.Fatal("pinned original or canonical jet differs from the exact source tensor", sample)
					}
				}
			}
		})
	}
}
