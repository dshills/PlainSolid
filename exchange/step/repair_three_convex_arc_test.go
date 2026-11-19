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
	"reflect"
	"slices"
	"testing"

	"plainsolid/geom"
)

const threeConvexArcSource = "testdata/occt-three-default-tilted-oblique-prism-fillet.step"
const threeConvexArcUnrepresentableSource = "testdata/occt-three-default-tilted-oblique-prism-fillet-nondyadic.step"
const threeConvexArcDistance = 1e-12

func checkThreeConvexArcSource(t *testing.T, raw []byte) (RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion, []byte) {
	t.Helper()
	ctx := context.Background()
	if _, err := ReadOCCTExactCircularLoopExtrusion(ctx, bytes.NewReader(raw), Limits{}); err == nil {
		t.Fatal("strict import accepted the unrepaired three-fillet pcurve graph")
	}
	m, err := RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{MaxDistance: threeConvexArcDistance})
	if err != nil {
		t.Fatal(err)
	}
	d := m.LocalDescriptor()
	if m.OriginalSTEP() != string(raw) || d.LengthUnit != .001 || d.Height != 5 || len(d.Segments) != 6 ||
		len(m.Solid().Faces()) != 8 || len(m.PCurves()) != 34 || len(m.RationalCaps()) != 2 || len(m.FiniteTrims()) != 2 ||
		len(m.Repairs()) != 10 || len(m.Tangencies()) != 12 || len(m.Reversals()) != 2 || len(m.AxialTrims()) != 2 ||
		len(m.FiniteLineOwners()) != 8 || len(m.CylindricalSpanOwners()) != 0 {
		t.Fatal("three-fillet repair lost source material or owner evidence")
	}
	var radii []float64
	for _, segment := range d.Segments {
		if segment.Radius != 0 {
			radii = append(radii, segment.Radius)
		}
	}
	slices.Sort(radii)
	if !slices.Equal(radii, []float64{5. / 32, 1. / 4, 5. / 16}) {
		t.Fatal("three independent circular supports changed", radii)
	}
	pose, err := m.Pose().Matrix()
	if err != nil {
		t.Fatal(err)
	}
	wantPose := [3][4]string{{"0", "3/5", "4/5", "0"}, {"-1", "0", "0", "0"}, {"0", "-4/5", "3/5", "0"}}
	for i := range pose {
		for j := range pose[i] {
			if pose[i][j].RatString() != wantPose[i][j] {
				t.Fatal("three-fillet repair changed the exact source pose", pose)
			}
		}
	}

	f, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	wantHeight := map[int64]string{210: "0", 425: "5"}
	residualLimit, _ := new(big.Rat).SetString("1e-25")
	for _, owner := range m.PCurves() {
		if owner.Kind != "phase-cylinder-circle" {
			continue
		}
		height, ok := wantHeight[owner.PCurveID]
		uv, speed, err := convexUVLine(f, owner.GeometryID, 2)
		if !ok || err != nil || uv[0].RatString() != "-2498091544797/1000000000000" || uv[1].RatString() != height ||
			speed[0].RatString() != "1" || speed[1].Sign() != 0 ||
			owner.DistanceSquaredUpper.Sign() <= 0 || owner.DistanceSquaredUpper.Cmp(residualLimit) > 0 {
			t.Fatal("shifted cylinder owner lost its original law or lift bound", owner.PCurveID, err)
		}
		delete(wantHeight, owner.PCurveID)
	}
	if len(wantHeight) != 0 {
		t.Fatal("missing a shifted cylinder owner", wantHeight)
	}

	canonical := encodeRationalWorldCircularLoop(t, m, false)
	strict, err := ReadRationalPosedExactCircularLoopExtrusion(ctx, bytes.NewReader(canonical), Limits{})
	if err != nil || strict.Descriptor.LengthUnit != d.LengthUnit || len(strict.Descriptor.Segments) != len(d.Segments) {
		t.Fatal("strict canonical consumer lost the three-filleted profile", err)
	}
	strictPose, err := strict.Pose.Matrix()
	if err != nil || !reflect.DeepEqual(strictPose, pose) {
		t.Fatal("strict canonical reopen changed world pose", strictPose, err)
	}
	native, err := prismNativeBytes(m.Solid())
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := prismNativeBytes(strict.Native)
	if err != nil || !bytes.Equal(native, reopened) {
		t.Fatal("strict canonical reopen changed native material", err)
	}
	var again bytes.Buffer
	if err := WriteRationalPosedExactCircularLoopExtrusion(ctx, &again, strict.Descriptor, strict.Pose); err != nil || !bytes.Equal(canonical, again.Bytes()) {
		t.Fatal("strict canonical re-export changed the STEP bytes", err)
	}
	return m, canonical
}

func TestRepairThreeConvexArcSource(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	raw, err := os.ReadFile(threeConvexArcSource)
	if err != nil {
		t.Fatal(err)
	}
	m, _ := checkThreeConvexArcSource(t, raw)
	if rejected, err := RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{MaxDistance: 1e-13}); err == nil || rejected.OriginalSTEP() != "" {
		t.Fatal("three-fillet repair accepted an insufficient source distance")
	}
	var receipt bytes.Buffer
	if err := WriteRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, &receipt, m); err != nil {
		t.Fatal(err)
	}
	reloaded, err := ReadRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, bytes.NewReader(receipt.Bytes()), IndependentMaterialJSONLimits{})
	if err != nil || reloaded.OriginalSTEP() != string(raw) || !reflect.DeepEqual(reloaded.PCurves(), m.PCurves()) ||
		!reflect.DeepEqual(reloaded.AxialTrims(), m.AxialTrims()) {
		t.Fatal("receipt reproof lost original owner evidence", err)
	}
	var again bytes.Buffer
	if err := WriteRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, &again, reloaded); err != nil || !bytes.Equal(receipt.Bytes(), again.Bytes()) {
		t.Fatal("three-fillet receipt bytes changed on reproof", err)
	}
	forged := bytes.Replace(receipt.Bytes(), []byte(`"Kind":"phase-cylinder-circle"`), []byte(`"Kind":"cylinder-circle"`), 1)
	if bytes.Equal(forged, receipt.Bytes()) {
		t.Fatal("receipt omitted shifted owner claim")
	}
	if _, err := ReadRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, bytes.NewReader(forged), IndependentMaterialJSONLimits{}); err == nil {
		t.Fatal("receipt reproof accepted a forged shifted-cylinder claim")
	}
}

func TestRepairThreeConvexArcRejectsChangedOwners(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(threeConvexArcSource)
	if err != nil {
		t.Fatal(err)
	}
	for _, edit := range [][3]string{
		{"changed phase", "#218 = CARTESIAN_POINT('',(-2.498091544797,0.));", "#218 = CARTESIAN_POINT('',(-2.49,0.));"},
		{"erased phase", "#428 = CARTESIAN_POINT('',(-2.498091544797,5.));", "#428 = CARTESIAN_POINT('',(0.,5.));"},
		{"changed traversal direction", "#220 = DIRECTION('',(1.,0.));", "#220 = DIRECTION('',(-1.,0.));"},
		{"inexact cylinder frame", "#215 = DIRECTION('',(0.36,0.8,-0.48));", "#215 = DIRECTION('',(0.36,0.8000000000000001,-0.48));"},
		{"reassigned owner", "#197 = SURFACE_CURVE('',#198,(#203,#210),.PCURVE_S1.);", "#197 = SURFACE_CURVE('',#198,(#203,#425),.PCURVE_S1.);"},
	} {
		t.Run(edit[0], func(t *testing.T) {
			rejectChangedGroupedConversion(t, raw, threeConvexArcDistance, [][2]string{{edit[1], edit[2]}})
		})
	}
}

func checkGroupedConvexArcUnrepresentable(t *testing.T, raw []byte, distance float64) {
	t.Helper()
	m, err := RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion(context.Background(), bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{MaxDistance: distance})
	if !errors.Is(err, geom.ErrUnrepresentable) || m.OriginalSTEP() != "" {
		t.Fatal("non-binary tangent vertices yielded material or lost their exact rejection", err)
	}
}

func TestRepairThreeConvexArcRejectsUnrepresentableTangencies(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(threeConvexArcUnrepresentableSource)
	if err != nil {
		t.Fatal(err)
	}
	checkGroupedConvexArcUnrepresentable(t, raw, threeConvexArcDistance)
}

func TestRepairThreeConvexArcLive(t *testing.T) {
	t.Parallel()
	oracle := os.Getenv("PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL")
	if oracle == "" {
		t.Skip("set PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL to the pinned author helper")
	}
	dir := t.TempDir()
	original := filepath.Join(dir, "original.step")
	if out, err := exec.Command(oracle, "write-three-default-tilted", original).CombinedOutput(); err != nil {
		t.Fatalf("pinned OCCT author: %v\n%s", err, out)
	}
	raw, err := os.ReadFile(original)
	if err != nil {
		t.Fatal(err)
	}
	m, canonical := checkThreeConvexArcSource(t, raw)
	witness, err := exec.Command(oracle, "shifted-cylinder-witness", original).CombinedOutput()
	if err != nil {
		t.Fatalf("pinned OCCT shifted-cylinder evaluation: %v\n%s", err, witness)
	}
	var phase struct {
		Owners, Samples                 int
		MaxSourceDistance, WithoutPhase float64
	}
	if err = json.Unmarshal(witness, &phase); err != nil || phase.Owners != 2 || phase.Samples != 6 ||
		phase.MaxSourceDistance <= 0 || phase.MaxSourceDistance > 1e-12 || phase.WithoutPhase < .25 {
		t.Fatalf("unexpected independent phase witness: %s (%v)", witness, err)
	}
	unit := rationalTiltUnitFixture{name: "millimetre", metresPerUnit: .001, mmNumerator: 1, mmDenominator: 1}
	want := rationalTiltUnitMeasurements{Volume: 28.968988172080639, Area: 64.184278587623695,
		Centroid: [3]float64{2.7654896548128538, 1.0201651571311465, .47934712691619552}, Faces: 8, Solids: 1}
	for _, measured := range readRationalTiltUnitWorldSTEP(t, oracle, original, m.CorrectedModelSTEP(), canonical) {
		checkScaledRationalTiltUnitMeasures(t, unit, measured, want)
	}

	unrepresentable := filepath.Join(dir, "nondyadic.step")
	if out, err := exec.Command(oracle, "write-three-default-tilted-nondyadic", unrepresentable).CombinedOutput(); err != nil {
		t.Fatalf("pinned OCCT unrepresentable author: %v\n%s", err, out)
	}
	raw, err = os.ReadFile(unrepresentable)
	if err != nil {
		t.Fatal(err)
	}
	checkGroupedConvexArcUnrepresentable(t, raw, threeConvexArcDistance)
}
