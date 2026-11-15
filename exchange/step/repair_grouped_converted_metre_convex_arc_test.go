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

const groupedConvertedMetreFixture = "testdata/occt-both-default-tilted-oblique-prism-fillet-converted-metre.step"

func checkGroupedConvertedMetre(t *testing.T, raw []byte) (RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion, []byte) {
	t.Helper()
	ctx := context.Background()
	if _, err := ReadRationalPosedExactCircularLoopExtrusion(ctx, bytes.NewReader(raw), Limits{}); err == nil {
		t.Fatal("original grouped conversion unexpectedly qualified as strict STEP")
	}
	m, err := RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{MaxDistance: 1e-12})
	if err != nil {
		t.Fatal(err)
	}
	trims := m.AxialTrims()
	spans, lines := m.CylindricalSpanOwners(), m.FiniteLineOwners()
	if m.OriginalSTEP() != string(raw) || m.LocalDescriptor().LengthUnit != 1 || m.LocalDescriptor().Height != 5 ||
		m.ReferenceCircleID() != 144 || len(m.Repairs()) != 8 || len(m.Reversals()) != 2 || len(m.PCurves()) != 28 ||
		len(lines) != 18 || len(spans) != 8 || len(trims) != 1 || trims[0].EdgeID != 248 {
		t.Fatal("grouped conversion lost its exact model or source-owner evidence")
	}
	for _, line := range lines {
		if line.PCurveParameterScale == nil || line.PCurveParameterScale.RatString() != "1/1000" {
			t.Fatal("changed original planar owner map", line)
		}
		if line.PCurveID == 262 && (line.StartParameter.Cmp(trims[0].OriginalParameters[0]) != 0 ||
			line.EndParameter.Cmp(trims[0].OriginalParameters[1]) != 0) {
			t.Fatal("planar owner lost the original directed unequal trim", line)
		}
	}
	const sweep = "2498091544797/1000000000000"
	const quarter = "314159265359/200000000000"
	want := map[int64][5]string{
		71: {"5000", "0", "0", "0", "1/1000"}, 124: {"5000", "0", "0", "0", "1/1000"},
		156: {sweep, "0", "0", "1", "0"}, 329: {sweep, "0", "5", "1", "0"},
		237: {quarter, "0", "0", "-1", "0"}, 277: {quarter, "0", "5", "-1", "0"},
		256: {"5000", "-" + quarter, "0", "0", "1/1000"}, 308: {"5000", sweep, "0", "0", "1/1000"},
	}
	for _, span := range spans {
		coefficients := [5]string{span.ParameterInterval[1].RatString(), span.UVStart[0].RatString(), span.UVStart[1].RatString(),
			span.UVDerivative[0].RatString(), span.UVDerivative[1].RatString()}
		if span.ParameterInterval[0].Sign() != 0 || coefficients != want[span.PCurveID] {
			t.Fatal("changed original cylindrical coefficients", span)
		}
		delete(want, span.PCurveID)
		if span.PCurveID == 256 {
			checkConvertedGroupAxialMap(t, span, trims[0])
		} else if span.RepairedToOwner != nil {
			t.Fatal("unexpected change in a cylindrical parameter", span)
		}
	}
	caps, finite := m.RationalCaps(), m.FiniteTrims()
	if len(caps) != 2 || len(finite) != 2 || caps[0].Owner.PCurveID != 226 || caps[1].Owner.PCurveID != 283 {
		t.Fatal("grouped conversion lost an original rational cap")
	}
	for i, cap := range caps {
		trim := finite[i]
		if !cap.PositiveAngularVelocity || !cap.ClosedOneTurn || cap.RadialDistanceUpper.Sign() <= 0 ||
			cap.RadialDistanceUpper.Cmp(big.NewRat(1, 1e12)) > 0 || trim.CapOwner.PCurveID != cap.Owner.PCurveID ||
			trim.CylinderOwner.PCurveID != [2]int64{237, 277}[i] || trim.CylinderOwner.Kind != "finite-reversed-cylinder-circle" ||
			trim.EndStation == nil || trim.SourceStart.Sign() != 0 || trim.SourceEnd.Lower.Cmp(trim.SourceEnd.Upper) >= 0 {
			t.Fatal("grouped source cap lost its finite reversed cylinder pairing", trim)
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
	return m, checkRationalTiltUnitCanonical(t, rationalTiltUnitFixtures[0], m, new(big.Rat), new(big.Rat))
}

func checkConvertedGroupAxialMap(t *testing.T, span ConvexArcCylinderSpanOwner, trim ConvexArcAxialTrimRepair) {
	t.Helper()
	mapping := span.RepairedToOwner
	if mapping == nil || mapping[1].Sign() <= 0 || trim.OriginalParameters[0].Sign() != 0 ||
		trim.RepairedParameters[0].RatString() != "-3/500000000000000000000000000" {
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
	ctx := context.Background()
	raw, err := os.ReadFile(groupedConvertedMetreFixture)
	if err != nil {
		t.Fatal(err)
	}
	m, _ := checkGroupedConvertedMetre(t, raw)
	if rejected, err := RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{MaxDistance: 1e-13}); err == nil || rejected.OriginalSTEP() != "" {
		t.Fatal("grouped conversion accepted an insufficient owner budget")
	}
	var record bytes.Buffer
	if err := WriteRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, &record, m); err != nil {
		t.Fatal(err)
	}
	reloaded, err := ReadRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, bytes.NewReader(record.Bytes()), IndependentMaterialJSONLimits{})
	if err != nil || reloaded.OriginalSTEP() != string(raw) || reloaded.LocalDescriptor().LengthUnit != 1 ||
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
	raw, err := os.ReadFile(groupedConvertedMetreFixture)
	if err != nil {
		t.Fatal(err)
	}
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
			mutated := raw
			for _, edit := range test.edits {
				changed := bytes.Replace(mutated, []byte(edit[0]), []byte(edit[1]), 1)
				if bytes.Equal(changed, mutated) {
					t.Fatal("chart mutation did not apply", edit[0])
				}
				mutated = changed
			}
			m, err := RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion(context.Background(), bytes.NewReader(mutated), Limits{}, ConvexArcRepairOptions{MaxDistance: 1e-12})
			if err == nil || m.OriginalSTEP() != "" {
				t.Fatal("changed chart yielded repaired material", err)
			}
		})
	}
}

func TestRepairGroupedConvertedMetreConvexArcLive(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL")
	if oracle == "" {
		t.Skip("set PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL to the pinned author helper")
	}
	input := filepath.Join(t.TempDir(), "original.step")
	if out, err := exec.Command(oracle, "write-both-default-tilted-converted-metre", input).CombinedOutput(); err != nil {
		t.Fatalf("pinned OCCT author: %v\n%s", err, out)
	}
	raw, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	m, canonical := checkGroupedConvertedMetre(t, raw)
	witness, err := exec.Command(oracle, "grouped-converted-metre-witness", input).CombinedOutput()
	if err != nil {
		t.Fatalf("pinned OCCT original owner evaluation: %v\n%s", err, witness)
	}
	var owners struct {
		PlaneLines, PlaneCircles, CylinderLines, CylinderCircles, RationalCaps int
		MaxSourceDistance, WithoutPlanarMap, MaxCapDistance                    float64
	}
	if err = json.Unmarshal(witness, &owners); err != nil || owners.PlaneLines != 18 || owners.PlaneCircles != 2 ||
		owners.CylinderLines != 4 || owners.CylinderCircles != 4 || owners.RationalCaps != 2 ||
		owners.MaxSourceDistance <= 0 || owners.MaxSourceDistance > 1e-12 || owners.WithoutPlanarMap < 1000 ||
		owners.MaxCapDistance <= 0 || owners.MaxCapDistance > 1e-12 {
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
	if err = json.Unmarshal(witness, &caps); err != nil || caps.Caps != 2 || caps.MaxEndpointDistance <= 0 || caps.MaxEndpointDistance > 1e-12 {
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
		checkRationalTiltGroupedScaledMeasures(t, rationalTiltUnitFixtures[0], got)
	}
}
