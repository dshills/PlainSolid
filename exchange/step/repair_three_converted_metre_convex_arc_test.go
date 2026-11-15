package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

const threeConvertedSource = "occt-three-default-tilted-oblique-prism-fillet-converted"

func checkThreeConvertedMetreConvexArc(t *testing.T, raw []byte) (RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion, []byte) {
	t.Helper()
	ctx := context.Background()
	if _, err := ReadRationalPosedExactCircularLoopExtrusion(ctx, bytes.NewReader(raw), Limits{}); err == nil {
		t.Fatal("strict STEP accepted the original converted pcurves")
	}
	m, err := RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{MaxDistance: 1e-12})
	if err != nil {
		t.Fatal(err)
	}
	d := m.LocalDescriptor()
	owners, lines, spans := m.PCurves(), m.FiniteLineOwners(), m.CylindricalSpanOwners()
	trims, caps, finite := m.AxialTrims(), m.RationalCaps(), m.FiniteTrims()
	if m.OriginalSTEP() != string(raw) || d.LengthUnit != 1 || d.Height != 5 || len(d.Segments) != 6 ||
		len(m.Solid().Faces()) != 8 || m.ReferenceCircleID() != 144 || len(m.Repairs()) != 10 ||
		len(m.Tangencies()) != 12 || len(m.Reversals()) != 2 || len(owners) != 34 || len(lines) != 18 ||
		len(spans) != 12 || len(trims) != 2 || len(caps) != 2 || len(finite) != 2 {
		t.Fatal("converted source lost exact model or owner evidence")
	}
	for _, line := range lines {
		if line.PCurveParameterScale == nil || line.PCurveParameterScale.RatString() != "1/1000" {
			t.Fatal("lost original planar owner map", line)
		}
	}
	if trims[0].EdgeID != 276 || trims[1].EdgeID != 449 {
		t.Fatal("wrong original unequal endpoint trims", trims)
	}
	wantShifted := map[int64]string{207: "0", 416: "5"}
	wantAxial := map[int64]int{284: 0, 462: 1}
	for _, span := range spans {
		if i, ok := wantAxial[span.PCurveID]; ok {
			wantStart := [2]string{"-3/500000000000000000000000000", "-1/25000000000000000000000000"}
			checkConvertedGroupAxialMap(t, span, trims[i], wantStart[i])
			delete(wantAxial, span.PCurveID)
		} else if span.RepairedToOwner != nil {
			t.Fatal("unexpected finite affine owner map", span)
		}
		height, shifted := wantShifted[span.PCurveID]
		if !shifted {
			if span.ModelAngleBounds != nil || span.ParameterInterval[0].Sign() != 0 {
				t.Fatal("unexpected shifted owner", span)
			}
			continue
		}
		if span.ParameterInterval[0].FloatString(12) != "2.498091544797" ||
			span.ParameterInterval[1].FloatString(12) != "4.712388980385" ||
			span.UVStart[0].Sign() != 0 || span.UVStart[1].RatString() != height ||
			span.UVDerivative[0].RatString() != "1" || span.UVDerivative[1].Sign() != 0 ||
			span.ModelAngleBounds == nil {
			t.Fatal("shifted cylindrical chart lost original knots, phase or travel bounds", span)
		}
		for i, interval := range *span.ModelAngleBounds {
			want := [2][2]string{{"2.498091544796", "2.498091544798"}, {"4.712388980384", "4.712388980386"}}
			for j, limit := range interval {
				if limit.FloatString(12) != want[i][j] {
					t.Fatal("changed rational model-angle enclosure", span)
				}
			}
		}
		delete(wantShifted, span.PCurveID)
	}
	if len(wantShifted) != 0 || len(wantAxial) != 0 {
		t.Fatal("missing shifted or unequal-axial owner", wantShifted, wantAxial)
	}
	wantDistanceSquared, _ := new(big.Rat).SetString("7.3e-26")
	shifted := 0
	for _, owner := range owners {
		if owner.Kind == "finite-phase-cylinder-circle" {
			shifted++
			if (owner.PCurveID != 207 && owner.PCurveID != 416) ||
				owner.DistanceSquaredUpper.Sign() <= 0 || owner.DistanceSquaredUpper.Cmp(wantDistanceSquared) > 0 {
				t.Fatal("shifted whole-span travel lost its bound", owner.PCurveID)
			}
		}
	}
	if shifted != 2 {
		t.Fatal("lost shifted cylindrical owners", shifted)
	}
	f, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	for i, cap := range caps {
		trim := finite[i]
		if !cap.PositiveAngularVelocity || !cap.ClosedOneTurn ||
			cap.Owner.PCurveID != [2]int64{254, 311}[i] || trim.CapOwner.PCurveID != cap.Owner.PCurveID ||
			trim.CylinderOwner.PCurveID != [2]int64{265, 305}[i] ||
			trim.CylinderOwner.Kind != "finite-reversed-cylinder-circle" || trim.EndStation == nil {
			t.Fatal("rational cap lost its original finite reversed-cylinder pair", cap.Owner.PCurveID)
		}
		curve, err := simple(f, trim.CapOwner.SurfaceCurveID, "SURFACE_CURVE", 4)
		if err != nil {
			t.Fatal(err)
		}
		order, err := listRefs(curve.Args[2], 2)
		if err != nil || !reflect.DeepEqual(order, [2][]int64{{254, 265}, {305, 311}}[i]) {
			t.Fatal("authored cap and cylinder order changed", order, err)
		}
	}
	pose, err := m.Pose().Matrix()
	if err != nil {
		t.Fatal(err)
	}
	wantPose := [3][4]string{{"0", "3/5", "4/5", "0"}, {"-1", "0", "0", "0"}, {"0", "-4/5", "3/5", "0"}}
	for i := range pose {
		for j := range pose[i] {
			if pose[i][j].RatString() != wantPose[i][j] {
				t.Fatal("converted source pose changed", pose)
			}
		}
	}
	return m, checkRationalTiltUnitCanonical(t, rationalTiltUnitFixtures[0], m, new(big.Rat), new(big.Rat))
}

func TestRepairThreeConvertedMetreConvexArcSource(t *testing.T) {
	ctx := context.Background()
	raw := rationalTiltUnitFixtures[0].source(t, threeConvertedSource)
	m, _ := checkThreeConvertedMetreConvexArc(t, raw)
	rejectChangedGroupedConversion(t, raw, 1e-13, nil)
	var record bytes.Buffer
	if err := WriteRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, &record, m); err != nil {
		t.Fatal(err)
	}
	reloaded, err := ReadRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, bytes.NewReader(record.Bytes()), IndependentMaterialJSONLimits{})
	if err != nil || reloaded.OriginalSTEP() != string(raw) ||
		!reflect.DeepEqual(reloaded.CylindricalSpanOwners(), m.CylindricalSpanOwners()) ||
		!reflect.DeepEqual(reloaded.FiniteLineOwners(), m.FiniteLineOwners()) ||
		!reflect.DeepEqual(reloaded.AxialTrims(), m.AxialTrims()) ||
		!reflect.DeepEqual(reloaded.RationalCaps(), m.RationalCaps()) {
		t.Fatal("native reproof lost the original converted charts", err)
	}
	var again bytes.Buffer
	if err := WriteRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, &again, reloaded); err != nil || !bytes.Equal(record.Bytes(), again.Bytes()) {
		t.Fatal("converted proof bytes changed on reproof", err)
	}
	forged := bytes.Replace(record.Bytes(), []byte(`"ModelAngleBounds":[["624522886199/`), []byte(`"ModelAngleBounds":[["624522886198/`), 1)
	if bytes.Equal(forged, record.Bytes()) {
		t.Fatal("proof omitted the source model-angle bounds")
	}
	if _, err := ReadRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, bytes.NewReader(forged), IndependentMaterialJSONLimits{}); err == nil {
		t.Fatal("native reproof accepted a forged angular enclosure")
	}
}

func TestRepairThreeConvertedMetreConvexArcRejectsChangedCharts(t *testing.T) {
	raw := rationalTiltUnitFixtures[0].source(t, threeConvertedSource)
	for _, test := range []struct {
		name  string
		edits [][2]string
	}{
		{"short final knot with unchanged slope", [][2]string{
			{"(2.498091544797,4.712388980385),.PIECEWISE_BEZIER_KNOTS.);\n#215", "(2.498091544797,4.712387980385),.PIECEWISE_BEZIER_KNOTS.);\n#215"},
			{"#216 = CARTESIAN_POINT('',(2.214297435588,0.));", "#216 = CARTESIAN_POINT('',(2.214296435588,0.));"},
		}},
		{"changed phase origin", [][2]string{{"#215 = CARTESIAN_POINT('',(0.,0.));", "#215 = CARTESIAN_POINT('',(0.001,0.));"}}},
		{"changed traversal", [][2]string{{"#420 = CARTESIAN_POINT('',(2.214297435588,5.));", "#420 = CARTESIAN_POINT('',(-2.214297435588,5.));"}}},
		{"borrowed opposite cap chart", [][2]string{{"#194 = SURFACE_CURVE('',#195,(#200,#207),.PCURVE_S1.);", "#194 = SURFACE_CURVE('',#195,(#200,#416),.PCURVE_S1.);"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			rejectChangedGroupedConversion(t, raw, 1e-12, test.edits)
		})
	}
}

func TestRepairThreeConvertedMetreConvexArcLive(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL")
	if oracle == "" {
		t.Skip("set PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL to the pinned author helper")
	}
	original := filepath.Join(t.TempDir(), "original.step")
	if out, err := exec.Command(oracle, "write-three-default-tilted-converted-metre", original).CombinedOutput(); err != nil {
		t.Fatalf("pinned OCCT converted author: %v\n%s", err, out)
	}
	raw, err := os.ReadFile(original)
	if err != nil {
		t.Fatal(err)
	}
	m, canonical := checkThreeConvertedMetreConvexArc(t, raw)
	witness, err := exec.Command(oracle, "three-converted-metre-witness", original).CombinedOutput()
	if err != nil {
		t.Fatalf("pinned OCCT finite owner evaluation: %v\n%s", err, witness)
	}
	var result struct {
		PlaneLines, PlaneCircles, CylinderLines, CylinderCircles int
		RationalCaps, ShiftedCylinderCircles, ShiftedSamples     int
		MaxSourceDistance, MaxCapDistance, MaxShiftedDistance    float64
		WithoutPlanarMap                                         float64
	}
	if err := json.Unmarshal(witness, &result); err != nil || result.PlaneLines != 18 || result.PlaneCircles != 4 ||
		result.CylinderLines != 6 || result.CylinderCircles != 6 || result.RationalCaps != 2 ||
		result.ShiftedCylinderCircles != 2 || result.ShiftedSamples != 6 || result.MaxSourceDistance > 1e-12 ||
		result.MaxCapDistance > 1e-12 || result.MaxShiftedDistance > 2.7e-13 || result.WithoutPlanarMap < 100 {
		t.Fatalf("unexpected pinned converted chart witness: %s (%v)", witness, err)
	}
	want := rationalTiltUnitMeasurements{Volume: 28.968988172080639, Area: 64.184278587623695,
		Centroid: [3]float64{2.7654896548128538, 1.0201651571311465, .47934712691619552}, Faces: 8, Solids: 1}
	for _, measured := range readRationalTiltUnitWorldSTEP(t, oracle, original, m.CorrectedModelSTEP(), canonical) {
		checkScaledRationalTiltUnitMeasures(t, rationalTiltUnitFixtures[0], measured, want)
	}
}
