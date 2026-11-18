package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/big"
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

var roundedConicalFrameOptions = MixedConicalAP242FrameRepairOptions{OccurrenceID: 251, MaxAxisChord: 1e-12, MaxDisplacement: 1e-10}

func TestRepairMixedConicalAP242Frame(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw := roundedConicalFrameFixture(t)
	l := MixedConicalAP242DAGLimits{MaxOccurrenceDepth: 3, MaxLeafInstances: 7}
	if a, err := ReadMixedConicalAP242DAG(ctx, bytes.NewReader(raw), l, geom.SurfacePatchOptions{}); err == nil || a.Assembly.Source != nil {
		t.Fatal("strict reader silently corrected the independently rounded axes", err)
	}
	m, err := RepairMixedConicalAP242Frame(ctx, bytes.NewReader(raw), l, roundedConicalFrameOptions)
	if err != nil {
		t.Fatal(err)
	}
	a := m.Assembly()
	e := m.Evidence()
	if m.OriginalSTEP() != string(raw) || a.Assembly.LengthUnit != .001 || len(a.Parts) != 2 || len(a.Instances) != 7 ||
		e.OccurrenceID != 251 || e.CDSRID != 247 || e.RelationshipID != 248 || e.TransformationID != 249 ||
		e.SourcePlacementID != 11 || e.TargetPlacementID != 68 || e.OriginID != 69 || e.BodyID != 95 ||
		e.DirectionIDs != [2]int64{71, 70} || e.PrimitiveDirections != [2][3]int64{{-15, 10, 6}, {10, 6, 15}} {
		t.Fatal("lost original relationship identity, axes, origin or retained source")
	}
	upper := func(text string) *big.Rat {
		t.Helper()
		r, ok := new(big.Rat).SetString(text)
		if !ok {
			t.Fatal("not an exact bound:", text)
		}
		return r
	}
	dx, dz := upper(e.AxisChordUpper[0]), upper(e.AxisChordUpper[1])
	frame, extent, distance := upper(e.FrameChordUpper), upper(e.LocalExtent), upper(e.DisplacementUpper)
	if dx.Cmp(big.NewRat(3, 10000000000000)) <= 0 || dz.Cmp(dx) != 0 ||
		frame.Cmp(new(big.Rat).Mul(dx, big.NewRat(10, 1))) != 0 || extent.Cmp(big.NewRat(57, 4)) != 0 ||
		distance.Cmp(new(big.Rat).Mul(frame, extent)) != 0 || distance.Cmp(big.NewRat(1, 10000000000)) >= 0 {
		t.Fatal("incorrect finite axis, frame or material displacement enclosure", e)
	}
	before, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	after := a.Assembly.Source
	if !slices.Equal(before.Order, after.Order) {
		t.Fatal("changed source entity identities")
	}
	for _, id := range before.Order {
		changed := !reflect.DeepEqual(before.Entities[id], after.Entities[id])
		if changed != (id == 70 || id == 71) {
			t.Fatalf("unexpected correction footprint at original #%d", id)
		}
	}
	for i, want := range map[int][3][4]string{
		1: {{"78/95", "54/95", "1/19", "649/5"}, {"-21/95", "22/95", "18/19", "82/5"}, {"10/19", "-15/19", "6/19", "30"}},
		4: {{"-21/95", "22/95", "18/19", "-598/5"}, {"-162/475", "-441/475", "14/95", "-731/25"}, {"434/475", "-138/475", "27/95", "1892/25"}},
	} {
		if got := a.Instances[i]; !reflect.DeepEqual(got.Matrix(), conicalRotationMatrix(t, want)) {
			t.Fatal("wrong exact world frame on original shared occurrence", got.Path, got.Matrix())
		}
	}
	if !slices.Equal(a.Instances[1].Path, []int64{54, 85, 251}) || !slices.Equal(a.Instances[4].Path, []int64{466, 472, 251}) {
		t.Fatal("changed either original expanded relationship path")
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_MIXED_CONICAL_AP242_FRAME_REPAIR"); oracle != "" {
		local := map[int64]trimmed.Solid{89: a.Parts[89].Rounded.Solid, 254: a.Parts[254].Chamfered.Solid}
		for _, source := range [][]byte{raw, []byte(m.CorrectedModelSTEP())} {
			checkMixedConicalAP242DAGOracle(t, oracle, source, a.Instances, local, 254)
		}
	}
}

func TestRepairMixedConicalAP242FrameReject(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw := roundedConicalFrameFixture(t)
	reject := func(source []byte, options MixedConicalAP242FrameRepairOptions, limits MixedConicalAP242DAGLimits, cause error) {
		t.Helper()
		got, err := RepairMixedConicalAP242Frame(ctx, bytes.NewReader(source), limits, options)
		if err == nil || cause != nil && !errors.Is(err, cause) ||
			got.OriginalSTEP() != "" || got.CorrectedModelSTEP() != "" || got.Assembly().Assembly.Source != nil {
			t.Fatal("repair exposed partially corrected data on refusal", err)
		}
	}
	l := MixedConicalAP242DAGLimits{}
	o := roundedConicalFrameOptions
	o.MaxAxisChord = 3e-13
	reject(raw, o, l, geom.ErrConvergence)
	o = roundedConicalFrameOptions
	o.MaxDisplacement = 1e-11
	reject(raw, o, l, geom.ErrConvergence)
	o = roundedConicalFrameOptions
	o.MaxAxisChord = .001
	reject(raw, o, l, errAmbiguousConicalFrameRatio)
	o = roundedConicalFrameOptions
	o.MaxPrimitiveComponent = 14
	reject(raw, o, l, geom.ErrUnrepresentable)
	o = roundedConicalFrameOptions
	o.OccurrenceID = 54 // This relationship has descendants.
	reject(raw, o, l, geom.ErrUnrepresentable)
	reject(raw, roundedConicalFrameOptions, MixedConicalAP242DAGLimits{MaxLeafInstances: 6}, geom.ErrConvergence)
	shared := replaceMixedDAGAppearance(t, raw, [2]string{"#64 = AXIS2_PLACEMENT_3D('',#65,#66,#67);", "#64 = AXIS2_PLACEMENT_3D('',#65,#66,#71);"})
	if _, err := ReadAssembly(bytes.NewReader(shared), Limits{}); err != nil {
		t.Fatal("shared-owner control failed the general structural reader", err)
	}
	reject(shared, roundedConicalFrameOptions, l, geom.ErrUnrepresentable)
	exact := replaceMixedDAGAppearance(t, raw, [2]string{
		"#70 = DIRECTION('',(0.526315789474,0.315789473684,0.789473684211));", "#70 = DIRECTION('',(10.,6.,15.));",
	})
	exact = replaceMixedDAGAppearance(t, exact, [2]string{
		"#71 = DIRECTION('',(-0.789473684211,0.526315789474,0.315789473684));", "#71 = DIRECTION('',(-15.,10.,6.));",
	})
	if _, err := ReadMixedConicalAP242DAG(ctx, bytes.NewReader(exact), l, geom.SurfacePatchOptions{}); err != nil {
		t.Fatal("exact-frame negative did not qualify strict import", err)
	}
	reject(exact, roundedConicalFrameOptions, l, geom.ErrUnrepresentable)
}

func TestRepairedMixedConicalAP242FrameReceipt(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw := roundedConicalFrameFixture(t)
	m, err := RepairMixedConicalAP242Frame(ctx, bytes.NewReader(raw), MixedConicalAP242DAGLimits{}, roundedConicalFrameOptions)
	if err != nil {
		t.Fatal(err)
	}
	var first, second bytes.Buffer
	if err := WriteRepairedMixedConicalAP242FrameJSON(ctx, &first, m); err != nil {
		t.Fatal(err)
	}
	replayed, err := ReadRepairedMixedConicalAP242FrameJSON(ctx, bytes.NewReader(first.Bytes()), MixedConicalAP242DAGRepairJSONLimits{})
	if err != nil || replayed.OriginalSTEP() != string(raw) || replayed.CorrectedModelSTEP() != m.CorrectedModelSTEP() || replayed.Evidence() != m.Evidence() {
		t.Fatal("receipt changed the source, scope or reconstructed frame", err)
	}
	if err := WriteRepairedMixedConicalAP242FrameJSON(ctx, &second, replayed); err != nil || !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("frame repair receipt failed byte-identical replay", err)
	}
	q, err := readRationalPosedPCurveRepairEnvelope(first.Bytes())
	if err != nil || !bytes.Equal(q.Source, raw) {
		t.Fatal("receipt did not retain the raw producer file", err)
	}
	for _, change := range []func(rationalPosedPCurveRepairEnvelope) rationalPosedPCurveRepairEnvelope{
		func(q rationalPosedPCurveRepairEnvelope) rationalPosedPCurveRepairEnvelope {
			q.Claims = replaceMixedDAGAppearance(t, q.Claims, [2]string{`"DirectionIDs":[71,70]`, `"DirectionIDs":[70,71]`})
			return q
		},
		func(q rationalPosedPCurveRepairEnvelope) rationalPosedPCurveRepairEnvelope {
			q.Source = append(slices.Clone(q.Source), '\n')
			return q
		},
	} {
		data, err := json.Marshal(change(q))
		if err != nil {
			t.Fatal(err)
		}
		if out, err := ReadRepairedMixedConicalAP242FrameJSON(ctx, bytes.NewReader(data), MixedConicalAP242DAGRepairJSONLimits{}); !errors.Is(err, geom.ErrInvalid) || out.OriginalSTEP() != "" {
			t.Fatal("receipt trusted a changed original file or owner claim", err)
		}
	}
	short := MixedConicalAP242DAGRepairJSONLimits{MaxBytes: int64(first.Len() - 1)}
	if out, err := ReadRepairedMixedConicalAP242FrameJSON(ctx, bytes.NewReader(first.Bytes()), short); !errors.Is(err, geom.ErrConvergence) || out.OriginalSTEP() != "" {
		t.Fatal("receipt bypassed byte budget", err)
	}
	if out, err := ReadRepairedMixedConicalAP242DAGJSON(ctx, bytes.NewReader(first.Bytes()), MixedConicalAP242DAGRepairJSONLimits{}); err == nil || out.OriginalSTEP() != "" {
		t.Fatal("the existing geometry repair receipt reader accepted the frame tag", err)
	}
}

func TestRepairMixedConicalAP242FramePinnedAuthor(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_MIXED_CONICAL_AP242_FRAME_REPAIR")
	if oracle == "" {
		t.Skip("pinned rounded occurrence author not configured")
	}
	file := filepath.Join(t.TempDir(), "rounded-occurrence.step")
	if data, err := exec.Command(oracle, "write-dag-rounded-frame", file).CombinedOutput(); err != nil {
		t.Fatalf("independent rounded frame author: %v\n%s", err, data)
	}
	fresh, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	dataset := func(source []byte) string {
		_, data, _ := strings.Cut(string(source), "\nDATA;\n")
		return data
	}
	if data := dataset(fresh); data == "" || data != dataset(roundedConicalFrameFixture(t)) {
		t.Fatal("the pinned writer changed its original rounded axes or assembly data")
	}
	m, err := RepairMixedConicalAP242Frame(context.Background(), bytes.NewReader(fresh), MixedConicalAP242DAGLimits{}, roundedConicalFrameOptions)
	if err != nil || m.Evidence().PrimitiveDirections != [2][3]int64{{-15, 10, 6}, {10, 6, 15}} {
		t.Fatal("fresh pinned rounded source did not reproduce its proved frame", err)
	}
}

func roundedConicalFrameFixture(t *testing.T) []byte {
	t.Helper()
	p, err := os.ReadFile("testdata/occt-dag-mixed-conical-ap242-rounded-spatial-frame.step")
	if err != nil {
		t.Fatal(err)
	}
	return p
}
