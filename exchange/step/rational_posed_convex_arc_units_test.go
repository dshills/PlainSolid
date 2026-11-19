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
	"plainsolid/internal/slowtest"
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

func (tc rationalTiltUnitFixture) source(t *testing.T, name string) []byte {
	t.Helper()
	path := "testdata/" + name + "-" + tc.name + ".step"
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
	return m, checkRationalTiltUnitCanonical(t, tc, m, big.NewRat(8, 5), big.NewRat(6, 5))
}

func checkRationalTiltUnitCanonical(t *testing.T, tc rationalTiltUnitFixture, m rationalPosedPCurveRepairMaterial, x, z *big.Rat) []byte {
	t.Helper()
	ctx := context.Background()
	canonical := encodeRationalWorldCircularLoop(t, m, false)
	switch tc.name {
	case "inch":
		const exact = "LENGTH_MEASURE(0.0254)"
		if !bytes.Contains(canonical, []byte(exact)) {
			t.Fatal("canonical inch factor must be 127/5000 metres exactly")
		}
		forged := bytes.Replace(canonical, []byte(exact), []byte("LENGTH_MEASURE(0.025399999999999999)"), 1)
		if _, err := ReadRationalPosedExactCircularLoopExtrusion(ctx, bytes.NewReader(forged), Limits{}); err == nil {
			t.Fatal("strict canonical reader accepted an inexact inch factor")
		}
	case "metre":
		if !bytes.Contains(canonical, []byte("SI_UNIT($,.METRE.)")) {
			t.Fatal("canonical metre model lost its exact SI declaration")
		}
	case "millimetre":
		if !bytes.Contains(canonical, []byte("SI_UNIT(.MILLI.,.METRE.)")) {
			t.Fatal("canonical millimetre model lost its exact SI declaration")
		}
	}
	strict, err := ReadRationalPosedExactCircularLoopExtrusion(ctx, bytes.NewReader(canonical), Limits{})
	if err != nil || strict.Descriptor.LengthUnit != tc.metresPerUnit {
		t.Fatal("canonical STEP lost source units", strict.Descriptor.LengthUnit, err)
	}
	want, err := m.Pose().Matrix()
	if err != nil {
		t.Fatal(err)
	}
	got, err := strict.Pose.Matrix()
	if err != nil || !reflect.DeepEqual(want, got) || got[0][3].Cmp(x) != 0 || got[2][3].Cmp(z) != 0 {
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
	return canonical
}

type rationalTiltUnitMeasurements struct {
	Volume, Area  float64
	Centroid      [3]float64
	Faces, Solids int
}

func checkRationalTiltUnitMeasures(t *testing.T, tc rationalTiltUnitFixture, got rationalTiltUnitMeasurements) {
	t.Helper()
	want := rationalTiltUnitMeasurements{Volume: 29.145041881053842, Area: 66.186284791166074,
		Centroid: [3]float64{3.7610838977055483, 1.0248794567287747, 2.4852214697259356}, Faces: 6, Solids: 1}
	checkScaledRationalTiltUnitMeasures(t, tc, got, want)
}

func checkScaledRationalTiltUnitMeasures(t *testing.T, tc rationalTiltUnitFixture, got, source rationalTiltUnitMeasurements) {
	t.Helper()
	unit := big.NewRat(tc.mmNumerator, tc.mmDenominator)
	mm, _ := unit.Float64()
	want := []float64{
		source.Volume * mm * mm * mm, source.Area * mm * mm,
		source.Centroid[0] * mm, source.Centroid[1] * mm, source.Centroid[2] * mm,
	}
	if got.Faces != source.Faces || got.Solids != source.Solids {
		t.Fatal("world topology changed", got)
	}
	for i, value := range []float64{got.Volume, got.Area, got.Centroid[0], got.Centroid[1], got.Centroid[2]} {
		if !finiteNumber(value) || math.Abs(value-want[i]) > 1e-11*want[i] {
			t.Fatal("incorrect millimetre world measure", i, value, want[i])
		}
	}
}

func measureRationalTiltUnitMaterial(t *testing.T, tc rationalTiltUnitFixture, m rationalPosedPCurveRepairMaterial) rationalTiltUnitMeasurements {
	t.Helper()
	ctx := context.Background()
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
	return got
}

const singleTiltUnitSource = "occt-default-tilted-translated-oblique-prism-fillet"

func TestRationalPosedConvexArcNonMillimetreUnits(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for _, tc := range rationalTiltUnitFixtures {
		t.Run(tc.name, func(t *testing.T) {
			raw := tc.source(t, singleTiltUnitSource)
			m, _ := checkRationalTiltUnitReadback(t, tc, raw)
			var proof bytes.Buffer
			if err := WriteRationalPosedPCurveRepairedOCCTConvexArcJSON(ctx, &proof, m); err != nil {
				t.Fatal(err)
			}
			m, err := ReadRationalPosedPCurveRepairedOCCTConvexArcJSON(ctx, &proof, IndependentMaterialJSONLimits{})
			if err != nil || m.OriginalSTEP() != string(raw) || m.LocalDescriptor().LengthUnit != tc.metresPerUnit {
				t.Fatal("native proof lost the original unit declaration", err)
			}
			checkRationalTiltUnitMeasures(t, tc, measureRationalTiltUnitMaterial(t, tc, m))
		})
	}
}

func TestRationalPosedConvexArcRejectsInexactInchDeclaration(t *testing.T) {
	t.Parallel()
	raw := rationalTiltUnitFixtures[1].source(t, singleTiltUnitSource)
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
	t.Parallel()
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
			for _, measured := range readRationalTiltUnitWorldSTEP(t, oracle, original, m.CorrectedModelSTEP(), canonical) {
				checkRationalTiltUnitMeasures(t, tc, measured)
			}
		})
	}
}

func readRationalTiltUnitWorldSTEP(t *testing.T, oracle, original, corrected string, canonical []byte) []rationalTiltUnitMeasurements {
	t.Helper()
	dir := filepath.Dir(original)
	repaired, generated := filepath.Join(dir, "corrected.step"), filepath.Join(dir, "canonical.step")
	if err := os.WriteFile(repaired, []byte(corrected), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(generated, canonical, 0600); err != nil {
		t.Fatal(err)
	}
	var results []rationalTiltUnitMeasurements
	for _, path := range []string{original, repaired, generated} {
		out, err := exec.Command(oracle, "read", path).CombinedOutput()
		if err != nil {
			t.Fatalf("pinned OCCT transfer of %s: %v\n%s", path, err, out)
		}
		var measured rationalTiltUnitMeasurements
		if err = json.Unmarshal([]byte(strings.TrimSpace(string(out))), &measured); err != nil {
			t.Fatalf("invalid pinned OCCT measurements: %v\n%s", err, out)
		}
		results = append(results, measured)
	}
	return results
}
