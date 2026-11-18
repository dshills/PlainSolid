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
	"strconv"
	"testing"

	"plainsolid/geom"
)

type groupedConvertedFixture struct {
	unit                                  rationalTiltUnitFixture
	maxDistance, rejectedDistance, height float64
	ownerScale, axialEnd, repairedStart   string
}

var groupedConvertedMetre = groupedConvertedFixture{
	unit: rationalTiltUnitFixtures[0], maxDistance: 1e-12, rejectedDistance: 1e-13, height: 5,
	ownerScale: "1/1000", axialEnd: "5000", repairedStart: "-3/500000000000000000000000000",
}

var groupedConvertedInch = groupedConvertedFixture{
	unit: rationalTiltUnitFixtures[1], maxDistance: 1e-11, rejectedDistance: 1e-12, height: 25,
	ownerScale: "5/127", axialEnd: "635", repairedStart: "1/984251968504000000000000000",
}

func (tc groupedConvertedFixture) source(t *testing.T) []byte {
	t.Helper()
	return tc.unit.source(t, groupedTiltUnitSource+"-converted")
}

func checkGroupedConvertedConvexArc(t *testing.T, raw []byte, tc groupedConvertedFixture) (RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion, []byte) {
	t.Helper()
	ctx := context.Background()
	if _, err := ReadRationalPosedExactCircularLoopExtrusion(ctx, bytes.NewReader(raw), Limits{}); err == nil {
		t.Fatal("original grouped conversion unexpectedly qualified as strict STEP")
	}
	m, err := RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{MaxDistance: tc.maxDistance})
	if err != nil {
		t.Fatal(err)
	}
	trims := m.AxialTrims()
	spans, lines := m.CylindricalSpanOwners(), m.FiniteLineOwners()
	if m.OriginalSTEP() != string(raw) || m.LocalDescriptor().LengthUnit != tc.unit.metresPerUnit || m.LocalDescriptor().Height != tc.height ||
		m.ReferenceCircleID() != 144 || len(m.Repairs()) != 8 || len(m.Reversals()) != 2 || len(m.PCurves()) != 28 ||
		len(lines) != 18 || len(spans) != 8 || len(trims) != 1 || trims[0].EdgeID != 248 {
		t.Fatal("grouped conversion lost its exact model or source-owner evidence")
	}
	for _, line := range lines {
		if line.PCurveParameterScale == nil || line.PCurveParameterScale.RatString() != tc.ownerScale {
			t.Fatal("changed original planar owner map", line)
		}
		if line.PCurveID == 262 && (line.StartParameter.Cmp(trims[0].OriginalParameters[0]) != 0 ||
			line.EndParameter.Cmp(trims[0].OriginalParameters[1]) != 0) {
			t.Fatal("planar owner lost the original directed unequal trim", line)
		}
	}
	const sweep = "2498091544797/1000000000000"
	const quarter = "314159265359/200000000000"
	height := strconv.FormatFloat(tc.height, 'f', 0, 64)
	want := map[int64][5]string{
		71: {tc.axialEnd, "0", "0", "0", tc.ownerScale}, 124: {tc.axialEnd, "0", "0", "0", tc.ownerScale},
		156: {sweep, "0", "0", "1", "0"}, 329: {sweep, "0", height, "1", "0"},
		237: {quarter, "0", "0", "-1", "0"}, 277: {quarter, "0", height, "-1", "0"},
		256: {tc.axialEnd, "-" + quarter, "0", "0", tc.ownerScale}, 308: {tc.axialEnd, sweep, "0", "0", tc.ownerScale},
	}
	for _, span := range spans {
		coefficients := [5]string{span.ParameterInterval[1].RatString(), span.UVStart[0].RatString(), span.UVStart[1].RatString(),
			span.UVDerivative[0].RatString(), span.UVDerivative[1].RatString()}
		if span.ParameterInterval[0].Sign() != 0 || coefficients != want[span.PCurveID] {
			t.Fatal("changed original cylindrical coefficients", span)
		}
		delete(want, span.PCurveID)
		if span.PCurveID == 256 {
			checkConvertedGroupAxialMap(t, span, trims[0], tc.repairedStart)
		} else if span.RepairedToOwner != nil {
			t.Fatal("unexpected change in a cylindrical parameter", span)
		}
	}
	caps, finite := m.RationalCaps(), m.FiniteTrims()
	if len(caps) != 2 || len(finite) != 2 || caps[0].Owner.PCurveID != 226 || caps[1].Owner.PCurveID != 283 {
		t.Fatal("grouped conversion lost an original rational cap")
	}
	source, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	for i, cap := range caps {
		trim := finite[i]
		if !cap.PositiveAngularVelocity || !cap.ClosedOneTurn || cap.RadialDistanceUpper.Sign() <= 0 ||
			cap.RadialDistanceUpper.Cmp(new(big.Rat).SetFloat64(tc.maxDistance)) > 0 || trim.CapOwner.PCurveID != cap.Owner.PCurveID ||
			trim.CylinderOwner.PCurveID != [2]int64{237, 277}[i] || trim.CylinderOwner.Kind != "finite-reversed-cylinder-circle" ||
			trim.EndStation == nil || trim.SourceStart.Sign() != 0 || trim.SourceEnd.Lower.Cmp(trim.SourceEnd.Upper) >= 0 {
			t.Fatal("grouped source cap lost its finite reversed cylinder pairing", trim)
		}
		curve, err := simple(source, trim.CapOwner.SurfaceCurveID, "SURFACE_CURVE", 4)
		if err != nil || trim.CapOwner.SurfaceCurveID != trim.CylinderOwner.SurfaceCurveID {
			t.Fatal("cap and cylinder lost their original model curve", err)
		}
		owners, err := listRefs(curve.Args[2], 2)
		wantOrder := [][]int64{{226, 237}, {277, 283}}
		if err != nil || !reflect.DeepEqual(owners, wantOrder[i]) {
			t.Fatal("authored cap and cylinder order changed", owners, err)
		}
	}
	pose, err := m.Pose().Matrix()
	wantPose := [3][4]string{{"0", "3/5", "4/5", "0"}, {"-1", "0", "0", "0"}, {"0", "-4/5", "3/5", "0"}}
	if err != nil {
		t.Fatal(err)
	}
	for i := range pose {
		for j := range pose[i] {
			if pose[i][j].RatString() != wantPose[i][j] {
				t.Fatal("grouped conversion changed its exact world pose", pose)
			}
		}
	}
	return m, checkRationalTiltUnitCanonical(t, tc.unit, m, new(big.Rat), new(big.Rat))
}

func checkConvertedGroupAxialMap(t *testing.T, span ConvexArcCylinderSpanOwner, trim ConvexArcAxialTrimRepair, repairedStart string) {
	t.Helper()
	mapping := span.RepairedToOwner
	if mapping == nil || mapping[1].Sign() <= 0 || trim.OriginalParameters[0].Sign() != 0 ||
		trim.RepairedParameters[0].RatString() != repairedStart {
		t.Fatal("unequal axial correction used no finite map", trim)
	}
	for i, endpoint := range trim.RepairedParameters {
		u := new(big.Rat).Add(mapping[0], new(big.Rat).Mul(mapping[1], endpoint))
		if u.Cmp(trim.OriginalParameters[i]) != 0 || u.Cmp(span.ParameterInterval[0]) < 0 || u.Cmp(span.ParameterInterval[1]) > 0 {
			t.Fatal("finite map leaves the original authored knots", u)
		}
	}
}

func TestRepairGroupedConvertedMetreConvexArcSource(t *testing.T) {
	t.Parallel()
	checkGroupedConvertedSource(t, groupedConvertedMetre)
}

func TestRepairGroupedConvertedInchConvexArcSource(t *testing.T) {
	t.Parallel()
	checkGroupedConvertedSource(t, groupedConvertedInch)
}

func checkGroupedConvertedSource(t *testing.T, tc groupedConvertedFixture) {
	t.Helper()
	ctx := context.Background()
	raw := tc.source(t)
	m, _ := checkGroupedConvertedConvexArc(t, raw, tc)
	if rejected, err := RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{MaxDistance: tc.rejectedDistance}); err == nil || rejected.OriginalSTEP() != "" {
		t.Fatal("grouped conversion accepted an insufficient owner budget")
	}
	var record bytes.Buffer
	if err := WriteRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, &record, m); err != nil {
		t.Fatal(err)
	}
	reloaded, err := ReadRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, bytes.NewReader(record.Bytes()), IndependentMaterialJSONLimits{})
	if err != nil || reloaded.OriginalSTEP() != string(raw) || reloaded.LocalDescriptor().LengthUnit != tc.unit.metresPerUnit ||
		!reflect.DeepEqual(reloaded.FiniteLineOwners(), m.FiniteLineOwners()) || !reflect.DeepEqual(reloaded.CylindricalSpanOwners(), m.CylindricalSpanOwners()) ||
		!reflect.DeepEqual(reloaded.AxialTrims(), m.AxialTrims()) || !reflect.DeepEqual(reloaded.RationalCaps(), m.RationalCaps()) {
		t.Fatal("native reproof lost the original grouped chart ledgers", err)
	}
	var again bytes.Buffer
	if err := WriteRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, &again, reloaded); err != nil || !bytes.Equal(record.Bytes(), again.Bytes()) {
		t.Fatal("native grouped certificate bytes changed on reproof", err)
	}
	forged := bytes.Replace(record.Bytes(), []byte(`"RepairedToOwner":["`), []byte(`"RepairedToOwner":["1`), 1)
	if bytes.Equal(forged, record.Bytes()) {
		t.Fatal("missing axial parameter map in grouped receipt")
	}
	if _, err := ReadRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, bytes.NewReader(forged), IndependentMaterialJSONLimits{}); err == nil {
		t.Fatal("native reproof accepted a forged finite parameter map")
	}
}

func TestRepairGroupedConvertedMetreConvexArcRejectsChangedCharts(t *testing.T) {
	t.Parallel()
	raw := groupedConvertedMetre.source(t)
	for _, test := range []struct {
		name  string
		edits [][2]string
	}{
		{"short reversed quarter span with unchanged slope", [][2]string{
			{"(0.,1.570796326795),.PIECEWISE_BEZIER_KNOTS.", "(0.,1.570796326794),.PIECEWISE_BEZIER_KNOTS."},
			{"#241 = CARTESIAN_POINT('',(-1.570796326795,0.));", "#241 = CARTESIAN_POINT('',(-1.570796326794,0.));"},
		}},
		{"reversed chart with positive sweep", [][2]string{
			{"#241 = CARTESIAN_POINT('',(-1.570796326795,0.));", "#241 = CARTESIAN_POINT('',(1.570796326795,0.));"},
		}},
		{"axial chart too short to contain the original interval", [][2]string{
			{"(0.,5.E+03),.PIECEWISE_BEZIER_KNOTS.);\n#259", "(0.,4.E+03),.PIECEWISE_BEZIER_KNOTS.);\n#259"},
			{"#260 = CARTESIAN_POINT('',(-1.570796326795,5.));", "#260 = CARTESIAN_POINT('',(-1.570796326795,4.));"},
		}},
		{"changed rational cap pole", [][2]string{{"#230 = CARTESIAN_POINT('',(-1.683012701892,-1.));", "#230 = CARTESIAN_POINT('',(-1.68,-1.));"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			rejectChangedGroupedConversion(t, raw, groupedConvertedMetre.maxDistance, test.edits)
		})
	}
}

func TestRepairGroupedConvertedInchConvexArcRejectsChangedCharts(t *testing.T) {
	t.Parallel()
	raw := groupedConvertedInch.source(t)
	for _, test := range []struct {
		name  string
		edits [][2]string
	}{
		{"rounded inch length-unit factor", [][2]string{{"LENGTH_MEASURE_WITH_UNIT(25.4,", "LENGTH_MEASURE_WITH_UNIT(25.40000000000000001,"}}},
		{"changed rounded model speed", [][2]string{{"#29 = VECTOR('',#30,3.937007874016E-02);", "#29 = VECTOR('',#30,3.93700787401600001E-02);"}}},
		{"reassigned cylindrical owner", [][2]string{{"#220 = SURFACE_CURVE('',#221,(#226,#237),.PCURVE_S1.);", "#220 = SURFACE_CURVE('',#221,(#226,#277),.PCURVE_S1.);"}}},
		{"changed master representation", [][2]string{{"#271 = SURFACE_CURVE('',#272,(#277,#283),.PCURVE_S1.);", "#271 = SURFACE_CURVE('',#272,(#277,#283),.PCURVE_S2.);"}}},
		{"short reversed quarter span with unchanged slope", [][2]string{
			{"(0.,1.570796326795),.PIECEWISE_BEZIER_KNOTS.", "(0.,1.570796326794),.PIECEWISE_BEZIER_KNOTS."},
			{"#241 = CARTESIAN_POINT('',(-1.570796326795,0.));", "#241 = CARTESIAN_POINT('',(-1.570796326794,0.));"},
		}},
		{"short axial chart with unchanged inch slope", [][2]string{
			{"(0.,635.),.PIECEWISE_BEZIER_KNOTS.);\n#259", "(0.,508.),.PIECEWISE_BEZIER_KNOTS.);\n#259"},
			{"#260 = CARTESIAN_POINT('',(-1.570796326795,25.));", "#260 = CARTESIAN_POINT('',(-1.570796326795,20.));"},
		}},
		{"changed rational cap pole", [][2]string{{"#230 = CARTESIAN_POINT('',(-8.415063509461,-5.));", "#230 = CARTESIAN_POINT('',(-8.41,-5.));"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			rejectChangedGroupedConversion(t, raw, groupedConvertedInch.maxDistance, test.edits)
		})
	}
}

func rejectChangedGroupedConversion(t *testing.T, raw []byte, maxDistance float64, edits [][2]string) {
	t.Helper()
	mutated := raw
	for _, edit := range edits {
		changed := bytes.Replace(mutated, []byte(edit[0]), []byte(edit[1]), 1)
		if bytes.Equal(changed, mutated) {
			t.Fatal("chart mutation did not apply", edit[0])
		}
		mutated = changed
	}
	m, err := RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion(context.Background(), bytes.NewReader(mutated), Limits{}, ConvexArcRepairOptions{MaxDistance: maxDistance})
	if err == nil || m.OriginalSTEP() != "" {
		t.Fatal("changed chart yielded repaired material", err)
	}
}

func TestRepairGroupedConvertedInchConvexArcRejectsRoundedSupports(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("testdata/" + groupedTiltUnitSource + "-converted-inch-unscaled.step")
	if err != nil {
		t.Fatal(err)
	}
	checkGroupedUnscaledInchSupports(t, raw)
}

func checkGroupedUnscaledInchSupports(t *testing.T, raw []byte) {
	t.Helper()
	checkUnscaledInchSupports(t, raw, []roundedInchSupport{
		{247, 401, 252, "1/5000000000000000"},
		{301, 428, 306, "147/625000000000000"},
	})
}

type roundedInchSupport struct {
	cylinder, plane, edge int64
	gap                   string
}

func checkUnscaledInchSupports(t *testing.T, raw []byte, pairs []roundedInchSupport) {
	t.Helper()
	source, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	for _, pair := range pairs {
		cylinder, err := occtLoopReadFace(source, pair.cylinder)
		if err != nil || cylinder.kind != "CYLINDRICAL_SURFACE" || !convexUVUnit(cylinder.z[:]) {
			t.Fatal("missing original cylindrical support", pair.cylinder, err)
		}
		plane, err := occtLoopReadFace(source, pair.plane)
		if err != nil || plane.kind != "PLANE" || !convexUVUnit(plane.z[:]) || occtLoopDot(cylinder.z, plane.z).Sign() != 0 {
			t.Fatal("missing original side-plane support", pair.plane, err)
		}
		for _, face := range []occtLoopFace{cylinder, plane} {
			found := false
			for _, use := range face.uses {
				found = found || use.id == pair.edge
			}
			if !found {
				t.Fatal("rounded supports no longer share the original edge", pair.edge)
			}
		}
		distance := new(big.Rat).Abs(occtLoopDot(occtLoopSub(cylinder.origin, plane.origin), plane.z))
		if gap := new(big.Rat).Sub(distance, cylinder.radius); gap.RatString() != pair.gap {
			t.Fatal("authored plane-to-cylinder tangency gap changed", pair, gap)
		}
	}
	m, err := RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion(context.Background(), bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{MaxDistance: 1e-9})
	if !errors.Is(err, geom.ErrDomain) || m.OriginalSTEP() != "" {
		t.Fatal("rounded support pairs yielded repaired material", err)
	}
}

func TestRepairGroupedConvertedMetreConvexArcLive(t *testing.T) {
	t.Parallel()
	checkGroupedConvertedLive(t, groupedConvertedMetre)
}

func TestRepairGroupedConvertedInchConvexArcLive(t *testing.T) {
	t.Parallel()
	checkGroupedConvertedLive(t, groupedConvertedInch)
}

func checkGroupedConvertedLive(t *testing.T, tc groupedConvertedFixture) {
	t.Helper()
	oracle := os.Getenv("PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL")
	if oracle == "" {
		t.Skip("set PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL to the pinned author helper")
	}
	input := filepath.Join(t.TempDir(), "original.step")
	if out, err := exec.Command(oracle, "write-both-default-tilted-converted-"+tc.unit.name, input).CombinedOutput(); err != nil {
		t.Fatalf("pinned OCCT author: %v\n%s", err, out)
	}
	raw, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	m, canonical := checkGroupedConvertedConvexArc(t, raw, tc)
	witness, err := exec.Command(oracle, "grouped-converted-"+tc.unit.name+"-witness", input).CombinedOutput()
	if err != nil {
		t.Fatalf("pinned OCCT original owner evaluation: %v\n%s", err, witness)
	}
	var owners struct {
		PlaneLines, PlaneCircles, CylinderLines, CylinderCircles, RationalCaps int
		MaxSourceDistance, WithoutPlanarMap, MaxCapDistance                    float64
	}
	if err = json.Unmarshal(witness, &owners); err != nil || owners.PlaneLines != 18 || owners.PlaneCircles != 2 ||
		owners.CylinderLines != 4 || owners.CylinderCircles != 4 || owners.RationalCaps != 2 ||
		owners.MaxSourceDistance <= 0 || owners.MaxSourceDistance > tc.maxDistance || owners.WithoutPlanarMap < 100 ||
		owners.MaxCapDistance <= 0 || owners.MaxCapDistance > tc.maxDistance {
		t.Fatalf("unexpected independent original-chart evaluation: %s (%v)", witness, err)
	}
	witness, err = exec.Command(oracle, "cap-trim", input).CombinedOutput()
	if err != nil {
		t.Fatalf("pinned OCCT cap endpoints: %v\n%s", err, witness)
	}
	var caps struct {
		Caps                                        int
		MinStation, MaxStation, MaxEndpointDistance float64
	}
	if err = json.Unmarshal(witness, &caps); err != nil || caps.Caps != 2 || caps.MaxEndpointDistance <= 0 || caps.MaxEndpointDistance > tc.maxDistance {
		t.Fatalf("unexpected independent finite cap evaluation: %s (%v)", witness, err)
	}
	for _, trim := range m.FiniteTrims() {
		lo, _ := trim.SourceEnd.Lower.Float64()
		hi, _ := trim.SourceEnd.Upper.Float64()
		if !finiteNumber(caps.MinStation) || !finiteNumber(caps.MaxStation) ||
			caps.MinStation < lo-1e-12 || caps.MaxStation > hi+1e-12 {
			t.Fatal("independent OCCT endpoint does not agree with the exact cap enclosure", caps, lo, hi)
		}
	}
	for _, got := range readRationalTiltUnitWorldSTEP(t, oracle, input, m.CorrectedModelSTEP(), canonical) {
		// The inch fixture's model is five times the source-unit base
		// profile. Combine that size with the STEP file's physical unit.
		physicalScale := tc.unit
		physicalScale.mmNumerator *= int64(tc.height / 5)
		checkRationalTiltGroupedScaledMeasures(t, physicalScale, got)
	}
	if tc.unit.name == "inch" {
		unscaled := filepath.Join(filepath.Dir(input), "unscaled.step")
		if out, err := exec.Command(oracle, "write-both-default-tilted-converted-inch-unscaled", unscaled).CombinedOutput(); err != nil {
			t.Fatalf("pinned OCCT unscaled author: %v\n%s", err, out)
		}
		raw, err := os.ReadFile(unscaled)
		if err != nil {
			t.Fatal(err)
		}
		checkGroupedUnscaledInchSupports(t, raw)
		if out, err := exec.Command(oracle, "read", unscaled).CombinedOutput(); err != nil {
			t.Fatalf("pinned tolerance-based OCCT read: %v\n%s", err, out)
		}
	}
}
