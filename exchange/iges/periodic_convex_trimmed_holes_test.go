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

func TestReadPeriodicConvexTrimmedFaceWithHoles(t *testing.T) {
	ctx := context.Background()
	raw := periodicHoleFixture(t, "exact-convex-affine-two-holes")
	f, err := ReadPeriodicConvexTrimmedFaceWithHoles(ctx, bytes.NewReader(raw), Limits{MaxEntities: 35, MaxControlPoints: 62})
	if err != nil {
		t.Fatal(err)
	}
	if f.OriginalIGES() != string(raw) || f.LengthUnit != .001 || f.Patch == nil ||
		[5]int{f.FaceID, f.SupportID, f.ContourID, f.UVCompositeID, f.ModelCompositeID} != ([5]int{1, 3, 5, 7, 17}) ||
		f.ProjectionAxes != [2]int{0, 1} || f.ProjectionOrientation != -1 ||
		periodicTrimIntervalStrings(f.Bounds) != ([2][2]string{{"5/4", "7/4"}, {"1", "3"}}) {
		t.Fatal("lost original outer owners, chart or fixed projection")
	}
	checkPeriodicTrimSourceWire(t, f.Edges[:], 9, 19, 64, "1/2")
	for i, h := range f.Holes {
		want := [2][2][2]string{{{"21/16", "25/16"}, {"7/4", "11/4"}}, {{"23/16", "27/16"}, {"5/4", "9/4"}}}
		if [3]int{h.ContourID, h.UVCompositeID, h.ModelCompositeID} != ([3]int{27 + 22*i, 29 + 22*i, 39 + 22*i}) ||
			periodicTrimIntervalStrings(h.Bounds) != want[i] {
			t.Fatal("lost original hole order, indexed owners, or UV bounds", i, h)
		}
		checkPeriodicTrimSourceWire(t, h.Edges[:], 31+22*i, 41+22*i, 80+16*i, "-3/32")
	}
	a, b := f.Holes[0], f.Holes[1]
	for axis := range 2 {
		if a.Bounds[axis].Upper.Cmp(b.Bounds[axis].Lower) <= 0 || b.Bounds[axis].Upper.Cmp(a.Bounds[axis].Lower) <= 0 {
			t.Fatal("positive fixture must overlap its bounding boxes in both original axes")
		}
	}
	normal := [2]*big.Rat{big.NewRat(-4, 1), big.NewRat(1, 1)}
	first := periodicUVProjection(periodicAffineWireVertices(a.Edges[:]), normal)
	second := periodicUVProjection(periodicAffineWireVertices(b.Edges[:]), normal)
	if first.Lower.Cmp(big.NewRat(-15, 4)) != 0 || first.Upper.Cmp(big.NewRat(-13, 4)) != 0 ||
		second.Lower.Cmp(big.NewRat(-19, 4)) != 0 || second.Upper.Cmp(big.NewRat(-17, 4)) != 0 ||
		new(big.Rat).Sub(first.Lower, second.Upper).Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatal("lost exact diagonal separating witness", first, second)
	}
	if prior, err := ReadPeriodicConvexTrimmedFaceWithHole(ctx, bytes.NewReader(raw), Limits{}); err == nil || prior.Support != nil {
		t.Fatal("single-hole reader admitted a second original hole", err)
	}
}

func TestReadPeriodicConvexTrimmedFaceWithHolesReject(t *testing.T) {
	ctx := context.Background()
	refuse := func(raw []byte, limits Limits, detail string) {
		t.Helper()
		f, err := ReadPeriodicConvexTrimmedFaceWithHoles(ctx, bytes.NewReader(raw), limits)
		if err == nil || !strings.Contains(err.Error(), detail) || f.FaceID != 0 || f.Support != nil ||
			f.Holes[0].ContourID != 0 || f.Holes[1].ContourID != 0 || f.OriginalIGES() != "" {
			t.Fatal("unsafe source returned a partial proof or wrong refusal", err)
		}
	}
	touch := periodicHoleFixture(t, "exact-convex-affine-two-holes-touching")
	refuse(touch, Limits{}, "convex trim holes #27 and #49 overlap or meet")
	refuse(periodicHoleFixture(t, "exact-convex-affine-two-holes-overlap"), Limits{}, "convex trim holes #27 and #49 overlap or meet")
	doc := periodicTrimmedRecords(t, touch)
	a, b, err := periodicTrimUVLine(doc.entities[(37-1)/2])
	if err != nil {
		t.Fatal(err)
	}
	c, d, err := periodicTrimUVLine(doc.entities[(55-1)/2])
	if err != nil || a[0].Cmp(d[0]) != 0 || a[1].Cmp(d[1]) != 0 || b[0].Cmp(c[0]) != 0 || b[1].Cmp(c[1]) != 0 {
		t.Fatal("independent contact control lost its exact common reversed edge", a, b, c, d, err)
	}

	raw := periodicHoleFixture(t, "exact-convex-affine-two-holes")
	doc = periodicTrimmedRecords(t, raw)
	for _, edit := range []struct {
		id, index     int
		value, detail string
	}{
		{1, 6, "27", "duplicated or wrong periodic trim owner #27"},
		{49, 2, "7", "mismatched periodic trim contour support"},
		{61, 3, "63", "duplicated or wrong periodic trim owner #63"},
		{63, 22, "63392.00000000000000001", "model edge #63 disagrees with UV line #53"},
	} {
		refuse(changePeriodicTrimmedRecord(t, doc, edit.id, edit.index, edit.value), Limits{}, edit.detail)
	}
	refuse(raw, Limits{MaxEntities: 34}, "entity limit")
	refuse(raw, Limits{MaxControlPoints: 61}, "model edge #69")
}

func TestWritePeriodicConvexTrimmedFaceWithHoles(t *testing.T) {
	ctx := context.Background()
	raw := periodicHoleFixture(t, "exact-convex-affine-two-holes")
	labeled := []byte(replaceField(string(raw), 'D', 50, 7, "SECOND"))
	for _, input := range [][]byte{raw, labeled} {
		f, err := ReadPeriodicConvexTrimmedFaceWithHoles(ctx, bytes.NewReader(input), Limits{})
		if err != nil {
			t.Fatal(err)
		}
		var first, second bytes.Buffer
		if err := WritePeriodicConvexTrimmedFaceWithHoles(ctx, &first, f); err != nil {
			t.Fatal(err)
		}
		comparePeriodicTrimRecords(t, input, first.Bytes())
		again, err := ReadPeriodicConvexTrimmedFaceWithHoles(ctx, bytes.NewReader(first.Bytes()), Limits{})
		if err != nil {
			t.Fatal(err)
		}
		if err := WritePeriodicConvexTrimmedFaceWithHoles(ctx, &second, again); err != nil || !bytes.Equal(first.Bytes(), second.Bytes()) {
			t.Fatal("canonical affine holes did not repeat byte-for-byte", err)
		}
		again.original = f.original
		if !reflect.DeepEqual(again, f) || f.OriginalIGES() != string(input) {
			t.Fatal("strict reopen changed an original owner, order or retained source")
		}
		out := bytes.NewBufferString("untouched")
		f.Holes[0], f.Holes[1] = f.Holes[1], f.Holes[0]
		if err := WritePeriodicConvexTrimmedFaceWithHoles(ctx, out, f); err == nil || out.String() != "untouched" {
			t.Fatal("reordered native holes published a partial carrier", err)
		}
		f.original = string(periodicHoleFixture(t, "exact-convex-affine-two-holes-touching"))
		if err := WritePeriodicConvexTrimmedFaceWithHoles(ctx, out, f); err == nil || out.String() != "untouched" {
			t.Fatal("forged touching original published a partial carrier", err)
		}
	}
}

func TestPeriodicConvexTransferRejectsRoundedHoleSeparation(t *testing.T) {
	raw := periodicHoleFixture(t, "exact-convex-affine-two-holes-touching")
	doc := periodicTrimmedRecords(t, raw)
	q := func(n, d int64) *big.Rat { return big.NewRat(n, d) }
	var f PeriodicConvexTrimmedFace
	var holes [2]PeriodicConvexTrimHole
	for i, wire := range [3]*[4]PeriodicTrimEdge{&f.Edges, &holes[0].Edges, &holes[1].Edges} {
		for j := range wire {
			point, _, err := periodicTrimUVLine(doc.entities[4+11*i+j])
			if err != nil {
				t.Fatal(err)
			}
			if i == 2 {
				// Move parallel to the contact edge, then lower by
				// 1e-17. No original coordinate stations coincide.
				point[0].Add(point[0], q(1, 64))
				point[1].Add(point[1], q(1, 16)).Sub(point[1], q(1, 100000000000000000))
			}
			wire[j].UVStart = point
		}
	}
	f.KnotOwners = [2]geom.ExactSurfaceKnotSpan{
		{Domain: geom.ExactParameterInterval{Lower: q(1, 1), Upper: q(2, 1)}},
		{Domain: geom.ExactParameterInterval{Lower: q(0, 1), Upper: q(4, 1)}},
	}
	o, a, b := periodicAffineWireVertices(f.Edges[:]), periodicAffineWireVertices(holes[0].Edges[:]), periodicAffineWireVertices(holes[1].Edges[:])
	if !periodicUVStrictlyContains(o, a) || !periodicUVStrictlyContains(o, b) || !periodicConvexWiresStrictlyDisjoint(a, b) {
		t.Fatal("precision control must have exact contained holes and a positive diagonal gap")
	}
	if err := checkPeriodicConvexTransfer(f, holes[:]...); err == nil || !strings.Contains(err.Error(), "lose strict separation") {
		t.Fatal("binary64 transfer failed to distinguish two exactly separated wires", err)
	}
}

func TestReadPeriodicConvexTrimmedFaceWithHolesPinnedOCCT(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_PERIODIC_128_TRIM_IGES")
	if oracle == "" {
		t.Skip("optional pinned OCCT author and reader for two affine holes")
	}
	ctx := context.Background()
	for _, example := range []struct {
		name          string
		topology      [4]int
		validTransfer bool
		validOriginal bool
	}{
		{"exact-convex-affine-two-holes", [4]int{1, 3, 12, 12}, true, true},
		{"exact-convex-affine-two-holes-touching", [4]int{1, 3, 12, 10}, true, false},
		{"exact-convex-affine-two-holes-overlap", [4]int{1, 3, 16, 14}, false, false},
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
				t.Fatal("pinned author changed original contour graph")
			}
			output, err := exec.Command(oracle, "read-wires", before).CombinedOutput()
			if example.validTransfer {
				if err != nil {
					t.Fatalf("pinned read: %v\n%s", err, output)
				}
			} else if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 6 {
				t.Fatalf("pinned reader did not detect invalid crossing: %v\n%s", err, output)
			}
			source := parsePeriodicTrimOracleReport(t, output)
			if source.Valid != example.validTransfer || !native.Valid || !source.PeriodicU || source.PeriodicV ||
				source.Topology != example.topology || source.Bounds != [4]float64{1.25, 1.75, 1, 3} ||
				len(source.Wires) != 3 || !reflect.DeepEqual(native.Samples, source.Samples) {
				t.Fatal("pinned transfer changed expected topology, bounds or physical surface samples", source)
			}
			f, err := ReadPeriodicConvexTrimmedFaceWithHoles(ctx, bytes.NewReader(raw), Limits{})
			if !example.validOriginal {
				if err == nil || f.Support != nil {
					t.Fatal("strict reader admitted original hole contact or overlap", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var carrier bytes.Buffer
			if err := WritePeriodicConvexTrimmedFaceWithHoles(ctx, &carrier, f); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(after, carrier.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			reopened := periodicTrimmedOracle(t, oracle, "read-wires", after)
			if !reflect.DeepEqual(source, reopened) {
				t.Fatal("canonical transfer changed the directed pairs, ranges, measures or jets", reopened)
			}
			checkAffineHolePinnedWire(t, ctx, f, reopened.Wires[0], f.Edges[:], true)
			for i, hole := range f.Holes {
				checkAffineHolePinnedWire(t, ctx, f, reopened.Wires[i+1], hole.Edges[:], false)
			}
			for _, sample := range reopened.Samples {
				jet, err := f.Support.EvaluateRational(ctx, rat(sample.UV[0]), rat(sample.UV[1]), geom.SurfacePatchOptions{})
				if err != nil {
					t.Fatal(err)
				}
				for k, p := range sample.Point {
					if jet.Point[k].Cmp(rat(p)) != 0 || jet.Du[k].Cmp(rat(sample.Du[k])) != 0 || jet.Dv[k].Cmp(rat(sample.Dv[k])) != 0 {
						t.Fatal("transferred physical jet differs from exact chart", sample)
					}
				}
			}
		})
	}
}

func checkAffineHolePinnedWire(t *testing.T, ctx context.Context, f PeriodicConvexTrimmedFaceWithHoles,
	wire periodicTrimOracleWire, pairs []PeriodicTrimEdge, outer bool) {
	t.Helper()
	// The independently authored stations and midpoints are on a
	// 1/64 grid. OCCT's reparameterized line evaluation can differ by
	// one binary64 ULP. Authenticate that residual before comparing.
	wire.Edges = append([]periodicTrimOracleEdge(nil), wire.Edges...)
	for i := range wire.Edges {
		edge := &wire.Edges[i]
		for _, point := range []*[2]float64{&edge.UVStart, &edge.UVMid, &edge.UVEnd} {
			for axis, x := range point {
				exact := math.Round(x*64) / 64
				ulp := math.Abs(math.Nextafter(exact, math.Inf(1)) - exact)
				if math.Abs(x-exact) > ulp {
					t.Fatal("pinned UV line evaluation changed the original affine station", x, exact)
				}
				point[axis] = exact
			}
		}
	}
	checkPeriodicTrimOracleWire(t, ctx, f.Support, wire, pairs, outer, f.ProjectionAxes, f.ProjectionOrientation)
}
