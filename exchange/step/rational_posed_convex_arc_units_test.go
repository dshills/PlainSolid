package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"plainsolid/brep/parametric"
)

type rationalTiltUnitFixture struct {
	name          string
	metresPerUnit float64
	mmNumerator   int64
	mmDenominator int64
}

var rationalTiltUnitFixtures = []rationalTiltUnitFixture{
	{name: "metre", metresPerUnit: 1, mmNumerator: 1000, mmDenominator: 1},
	{name: "inch", metresPerUnit: .0254, mmNumerator: 127, mmDenominator: 5},
}

func (tc rationalTiltUnitFixture) source(t *testing.T) []byte {
	t.Helper()
	path := "testdata/occt-default-tilted-translated-oblique-prism-fillet-" + tc.name + ".step"
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func checkRationalTiltUnitReadback(t *testing.T, tc rationalTiltUnitFixture, raw []byte) (RationalPosedPCurveRepairedOCCTConvexArcExtrusion, []byte) {
	t.Helper()
	ctx := context.Background()
	if _, err := ReadOCCTExactCircularLoopExtrusion(ctx, bytes.NewReader(raw), Limits{}); err == nil {
		t.Fatal("unrepaired source unexpectedly qualified as strict model geometry")
	}
	m, err := RepairRationalPosedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{MaxDistance: 1e-12})
	if err != nil {
		t.Fatal(err)
	}
	if m.OriginalSTEP() != string(raw) || m.LocalDescriptor().LengthUnit != tc.metresPerUnit ||
		len(m.Repairs()) != 3 || len(m.PCurves()) != 24 || len(m.FiniteLineOwners()) != 8 {
		t.Fatal("original source units or owner evidence were lost")
	}
	canonical := encodeRationalWorldCircularLoop(t, m, false)
	strict, err := ReadRationalPosedExactCircularLoopExtrusion(ctx, bytes.NewReader(canonical), Limits{})
	if err != nil || strict.Descriptor.LengthUnit != tc.metresPerUnit {
		t.Fatal("canonical STEP lost source units", strict.Descriptor.LengthUnit, err)
	}
	want, err := m.Pose().Matrix()
	if err != nil {
		t.Fatal(err)
	}
	got, err := strict.Pose.Matrix()
	if err != nil || !reflect.DeepEqual(want, got) || got[0][3].Cmp(big.NewRat(8, 5)) != 0 || got[2][3].Cmp(big.NewRat(6, 5)) != 0 {
		t.Fatal("canonical STEP changed exact source-unit placement", got, err)
	}
	original, err := prismNativeBytes(m.Solid())
	if err != nil {
		t.Fatal(err)
	}
	native, err := prismNativeBytes(strict.Native)
	if err != nil || !bytes.Equal(original, native) {
		t.Fatal("canonical STEP rounded product-local geometry", err)
	}
	var again bytes.Buffer
	if err = WriteRationalPosedExactCircularLoopExtrusion(ctx, &again, strict.Descriptor, strict.Pose); err != nil || !bytes.Equal(canonical, again.Bytes()) {
		t.Fatal("world canonical bytes changed on native re-export", err)
	}
	return m, canonical
}

type rationalTiltUnitMeasurements struct {
	Volume, Area  float64
	Centroid      [3]float64
	Faces, Solids int
}

func checkRationalTiltUnitMeasures(t *testing.T, tc rationalTiltUnitFixture, got rationalTiltUnitMeasurements) {
	t.Helper()
	unit := big.NewRat(tc.mmNumerator, tc.mmDenominator)
	mm, _ := unit.Float64()
	want := []float64{
		29.145041881053842 * mm * mm * mm, 66.186284791166074 * mm * mm,
		3.7610838977055483 * mm, 1.0248794567287747 * mm, 2.4852214697259356 * mm,
	}
	if got.Faces != 6 || got.Solids != 1 {
		t.Fatal("world topology changed", got)
	}
	for i, value := range []float64{got.Volume, got.Area, got.Centroid[0], got.Centroid[1], got.Centroid[2]} {
		if !finiteNumber(value) || math.Abs(value-want[i]) > 1e-11*want[i] {
			t.Fatal("incorrect millimetre world measure", i, value, want[i])
		}
	}
}

func TestRationalPosedConvexArcNonMillimetreUnits(t *testing.T) {
	ctx := context.Background()
	for _, tc := range rationalTiltUnitFixtures {
		t.Run(tc.name, func(t *testing.T) {
			raw := tc.source(t)
			m, canonical := checkRationalTiltUnitReadback(t, tc, raw)
			if tc.name == "inch" {
				const exact = "LENGTH_MEASURE(0.0254)"
				if !bytes.Contains(canonical, []byte(exact)) {
					t.Fatal("canonical inch factor must be 127/5000 metres exactly")
				}
				forged := bytes.Replace(canonical, []byte(exact), []byte("LENGTH_MEASURE(0.025399999999999999)"), 1)
				if _, err := ReadRationalPosedExactCircularLoopExtrusion(ctx, bytes.NewReader(forged), Limits{}); err == nil {
					t.Fatal("strict canonical reader accepted an inexact inch factor")
				}
			}
			var proof bytes.Buffer
			if err := WriteRationalPosedPCurveRepairedOCCTConvexArcJSON(ctx, &proof, m); err != nil {
				t.Fatal(err)
			}
			m, err := ReadRationalPosedPCurveRepairedOCCTConvexArcJSON(ctx, &proof, IndependentMaterialJSONLimits{})
			if err != nil || m.OriginalSTEP() != string(raw) || m.LocalDescriptor().LengthUnit != tc.metresPerUnit {
				t.Fatal("native proof lost the original unit declaration", err)
			}
			p, err := m.Solid().MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-10})
			if err != nil {
				t.Fatal(err)
			}
			center, err := m.Pose().TransformPoint(ctx, occtLoopVec(p.Centroid), false, 512)
			if err != nil {
				t.Fatal(err)
			}
			mm := big.NewRat(tc.mmNumerator, tc.mmDenominator)
			mm2 := new(big.Rat).Mul(mm, mm)
			mm3 := new(big.Rat).Mul(mm2, mm)
			got := rationalTiltUnitMeasurements{Faces: len(m.Solid().Faces()), Solids: len(m.Solid().Topology().Shells)}
			got.Volume, _ = new(big.Rat).Mul(sideRat(p.Volume), mm3).Float64()
			got.Area, _ = new(big.Rat).Mul(sideRat(p.Area), mm2).Float64()
			for i := range center {
				got.Centroid[i], _ = center[i].Mul(center[i], mm).Float64()
			}
			checkRationalTiltUnitMeasures(t, tc, got)
		})
	}
}

func TestRationalPosedConvexArcRejectsInexactInchDeclaration(t *testing.T) {
	raw := rationalTiltUnitFixtures[1].source(t)
	for _, edit := range []struct{ name, old, replacement string }{
		{"same binary64 scale", "LENGTH_MEASURE_WITH_UNIT(25.4,", "LENGTH_MEASURE_WITH_UNIT(25.40000000000000001,"},
		{"wrong dimension", "DIMENSIONAL_EXPONENTS(1.,", "DIMENSIONAL_EXPONENTS(0.,"},
		{"wrong base unit", "SI_UNIT(.MILLI.,.METRE.)", "SI_UNIT(.CENTI.,.METRE.)"},
	} {
		t.Run(edit.name, func(t *testing.T) {
			source := bytes.Replace(raw, []byte(edit.old), []byte(edit.replacement), 1)
			if bytes.Equal(source, raw) {
				t.Fatal("unit mutation did not change the source")
			}
			m, err := RepairRationalPosedPCurveOCCTConvexArcExtrusion(context.Background(), bytes.NewReader(source), Limits{}, ConvexArcRepairOptions{MaxDistance: 1e-12})
			if err == nil || m.OriginalSTEP() != "" {
				t.Fatal("forged inch declaration yielded native material", err)
			}
		})
	}
}

func TestRationalPosedConvexArcNonMillimetreUnitsLive(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL")
	if oracle == "" {
		t.Skip("set PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL to the pinned author helper")
	}
	for _, tc := range rationalTiltUnitFixtures {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			original := filepath.Join(dir, "original.step")
			if out, err := exec.Command(oracle, "write-default-tilted-translated-"+tc.name, original).CombinedOutput(); err != nil {
				t.Fatalf("pinned OCCT author: %v\n%s", err, out)
			}
			raw, err := os.ReadFile(original)
			if err != nil {
				t.Fatal(err)
			}
			m, canonical := checkRationalTiltUnitReadback(t, tc, raw)
			corrected, generated := filepath.Join(dir, "corrected.step"), filepath.Join(dir, "canonical.step")
			if err = os.WriteFile(corrected, []byte(m.CorrectedModelSTEP()), 0600); err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(generated, canonical, 0600); err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{original, corrected, generated} {
				out, err := exec.Command(oracle, "read", path).CombinedOutput()
				if err != nil {
					t.Fatalf("pinned OCCT transfer of %s: %v\n%s", path, err, out)
				}
				var measured rationalTiltUnitMeasurements
				if err = json.Unmarshal([]byte(strings.TrimSpace(string(out))), &measured); err != nil {
					t.Fatalf("invalid pinned OCCT measurements: %v\n%s", err, out)
				}
				checkRationalTiltUnitMeasures(t, tc, measured)
			}
		})
	}
}
