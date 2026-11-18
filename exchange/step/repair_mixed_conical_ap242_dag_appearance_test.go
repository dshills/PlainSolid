package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/internal/slowtest"
	"reflect"
	"slices"
	"strings"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestRepairMixedConicalAP242DAGAppearance(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw := roundedMixedConicalDAGAppearanceFixture(t)
	l := MixedConicalAP242DAGLimits{MaxOccurrenceDepth: 3, MaxLeafInstances: 7}
	if m, err := ReadMixedConicalAP242DAGAppearance(ctx, bytes.NewReader(raw), l, geom.SurfacePatchOptions{}); err == nil ||
		!strings.Contains(err.Error(), "rolling-rim product 89") || m.OriginalSTEP() != "" {
		t.Fatal("strict appearance reader did not reject the original rounded surface", err)
	}
	m, err := RepairMixedConicalAP242DAGAppearance(ctx, bytes.NewReader(raw), l, MixedConicalAP242DAGRepairOptions{3e-12})
	if err != nil {
		t.Fatal(err)
	}
	a := m.Appearance()
	if m.OriginalSTEP() != string(raw) || a.OriginalSTEP() != m.CorrectedModelSTEP() || a.OriginalSTEP() == m.OriginalSTEP() ||
		len(a.DAG.Parts) != 2 || len(a.DAG.Occurrences) != 8 || len(a.DAG.Instances) != 7 || a.DAG.Assembly.LengthUnit != .001 {
		t.Fatal("lost original source or repaired assembly hierarchy")
	}
	wantProduct := []MixedConicalAP242SurfaceStyle{
		{DefinitionID: 89, BodyID: 95, StyledItemID: 505, PresentationID: 504, AssignmentID: 506, Color: RGB{R: 1}},
		{DefinitionID: 257, BodyID: 263, StyledItemID: 522, PresentationID: 521, AssignmentID: 523, Color: RGB{G: 1}},
	}
	wantFace := []MixedConicalAP242SurfaceStyle{{
		DefinitionID: 257, BodyID: 263, FaceID: 348, StyledItemID: 530,
		PresentationID: 521, AssignmentID: 531, NativeFaceIndex: 3, Color: RGB{R: 1, G: 1},
	}}
	wantOverride := DeepSHUOColor{PrefixSHUOID: 500, TerminalSHUOID: 499, Path: [3]int64{54, 85, 254}, Context: OccurrenceSurfaceColor{
		OccurrenceID: 254, BodyID: 95, StyledItemID: 513, BaseStyledItemID: 505, AssignmentID: 514, ContextRepresentationID: 503,
		DefinitionRepresentationID: 501, ProductShapeID: 502, CDSRID: 250, TransformationID: 252, PlacementID: 68, Color: RGB{B: 1},
	}}
	if !slices.Equal(a.ProductStyles, wantProduct) || !slices.Equal(a.FaceStyles, wantFace) || a.PathOverride != wantOverride ||
		a.DAG.Parts[257].Chamfered.NativeFaceSourceIDs[3] != 348 {
		t.Fatal("repair moved an original body, cap or contextual fill", a.ProductStyles, a.FaceStyles, a.PathOverride)
	}
	wantPaths := [][]int64{{54, 85, 248}, {54, 85, 254}, {54, 85, 464}, {487, 493, 248}, {487, 493, 254}, {487, 493, 464}, {498}}
	exact, err := ReadMixedConicalAP242DAGAppearance(ctx, bytes.NewReader(mixedDAGAppearanceFixture(t)), l, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for i, u := range a.DAG.Instances {
		if !slices.Equal(u.Path, wantPaths[i]) || !reflect.DeepEqual(u.Matrix(), exact.DAG.Instances[i].Matrix()) {
			t.Fatalf("changed exact world placement or original occurrence path %v", u.Path)
		}
	}
	parts := m.PartRepairs()
	if parts[0].DefinitionID != 89 || parts[0].RollingRim == nil || len(parts[0].GeneratedEntityIDs) != 2 ||
		parts[1].DefinitionID != 257 || len(parts[1].ChamferWalls) != 6 || len(parts[1].GeneratedEntityIDs) != 6 {
		t.Fatal("wrong independent source repair ledgers")
	}
	for i, native := range []trimmed.Solid{exact.DAG.Parts[89].Rounded.Solid, exact.DAG.Parts[254].Chamfered.Solid} {
		body, err := prismNativeBytes(native)
		if err != nil || rationalPosedRepairDigest(body) != parts[i].NativeSHA256 {
			t.Fatal("repair differs from the independent exact native geometry", err)
		}
	}
	checkRoundedMixedConicalStylesRetained(t, raw, a.DAG.Assembly.Source, parts)
	if oracle := os.Getenv("PLAINSOLID_OCCT_MIXED_CONICAL_AP242_APPEARANCE"); oracle != "" {
		local := map[int64]trimmed.Solid{89: a.DAG.Parts[89].Rounded.Solid, 257: a.DAG.Parts[257].Chamfered.Solid}
		for _, src := range [][]byte{raw, []byte(m.CorrectedModelSTEP())} {
			checkMixedDAGAppearanceOracle(t, oracle, src)
			checkMixedConicalAP242DAGOracle(t, oracle, src, a.DAG.Instances, local, 257)
		}
	}
}

func checkRoundedMixedConicalStylesRetained(t *testing.T, raw []byte, after *File, parts [2]NestedMixedConicalAP242PartRepair) {
	t.Helper()
	before, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	local := map[int64]bool{}
	added := map[int64]bool{}
	for _, p := range parts {
		geometry, err := nauoClosure(before, []int64{p.BodyID}, 16)
		if err != nil {
			t.Fatal(err)
		}
		for id := range geometry {
			local[id] = true
		}
		for _, id := range p.GeneratedEntityIDs {
			if id <= 537 || added[id] || after.Entities[id].ID != id {
				t.Fatalf("repair misallocated new support #%d", id)
			}
			added[id] = true
		}
	}
	for _, id := range before.Order {
		if !local[id] && !reflect.DeepEqual(before.Entities[id], after.Entities[id]) {
			t.Fatalf("changed original presentation or hierarchy record #%d", id)
		}
	}
}

func TestRepairMixedConicalAP242DAGAppearanceReject(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	raw := roundedMixedConicalDAGAppearanceFixture(t)
	check := func(input []byte, l MixedConicalAP242DAGLimits, budget float64, cause error, detail string) {
		t.Helper()
		m, err := RepairMixedConicalAP242DAGAppearance(ctx, bytes.NewReader(input), l, MixedConicalAP242DAGRepairOptions{budget})
		if err == nil || m.OriginalSTEP() != "" || m.CorrectedModelSTEP() != "" || m.Appearance().DAG.Assembly.Source != nil ||
			cause != nil && !errors.Is(err, cause) || !strings.Contains(err.Error(), detail) {
			t.Fatalf("failed to reject rounded styles atomically: %v", err)
		}
	}
	check(raw, MixedConicalAP242DAGLimits{}, 2.8e-12, nil, "finite cone angle displacement exceeds the source-unit limit")
	check(raw, MixedConicalAP242DAGLimits{MaxOccurrenceDepth: 2}, 3e-12, geom.ErrConvergence, "")
	check(raw, MixedConicalAP242DAGLimits{MaxLeafInstances: 6}, 3e-12, geom.ErrConvergence, "")
	check(raw, MixedConicalAP242DAGLimits{Source: Limits{MaxBytes: int64(len(raw) - 1)}}, 3e-12, geom.ErrConvergence, "")
	check(mixedConicalAP242DAGFixture(t, "rounded"), MixedConicalAP242DAGLimits{}, 3e-12, nil, "authenticate original AP242 styles")
	for _, edit := range [][2]string{
		{"#503 = SHAPE_REPRESENTATION('',(#68),#239);", "#503 = SHAPE_REPRESENTATION('',(#64),#239);"},
		{"#348,#522);", "#97,#522);"},
		{"#348,#522);", "#348,#505);"},
		{"#5,#58,$,#54,#85);", "#5,#58,$,#487,#85);"},
	} {
		check(replaceMixedDAGAppearance(t, raw, edit), MixedConicalAP242DAGLimits{}, 3e-12, nil, "")
	}
	if m, err := RepairMixedConicalAP242DAG(ctx, bytes.NewReader(raw), MixedConicalAP242DAGLimits{}, MixedConicalAP242DAGRepairOptions{3e-12}); err == nil || m.OriginalSTEP() != "" {
		t.Fatal("older unstyled repair silently admitted appearance")
	}
}

func TestRepairedMixedConicalAP242DAGAppearanceReceipt(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw := roundedMixedConicalDAGAppearanceFixture(t)
	m, err := RepairMixedConicalAP242DAGAppearance(ctx, bytes.NewReader(raw), MixedConicalAP242DAGLimits{}, MixedConicalAP242DAGRepairOptions{3e-12})
	if err != nil {
		t.Fatal(err)
	}
	var receipt, replay bytes.Buffer
	if err := WriteRepairedMixedConicalAP242DAGAppearanceJSON(ctx, &receipt, m); err != nil {
		t.Fatal(err)
	}
	got, err := ReadRepairedMixedConicalAP242DAGAppearanceJSON(ctx, bytes.NewReader(receipt.Bytes()), MixedConicalAP242DAGRepairJSONLimits{})
	if err != nil || got.OriginalSTEP() != string(raw) || got.CorrectedModelSTEP() != m.CorrectedModelSTEP() ||
		!reflect.DeepEqual(got.PartRepairs(), m.PartRepairs()) || got.Appearance().PathOverride != m.Appearance().PathOverride {
		t.Fatal("source receipt changed its proven geometry or appearance", err)
	}
	a := got.Appearance()
	a.ProductStyles[0].StyledItemID = 0
	a.DAG.Instances[0].Path[0] = 0
	delete(a.DAG.Assembly.Source.Entities, 500)
	fresh := got.Appearance()
	if fresh.ProductStyles[0].StyledItemID != 505 || fresh.DAG.Instances[0].Path[0] != 54 || fresh.DAG.Assembly.Source.Entities[500].ID != 500 {
		t.Fatal("callers can mutate retained appearance proof")
	}
	if err := WriteRepairedMixedConicalAP242DAGAppearanceJSON(ctx, &replay, got); err != nil || !bytes.Equal(receipt.Bytes(), replay.Bytes()) {
		t.Fatal("styled receipt did not replay byte-for-byte", err)
	}
	l := MixedConicalAP242DAGRepairJSONLimits{Source: MixedConicalAP242DAGLimits{MaxLeafInstances: 6}}
	if bad, err := ReadRepairedMixedConicalAP242DAGAppearanceJSON(ctx, bytes.NewReader(receipt.Bytes()), l); !errors.Is(err, geom.ErrConvergence) || bad.OriginalSTEP() != "" {
		t.Fatal("styled receipt bypassed expansion budget", err)
	}
	if bad, err := ReadRepairedMixedConicalAP242DAGJSON(ctx, bytes.NewReader(receipt.Bytes()), MixedConicalAP242DAGRepairJSONLimits{}); err == nil || bad.OriginalSTEP() != "" {
		t.Fatal("styled receipt fell through the original unstyled tag", err)
	}
	q, err := readRationalPosedPCurveRepairEnvelope(receipt.Bytes())
	if err != nil || !bytes.Equal(q.Source, raw) {
		t.Fatal("styled receipt lost the exact original producer bytes", err)
	}
	for _, mutate := range []func(rationalPosedPCurveRepairEnvelope) rationalPosedPCurveRepairEnvelope{
		func(q rationalPosedPCurveRepairEnvelope) rationalPosedPCurveRepairEnvelope {
			q.Claims = replaceMixedDAGAppearance(t, q.Claims, [2]string{`"PrefixSHUOID":500`, `"PrefixSHUOID":499`})
			return q
		},
		func(q rationalPosedPCurveRepairEnvelope) rationalPosedPCurveRepairEnvelope {
			q.Source = replaceMixedDAGAppearance(t, q.Source, [2]string{"DRAUGHTING_PRE_DEFINED_COLOUR('blue')", "DRAUGHTING_PRE_DEFINED_COLOUR('cyan')"})
			return q
		},
	} {
		bad, err := json.Marshal(mutate(q))
		if err != nil {
			t.Fatal(err)
		}
		if p, err := ReadRepairedMixedConicalAP242DAGAppearanceJSON(ctx, bytes.NewReader(bad), MixedConicalAP242DAGRepairJSONLimits{}); !errors.Is(err, geom.ErrInvalid) || p.OriginalSTEP() != "" {
			t.Fatal("receipt trusted changed source or stored appearance owner", err)
		}
	}
}

func TestRepairMixedConicalAP242DAGAppearancePinnedAuthor(t *testing.T) {
	t.Parallel()
	oracle := os.Getenv("PLAINSOLID_OCCT_MIXED_CONICAL_AP242_ROUNDED_APPEARANCE")
	if oracle == "" {
		t.Skip("pinned XCAF rounded appearance author not configured")
	}
	path := filepath.Join(t.TempDir(), "rounded-appearance.step")
	if out, err := exec.Command(oracle, "write-dag-rounded-appearance", path).CombinedOutput(); err != nil {
		t.Fatalf("pinned author: %v\n%s", err, out)
	}
	fresh, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Fresh exports may exchange the two independent presentation records.
	// Non-style geometry and original SHUO anchors must reproduce exactly.
	sourcePrefix := func(raw []byte) []byte {
		start := bytes.Index(raw, []byte("\nDATA;\n"))
		stop := bytes.Index(raw, []byte("\n#504 = MECHANICAL_DESIGN_GEOMETRIC_PRESENTATION_REPRESENTATION"))
		if start < 0 || stop <= start {
			return nil
		}
		return raw[start:stop]
	}
	reference := sourcePrefix(roundedMixedConicalDAGAppearanceFixture(t))
	if len(reference) == 0 || !bytes.Equal(reference, sourcePrefix(fresh)) {
		t.Fatal("independent rounded source changed its original geometry or usage anchors")
	}
	m, err := RepairMixedConicalAP242DAGAppearance(context.Background(), bytes.NewReader(fresh),
		MixedConicalAP242DAGLimits{}, MixedConicalAP242DAGRepairOptions{3e-12})
	if err != nil {
		t.Fatal(err)
	}
	a := m.Appearance()
	colors := map[int64]RGB{}
	for _, paint := range a.ProductStyles {
		colors[paint.BodyID] = paint.Color
	}
	if !reflect.DeepEqual(colors, map[int64]RGB{95: {R: 1}, 263: {G: 1}}) ||
		len(a.FaceStyles) != 1 || a.FaceStyles[0].FaceID != 348 || a.FaceStyles[0].Color != (RGB{R: 1, G: 1}) ||
		a.PathOverride.Path != [3]int64{54, 85, 254} || a.PathOverride.Context.Color != (RGB{B: 1}) {
		t.Fatal("independent rounded source changed original face or paint owners")
	}
	checkMixedDAGAppearanceOracle(t, oracle, []byte(m.CorrectedModelSTEP()))
}

func roundedMixedConicalDAGAppearanceFixture(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/occt-dag-mixed-conical-ap242-rounded-appearance.step")
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
