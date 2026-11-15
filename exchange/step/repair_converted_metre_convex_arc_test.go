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

const convertedMetreConvexArcFixture = "testdata/occt-default-tilted-translated-oblique-prism-fillet-converted-metre.step"

func checkConvertedMetreConvexArc(t *testing.T, raw []byte) (RationalPosedPCurveRepairedOCCTConvexArcExtrusion, []byte) {
	t.Helper()
	ctx := context.Background()
	if _, err := ReadOCCTExactCircularLoopExtrusion(ctx, bytes.NewReader(raw), Limits{}); err == nil {
		t.Fatal("converted original unexpectedly passed strict model-only import")
	}
	m, err := RepairRationalPosedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{MaxDistance: 1e-12})
	if err != nil {
		t.Fatal(err)
	}
	lines, spans := m.FiniteLineOwners(), m.CylindricalSpanOwners()
	if m.OriginalSTEP() != string(raw) || m.LocalDescriptor().LengthUnit != 1 || len(m.Repairs()) != 3 ||
		len(m.PCurves()) != 24 || len(lines) != 18 || len(spans) != 4 {
		t.Fatal("converted source, units, or original owner evidence was lost")
	}
	for _, owner := range lines {
		if owner.PCurveParameterScale == nil || owner.PCurveParameterScale.Cmp(big.NewRat(1, 1000)) != 0 || owner.StartParameter.Cmp(owner.EndParameter) == 0 {
			t.Fatal("missing exact planar owner parameter map", owner)
		}
	}
	for _, span := range spans {
		var expected [3]*big.Rat
		switch span.PCurveID {
		case 125, 225:
			expected = [3]*big.Rat{big.NewRat(5000, 1), new(big.Rat), big.NewRat(1, 1000)}
		case 157, 246:
			expected = [3]*big.Rat{big.NewRat(2498091544797, 1000000000000), big.NewRat(1, 1), new(big.Rat)}
		default:
			t.Fatal("unexpected cylindrical owner", span.PCurveID)
		}
		if span.ParameterInterval[0].Sign() != 0 || span.ParameterInterval[1].Cmp(expected[0]) != 0 ||
			span.UVDerivative[0].Cmp(expected[1]) != 0 || span.UVDerivative[1].Cmp(expected[2]) != 0 {
			t.Fatal("changed original knot interval or affine UV coefficients", span)
		}
	}
	canonical := checkRationalTiltUnitCanonical(t, rationalTiltUnitFixtures[0], m, big.NewRat(8, 5), big.NewRat(6, 5))
	return m, canonical
}

func TestRepairConvertedMetreConvexArcSource(t *testing.T) {
	ctx := context.Background()
	raw, err := os.ReadFile(convertedMetreConvexArcFixture)
	if err != nil {
		t.Fatal(err)
	}
	m, _ := checkConvertedMetreConvexArc(t, raw)
	if _, err = RepairRationalPosedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{MaxDistance: 1e-14}); err == nil {
		t.Fatal("converted chart accepted an insufficient distance budget")
	}
	var record bytes.Buffer
	if err = WriteRationalPosedPCurveRepairedOCCTConvexArcJSON(ctx, &record, m); err != nil {
		t.Fatal(err)
	}
	reloaded, err := ReadRationalPosedPCurveRepairedOCCTConvexArcJSON(ctx, bytes.NewReader(record.Bytes()), IndependentMaterialJSONLimits{})
	if err != nil || reloaded.OriginalSTEP() != string(raw) || reloaded.LocalDescriptor().LengthUnit != 1 ||
		!reflect.DeepEqual(reloaded.FiniteLineOwners(), m.FiniteLineOwners()) || !reflect.DeepEqual(reloaded.CylindricalSpanOwners(), m.CylindricalSpanOwners()) {
		t.Fatal("reproof changed the source or parameter maps", err)
	}
	const slope = `"UVDerivative":["0","1/1000"]`
	forged := bytes.Replace(record.Bytes(), []byte(slope), []byte(`"UVDerivative":["0","1/999"]`), 1)
	if bytes.Equal(forged, record.Bytes()) {
		t.Fatal("no original coefficient in native receipt")
	}
	if _, err = ReadRationalPosedPCurveRepairedOCCTConvexArcJSON(ctx, bytes.NewReader(forged), IndependentMaterialJSONLimits{}); err == nil {
		t.Fatal("native reproof accepted a forged cylinder coefficient")
	}
}

func TestRepairConvertedMetreConvexArcRejectsChangedCharts(t *testing.T) {
	raw, err := os.ReadFile(convertedMetreConvexArcFixture)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		edits [][2]string
	}{
		{"wrong SI unit", [][2]string{{"SI_UNIT($,.METRE.)", "SI_UNIT(.MILLI.,.METRE.)"}}},
		{"planar owner speed", [][2]string{{"#40 = VECTOR('',#41,1.);", "#40 = VECTOR('',#41,1.0000000000000001);"}}},
		{"axial span slope", [][2]string{{"#134 = CARTESIAN_POINT('',(0.,5.));", "#134 = CARTESIAN_POINT('',(0.,4.9999999999999999));"}}},
		{"short circular span with unchanged coefficients", [][2]string{
			{"(0.,2.498091544797),.PIECEWISE_BEZIER_KNOTS.", "(0.,2.498091544796),.PIECEWISE_BEZIER_KNOTS."},
			{"#161 = CARTESIAN_POINT('',(2.498091544797,0.));", "#161 = CARTESIAN_POINT('',(2.498091544796,0.));"},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			forged := raw
			for _, edit := range tc.edits {
				changed := bytes.Replace(forged, []byte(edit[0]), []byte(edit[1]), 1)
				if bytes.Equal(changed, forged) {
					t.Fatal("source mutation did not apply", edit[0])
				}
				forged = changed
			}
			m, err := RepairRationalPosedPCurveOCCTConvexArcExtrusion(context.Background(), bytes.NewReader(forged), Limits{}, ConvexArcRepairOptions{MaxDistance: 1e-12})
			if err == nil || m.OriginalSTEP() != "" {
				t.Fatal("changed chart yielded repaired material", err)
			}
		})
	}
}

func TestRepairConvertedMetreConvexArcLive(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL")
	if oracle == "" {
		t.Skip("set PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL to the pinned author helper")
	}
	input := filepath.Join(t.TempDir(), "original.step")
	if out, err := exec.Command(oracle, "write-default-tilted-translated-converted-metre", input).CombinedOutput(); err != nil {
		t.Fatalf("pinned OCCT author: %v\n%s", err, out)
	}
	raw, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	m, canonical := checkConvertedMetreConvexArc(t, raw)
	witness, err := exec.Command(oracle, "converted-metre-witness", input).CombinedOutput()
	if err != nil {
		t.Fatalf("pinned OCCT original owner evaluation: %v\n%s", err, witness)
	}
	var owners struct {
		PlaneLines, PlaneCircles, CylinderLines, CylinderCircles int
		MaxSourceDistance, WithoutPlanarMap                      float64
	}
	if err = json.Unmarshal(bytes.TrimSpace(witness), &owners); err != nil || owners.PlaneLines != 18 || owners.PlaneCircles != 2 ||
		owners.CylinderLines != 2 || owners.CylinderCircles != 2 || owners.MaxSourceDistance <= 0 || owners.MaxSourceDistance > 1e-12 ||
		owners.WithoutPlanarMap < 1000 {
		t.Fatalf("unexpected independent original-chart evaluation: %s (%v)", witness, err)
	}
	for _, got := range readRationalTiltUnitWorldSTEP(t, oracle, input, m.CorrectedModelSTEP(), canonical) {
		checkRationalTiltUnitMeasures(t, rationalTiltUnitFixtures[0], got)
	}
}
