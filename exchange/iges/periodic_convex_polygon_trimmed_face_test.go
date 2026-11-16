package iges

import (
	"bytes"
	"context"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"plainsolid/geom"
)

type periodicPolygonFixtureLoop struct {
	count, contour, uvComposite, modelComposite int
	firstUV, firstModel, firstStation           int
	area                                        string
}

var periodicPolygonFixtures = []struct {
	name            string
	entities, poles int
	outer, hole     periodicPolygonFixtureLoop
	pinnedArea      float64
}{
	{"exact-convex-polygon-triangle", 11, 26,
		periodicPolygonFixtureLoop{3, 5, 7, 15, 9, 17, 112, "15/32"},
		periodicPolygonFixtureLoop{}, 214011374232.95422},
	{"exact-convex-polygon-triangle-pentagon", 24, 45,
		periodicPolygonFixtureLoop{3, 5, 7, 15, 9, 17, 112, "15/32"},
		periodicPolygonFixtureLoop{5, 23, 25, 37, 27, 39, 128, "-43/1024"}, 194525232422.55716},
	{"exact-convex-polygon-pentagon-triangle", 24, 46,
		periodicPolygonFixtureLoop{5, 5, 7, 19, 9, 21, 112, "71/128"},
		periodicPolygonFixtureLoop{3, 31, 33, 41, 35, 43, 128, "-15/512"}, 251181253931.9065},
}

func TestReadPeriodicConvexPolygonTrimmedFace(t *testing.T) {
	for _, example := range periodicPolygonFixtures {
		t.Run(example.name, func(t *testing.T) {
			raw := periodicHoleFixture(t, example.name)
			f, err := ReadPeriodicConvexPolygonTrimmedFace(context.Background(), bytes.NewReader(raw), Limits{
				MaxEntities: example.entities, MaxControlPoints: example.poles,
			})
			if err != nil {
				t.Fatal(err)
			}
			u, v := f.KnotOwners[0], f.KnotOwners[1]
			if f.OriginalIGES() != string(raw) || f.LengthUnit != .001 || f.Patch == nil ||
				f.FaceID != 1 || f.SupportID != 3 || f.ProjectionAxes != [2]int{0, 1} || f.ProjectionOrientation != -1 ||
				periodicTrimIntervalStrings(f.Outer.Bounds) != ([2][2]string{{"5/4", "7/4"}, {"1", "3"}}) ||
				periodicTrimIntervalStrings([2]geom.ExactParameterInterval{u.Domain, v.Domain}) != ([2][2]string{{"1", "2"}, {"0", "4"}}) ||
				u.Span != 1 || v.Span != 0 || u.Cycle.Sign() != 0 || v.Cycle != nil {
				t.Fatal("lost original owners, units or knot cell")
			}
			checkPeriodicPolygonFixtureLoop(t, f.Outer, example.outer)
			wantHoles := 0
			if example.hole.count != 0 {
				wantHoles = 1
			}
			if len(f.Holes) != wantHoles {
				t.Fatal("lost original hole topology", f.Holes)
			}
			if wantHoles != 0 {
				checkPeriodicPolygonFixtureLoop(t, f.Holes[0], example.hole)
			}
		})
	}
}

func checkPeriodicPolygonFixtureLoop(t *testing.T, loop PeriodicConvexPolygonTrimLoop, want periodicPolygonFixtureLoop) {
	t.Helper()
	if len(loop.Edges) != want.count || loop.ContourID != want.contour || loop.UVCompositeID != want.uvComposite ||
		loop.ModelCompositeID != want.modelComposite {
		t.Fatal("lost original polygon count or indexed owner", loop)
	}
	checkPeriodicTrimSourceWire(t, loop.Edges, want.firstUV, want.firstModel, want.firstStation, want.area)
}

func TestReadPeriodicConvexPolygonTrimmedFaceReject(t *testing.T) {
	refuse := func(raw []byte, limits Limits, detail string) {
		t.Helper()
		f, err := ReadPeriodicConvexPolygonTrimmedFace(context.Background(), bytes.NewReader(raw), limits)
		if err == nil || !strings.Contains(err.Error(), detail) || !reflect.DeepEqual(f, PeriodicConvexPolygonTrimmedFace{}) {
			t.Fatal("unsafe trim returned a partial proof or wrong refusal", err)
		}
	}
	refuse(periodicHoleFixture(t, "exact-convex-polygon-collinear"), Limits{}, "strictly convex")
	raw := periodicHoleFixture(t, "exact-convex-polygon-triangle-pentagon")
	doc := periodicTrimmedRecords(t, raw)
	for _, edit := range []struct {
		id, field     int
		value, detail string
	}{
		{1, 5, "5", "duplicated or wrong periodic trim owner #5"},
		{7, 1, "9", "wrong periodic trim edge count on composite #7"},
		{15, 1, "5", "wrong periodic trim edge count on composite #15"},
		{17, 22, "317824.00000000000000001", "model edge #17 disagrees with UV line #9"},
		{37, 3, "39", "duplicated or wrong periodic trim owner #39"},
	} {
		refuse(changePeriodicTrimmedRecord(t, doc, edit.id, edit.field, edit.value), Limits{}, edit.detail)
	}
	refuse(raw, Limits{MaxEntities: 23}, "entity limit")
	refuse(raw, Limits{MaxControlPoints: 44}, "model edge #47")
}

func TestWritePeriodicConvexPolygonTrimmedFace(t *testing.T) {
	ctx := context.Background()
	for _, example := range periodicPolygonFixtures {
		t.Run(example.name, func(t *testing.T) {
			raw := periodicHoleFixture(t, example.name)
			checkPeriodicPolygonCanonical(t, raw)
		})
	}
	t.Run("directory roles reordered", func(t *testing.T) {
		raw := periodicHoleFixture(t, "exact-convex-polygon-triangle-pentagon")
		doc := periodicTrimmedRecords(t, raw)
		doc.entities[0], doc.entities[1] = doc.entities[1], doc.entities[0]
		var author topologyAuthor
		root, contour := 0, 10500
		for _, record := range doc.entities {
			values := make([]string, len(record.values))
			for i, value := range record.values {
				values[i] = value.text
			}
			switch record.kind {
			case 144:
				values[1] = "1"
			case 142:
				values[2] = "1"
			}
			author.add(record.kind, 0, joinTopologyValues(values))
			emitted := &author.records[len(author.records)-1]
			switch record.kind {
			case 144:
				emitted.status = &root
			case 142:
				emitted.status = &contour
			}
		}
		reordered, err := author.file(doc.unit)
		if err != nil {
			t.Fatal(err)
		}
		f := checkPeriodicPolygonCanonical(t, reordered)
		if f.FaceID != 3 || f.SupportID != 1 || f.Outer.ContourID != 5 || f.Holes[0].ContourID != 23 {
			t.Fatal("changed original topology roles when numbering differed", f)
		}
		out := bytes.NewBufferString("untouched")
		f.Holes[0].Edges[0].ModelParameter = f.Holes[0].Edges[1].ModelParameter
		if err := WritePeriodicConvexPolygonTrimmedFace(ctx, out, f); err == nil || out.String() != "untouched" {
			t.Fatal("changed source range published a carrier", err)
		}
	})
}

func checkPeriodicPolygonCanonical(t *testing.T, input []byte) PeriodicConvexPolygonTrimmedFace {
	t.Helper()
	ctx := context.Background()
	f, err := ReadPeriodicConvexPolygonTrimmedFace(ctx, bytes.NewReader(input), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	var first, second bytes.Buffer
	if err := WritePeriodicConvexPolygonTrimmedFace(ctx, &first, f); err != nil {
		t.Fatal(err)
	}
	comparePeriodicTrimRecords(t, input, first.Bytes())
	reopened, err := ReadPeriodicConvexPolygonTrimmedFace(ctx, bytes.NewReader(first.Bytes()), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if err := WritePeriodicConvexPolygonTrimmedFace(ctx, &second, reopened); err != nil || !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("canonical output did not repeat byte-for-byte", err)
	}
	reopened.original = f.original
	if !reflect.DeepEqual(reopened, f) || f.OriginalIGES() != string(input) {
		t.Fatal("strict reopen changed original owner data")
	}
	return f
}

func TestWritePeriodicConvexPolygonTrimmedFacePinnedOCCT(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_PERIODIC_128_TRIM_IGES")
	if oracle == "" {
		t.Skip("optional pinned OCCT author and reader for convex polygons")
	}
	ctx := context.Background()
	for _, example := range periodicPolygonFixtures {
		t.Run(example.name, func(t *testing.T) {
			dir := t.TempDir()
			before, after := filepath.Join(dir, "original.iges"), filepath.Join(dir, "canonical.iges")
			native := periodicTrimmedOracle(t, oracle, "write-"+example.name, before)
			raw, err := os.ReadFile(before)
			if err != nil {
				t.Fatal(err)
			}
			checkPeriodicPolygonAuthoredRecords(t, example.name, raw)
			f, err := ReadPeriodicConvexPolygonTrimmedFace(ctx, bytes.NewReader(raw), Limits{})
			if err != nil {
				t.Fatal(err)
			}
			var carrier bytes.Buffer
			if err := WritePeriodicConvexPolygonTrimmedFace(ctx, &carrier, f); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(after, carrier.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			source := periodicTrimmedOracle(t, oracle, "read-wires", before)
			reopened := periodicTrimmedOracle(t, oracle, "read-wires", after)
			edges := example.outer.count + example.hole.count
			wires := 1 + len(f.Holes)
			if !source.Valid || !native.Valid || !source.PeriodicU || source.PeriodicV ||
				source.Topology != [4]int{1, wires, edges, edges} || len(source.Wires) != wires ||
				math.Abs(source.Area/example.pinnedArea-1) > 1e-10 || !reflect.DeepEqual(source, reopened) {
				t.Fatal("pinned transfer changed topology, measures, owners, ranges or jets", source, reopened)
			}
			for i, station := range source.Bounds {
				checkPeriodicPolygonStation(t, station, native.Bounds[i])
			}
			checkPeriodicPolygonPinnedWire(t, ctx, f, reopened.Wires[0], f.Outer.Edges, true)
			for i, hole := range f.Holes {
				checkPeriodicPolygonPinnedWire(t, ctx, f, reopened.Wires[i+1], hole.Edges, false)
			}
			if len(native.Samples) != len(source.Samples) {
				t.Fatal("pinned source changed physical jet sample count")
			}
			for i, sample := range source.Samples {
				original := native.Samples[i]
				for axis, station := range sample.UV {
					checkPeriodicPolygonStation(t, station, original.UV[axis])
				}
				jet, err := f.Support.EvaluateRational(ctx, rat(original.UV[0]), rat(original.UV[1]), geom.SurfacePatchOptions{})
				if err != nil {
					t.Fatal(err)
				}
				for k, p := range original.Point {
					if jet.Point[k].Cmp(rat(p)) != 0 || jet.Du[k].Cmp(rat(original.Du[k])) != 0 || jet.Dv[k].Cmp(rat(original.Dv[k])) != 0 {
						t.Fatal("native exact support differs from the original physical jet", original)
					}
				}
				checkPeriodic128Vector(t, jet.Point, sample.Point)
				checkPeriodic128Vector(t, jet.Du, sample.Du)
				checkPeriodic128Vector(t, jet.Dv, sample.Dv)
			}
		})
	}
	t.Run("collinear control", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "collinear.iges")
		report := periodicTrimmedOracle(t, oracle, "write-exact-convex-polygon-collinear", path)
		source := periodicTrimmedOracle(t, oracle, "read-wires", path)
		if !report.Valid || !source.Valid || source.Topology != [4]int{1, 2, 8, 8} {
			t.Fatal("independent collinear trim failed transfer", source)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		checkPeriodicPolygonAuthoredRecords(t, "exact-convex-polygon-collinear", raw)
		f, err := ReadPeriodicConvexPolygonTrimmedFace(ctx, bytes.NewReader(raw), Limits{})
		if err == nil || f.Support != nil {
			t.Fatal("strict convex reader admitted a straight turn", err)
		}
	})
}

func checkPeriodicPolygonAuthoredRecords(t *testing.T, name string, raw []byte) {
	t.Helper()
	got, want := periodicTrimmedRecords(t, raw), periodicTrimmedRecords(t, periodicHoleFixture(t, name))
	if got.unit != want.unit || !reflect.DeepEqual(got.entities, want.entities) {
		t.Fatal("pinned author changed original polygon records")
	}
}

func checkPeriodicPolygonStation(t *testing.T, actual, expected float64) {
	t.Helper()
	const maxSteps = 4
	for i := 0; i < maxSteps && actual != expected; i++ {
		actual = math.Nextafter(actual, expected)
	}
	if actual != expected {
		t.Fatal("pinned curve evaluation exceeded four binary64 steps from original station", actual, expected)
	}
}

func checkPeriodicPolygonPinnedWire(t *testing.T, ctx context.Context, f PeriodicConvexPolygonTrimmedFace,
	wire periodicTrimOracleWire, pairs []PeriodicTrimEdge, outer bool) {
	t.Helper()
	// The author uses a 1/32 grid for trim stations and 1/64
	// for their midpoints. Check the pinned evaluator's binary64
	// drift before matching the directions and exact model clocks.
	wire.Edges = append([]periodicTrimOracleEdge(nil), wire.Edges...)
	for i := range wire.Edges {
		edge := &wire.Edges[i]
		for _, p := range []*[2]float64{&edge.UVStart, &edge.UVMid, &edge.UVEnd} {
			for axis, actual := range p {
				exact := math.Round(actual*64) / 64
				checkPeriodicPolygonStation(t, actual, exact)
				p[axis] = exact
			}
		}
	}
	checkPeriodicTrimOracleWire(t, ctx, f.Support, wire, pairs, outer, f.ProjectionAxes, f.ProjectionOrientation)
}
