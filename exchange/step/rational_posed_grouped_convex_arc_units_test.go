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
	"testing"
)

const groupedTiltUnitSource = "occt-both-default-tilted-oblique-prism-fillet"

func checkRationalTiltGroupedUnitReadback(t *testing.T, tc rationalTiltUnitFixture, raw []byte) (RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion, []byte) {
	t.Helper()
	ctx := context.Background()
	if _, err := ReadOCCTExactCircularLoopExtrusion(ctx, bytes.NewReader(raw), Limits{}); err == nil {
		t.Fatal("unrepaired grouped source unexpectedly qualified as strict model geometry")
	}
	m, err := RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{MaxDistance: 1e-12})
	if err != nil {
		t.Fatal(err)
	}
	if m.OriginalSTEP() != string(raw) || m.LocalDescriptor().LengthUnit != tc.metresPerUnit || m.ReferenceCircleID() != 146 ||
		len(m.Repairs()) != 8 || len(m.Tangencies()) != 8 || len(m.Reversals()) != 2 || len(m.PCurves()) != 28 ||
		len(m.FiniteLineOwners()) != 8 || len(m.RationalCaps()) != 2 || len(m.FiniteTrims()) != 2 || len(m.AxialTrims()) != 1 {
		t.Fatal("grouped repair lost original unit or owner evidence")
	}
	want := [3][4]*big.Rat{
		{big.NewRat(0, 1), big.NewRat(3, 5), big.NewRat(4, 5), big.NewRat(0, 1)},
		{big.NewRat(-1, 1), big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(0, 1)},
		{big.NewRat(0, 1), big.NewRat(-4, 5), big.NewRat(3, 5), big.NewRat(0, 1)},
	}
	pose, err := m.Pose().Matrix()
	if err != nil || !reflect.DeepEqual(pose, want) {
		t.Fatal("grouped repair changed the exact source-world placement", pose, err)
	}
	return m, checkRationalTiltUnitCanonical(t, tc, m, new(big.Rat), new(big.Rat))
}

func checkRationalTiltGroupedScaledMeasures(t *testing.T, tc rationalTiltUnitFixture, got rationalTiltUnitMeasurements) {
	t.Helper()
	want := rationalTiltUnitMeasurements{Volume: 29.077978807115539, Area: 65.622954970084379,
		Centroid: [3]float64{2.7627619257740741, 1.0271143655601034, .48298409896790118}, Faces: 7, Solids: 1}
	checkScaledRationalTiltUnitMeasures(t, tc, got, want)
}

func TestRationalPosedGroupedConvexArcNonMillimetreUnits(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for i, tc := range rationalTiltUnitFixtures {
		t.Run(tc.name, func(t *testing.T) {
			raw := tc.source(t, groupedTiltUnitSource)
			if _, err := RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{MaxDistance: 1e-14}); err == nil {
				t.Fatal("grouped repair exceeded the smaller source-unit distance budget")
			}
			m, _ := checkRationalTiltGroupedUnitReadback(t, tc, raw)
			var proof bytes.Buffer
			if err := WriteRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, &proof, m); err != nil {
				t.Fatal(err)
			}
			reloaded, err := ReadRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, bytes.NewReader(proof.Bytes()), IndependentMaterialJSONLimits{})
			if err != nil || reloaded.OriginalSTEP() != string(raw) || reloaded.LocalDescriptor().LengthUnit != tc.metresPerUnit ||
				!reflect.DeepEqual(reloaded.AxialTrims(), m.AxialTrims()) || !reflect.DeepEqual(reloaded.RationalCaps(), m.RationalCaps()) {
				t.Fatal("native grouped reload changed original units or finite-owner evidence", err)
			}
			var again bytes.Buffer
			if err = WriteRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, &again, reloaded); err != nil || !bytes.Equal(proof.Bytes(), again.Bytes()) {
				t.Fatal("native grouped certificate changed after reproof", err)
			}
			checkRationalTiltGroupedScaledMeasures(t, tc, measureRationalTiltUnitMaterial(t, tc, reloaded))

			var forged rationalPosedPCurveRepairEnvelope
			if err = json.Unmarshal(proof.Bytes(), &forged); err != nil {
				t.Fatal(err)
			}
			// The opposite unit is independently valid for the same numeric
			// solid. It must still fail against this source's physical claims.
			other := rationalTiltUnitFixtures[(i+1)%len(rationalTiltUnitFixtures)]
			forged.Source = other.source(t, groupedTiltUnitSource)
			changed, err := json.Marshal(forged)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = ReadRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, bytes.NewReader(changed), IndependentMaterialJSONLimits{}); err == nil {
				t.Fatal("grouped proof accepted the opposite original unit")
			}
		})
	}
}

func TestRationalPosedGroupedConvexArcNonMillimetreUnitsLive(t *testing.T) {
	t.Parallel()
	oracle := os.Getenv("PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL")
	if oracle == "" {
		t.Skip("set PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL to the pinned author helper")
	}
	for _, tc := range rationalTiltUnitFixtures {
		t.Run(tc.name, func(t *testing.T) {
			original := filepath.Join(t.TempDir(), "original.step")
			if out, err := exec.Command(oracle, "write-both-default-tilted-"+tc.name, original).CombinedOutput(); err != nil {
				t.Fatalf("pinned OCCT author: %v\n%s", err, out)
			}
			raw, err := os.ReadFile(original)
			if err != nil {
				t.Fatal(err)
			}
			m, canonical := checkRationalTiltGroupedUnitReadback(t, tc, raw)
			for _, measured := range readRationalTiltUnitWorldSTEP(t, oracle, original, m.CorrectedModelSTEP(), canonical) {
				checkRationalTiltGroupedScaledMeasures(t, tc, measured)
			}
		})
	}
}
