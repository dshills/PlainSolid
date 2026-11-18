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
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

const roundedConicalFrameAppearanceOracle = "PLAINSOLID_OCCT_MIXED_CONICAL_AP242_FRAME_APPEARANCE_REPAIR"

var roundedConicalFrameAppearanceOptions = MixedConicalAP242DAGFrameAppearanceRepairOptions{
	Frame:    MixedConicalAP242FrameRepairOptions{OccurrenceID: 254, MaxAxisChord: 1e-12, MaxDisplacement: 1e-10},
	Geometry: MixedConicalAP242DAGRepairOptions{MaxDistance: 3e-12},
}

func TestRepairMixedConicalAP242DAGFrameAppearance(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw := roundedConicalFrameAppearanceFixture(t)
	l := MixedConicalAP242DAGLimits{MaxOccurrenceDepth: 3, MaxLeafInstances: 7}
	o := roundedConicalFrameAppearanceOptions
	m, err := RepairMixedConicalAP242DAGFrameAppearance(ctx, bytes.NewReader(raw), l, o)
	if err != nil {
		t.Fatal(err)
	}
	a, e := m.Appearance(), m.FrameEvidence()
	if m.OriginalSTEP() != string(raw) || a.OriginalSTEP() != m.CorrectedModelSTEP() ||
		len(a.DAG.Parts) != 2 || len(a.DAG.Occurrences) != 8 || len(a.DAG.Instances) != 7 || a.DAG.Assembly.LengthUnit != .001 ||
		e.OccurrenceID != 254 || e.ParentID != 58 || e.ChildID != 89 || e.BodyID != 95 ||
		e.CDSRID != 250 || e.RelationshipID != 251 || e.TransformationID != 252 ||
		e.SourcePlacementID != 11 || e.TargetPlacementID != 68 || e.OriginID != 69 || e.DirectionIDs != [2]int64{71, 70} ||
		e.PrimitiveDirections != [2][3]int64{{-15, 10, 6}, {10, 6, 15}} || e.LocalExtent != "57/4" {
		t.Fatal("changed original source, repair scope or hierarchy", e)
	}
	wantProducts := []MixedConicalAP242SurfaceStyle{
		{DefinitionID: 257, BodyID: 263, StyledItemID: 505, PresentationID: 504, AssignmentID: 506, Color: RGB{G: 1}},
		{DefinitionID: 89, BodyID: 95, StyledItemID: 522, PresentationID: 521, AssignmentID: 523, Color: RGB{R: 1}},
	}
	wantFace := MixedConicalAP242SurfaceStyle{DefinitionID: 257, BodyID: 263, FaceID: 348, StyledItemID: 513,
		PresentationID: 504, AssignmentID: 514, NativeFaceIndex: 3, Color: RGB{R: 1, G: 1}}
	wantOverride := DeepSHUOColor{PrefixSHUOID: 500, TerminalSHUOID: 499, Path: [3]int64{54, 85, 254}, Context: OccurrenceSurfaceColor{
		OccurrenceID: 254, BodyID: 95, StyledItemID: 530, BaseStyledItemID: 522, AssignmentID: 531, ContextRepresentationID: 503,
		DefinitionRepresentationID: 501, ProductShapeID: 502, CDSRID: 250, TransformationID: 252, PlacementID: 68, Color: RGB{B: 1},
	}}
	if !slices.Equal(a.ProductStyles, wantProducts) || !slices.Equal(a.FaceStyles, []MixedConicalAP242SurfaceStyle{wantFace}) ||
		a.PathOverride != wantOverride || a.DAG.Parts[257].Chamfered.NativeFaceSourceIDs[3] != 348 {
		t.Fatal("changed original product, cap or occurrence paint ownership", a.ProductStyles, a.FaceStyles, a.PathOverride)
	}
	for i, want := range map[int]struct {
		path []int64
		pose [3][4]string
	}{
		1: {[]int64{54, 85, 254}, [3][4]string{{"-15/19", "-6/19", "10/19", "82"}, {"10/19", "-15/19", "6/19", "48"}, {"6/19", "10/19", "15/19", "9"}}},
		4: {[]int64{487, 493, 254}, [3][4]string{{"-21/95", "22/95", "18/19", "-598/5"}, {"-162/475", "-441/475", "14/95", "-731/25"}, {"434/475", "-138/475", "27/95", "1892/25"}}},
	} {
		got := a.DAG.Instances[i]
		if got.ProductID != 89 || !slices.Equal(got.Path, want.path) || !reflect.DeepEqual(got.Matrix(), conicalRotationMatrix(t, want.pose)) {
			t.Fatal("wrong exact composed world pose", got.Path, got.Matrix())
		}
	}
	before, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	staged, err := Parse(bytes.NewBufferString(m.geometry.OriginalSTEP()), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(before.Order, staged.Order) || !reflect.DeepEqual(before.Header, staged.Header) {
		t.Fatal("frame stage changed original entity ordering or header")
	}
	for _, id := range before.Order {
		changed := !reflect.DeepEqual(before.Entities[id], staged.Entities[id])
		if changed != (id == 70 || id == 71) {
			t.Fatalf("unexpected frame correction at original #%d", id)
		}
	}
	parts := m.PartRepairs()
	if parts[0].DefinitionID != 89 || parts[0].RollingRim == nil || len(parts[0].GeneratedEntityIDs) != 2 ||
		parts[1].DefinitionID != 257 || len(parts[1].ChamferWalls) != 6 || len(parts[1].GeneratedEntityIDs) != 6 {
		t.Fatal("lost independently proved original material corrections")
	}
	checkRoundedMixedConicalStylesRetained(t, []byte(m.geometry.OriginalSTEP()), a.DAG.Assembly.Source, parts)
	if oracle := os.Getenv(roundedConicalFrameAppearanceOracle); oracle != "" {
		local := map[int64]trimmed.Solid{89: a.DAG.Parts[89].Rounded.Solid, 257: a.DAG.Parts[257].Chamfered.Solid}
		for _, source := range [][]byte{raw, []byte(m.CorrectedModelSTEP())} {
			checkMixedDAGAppearanceOracle(t, oracle, source)
			checkMixedConicalAP242DAGOracle(t, oracle, source, a.DAG.Instances, local, 257)
		}
	}
}

func TestRepairMixedConicalAP242DAGFrameAppearanceReject(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw := roundedConicalFrameAppearanceFixture(t)
	l := MixedConicalAP242DAGLimits{}
	o := roundedConicalFrameAppearanceOptions
	if a, err := ReadMixedConicalAP242DAGAppearance(ctx, bytes.NewReader(raw), l, geom.SurfacePatchOptions{}); err == nil || a.OriginalSTEP() != "" {
		t.Fatal("strict reader silently repaired combined original", err)
	}
	if m, err := RepairMixedConicalAP242DAGAppearance(ctx, bytes.NewReader(raw), l, o.Geometry); err == nil || m.OriginalSTEP() != "" {
		t.Fatal("geometry-only mode silently repaired original frame", err)
	}
	if m, err := RepairMixedConicalAP242Frame(ctx, bytes.NewReader(raw), l, o.Frame); err == nil || m.OriginalSTEP() != "" {
		t.Fatal("unstyled frame mode silently repaired styled geometry", err)
	}

	reject := func(src []byte, l MixedConicalAP242DAGLimits, o MixedConicalAP242DAGFrameAppearanceRepairOptions, cause error) {
		t.Helper()
		m, err := RepairMixedConicalAP242DAGFrameAppearance(ctx, bytes.NewReader(src), l, o)
		if err == nil || cause != nil && !errors.Is(err, cause) || m.OriginalSTEP() != "" ||
			m.CorrectedModelSTEP() != "" || m.Appearance().DAG.Assembly.Source != nil ||
			m.FrameEvidence() != (MixedConicalAP242FrameRepairEvidence{}) || m.PartRepairs()[0].DefinitionID != 0 {
			t.Fatal("repair exposed a partially corrected result on refusal", err)
		}
	}
	tight := o
	tight.Frame.MaxAxisChord = 3e-13
	reject(raw, l, tight, geom.ErrConvergence)
	tight = o
	tight.Frame.MaxDisplacement = 1e-11
	reject(raw, l, tight, geom.ErrConvergence)
	tight = o
	tight.Geometry.MaxDistance = 2.8e-12
	reject(raw, l, tight, nil)
	tight = o
	tight.Frame.MaxAxisChord = .001
	reject(raw, l, tight, errAmbiguousConicalFrameRatio)
	wrong := o
	wrong.Frame.OccurrenceID = 248
	reject(raw, l, wrong, geom.ErrUnrepresentable)
	reject(raw, MixedConicalAP242DAGLimits{MaxLeafInstances: 6}, o, geom.ErrConvergence)
	reject(roundedMixedConicalDAGAppearanceFixture(t), l, o, geom.ErrUnrepresentable)
	reject(roundedConicalFrameFixture(t), l, o, nil)
	for _, edit := range [][2]string{
		{"#503 = SHAPE_REPRESENTATION('',(#68),#239);", "#503 = SHAPE_REPRESENTATION('',(#64),#239);"},
		{"#64 = AXIS2_PLACEMENT_3D('',#65,#66,#67);", "#64 = AXIS2_PLACEMENT_3D('',#65,#66,#71);"},
		{"#348,#505);", "#97,#505);"},
	} {
		bad := replaceMixedDAGAppearance(t, raw, edit)
		reject(bad, l, o, nil)
	}
}

func TestRepairedMixedConicalAP242DAGFrameAppearanceReceipt(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw := roundedConicalFrameAppearanceFixture(t)
	l := MixedConicalAP242DAGRepairJSONLimits{}
	m, err := RepairMixedConicalAP242DAGFrameAppearance(ctx, bytes.NewReader(raw), l.Source, roundedConicalFrameAppearanceOptions)
	if err != nil {
		t.Fatal(err)
	}
	var receipt, replay bytes.Buffer
	if err := WriteRepairedMixedConicalAP242DAGFrameAppearanceJSON(ctx, &receipt, m); err != nil {
		t.Fatal(err)
	}
	got, err := ReadRepairedMixedConicalAP242DAGFrameAppearanceJSON(ctx, bytes.NewReader(receipt.Bytes()), l)
	if err != nil || got.OriginalSTEP() != string(raw) || got.CorrectedModelSTEP() != m.CorrectedModelSTEP() ||
		got.FrameEvidence() != m.FrameEvidence() || !reflect.DeepEqual(got.PartRepairs(), m.PartRepairs()) ||
		got.Appearance().PathOverride != m.Appearance().PathOverride {
		t.Fatal("receipt failed to reconstruct both proofs and original styles", err)
	}
	if err := WriteRepairedMixedConicalAP242DAGFrameAppearanceJSON(ctx, &replay, got); err != nil || !bytes.Equal(receipt.Bytes(), replay.Bytes()) {
		t.Fatal("composed receipt replay changed bytes", err)
	}
	q, err := readRationalPosedPCurveRepairEnvelope(receipt.Bytes())
	if err != nil || !bytes.Equal(q.Source, raw) {
		t.Fatal("lost original bytes before either correction", err)
	}
	reject := func(data []byte, l MixedConicalAP242DAGRepairJSONLimits, cause error) {
		t.Helper()
		got, err := ReadRepairedMixedConicalAP242DAGFrameAppearanceJSON(ctx, bytes.NewReader(data), l)
		if !errors.Is(err, cause) || got.OriginalSTEP() != "" {
			t.Fatal("receipt accepted forged source, owner or budget", err)
		}
	}
	reject(receipt.Bytes(), MixedConicalAP242DAGRepairJSONLimits{MaxBytes: int64(receipt.Len() - 1)}, geom.ErrConvergence)
	reject(receipt.Bytes(), MixedConicalAP242DAGRepairJSONLimits{Source: MixedConicalAP242DAGLimits{MaxOccurrenceDepth: 2}}, geom.ErrConvergence)
	for _, edit := range [][2]string{
		{`"DirectionIDs":[71,70]`, `"DirectionIDs":[70,71]`},
		{`"PrefixSHUOID":500`, `"PrefixSHUOID":499`},
		{`"FaceID":348`, `"FaceID":97`},
		{rationalPosedRepairDigest([]byte(m.geometry.OriginalSTEP())), rationalPosedRepairDigest(raw)},
	} {
		mutation := q
		mutation.Claims = replaceMixedDAGAppearance(t, mutation.Claims, edit)
		data, err := json.Marshal(mutation)
		if err != nil {
			t.Fatal(err)
		}
		reject(data, l, geom.ErrInvalid)
	}
	q.Source = append(slices.Clone(q.Source), '\n')
	data, err := json.Marshal(q)
	if err != nil {
		t.Fatal(err)
	}
	reject(data, l, geom.ErrInvalid)
}

func TestRepairMixedConicalAP242DAGFrameAppearancePinnedAuthor(t *testing.T) {
	oracle := os.Getenv(roundedConicalFrameAppearanceOracle)
	if oracle == "" {
		t.Skip("pinned combined rounded AP242 author unavailable")
	}
	path := filepath.Join(t.TempDir(), "rounded-frame-appearance.step")
	if out, err := exec.Command(oracle, "write-dag-rounded-frame-appearance", path).CombinedOutput(); err != nil {
		t.Fatalf("pinned author: %v\n%s", err, out)
	}
	fresh, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// XCAF may number the two private presentation graphs in either order.
	// All common geometry, hierarchy, and SHUO owner records must match.
	prefix := func(raw []byte) []byte {
		start := bytes.Index(raw, []byte("\nDATA;\n"))
		stop := bytes.Index(raw, []byte("\n#504 = MECHANICAL_DESIGN_GEOMETRIC_PRESENTATION_REPRESENTATION"))
		if start < 0 || stop <= start {
			return nil
		}
		return raw[start:stop]
	}
	if got := prefix(fresh); got == nil || !bytes.Equal(got, prefix(roundedConicalFrameAppearanceFixture(t))) {
		t.Fatal("pinned writer changed original geometry, frames or SHUO placement ownership")
	}
	m, err := RepairMixedConicalAP242DAGFrameAppearance(context.Background(), bytes.NewReader(fresh),
		MixedConicalAP242DAGLimits{}, roundedConicalFrameAppearanceOptions)
	if err != nil {
		t.Fatal(err)
	}
	checkMixedDAGAppearanceOracle(t, oracle, []byte(m.CorrectedModelSTEP()))
}

func roundedConicalFrameAppearanceFixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/occt-dag-mixed-conical-ap242-rounded-frame-appearance.step")
	if err != nil {
		t.Fatal(err)
	}
	return b
}
