package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/internal/slowtest"
	"reflect"
	"strconv"
	"testing"
)

const threeConvertedSource = "occt-three-default-tilted-oblique-prism-fillet-converted"

type threeConvertedFixture struct {
	unit                                   rationalTiltUnitFixture
	maxDistance, rejectedDistance          float64
	height, sourceScale, sampledShiftLimit float64
	ownerScale, axialKnot, squaredLimit    string
	repairedStart                          [2]string
}

var threeConvertedMetre = threeConvertedFixture{
	unit: rationalTiltUnitFixtures[0], maxDistance: 1e-12, rejectedDistance: 1e-13,
	height: 5, sourceScale: 1, sampledShiftLimit: 2.7e-13,
	ownerScale: "1/1000", axialKnot: "5000", squaredLimit: "7.3e-26",
	repairedStart: [2]string{"-3/500000000000000000000000000", "-1/25000000000000000000000000"},
}

var threeConvertedInch = threeConvertedFixture{
	unit: rationalTiltUnitFixtures[1], maxDistance: 1e-11, rejectedDistance: 1e-12,
	height: 25, sourceScale: 5, sampledShiftLimit: 1.35e-12,
	ownerScale: "5/127", axialKnot: "635", squaredLimit: "1.82e-24",
	repairedStart: [2]string{"1/984251968504000000000000000", "-1/196850393700800000000000000"},
}

func checkThreeConvertedConvexArc(t *testing.T, raw []byte, tc threeConvertedFixture) (RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion, []byte) {
	t.Helper()
	ctx := context.Background()
	if _, err := ReadRationalPosedExactCircularLoopExtrusion(ctx, bytes.NewReader(raw), Limits{}); err == nil {
		t.Fatal("strict STEP accepted the original converted pcurves")
	}
	m, err := RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{MaxDistance: tc.maxDistance})
	if err != nil {
		t.Fatal(err)
	}
	d := m.LocalDescriptor()
	owners, lines, spans := m.PCurves(), m.FiniteLineOwners(), m.CylindricalSpanOwners()
	trims, caps, finite := m.AxialTrims(), m.RationalCaps(), m.FiniteTrims()
	if m.OriginalSTEP() != string(raw) || d.LengthUnit != tc.unit.metresPerUnit || d.Height != tc.height || len(d.Segments) != 6 ||
		len(m.Solid().Faces()) != 8 || m.ReferenceCircleID() != 144 || len(m.Repairs()) != 10 ||
		len(m.Tangencies()) != 12 || len(m.Reversals()) != 2 || len(owners) != 34 || len(lines) != 18 ||
		len(spans) != 12 || len(trims) != 2 || len(caps) != 2 || len(finite) != 2 {
		t.Fatal("converted source lost exact model or owner evidence")
	}
	for _, line := range lines {
		if line.PCurveParameterScale == nil || line.PCurveParameterScale.RatString() != tc.ownerScale {
			t.Fatal("lost original planar owner map", line)
		}
	}
	if trims[0].EdgeID != 276 || trims[1].EdgeID != 449 {
		t.Fatal("wrong original unequal endpoint trims", trims)
	}
	height := strconv.FormatFloat(tc.height, 'f', 0, 64)
	const sweep = "2498091544797/1000000000000"
	const quarter = "314159265359/200000000000"
	wantSpans := map[int64][6]string{
		71: {"0", tc.axialKnot, "0", "0", "0", tc.ownerScale}, 124: {"0", tc.axialKnot, "0", "0", "0", tc.ownerScale},
		156: {"0", sweep, "0", "0", "1", "0"}, 357: {"0", sweep, "0", height, "1", "0"},
		207: {sweep, "942477796077/200000000000", "0", "0", "1", "0"}, 416: {sweep, "942477796077/200000000000", "0", height, "1", "0"},
		265: {"0", quarter, "0", "0", "-1", "0"}, 305: {"0", quarter, "0", height, "-1", "0"},
		284: {"0", tc.axialKnot, "-" + quarter, "0", "0", tc.ownerScale}, 336: {"0", tc.axialKnot, sweep, "0", "0", tc.ownerScale},
		462: {"0", tc.axialKnot, "553574358897/250000000000", "0", "0", tc.ownerScale}, 490: {"0", tc.axialKnot, "0", "0", "0", tc.ownerScale},
	}
	wantAxial := map[int64]int{284: 0, 462: 1}
	for _, span := range spans {
		values := [6]string{span.ParameterInterval[0].RatString(), span.ParameterInterval[1].RatString(),
			span.UVStart[0].RatString(), span.UVStart[1].RatString(), span.UVDerivative[0].RatString(), span.UVDerivative[1].RatString()}
		if values != wantSpans[span.PCurveID] {
			t.Fatal("changed original cylindrical chart", span)
		}
		delete(wantSpans, span.PCurveID)
		if i, ok := wantAxial[span.PCurveID]; ok {
			checkConvertedGroupAxialMap(t, span, trims[i], tc.repairedStart[i])
			delete(wantAxial, span.PCurveID)
		} else if span.RepairedToOwner != nil {
			t.Fatal("unexpected finite affine owner map", span)
		}
		if span.PCurveID != 207 && span.PCurveID != 416 {
			if span.ModelAngleBounds != nil {
				t.Fatal("unexpected shifted owner", span)
			}
			continue
		}
		if span.ModelAngleBounds == nil {
			t.Fatal("shifted cylindrical chart lost model angle bounds", span)
		}
		for i, interval := range *span.ModelAngleBounds {
			want := [2][2]string{{"624522886199/250000000000", "1249045772399/500000000000"},
				{"147262155637/31250000000", "2356194490193/500000000000"}}
			for j, limit := range interval {
				if limit.RatString() != want[i][j] {
					t.Fatal("changed rational model-angle enclosure", span)
				}
			}
		}
	}
	if len(wantSpans) != 0 || len(wantAxial) != 0 {
		t.Fatal("missing cylindrical owner or unequal axial map", wantSpans, wantAxial)
	}
	wantDistanceSquared, _ := new(big.Rat).SetString(tc.squaredLimit)
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
	return m, checkRationalTiltUnitCanonical(t, tc.unit, m, new(big.Rat), new(big.Rat))
}

func TestRepairThreeConvertedMetreConvexArcSource(t *testing.T) {
	slowtest.Skip(t)
	checkThreeConvertedConvexArcSource(t, threeConvertedMetre)
}

func TestRepairThreeConvertedInchConvexArcSource(t *testing.T) {
	slowtest.Skip(t)
	checkThreeConvertedConvexArcSource(t, threeConvertedInch)
}

func checkThreeConvertedConvexArcSource(t *testing.T, tc threeConvertedFixture) {
	t.Helper()
	ctx := context.Background()
	raw := tc.unit.source(t, threeConvertedSource)
	m, _ := checkThreeConvertedConvexArc(t, raw, tc)
	rejectChangedGroupedConversion(t, raw, tc.rejectedDistance, nil)
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

func TestRepairThreeConvertedInchConvexArcRejectsChangedCharts(t *testing.T) {
	raw := threeConvertedInch.unit.source(t, threeConvertedSource)
	for _, test := range []struct {
		name  string
		edits [][2]string
	}{
		{"inexact inch factor", [][2]string{{"LENGTH_MEASURE_WITH_UNIT(25.4,", "LENGTH_MEASURE_WITH_UNIT(25.40000000000000001,"}}},
		{"changed original line speed", [][2]string{{"#29 = VECTOR('',#30,3.937007874016E-02);", "#29 = VECTOR('',#30,3.93700787401600001E-02);"}}},
		{"shifted angle bracket outside source endpoint", [][2]string{
			{"(2.498091544797,4.712388980385),.PIECEWISE_BEZIER_KNOTS.);\n#419", "(2.498092544797,4.712388980385),.PIECEWISE_BEZIER_KNOTS.);\n#419"},
			{"#420 = CARTESIAN_POINT('',(2.214297435588,25.));", "#420 = CARTESIAN_POINT('',(2.214296435588,25.));"},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			rejectChangedGroupedConversion(t, raw, threeConvertedInch.maxDistance, test.edits)
		})
	}
}

func checkThreeConvertedInchUnscaled(t *testing.T, raw []byte) {
	t.Helper()
	checkUnscaledInchSupports(t, raw, []roundedInchSupport{
		{276, 454, 281, "1/5000000000000000"},
		{330, 481, 335, "147/625000000000000"},
		{508, 481, 488, "471/1250000000000000"},
	})
}

func TestRepairThreeConvertedInchConvexArcRejectsRoundedSupports(t *testing.T) {
	raw, err := os.ReadFile("testdata/" + threeConvertedSource + "-inch-unscaled.step")
	if err != nil {
		t.Fatal(err)
	}
	checkThreeConvertedInchUnscaled(t, raw)
}

func TestRepairThreeConvertedMetreConvexArcLive(t *testing.T) {
	checkThreeConvertedConvexArcLive(t, threeConvertedMetre)
}

func TestRepairThreeConvertedInchConvexArcLive(t *testing.T) {
	checkThreeConvertedConvexArcLive(t, threeConvertedInch)
	oracle := os.Getenv("PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL")
	unscaled := filepath.Join(t.TempDir(), "unscaled.step")
	if out, err := exec.Command(oracle, "write-three-default-tilted-converted-inch-unscaled", unscaled).CombinedOutput(); err != nil {
		t.Fatalf("pinned OCCT unscaled inch author: %v\n%s", err, out)
	}
	raw, err := os.ReadFile(unscaled)
	if err != nil {
		t.Fatal(err)
	}
	checkThreeConvertedInchUnscaled(t, raw)
	output, err := exec.Command(oracle, "read", unscaled).CombinedOutput()
	if err != nil {
		t.Fatalf("pinned OCCT unscaled inch transfer: %v\n%s", err, output)
	}
	var measured rationalTiltUnitMeasurements
	if err := json.Unmarshal(output, &measured); err != nil || measured.Faces != 8 || measured.Solids != 1 {
		t.Fatalf("pinned OCCT did not transfer the rounded unscaled source: %s (%v)", output, err)
	}
}

func checkThreeConvertedConvexArcLive(t *testing.T, tc threeConvertedFixture) {
	t.Helper()
	oracle := os.Getenv("PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL")
	if oracle == "" {
		t.Skip("set PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL to the pinned author helper")
	}
	original := filepath.Join(t.TempDir(), "original.step")
	if out, err := exec.Command(oracle, "write-three-default-tilted-converted-"+tc.unit.name, original).CombinedOutput(); err != nil {
		t.Fatalf("pinned OCCT converted author: %v\n%s", err, out)
	}
	raw, err := os.ReadFile(original)
	if err != nil {
		t.Fatal(err)
	}
	m, canonical := checkThreeConvertedConvexArc(t, raw, tc)
	witness, err := exec.Command(oracle, "three-converted-"+tc.unit.name+"-witness", original).CombinedOutput()
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
		result.ShiftedCylinderCircles != 2 || result.ShiftedSamples != 6 || result.MaxSourceDistance > tc.maxDistance ||
		result.MaxCapDistance > tc.maxDistance || result.MaxShiftedDistance > tc.sampledShiftLimit || result.WithoutPlanarMap < 100 {
		t.Fatalf("unexpected pinned converted chart witness: %s (%v)", witness, err)
	}
	want := rationalTiltUnitMeasurements{Volume: 28.968988172080639, Area: 64.184278587623695,
		Centroid: [3]float64{2.7654896548128538, 1.0201651571311465, .47934712691619552}, Faces: 8, Solids: 1}
	want.Volume *= tc.sourceScale * tc.sourceScale * tc.sourceScale
	want.Area *= tc.sourceScale * tc.sourceScale
	for i := range want.Centroid {
		want.Centroid[i] *= tc.sourceScale
	}
	for _, measured := range readRationalTiltUnitWorldSTEP(t, oracle, original, m.CorrectedModelSTEP(), canonical) {
		checkScaledRationalTiltUnitMeasures(t, tc.unit, measured, want)
	}
}
