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

const convertedInchConvexArcFixture = "testdata/occt-default-tilted-translated-oblique-prism-fillet-converted-inch.step"
const unscaledConvertedInchConvexArcFixture = "testdata/occt-default-tilted-translated-oblique-prism-fillet-converted-inch-unscaled.step"

func checkConvertedInchConvexArc(t *testing.T, raw []byte) (RationalPosedPCurveRepairedOCCTConvexArcExtrusion, []byte) {
	t.Helper()
	ctx := context.Background()
	if _, err := ReadRationalPosedExactCircularLoopExtrusion(ctx, bytes.NewReader(raw), Limits{}); err == nil {
		t.Fatal("converted original unexpectedly passed strict model-only import")
	}
	m, err := RepairRationalPosedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{MaxDistance: 1e-11})
	if err != nil {
		t.Fatal(err)
	}
	lines, spans := m.FiniteLineOwners(), m.CylindricalSpanOwners()
	if m.OriginalSTEP() != string(raw) || m.LocalDescriptor().LengthUnit != .0254 || m.LocalDescriptor().Height != 25 ||
		len(m.Repairs()) != 3 || len(m.PCurves()) != 24 || len(lines) != 18 || len(spans) != 4 {
		t.Fatal("converted source, physical length, or original owner evidence was lost")
	}
	for _, owner := range lines {
		if owner.PCurveParameterScale == nil || owner.PCurveParameterScale.RatString() != "5/127" ||
			owner.StartParameter.Cmp(owner.EndParameter) == 0 {
			t.Fatal("missing exact planar unit map", owner)
		}
	}
	const sweep = "2498091544797/1000000000000"
	want := map[int64][5]string{
		125: {"635", "0", "0", "0", "5/127"},
		225: {"635", sweep, "0", "0", "5/127"},
		157: {sweep, "0", "0", "1", "0"},
		246: {sweep, "0", "25", "1", "0"},
	}
	for _, span := range spans {
		coefficients := [5]string{span.ParameterInterval[1].RatString(), span.UVStart[0].RatString(), span.UVStart[1].RatString(),
			span.UVDerivative[0].RatString(), span.UVDerivative[1].RatString()}
		if span.ParameterInterval[0].Sign() != 0 || coefficients != want[span.PCurveID] {
			t.Fatal("changed original cylindrical span", span)
		}
		delete(want, span.PCurveID)
	}
	for _, owner := range m.PCurves() {
		if owner.PCurveID == 125 {
			// At the original model endpoint t=25/speed, the owner
			// has moved 25*(5/127)/speed. This is the whole error at u=0.
			drift := big.NewRat(25, 15625000000001)
			squared := new(big.Rat).Mul(drift, drift)
			if owner.Kind != "finite-cylinder-line" || owner.DistanceSquaredUpper.Cmp(squared) != 0 {
				t.Fatal("axial proof did not account for the exact source speed drift", owner.DistanceSquaredUpper)
			}
		}
	}
	return m, checkRationalTiltUnitCanonical(t, rationalTiltUnitFixtures[1], m, big.NewRat(8, 1), big.NewRat(6, 1))
}

func TestRepairConvertedInchConvexArcSource(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	raw, err := os.ReadFile(convertedInchConvexArcFixture)
	if err != nil {
		t.Fatal(err)
	}
	m, _ := checkConvertedInchConvexArc(t, raw)
	if rejected, err := RepairRationalPosedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{MaxDistance: 1e-12}); err == nil || rejected.OriginalSTEP() != "" {
		t.Fatal("original rounded-speed disagreement exceeded the repair budget")
	}
	var record bytes.Buffer
	if err := WriteRationalPosedPCurveRepairedOCCTConvexArcJSON(ctx, &record, m); err != nil {
		t.Fatal(err)
	}
	reloaded, err := ReadRationalPosedPCurveRepairedOCCTConvexArcJSON(ctx, bytes.NewReader(record.Bytes()), IndependentMaterialJSONLimits{})
	if err != nil || reloaded.OriginalSTEP() != string(raw) || reloaded.LocalDescriptor().LengthUnit != .0254 ||
		!reflect.DeepEqual(reloaded.FiniteLineOwners(), m.FiniteLineOwners()) || !reflect.DeepEqual(reloaded.CylindricalSpanOwners(), m.CylindricalSpanOwners()) {
		t.Fatal("source-only reload changed the originals, physical unit or owner ledgers", err)
	}
	forged := bytes.Replace(record.Bytes(), []byte(`"UVDerivative":["0","5/127"]`), []byte(`"UVDerivative":["0","1/25"]`), 1)
	if bytes.Equal(forged, record.Bytes()) {
		t.Fatal("no original axial coefficient in native receipt")
	}
	if _, err := ReadRationalPosedPCurveRepairedOCCTConvexArcJSON(ctx, bytes.NewReader(forged), IndependentMaterialJSONLimits{}); err == nil {
		t.Fatal("native reproof accepted a forged axial coefficient")
	}
}

func TestRepairConvertedInchConvexArcRejectsChangedCharts(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(convertedInchConvexArcFixture)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		edits [][2]string
	}{
		{"rounded model speed", [][2]string{{"#29 = VECTOR('',#30,3.937007874016E-02);", "#29 = VECTOR('',#30,3.93700787401600001E-02);"}}},
		{"changed axial slope", [][2]string{{"#134 = CARTESIAN_POINT('',(0.,25.));", "#134 = CARTESIAN_POINT('',(0.,25.0000000000000001));"}}},
		{"short axial interval with unchanged slope", [][2]string{
			{"(0.,635.),.PIECEWISE_BEZIER_KNOTS.", "(0.,508.),.PIECEWISE_BEZIER_KNOTS."},
			{"#134 = CARTESIAN_POINT('',(0.,25.));", "#134 = CARTESIAN_POINT('',(0.,20.));"},
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
			m, err := RepairRationalPosedPCurveOCCTConvexArcExtrusion(context.Background(), bytes.NewReader(forged), Limits{}, ConvexArcRepairOptions{MaxDistance: 1e-11})
			if err == nil || m.OriginalSTEP() != "" {
				t.Fatal("changed chart yielded repaired material", err)
			}
		})
	}
}

func checkUnscaledConvertedInchTangency(t *testing.T, raw []byte) {
	t.Helper()
	ctx := context.Background()
	source, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	cylinder, err := occtLoopReadFace(source, 214)
	if err != nil || cylinder.kind != "CYLINDRICAL_SURFACE" {
		t.Fatal("missing original cylindrical support", err)
	}
	plane, err := occtLoopReadFace(source, 340)
	if err != nil || plane.kind != "PLANE" || !convexUVUnit(plane.z[:]) {
		t.Fatal("missing original unit side-plane support", err)
	}
	distance := new(big.Rat).Abs(occtLoopDot(occtLoopSub(cylinder.origin, plane.origin), plane.z))
	gap := new(big.Rat).Sub(distance, cylinder.radius)
	want := big.NewRat(297, 1250000000000000)
	if gap.Cmp(want) != 0 {
		t.Fatal("original exact support gap changed", gap)
	}
	// Vertex edits cannot make the retained line/plane and cylinder supports
	// tangent. The gap is smaller than this otherwise admissible allowance.
	m, err := RepairRationalPosedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{MaxDistance: 1e-9})
	if err == nil || m.OriginalSTEP() != "" {
		t.Fatal("rounded support pair unexpectedly yielded exact material", err)
	}
}

func TestRepairConvertedInchConvexArcRejectsRoundedTangency(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(unscaledConvertedInchConvexArcFixture)
	if err != nil {
		t.Fatal(err)
	}
	checkUnscaledConvertedInchTangency(t, raw)
}

func TestRepairConvertedInchConvexArcLive(t *testing.T) {
	t.Parallel()
	oracle := os.Getenv("PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL")
	if oracle == "" {
		t.Skip("set PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL to the pinned author helper")
	}
	dir := t.TempDir()
	original := filepath.Join(dir, "scaled.step")
	if out, err := exec.Command(oracle, "write-default-tilted-translated-converted-inch", original).CombinedOutput(); err != nil {
		t.Fatalf("pinned OCCT author: %v\n%s", err, out)
	}
	raw, err := os.ReadFile(original)
	if err != nil {
		t.Fatal(err)
	}
	m, canonical := checkConvertedInchConvexArc(t, raw)
	witness, err := exec.Command(oracle, "converted-inch-witness", original).CombinedOutput()
	if err != nil {
		t.Fatalf("pinned OCCT owner witness: %v\n%s", err, witness)
	}
	var owners struct {
		PlaneLines, PlaneCircles, CylinderLines, CylinderCircles int
		MaxSourceDistance, WithoutPlanarMap                      float64
	}
	if err = json.Unmarshal(witness, &owners); err != nil || owners.PlaneLines != 18 || owners.PlaneCircles != 2 ||
		owners.CylinderLines != 2 || owners.CylinderCircles != 2 || owners.MaxSourceDistance <= 1e-12 ||
		owners.MaxSourceDistance > 1e-11 || owners.WithoutPlanarMap < 300 {
		t.Fatalf("unexpected independent original-chart evaluation: %s (%v)", witness, err)
	}
	tc := rationalTiltUnitFixtures[1]
	want := rationalTiltUnitMeasurements{Volume: 29.145041881053842 * 125, Area: 66.186284791166074 * 25,
		Centroid: [3]float64{3.7610838977055483 * 5, 1.0248794567287747 * 5, 2.4852214697259356 * 5}, Faces: 6, Solids: 1}
	for _, got := range readRationalTiltUnitWorldSTEP(t, oracle, original, m.CorrectedModelSTEP(), canonical) {
		checkScaledRationalTiltUnitMeasures(t, tc, got, want)
	}

	unscaled := filepath.Join(dir, "unscaled.step")
	if out, err := exec.Command(oracle, "write-default-tilted-translated-converted-inch-unscaled", unscaled).CombinedOutput(); err != nil {
		t.Fatalf("pinned OCCT author: %v\n%s", err, out)
	}
	raw, err = os.ReadFile(unscaled)
	if err != nil {
		t.Fatal(err)
	}
	checkUnscaledConvertedInchTangency(t, raw)
	// OCCT's ordinary tolerance-based consumer still transfers the original.
	if out, err := exec.Command(oracle, "read", unscaled).CombinedOutput(); err != nil {
		t.Fatalf("pinned OCCT could not transfer the unscaled original: %v\n%s", err, out)
	}
}
