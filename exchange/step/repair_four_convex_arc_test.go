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

const fourConvexArcSource = "testdata/occt-four-default-tilted-trapezoidal-prism-fillet"
const fourConvexArcDistance = 1e-12

var fourConvexArcUnit = rationalTiltUnitFixture{name: "millimetre", metresPerUnit: .001, mmNumerator: 1, mmDenominator: 1}

func checkFourConvexArcSource(t *testing.T, raw []byte) (RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion, []byte) {
	t.Helper()
	ctx := context.Background()
	if _, err := ReadOCCTExactCircularLoopExtrusion(ctx, bytes.NewReader(raw), Limits{}); err == nil {
		t.Fatal("strict STEP accepted the unrepaired four-fillet owner graph")
	}
	m, err := RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{MaxDistance: fourConvexArcDistance})
	if err != nil {
		t.Fatal(err)
	}
	d := m.LocalDescriptor()
	if m.OriginalSTEP() != string(raw) || m.ReferenceCircleID() != 146 || d.LengthUnit != .001 || d.Height != 5 || len(d.Segments) != 8 ||
		len(m.Solid().Faces()) != 10 || len(m.Repairs()) != 10 || len(m.Tangencies()) != 16 || len(m.Reversals()) != 2 ||
		len(m.FiniteLineOwners()) != 8 || len(m.CylindricalSpanOwners()) != 0 || len(m.PCurves()) != 46 {
		t.Fatal("four-fillet repair lost original geometry or owner evidence")
	}
	var radii []float64
	for i, segment := range d.Segments {
		if (segment.Radius == 0) == (d.Segments[(i+1)%len(d.Segments)].Radius == 0) {
			t.Fatal("lost a straight span between adjacent rounds", i)
		}
		if segment.Radius > 0 {
			radii = append(radii, segment.Radius)
		}
	}
	slices.Sort(radii)
	if !slices.Equal(radii, []float64{5. / 32, 1. / 4, 5. / 16, 3. / 8}) {
		t.Fatal("four independent round radii changed", radii)
	}
	pose, err := m.Pose().Matrix()
	if err != nil {
		t.Fatal(err)
	}
	wantPose := [3][4]string{{"0", "3/5", "4/5", "0"}, {"-1", "0", "0", "0"}, {"0", "-4/5", "3/5", "0"}}
	for i := range pose {
		for j := range pose[i] {
			if pose[i][j].RatString() != wantPose[i][j] {
				t.Fatal("source world pose changed", pose)
			}
		}
	}

	f, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	const quarter = "-314159265359/200000000000"
	const oblique = "-2498091544797/1000000000000"
	wantPhases := map[int64][2]string{210: {quarter, "0"}, 267: {oblique, "0"}, 482: {quarter, "5"}, 529: {oblique, "5"}}
	wantKinds := map[string]int{"plane-line": 16, "finite-plane-line": 8, "cylinder-line": 8, "plane-circle": 6,
		"cylinder-circle": 4, "phase-cylinder-circle": 4}
	phaseLimit, _ := new(big.Rat).SetString("1.2e-26")
	for _, owner := range m.PCurves() {
		wantKinds[owner.Kind]--
		if owner.Kind != "phase-cylinder-circle" {
			continue
		}
		phase, ok := wantPhases[owner.PCurveID]
		uv, speed, err := convexUVLine(f, owner.GeometryID, 2)
		if !ok || err != nil || uv[0].RatString() != phase[0] || uv[1].RatString() != phase[1] ||
			speed[0].RatString() != "1" || speed[1].Sign() != 0 || owner.DistanceSquaredUpper.Sign() <= 0 ||
			owner.DistanceSquaredUpper.Cmp(phaseLimit) > 0 {
			t.Fatal("original phase law or its full-span bound changed", owner.PCurveID, err)
		}
		delete(wantPhases, owner.PCurveID)
	}
	for kind, missing := range wantKinds {
		if missing != 0 {
			t.Fatal("wrong number of original owner proofs", kind, missing)
		}
	}
	if len(wantPhases) != 0 {
		t.Fatal("missing original cylinder phase", wantPhases)
	}
	trims := m.AxialTrims()
	if len(trims) != 2 {
		t.Fatal("missing unequal axial interval proof")
	}
	for i, trim := range trims {
		wantStarts := [2]string{"-3/500000000000000000000000000000", "1/25000000000000000000000000000"}
		if trim.EdgeID != [2]int64{338, 563}[i] || trim.OriginalParameters[0].Sign() != 0 ||
			trim.RepairedParameters[0].RatString() != wantStarts[i] ||
			trim.OriginalParameters[1].Cmp(trim.RepairedParameters[1]) != 0 {
			t.Fatal("unequal axial source stations changed", trim)
		}
	}
	caps, finite := m.RationalCaps(), m.FiniteTrims()
	if len(caps) != 2 || len(finite) != 2 {
		t.Fatal("missing the two original rational caps")
	}
	for i, cap := range caps {
		trim := finite[i]
		if !cap.PositiveAngularVelocity || !cap.ClosedOneTurn || cap.Owner.PCurveID != [2]int64{315, 375}[i] ||
			trim.CapOwner.PCurveID != cap.Owner.PCurveID || trim.CylinderOwner.PCurveID != [2]int64{326, 368}[i] ||
			trim.EndStation == nil || trim.SourceStart.Sign() != 0 || trim.SourceEnd.Lower.Cmp(trim.SourceEnd.Upper) >= 0 {
			t.Fatal("original cap lost its finite rational proof", cap.Owner.PCurveID)
		}
		curve, err := simple(f, trim.CapOwner.SurfaceCurveID, "SURFACE_CURVE", 4)
		if err != nil {
			t.Fatal(err)
		}
		order, err := listRefs(curve.Args[2], 2)
		if err != nil || !slices.Equal(order, [2][]int64{{315, 326}, {368, 375}}[i]) {
			t.Fatal("original cap and cylinder owner order changed", order, err)
		}
	}
	return m, checkRationalTiltUnitCanonical(t, fourConvexArcUnit, m, new(big.Rat), new(big.Rat))
}

func TestRepairFourConvexArcSource(t *testing.T) {
	raw, err := os.ReadFile(fourConvexArcSource + ".step")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	m, _ := checkFourConvexArcSource(t, raw)
	rejectChangedGroupedConversion(t, raw, 1e-13, nil)
	var proof bytes.Buffer
	if err := WriteRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, &proof, m); err != nil {
		t.Fatal(err)
	}
	reloaded, err := ReadRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, bytes.NewReader(proof.Bytes()), IndependentMaterialJSONLimits{})
	if err != nil || reloaded.OriginalSTEP() != string(raw) || !reflect.DeepEqual(reloaded.PCurves(), m.PCurves()) ||
		!reflect.DeepEqual(reloaded.AxialTrims(), m.AxialTrims()) || !reflect.DeepEqual(reloaded.FiniteLineOwners(), m.FiniteLineOwners()) ||
		!reflect.DeepEqual(reloaded.RationalCaps(), m.RationalCaps()) || !reflect.DeepEqual(reloaded.FiniteTrims(), m.FiniteTrims()) {
		t.Fatal("source-only reader lost original four-fillet owner evidence", err)
	}
	var again bytes.Buffer
	if err := WriteRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, &again, reloaded); err != nil || !bytes.Equal(proof.Bytes(), again.Bytes()) {
		t.Fatal("source-only proof changed its bytes on replay", err)
	}
	forged := bytes.Replace(proof.Bytes(), []byte(`"Kind":"phase-cylinder-circle"`), []byte(`"Kind":"cylinder-circle"`), 1)
	if bytes.Equal(forged, proof.Bytes()) {
		t.Fatal("source-only receipt missed shifted owner claim")
	}
	if _, err := ReadRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, bytes.NewReader(forged), IndependentMaterialJSONLimits{}); err == nil {
		t.Fatal("source-only reader accepted a forged phase claim")
	}
}

func TestRepairFourConvexArcRejectsChangedOwners(t *testing.T) {
	raw, err := os.ReadFile(fourConvexArcSource + ".step")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name  string
		edits [][2]string
	}{
		{"erased quarter phase", [][2]string{{"#218 = CARTESIAN_POINT('',(-1.570796326795,0.));", "#218 = CARTESIAN_POINT('',(0.,0.));"}}},
		{"changed oblique phase", [][2]string{{"#275 = CARTESIAN_POINT('',(-2.498091544797,0.));", "#275 = CARTESIAN_POINT('',(-2.49,0.));"}}},
		{"borrowed opposite cap owner", [][2]string{{"#197 = SURFACE_CURVE('',#198,(#203,#210),.PCURVE_S1.);", "#197 = SURFACE_CURVE('',#198,(#203,#482),.PCURVE_S1.);"}}},
		{"additional body in original root", [][2]string{
			{"#10 = ADVANCED_BREP_SHAPE_REPRESENTATION('',(#11,#15),#673);", "#10 = ADVANCED_BREP_SHAPE_REPRESENTATION('',(#11,#15,#679),#673);"},
			{"ENDSEC;\nEND-ISO-10303-21;", "#679 = MANIFOLD_SOLID_BREP('',#16);\nENDSEC;\nEND-ISO-10303-21;"},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			rejectChangedGroupedConversion(t, raw, fourConvexArcDistance, test.edits)
		})
	}
	if m, err := RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion(context.Background(), bytes.NewReader(raw), Limits{MaxEntities: 677},
		ConvexArcRepairOptions{MaxDistance: fourConvexArcDistance}); err == nil || m.OriginalSTEP() != "" {
		t.Fatal("qualified source ignored the STEP entity limit")
	}
}

func checkFourConvexArcAutomaticPlane(t *testing.T, raw []byte) {
	t.Helper()
	f, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	for _, pair := range [2][2]int64{{44, 310}, {100, 363}} {
		plane, err := simple(f, pair[0], "PLANE", 2)
		if err != nil {
			t.Fatal(err)
		}
		circle, err := simple(f, pair[1], "CIRCLE", 3)
		if err != nil {
			t.Fatal(err)
		}
		p, normal, _, err := occtLoopFrame(f, plane.Args[1].Ref)
		if err != nil {
			t.Fatal(err)
		}
		center, axis, _, err := occtLoopFrame(f, circle.Args[1].Ref)
		if err != nil || !convexUVUnit(normal[:]) || !occtLoopParallel(normal, axis) ||
			occtLoopDot(occtLoopSub(center, p), normal).RatString() != "1/5000000000000" {
			t.Fatal("automatic cap plane lost its exact axial offset from its circle", pair, err)
		}
	}
	m, err := RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion(context.Background(), bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{MaxDistance: fourConvexArcDistance})
	if !errors.Is(err, geom.ErrDomain) || m.OriginalSTEP() != "" {
		t.Fatal("automatic cap plane published an incompatible exact circle trim", err)
	}
}

func TestRepairFourConvexArcRejectsIndependentControls(t *testing.T) {
	for _, suffix := range []string{"-nondyadic", "-automatic-plane"} {
		t.Run(suffix[1:], func(t *testing.T) {
			raw, err := os.ReadFile(fourConvexArcSource + suffix + ".step")
			if err != nil {
				t.Fatal(err)
			}
			if suffix == "-nondyadic" {
				checkGroupedConvexArcUnrepresentable(t, raw, fourConvexArcDistance)
			} else {
				checkFourConvexArcAutomaticPlane(t, raw)
			}
		})
	}
}

func TestRepairFourConvexArcLive(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL")
	if oracle == "" {
		t.Skip("set PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL to the pinned author helper")
	}
	dir := t.TempDir()
	author := func(suffix string) (string, []byte) {
		path := filepath.Join(dir, "original"+suffix+".step")
		if out, err := exec.Command(oracle, "write-four-default-tilted"+suffix, path).CombinedOutput(); err != nil {
			t.Fatalf("pinned OCCT author: %v\n%s", err, out)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return path, raw
	}
	original, raw := author("")
	m, canonical := checkFourConvexArcSource(t, raw)
	evaluate := func(mode string, report any) {
		out, err := exec.Command(oracle, mode, original).CombinedOutput()
		if err != nil {
			t.Fatalf("pinned OCCT %s: %v\n%s", mode, err, out)
		}
		if err := json.Unmarshal(out, report); err != nil {
			t.Fatalf("pinned OCCT %s report: %v\n%s", mode, err, out)
		}
	}
	var owners struct {
		PlaneLines, PlaneCircles, CylinderLines, CylinderCircles, RationalCaps int
		MaxSourceDistance, MaxCapDistance                                      float64
	}
	evaluate("four-owner-witness", &owners)
	if owners.PlaneLines != 24 || owners.PlaneCircles != 6 || owners.CylinderLines != 8 || owners.CylinderCircles != 8 ||
		owners.RationalCaps != 2 || owners.MaxSourceDistance > fourConvexArcDistance || owners.MaxCapDistance > fourConvexArcDistance {
		t.Fatal("independent samples lost original owner coverage", owners)
	}
	var phase struct {
		Owners, Samples                 int
		MaxSourceDistance, WithoutPhase float64
	}
	evaluate("four-shifted-cylinder-witness", &phase)
	if phase.Owners != 4 || phase.Samples != 12 || phase.MaxSourceDistance > 1.1e-13 || phase.WithoutPhase < .4 {
		t.Fatal("independent samples lost the original cylinder phase", phase)
	}
	var cap struct {
		Caps                int
		MaxEndpointDistance float64
	}
	evaluate("cap-trim", &cap)
	if cap.Caps != 2 || cap.MaxEndpointDistance > fourConvexArcDistance {
		t.Fatal("independent samples lost the rational cap trim", cap)
	}
	want := rationalTiltUnitMeasurements{Volume: 22.155304183748054, Area: 51.012178844414059,
		Centroid: [3]float64{2.5325644799355218, 1.1687478248848513, .789914026752639}, Faces: 10, Solids: 1}
	for _, measured := range readRationalTiltUnitWorldSTEP(t, oracle, original, m.CorrectedModelSTEP(), canonical) {
		checkScaledRationalTiltUnitMeasures(t, fourConvexArcUnit, measured, want)
	}
	_, raw = author("-nondyadic")
	checkGroupedConvexArcUnrepresentable(t, raw, fourConvexArcDistance)
	automatic, raw := author("-automatic-plane")
	checkFourConvexArcAutomaticPlane(t, raw)
	out, err := exec.Command(oracle, "read", automatic).CombinedOutput()
	var rounded rationalTiltUnitMeasurements
	if err != nil || json.Unmarshal(out, &rounded) != nil {
		t.Fatalf("pinned OCCT automatic-plane read: %v\n%s", err, out)
	}
	checkScaledRationalTiltUnitMeasures(t, fourConvexArcUnit, rounded, want)
}
