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
	"strconv"
	"strings"
	"testing"
)

const fourConvertedMetreSource = fourConvexArcSource + "-converted-metre"
const fourConvertedInchSource = fourConvexArcSource + "-converted-inch"

func checkFourConvertedConvexArc(t *testing.T, raw []byte, tc groupedConvertedFixture) (RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion, []byte) {
	t.Helper()
	ctx := context.Background()
	if _, err := ReadRationalPosedExactCircularLoopExtrusion(ctx, bytes.NewReader(raw), Limits{}); err == nil {
		t.Fatal("strict STEP accepted the original converted owners")
	}
	if _, err := ReadOCCTExactCircularLoopExtrusion(ctx, bytes.NewReader(raw), Limits{}); err == nil {
		t.Fatal("ordinary OCCT strict STEP accepted the original converted owners")
	}
	m, err := RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{MaxDistance: tc.maxDistance})
	if err != nil {
		t.Fatal(err)
	}
	d := m.LocalDescriptor()
	lines, spans, trims := m.FiniteLineOwners(), m.CylindricalSpanOwners(), m.AxialTrims()
	if m.OriginalSTEP() != string(raw) || d.LengthUnit != tc.unit.metresPerUnit || d.Height != tc.height || len(d.Segments) != 8 || m.ReferenceCircleID() != 144 ||
		len(m.Solid().Faces()) != 10 || len(m.Repairs()) != 10 || len(m.Tangencies()) != 16 || len(m.PCurves()) != 46 ||
		len(m.Reversals()) != 2 || len(lines) != 24 || len(spans) != 16 || len(trims) != 2 {
		t.Fatal("four converted rounds lost the native solid or owner evidence")
	}
	for _, line := range lines {
		if line.PCurveParameterScale == nil || line.PCurveParameterScale.RatString() != tc.ownerScale {
			t.Fatal("changed the original planar parameter map", line.PCurveID)
		}
	}
	const quarter = "314159265359/200000000000"
	const oblique = "2498091544797/1000000000000"
	const end = "942477796077/200000000000"
	const smallU = "4440892098501/10000000000000000000000000000"
	const obliqueSlope = "7380991451959998519702633833/7380991451960000000000000000"
	height := strconv.FormatFloat(tc.height, 'f', 0, 64)
	alignedAxial := [6]string{"0", tc.axialEnd, "0", "0", "0", tc.ownerScale}
	wantSpans := map[int64][6]string{
		71: alignedAxial, 124: alignedAxial, 592: alignedAxial, 611: alignedAxial,
		156: {"0", quarter, "0", "0", "1", "0"}, 413: {"0", quarter, "0", height, "1", "0"},
		207: {quarter, oblique, "0", "0", "1", "0"}, 472: {quarter, oblique, "0", height, "1", "0"},
		263: {oblique, end, smallU, "0", obliqueSlope, "0"}, 518: {oblique, end, smallU, height, obliqueSlope, "0"},
		321: {"0", quarter, "0", "0", "-1", "0"}, 361: {"0", quarter, "0", height, "-1", "0"},
		340: {"0", tc.axialEnd, "-" + quarter, "0", "0", tc.ownerScale}, 392: {"0", tc.axialEnd, quarter, "0", "0", tc.ownerScale},
		564: {"0", tc.axialEnd, "553574358897/250000000000", "0", "0", tc.ownerScale},
		635: {"0", tc.axialEnd, "463647609001/500000000000", "0", "0", tc.ownerScale},
	}
	if trims[0].EdgeID != 332 || trims[1].EdgeID != 551 {
		t.Fatal("lost the two original unequal axial trims")
	}
	if tc.unit.name == "inch" {
		wantEnds := [2]string{"625000000000000000000000000001/984251968504000000000000000",
			"625000000000000000000000000003/984251968504000000000000000"}
		for i, trim := range trims {
			if trim.OriginalParameters[1].RatString() != wantEnds[i] || trim.RepairedParameters[1].Cmp(trim.OriginalParameters[1]) != 0 {
				t.Fatal("changed the exact projected inch end station", trim.EdgeID)
			}
		}
	}
	for _, span := range spans {
		coefficients := [6]string{span.ParameterInterval[0].RatString(), span.ParameterInterval[1].RatString(),
			span.UVStart[0].RatString(), span.UVStart[1].RatString(), span.UVDerivative[0].RatString(), span.UVDerivative[1].RatString()}
		if coefficients != wantSpans[span.PCurveID] {
			t.Fatal("original cylinder spline law changed", span)
		}
		delete(wantSpans, span.PCurveID)
		if span.ParameterInterval[0].Sign() != 0 {
			if span.ModelAngleBounds == nil {
				t.Fatal("shifted cylinder lost its directed angle bound", span.PCurveID)
			}
			for i, knot := range span.ParameterInterval {
				angularError := big.NewRat(1, 1000000000000)
				bounds := span.ModelAngleBounds[i]
				if bounds[0].Cmp(new(big.Rat).Sub(knot, angularError)) != 0 || bounds[1].Cmp(new(big.Rat).Add(knot, angularError)) != 0 {
					t.Fatal("shifted cylinder changed its rational angular bracket", span.PCurveID)
				}
			}
		} else if span.ModelAngleBounds != nil {
			t.Fatal("unexpected shifted chart evidence", span.PCurveID)
		}
		if span.PCurveID == 564 && tc.unit.name == "metre" {
			clipped, mapping, trim := span.ClippedOwnerParameters, span.RepairedToOwner, trims[1]
			excess, _ := new(big.Rat).SetString("4e-26")
			if trim.EdgeID != 551 || clipped == nil || mapping == nil ||
				trim.OriginalParameters[0].Sign() != 0 || trim.OriginalParameters[1].Cmp(new(big.Rat).Add(big.NewRat(5000, 1), excess)) != 0 ||
				trim.RepairedParameters[0].Cmp(excess) != 0 || trim.RepairedParameters[1].Cmp(trim.OriginalParameters[1]) != 0 ||
				clipped[0].Sign() != 0 || clipped[1].RatString() != "5000" ||
				mapping[0].Cmp(new(big.Rat).Neg(excess)) != 0 || mapping[1].RatString() != "1" {
				t.Fatal("source projection, repaired station and finite owner map were conflated", span, trim)
			}
			continue
		}
		if span.ClippedOwnerParameters != nil {
			t.Fatal("unexpected end clipping", span.PCurveID)
		}
		if span.PCurveID == 340 {
			checkConvertedGroupAxialMap(t, span, trims[0], tc.repairedStart)
		} else if span.PCurveID == 564 {
			checkConvertedGroupAxialMap(t, span, trims[1], "3/984251968504000000000000000")
		} else if span.RepairedToOwner != nil {
			t.Fatal("unexpected repaired-to-owner map", span.PCurveID)
		}
	}
	if len(wantSpans) != 0 {
		t.Fatal("missing original cylinder owners", wantSpans)
	}
	caps, finite := m.RationalCaps(), m.FiniteTrims()
	if len(caps) != 2 || len(finite) != 2 {
		t.Fatal("lost the original rational caps")
	}
	f, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	for i, cap := range caps {
		if cap.Owner.PCurveID != [2]int64{310, 367}[i] || !cap.PositiveAngularVelocity || !cap.ClosedOneTurn ||
			finite[i].CapOwner.PCurveID != cap.Owner.PCurveID || finite[i].CylinderOwner.PCurveID != [2]int64{321, 361}[i] ||
			finite[i].EndStation == nil {
			t.Fatal("changed original finite rational cap pair", cap.Owner.PCurveID)
		}
		curve, err := simple(f, cap.Owner.SurfaceCurveID, "SURFACE_CURVE", 4)
		if err != nil {
			t.Fatal(err)
		}
		order, err := listRefs(curve.Args[2], 2)
		if err != nil || !reflect.DeepEqual(order, [2][]int64{{310, 321}, {361, 367}}[i]) {
			t.Fatal("changed original converted cap owner order", order, err)
		}
	}
	matrix, err := m.Pose().Matrix()
	if err != nil {
		t.Fatal(err)
	}
	wantPose := [3][4]string{{"0", "3/5", "4/5", "0"}, {"-1", "0", "0", "0"}, {"0", "-4/5", "3/5", "0"}}
	for i := range matrix {
		for j := range matrix[i] {
			if matrix[i][j].RatString() != wantPose[i][j] {
				t.Fatal("unit conversion changed the source's rational world pose", matrix)
			}
		}
	}
	return m, checkRationalTiltUnitCanonical(t, tc.unit, m, new(big.Rat), new(big.Rat))
}

func TestRepairFourConvertedMetreConvexArcSource(t *testing.T) {
	checkFourConvertedConvexArcSource(t, groupedConvertedMetre, "#207", `"ClippedOwnerParameters":["0","5000"]`, `"ClippedOwnerParameters":["0","5001"]`)
}

func TestRepairFourConvertedInchConvexArcSource(t *testing.T) {
	checkFourConvertedConvexArcSource(t, groupedConvertedInch, "#64", `"RepairedToOwner":["-`, `"RepairedToOwner":["1`)
}

func checkFourConvertedConvexArcSource(t *testing.T, tc groupedConvertedFixture, failingOwner, claim, forgery string) {
	t.Helper()
	ctx := context.Background()
	raw, err := os.ReadFile(fourConvexArcSource + "-converted-" + tc.unit.name + ".step")
	if err != nil {
		t.Fatal(err)
	}
	m, _ := checkFourConvertedConvexArc(t, raw, tc)
	tooSmall, err := RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{MaxDistance: tc.rejectedDistance})
	if err == nil || !strings.Contains(err.Error(), "source pcurve "+failingOwner+":") || tooSmall.OriginalSTEP() != "" {
		t.Fatal("the tight source budget must reject the original lift", err)
	}
	var jsonRecord bytes.Buffer
	if err := WriteRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, &jsonRecord, m); err != nil {
		t.Fatal(err)
	}
	reproved, err := ReadRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, bytes.NewReader(jsonRecord.Bytes()), IndependentMaterialJSONLimits{})
	if err != nil || reproved.OriginalSTEP() != string(raw) || !reflect.DeepEqual(reproved.CylindricalSpanOwners(), m.CylindricalSpanOwners()) ||
		!reflect.DeepEqual(reproved.FiniteLineOwners(), m.FiniteLineOwners()) || !reflect.DeepEqual(reproved.AxialTrims(), m.AxialTrims()) ||
		!reflect.DeepEqual(reproved.FiniteTrims(), m.FiniteTrims()) {
		t.Fatal("native reproof lost the original finite charts", err)
	}
	var again bytes.Buffer
	if err := WriteRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, &again, reproved); err != nil || !bytes.Equal(again.Bytes(), jsonRecord.Bytes()) {
		t.Fatal("source proof changed on reproof", err)
	}
	forged := bytes.Replace(jsonRecord.Bytes(), []byte(claim), []byte(forgery), 1)
	if bytes.Equal(forged, jsonRecord.Bytes()) {
		t.Fatal("source proof omitted the actual finite owner endpoint")
	}
	if _, err = ReadRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, bytes.NewReader(forged), IndependentMaterialJSONLimits{}); err == nil {
		t.Fatal("native reproof accepted a forged finite owner endpoint")
	}
}

func TestRepairFourConvertedMetreConvexArcRejectsChangedCharts(t *testing.T) {
	raw, err := os.ReadFile(fourConvertedMetreSource + ".step")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name  string
		edits [][2]string
	}{
		{"short same-sense quarter", [][2]string{
			{"(0.,1.570796326795),.PIECEWISE_BEZIER_KNOTS.);\n#159", "(0.,1.570796326794),.PIECEWISE_BEZIER_KNOTS.);\n#159"},
			{"#160 = CARTESIAN_POINT('',(1.570796326795,0.));", "#160 = CARTESIAN_POINT('',(1.570796326794,0.));"},
		}},
		{"wrong shifted angular branch", [][2]string{
			{"(1.570796326795,2.498091544797),.PIECEWISE_BEZIER_KNOTS.);\n#215", "(1.570797326795,2.498092544797),.PIECEWISE_BEZIER_KNOTS.);\n#215"},
		}},
		{"excess angular coefficient error", [][2]string{
			{"#271 = CARTESIAN_POINT('',(4.440892098501E-16,0.));", "#271 = CARTESIAN_POINT('',(2.E-12,0.));"},
		}},
		{"axial knot insufficient even at correct speed", [][2]string{
			{"(0.,5.E+03),.PIECEWISE_BEZIER_KNOTS.);\n#567", "(0.,4999.),.PIECEWISE_BEZIER_KNOTS.);\n#567"},
			{"#568 = CARTESIAN_POINT('',(2.214297435588,5.));", "#568 = CARTESIAN_POINT('',(2.214297435588,4.999));"},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			rejectChangedGroupedConversion(t, raw, fourConvexArcDistance, test.edits)
		})
	}
	automatic, err := os.ReadFile(fourConvertedMetreSource + "-automatic-plane.step")
	if err != nil {
		t.Fatal(err)
	}
	checkFourConvexArcAutomaticPlane(t, automatic, [2][2]int64{{44, 305}, {99, 356}})
}

func TestRepairFourConvertedMetreConvexArcLive(t *testing.T) {
	checkFourConvertedConvexArcLive(t, groupedConvertedMetre)
}

func TestRepairFourConvertedInchConvexArcLive(t *testing.T) {
	checkFourConvertedConvexArcLive(t, groupedConvertedInch)
}

func checkFourConvertedConvexArcLive(t *testing.T, tc groupedConvertedFixture) {
	t.Helper()
	oracle := os.Getenv("PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL")
	if oracle == "" {
		t.Skip("set PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL to the pinned author helper")
	}
	dir := t.TempDir()
	author := func(suffix string) (string, []byte) {
		path := filepath.Join(dir, "original"+suffix+".step")
		if out, err := exec.Command(oracle, "write-four-default-tilted-converted-"+tc.unit.name+suffix, path).CombinedOutput(); err != nil {
			t.Fatalf("pinned OCCT author: %v\n%s", err, out)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return path, raw
	}
	original, raw := author("")
	m, canonical := checkFourConvertedConvexArc(t, raw, tc)
	out, err := exec.Command(oracle, "four-converted-"+tc.unit.name+"-witness", original).CombinedOutput()
	var owners struct {
		PlaneLines, PlaneCircles, CylinderLines, CylinderCircles, RationalCaps  int
		ShiftedCylinderCircles, ShiftedSamples                                  int
		MaxSourceDistance, WithoutPlanarMap, MaxShiftedDistance, MaxCapDistance float64
	}
	shiftLimit := 1.3e-13
	if tc.unit.name == "inch" {
		shiftLimit = 6.1e-13
	}
	if err != nil || json.Unmarshal(out, &owners) != nil ||
		owners.PlaneLines != 24 || owners.PlaneCircles != 6 || owners.CylinderLines != 8 || owners.CylinderCircles != 8 || owners.RationalCaps != 2 ||
		owners.ShiftedCylinderCircles != 4 || owners.ShiftedSamples != 12 || owners.MaxSourceDistance > tc.maxDistance ||
		owners.MaxShiftedDistance > shiftLimit || owners.MaxCapDistance > tc.maxDistance || owners.WithoutPlanarMap < 100 {
		t.Fatalf("pinned OCCT did not witness the original converted charts: %v\n%s", err, out)
	}
	want := rationalTiltUnitMeasurements{Volume: 22.155304183748054, Area: 51.012178844414059,
		Centroid: [3]float64{2.5325644799355218, 1.1687478248848513, .789914026752639}, Faces: 10, Solids: 1}
	scaled := want
	if tc.unit.name == "inch" {
		scaled.Volume *= 125
		scaled.Area *= 25
		for i := range scaled.Centroid {
			scaled.Centroid[i] *= 5
		}
	}
	for _, measured := range readRationalTiltUnitWorldSTEP(t, oracle, original, m.CorrectedModelSTEP(), canonical) {
		checkScaledRationalTiltUnitMeasures(t, tc.unit, measured, scaled)
	}
	var negative string
	negativeUnit := tc.unit
	if tc.unit.name == "inch" {
		negative, raw = author("-unscaled")
		checkFourConvertedUnscaledInch(t, raw)
		negativeUnit = fourConvexArcUnit
	} else {
		negative, raw = author("-automatic-plane")
		checkFourConvexArcAutomaticPlane(t, raw, [2][2]int64{{44, 305}, {99, 356}})
	}
	out, err = exec.Command(oracle, "read", negative).CombinedOutput()
	var measured rationalTiltUnitMeasurements
	if err != nil || json.Unmarshal(out, &measured) != nil {
		t.Fatalf("pinned OCCT could not transfer the negative original: %v\n%s", err, out)
	}
	checkScaledRationalTiltUnitMeasures(t, negativeUnit, measured, want)
}

func TestRepairFourConvertedInchConvexArcRejectsChangedCharts(t *testing.T) {
	raw, err := os.ReadFile(fourConvertedInchSource + ".step")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name  string
		edits [][2]string
	}{
		{"inexact inch factor", [][2]string{{"LENGTH_MEASURE_WITH_UNIT(25.4,", "LENGTH_MEASURE_WITH_UNIT(25.40000000000000001,"}}},
		{"changed original model LINE speed", [][2]string{{"#29 = VECTOR('',#30,3.937007874016E-02);", "#29 = VECTOR('',#30,3.93700787401600001E-02);"}}},
		{"short axial span at unchanged speed", [][2]string{
			{"(0.,635.),.PIECEWISE_BEZIER_KNOTS.);\n#567", "(0.,508.),.PIECEWISE_BEZIER_KNOTS.);\n#567"},
			{"#568 = CARTESIAN_POINT('',(2.214297435588,25.));", "#568 = CARTESIAN_POINT('',(2.214297435588,20.));"},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			rejectChangedGroupedConversion(t, raw, groupedConvertedInch.maxDistance, test.edits)
		})
	}
}

func checkFourConvertedUnscaledInch(t *testing.T, raw []byte) {
	t.Helper()
	checkUnscaledInchSupports(t, raw, []roundedInchSupport{
		{333, 558, 338, "1/5000000000000000"},
		{387, 585, 392, "1/250000000000000"},
		{612, 666, 619, "351/2500000000000000"},
	})
}

func TestRepairFourConvertedInchConvexArcRejectsRoundedSupports(t *testing.T) {
	raw, err := os.ReadFile(fourConvertedInchSource + "-unscaled.step")
	if err != nil {
		t.Fatal(err)
	}
	checkFourConvertedUnscaledInch(t, raw)
}
