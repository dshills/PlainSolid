package iges

import (
	"bytes"
	"context"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"plainsolid/geom"
)

func TestReadPeriodicConvexTrimmedFaceWithHole(t *testing.T) {
	ctx := context.Background()
	raw := periodicHoleFixture(t, "exact-convex-affine-hole")
	f, err := ReadPeriodicConvexTrimmedFaceWithHole(ctx, bytes.NewReader(raw), Limits{MaxEntities: 24, MaxControlPoints: 46})
	if err != nil {
		t.Fatal(err)
	}
	h := f.Hole
	if f.OriginalIGES() != string(raw) || f.LengthUnit != .001 || f.Patch == nil ||
		[5]int{f.FaceID, f.SupportID, f.ContourID, f.UVCompositeID, f.ModelCompositeID} != ([5]int{1, 3, 5, 7, 17}) ||
		[3]int{h.ContourID, h.UVCompositeID, h.ModelCompositeID} != ([3]int{27, 29, 39}) ||
		f.ProjectionAxes != [2]int{0, 1} || f.ProjectionOrientation != -1 ||
		periodicTrimIntervalStrings(f.Bounds) != ([2][2]string{{"5/4", "7/4"}, {"1", "3"}}) ||
		periodicTrimIntervalStrings(h.Bounds) != ([2][2]string{{"11/8", "13/8"}, {"3/2", "5/2"}}) {
		t.Fatal("lost the original outer or inner owner, shared bounds or chart orientation")
	}
	u, v := f.KnotOwners[0], f.KnotOwners[1]
	if periodicTrimIntervalStrings([2]geom.ExactParameterInterval{u.Domain, v.Domain}) != ([2][2]string{{"1", "2"}, {"0", "4"}}) ||
		u.Span != 1 || u.Cycle.Sign() != 0 || v.Span != 0 || v.Cycle != nil {
		t.Fatal("lost the common original knot chart", f.KnotOwners)
	}
	checkPeriodicTrimSourceWire(t, f.Edges[:], 9, 19, 64, "1/2")
	checkPeriodicTrimSourceWire(t, h.Edges[:], 31, 41, 80, "-1/8")
	vertices := [4][2]string{{"3/2", "3/2"}, {"11/8", "2"}, {"3/2", "5/2"}, {"13/8", "2"}}
	for i, edge := range h.Edges {
		if got := [2]string{edge.UVStart[0].RatString(), edge.UVStart[1].RatString()}; got != vertices[i] ||
			edge.UVStart[0].Cmp(edge.UVEnd[0]) == 0 || edge.UVStart[1].Cmp(edge.UVEnd[1]) == 0 {
			t.Fatal("lost an original clockwise diagonal edge station", i, got)
		}
		net, _, err := edge.ModelCurve.BezierControlNet(ctx, geom.SurfacePatchOptions{})
		if err != nil || len(net.Control) != 4 {
			t.Fatal("lost an original cubic hole edge", i, err)
		}
		x := new(big.Rat).Sub(net.Control[3][0], net.Control[0][0])
		inner := new(big.Rat).Sub(net.Control[1][0], net.Control[2][0])
		if x.Add(x, inner.Mul(inner, big.NewRat(3, 1))).Sign() == 0 {
			t.Fatal("hole fixture no longer exercises the cubic composition term", i)
		}
	}
	if old, err := ReadPeriodicConvexTrimmedFace(ctx, bytes.NewReader(raw), Limits{}); err == nil || old.Support != nil {
		t.Fatal("outer-only affine reader admitted the holed source", err)
	}
}

func TestReadPeriodicConvexTrimmedFaceWithHoleReject(t *testing.T) {
	ctx := context.Background()
	refuse := func(raw []byte, limits Limits, detail string) {
		t.Helper()
		f, err := ReadPeriodicConvexTrimmedFaceWithHole(ctx, bytes.NewReader(raw), limits)
		if err == nil || !strings.Contains(err.Error(), detail) || f.FaceID != 0 || f.Hole.ContourID != 0 ||
			f.Support != nil || f.Patch != nil || f.OriginalIGES() != "" {
			t.Fatal("unqualified affine hole returned a partial proof or wrong refusal", err)
		}
	}
	refuse(periodicHoleFixture(t, "exact-convex-affine-hole-ccw"), Limits{}, "strictly convex clockwise UV loop")
	for _, example := range []struct {
		name string
		sign int
	}{
		{"exact-convex-affine-hole-touching", 0},
		{"exact-convex-affine-hole-crossing", -1},
	} {
		raw := periodicHoleFixture(t, example.name)
		refuse(raw, Limits{}, "convex trim hole #27 is not strictly contained")
		doc := periodicTrimmedRecords(t, raw)
		vertices, _, err := periodicTrimUVLine(doc.entities[(37-1)/2])
		if err != nil {
			t.Fatal(err)
		}
		a := [2]*big.Rat{big.NewRat(3, 2), big.NewRat(1, 1)}
		b := [2]*big.Rat{big.NewRat(7, 4), big.NewRat(2, 1)}
		if vertices[0].Cmp(big.NewRat(5, 4)) <= 0 || vertices[0].Cmp(big.NewRat(7, 4)) >= 0 ||
			vertices[1].Cmp(big.NewRat(1, 1)) <= 0 || vertices[1].Cmp(big.NewRat(3, 1)) >= 0 ||
			periodicUVTurn(a, b, vertices) != example.sign {
			t.Fatal("independent control lost its exact slanted edge relation", example.name, vertices)
		}
	}

	raw := periodicHoleFixture(t, "exact-convex-affine-hole")
	doc := periodicTrimmedRecords(t, raw)
	for _, edit := range []struct {
		id, index     int
		value, detail string
	}{
		{1, 5, "5", "duplicated or wrong periodic trim owner #5"},
		{27, 2, "7", "mismatched periodic trim contour support"},
		{39, 3, "41", "duplicated or wrong periodic trim owner #41"},
		{41, 22, "287.50000000000000001", "model edge #41 disagrees with UV line #31"},
	} {
		refuse(changePeriodicTrimmedRecord(t, doc, edit.id, edit.index, edit.value), Limits{}, edit.detail)
	}
	refuse(raw, Limits{MaxEntities: 23}, "entity limit")
	refuse(raw, Limits{MaxControlPoints: 45}, "model edge #47")
}

func TestWritePeriodicConvexTrimmedFaceWithHole(t *testing.T) {
	ctx := context.Background()
	raw := periodicHoleFixture(t, "exact-convex-affine-hole")
	labeled := []byte(replaceField(string(raw), 'D', 28, 7, "AFFHOLE"))
	for _, input := range [][]byte{raw, labeled} {
		f, err := ReadPeriodicConvexTrimmedFaceWithHole(ctx, bytes.NewReader(input), Limits{})
		if err != nil {
			t.Fatal(err)
		}
		var first, second bytes.Buffer
		if err := WritePeriodicConvexTrimmedFaceWithHole(ctx, &first, f); err != nil {
			t.Fatal(err)
		}
		comparePeriodicTrimRecords(t, input, first.Bytes())
		again, err := ReadPeriodicConvexTrimmedFaceWithHole(ctx, bytes.NewReader(first.Bytes()), Limits{})
		if err != nil {
			t.Fatal(err)
		}
		if err := WritePeriodicConvexTrimmedFaceWithHole(ctx, &second, again); err != nil || !bytes.Equal(first.Bytes(), second.Bytes()) {
			t.Fatal("canonical holed affine face did not repeat byte-for-byte", err)
		}
		again.original = f.original
		if !reflect.DeepEqual(again, f) || f.OriginalIGES() != string(input) {
			t.Fatal("strict reopen changed a physical contour, edge range or original source")
		}
		out := bytes.NewBufferString("untouched")
		f.Hole.Edges[0].ModelParameter.Lower.SetInt64(79)
		if err := WritePeriodicConvexTrimmedFaceWithHole(ctx, out, f); err == nil || out.String() != "untouched" {
			t.Fatal("altered affine hole model interval published a partial carrier", err)
		}
		f.original = string(periodicHoleFixture(t, "exact-convex-affine-hole-touching"))
		if err := WritePeriodicConvexTrimmedFaceWithHole(ctx, out, f); err == nil || out.String() != "untouched" {
			t.Fatal("unqualified touching original published a partial carrier", err)
		}
	}
}

func TestPeriodicConvexTransferRejectsRoundedContainment(t *testing.T) {
	q := func(n, d int64) *big.Rat { return big.NewRat(n, d) }
	outer := [4][2]*big.Rat{{q(1, 1), q(1, 1)}, {q(5, 1), q(5, 1)}, {q(4, 1), q(7, 1)}, {q(0, 1), q(3, 1)}}
	almost := new(big.Rat).Add(q(2, 1), q(1, 100000000000000000))
	inner := [4][2]*big.Rat{{q(2, 1), almost}, {q(3, 2), q(7, 2)}, {q(5, 2), q(9, 2)}, {q(7, 2), q(4, 1)}}
	if !periodicUVStrictlyConvex(outer, 1) || !periodicUVStrictlyConvex(inner, -1) || !periodicUVStrictlyContains(outer, inner) {
		t.Fatal("precision control must have exact opposite winding and strict positive clearance")
	}
	var f PeriodicConvexTrimmedFace
	var hole PeriodicConvexTrimHole
	domain := geom.ExactParameterInterval{Lower: q(-1, 1), Upper: q(8, 1)}
	f.KnotOwners = [2]geom.ExactSurfaceKnotSpan{{Domain: domain}, {Domain: domain}}
	for i, p := range outer {
		f.Edges[i].UVStart, hole.Edges[i].UVStart = p, inner[i]
	}
	if err := checkPeriodicConvexTransfer(f, &hole); err == nil || !strings.Contains(err.Error(), "loses strict containment") {
		t.Fatal("positive exact clearance collapsed into boundary contact in binary64 transfer", err)
	}
}

func TestReadPeriodicConvexTrimmedFaceWithHolePinnedOCCT(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_PERIODIC_128_TRIM_IGES")
	if oracle == "" {
		t.Skip("optional pinned OCCT for the affine periodic inner trim")
	}
	ctx := context.Background()
	for _, example := range []struct {
		name                   string
		topology               [4]int
		outerEdges, innerEdges int
		validOriginal          bool
	}{
		{"exact-convex-affine-hole", [4]int{1, 2, 8, 8}, 4, 4, true},
		{"exact-convex-affine-hole-ccw", [4]int{1, 2, 8, 8}, 4, 4, false},
		{"exact-convex-affine-hole-touching", [4]int{1, 2, 9, 8}, 5, 4, false},
		{"exact-convex-affine-hole-crossing", [4]int{1, 2, 12, 10}, 6, 6, false},
	} {
		t.Run(example.name, func(t *testing.T) {
			dir := t.TempDir()
			before, after := filepath.Join(dir, "original.iges"), filepath.Join(dir, "canonical.iges")
			native := periodicTrimmedOracle(t, oracle, "write-"+example.name, before)
			raw, err := os.ReadFile(before)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := periodicTrimmedRecords(t, raw), periodicTrimmedRecords(t, periodicHoleFixture(t, example.name)); !reflect.DeepEqual(got.entities, want.entities) {
				t.Fatal("pinned author changed original affine hole owners or curve coefficients")
			}
			source := periodicTrimmedOracle(t, oracle, "read-wires", before)
			if !native.Valid || !source.Valid || !source.PeriodicU || source.PeriodicV || source.Topology != example.topology ||
				source.Bounds != [4]float64{1.25, 1.75, 1, 3} || len(source.Wires) != 2 ||
				len(source.Wires[0].Edges) != example.outerEdges || len(source.Wires[1].Edges) != example.innerEdges ||
				!reflect.DeepEqual(native.Samples, source.Samples) {
				t.Fatal("pinned transfer changed the expected original/reference topology or samples", source)
			}
			f, err := ReadPeriodicConvexTrimmedFaceWithHole(ctx, bytes.NewReader(raw), Limits{})
			if !example.validOriginal {
				if err == nil || f.Support != nil {
					t.Fatal("strict affine hole reader accepted a source altered by pinned repair", err)
				}
				if example.name == "exact-convex-affine-hole-ccw" && source.Wires[1].Edges[0].ModelRange != ([2]float64{86, 87}) {
					t.Fatal("pinned consumer did not rewind the original counterclockwise hole")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var carrier bytes.Buffer
			if err := WritePeriodicConvexTrimmedFaceWithHole(ctx, &carrier, f); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(after, carrier.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			reopened := periodicTrimmedOracle(t, oracle, "read-wires", after)
			if !reflect.DeepEqual(source, reopened) {
				t.Fatal("pinned canonical reopen changed original contour roles, ranges, derivatives or measures", reopened)
			}
			checkPeriodicTrimOracleWire(t, ctx, f.Support, reopened.Wires[0], f.Edges[:], true, f.ProjectionAxes, f.ProjectionOrientation)
			checkPeriodicTrimOracleWire(t, ctx, f.Support, reopened.Wires[1], f.Hole.Edges[:], false, f.ProjectionAxes, f.ProjectionOrientation)
			for _, sample := range reopened.Samples {
				jet, err := f.Support.EvaluateRational(ctx, rat(sample.UV[0]), rat(sample.UV[1]), geom.SurfacePatchOptions{})
				if err != nil {
					t.Fatal(err)
				}
				for k, p := range sample.Point {
					if jet.Point[k].Cmp(rat(p)) != 0 || jet.Du[k].Cmp(rat(sample.Du[k])) != 0 || jet.Dv[k].Cmp(rat(sample.Dv[k])) != 0 {
						t.Fatal("pinned source or canonical jet differs from the exact common chart", sample)
					}
				}
			}
		})
	}
}
