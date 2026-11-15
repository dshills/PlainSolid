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
	"strings"
	"testing"
)

const fiveConvertedMetreSource = fiveConvexArcSource + "-converted-metre"
const fiveConvertedInchSource = fiveConvexArcSource + "-converted-inch"

var fiveConvertedFixtures = []struct {
	groupedConvertedFixture
	failingOwner, speed, projectedEnd string
}{
	{groupedConvertedMetre, "#207", "1/1000", "1250000000000000000000000000001/250000000000000000000000000"},
	{groupedConvertedInch, "#64", "123031496063/3125000000000", "6250000000000000000000000000003/9842519685040000000000000000"},
}

func checkFiveConvertedConvexArc(t *testing.T, raw []byte, tc groupedConvertedFixture, speed, projectedEnd string) (RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion, []byte) {
	t.Helper()
	ctx := context.Background()
	if _, err := ReadRationalPosedExactCircularLoopExtrusion(ctx, bytes.NewReader(raw), Limits{}); err == nil {
		t.Fatal("strict STEP accepted the original five-fillet conversion")
	}
	m, err := RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{MaxDistance: tc.maxDistance})
	if err != nil {
		t.Fatal(err)
	}
	d := m.LocalDescriptor()
	lines, spans, trims := m.FiniteLineOwners(), m.CylindricalSpanOwners(), m.AxialTrims()
	if m.OriginalSTEP() != string(raw) || d.LengthUnit != tc.unit.metresPerUnit || d.Height != tc.height || len(d.Segments) != 10 || m.ReferenceCircleID() != 144 ||
		len(m.Solid().Faces()) != 12 || len(m.Repairs()) != 10 || len(m.Tangencies()) != 20 || len(m.PCurves()) != 58 ||
		len(m.Reversals()) != 2 || len(lines) != 30 || len(spans) != 20 || len(trims) != 2 {
		t.Fatal("five converted rounds lost the native solid or original owners")
	}
	for _, p := range lines {
		if p.PCurveParameterScale == nil || p.PCurveParameterScale.RatString() != tc.ownerScale {
			t.Fatal("planar owner lost its exact unit conversion", p.PCurveID)
		}
	}
	f, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	// The STEP model LINE magnitude and the planar chart scale are separate
	// source facts. In particular their exact inch ratios differ.
	const firstModelVector = 29
	vector, err := simple(f, firstModelVector, "VECTOR", 3)
	if err != nil {
		t.Fatal(err)
	}
	magnitude, err := liftSourceRat(vector.Args[2])
	if err != nil || magnitude.RatString() != speed {
		t.Fatal("changed the raw LINE magnitude", magnitude, err)
	}
	checkFiveConvertedSpans(t, f, m, tc)
	if trims[0].EdgeID != 388 || trims[1].EdgeID != 653 || trims[1].OriginalParameters[1].RatString() != projectedEnd ||
		trims[1].RepairedParameters[1].Cmp(trims[1].OriginalParameters[1]) != 0 {
		t.Fatal("lost the exact original axial projection", trims)
	}
	finite := m.FiniteTrims()
	if len(finite) != 2 {
		t.Fatal("lost the two rational cap trims")
	}
	for i, c := range finite {
		want := [2][2]int64{{366, 377}, {423, 417}}[i]
		if [2]int64{c.CapOwner.PCurveID, c.CylinderOwner.PCurveID} != want || c.EndStation == nil || c.SourceEnd.Lower.Cmp(c.SourceEnd.Upper) >= 0 {
			t.Fatal("lost an original cap root or owner role", c.CapOwner.PCurveID)
		}
		curve, err := simple(f, c.CapOwner.SurfaceCurveID, "SURFACE_CURVE", 4)
		if err != nil {
			t.Fatal(err)
		}
		order, err := listRefs(curve.Args[2], 2)
		if err != nil || !reflect.DeepEqual(order, [2][]int64{{366, 377}, {417, 423}}[i]) {
			t.Fatal("changed original cap owner order", order, err)
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
				t.Fatal("changed the exact original world pose", pose)
			}
		}
	}
	return m, checkRationalTiltUnitCanonical(t, tc.unit, m, new(big.Rat), new(big.Rat))
}

func checkFiveConvertedSpans(t *testing.T, f *File, m RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion, tc groupedConvertedFixture) {
	t.Helper()
	const quarter = "314159265359/200000000000"
	const oblique = "2498091544797/1000000000000"
	const half = "314159265359/100000000000"
	const last = "942477796077/200000000000"
	phaseKnots := map[int64][2]string{207: {quarter, oblique}, 263: {oblique, half}, 319: {half, last},
		528: {quarter, oblique}, 574: {oblique, half}, 620: {half, last}}
	kinds := map[int64]ConvexArcPCurveAgreement{}
	for _, owner := range m.PCurves() {
		kinds[owner.PCurveID] = owner
	}
	axial := 0
	for _, span := range m.CylindricalSpanOwners() {
		id := span.PCurveID
		q := kinds[id]
		if span.UVDerivative[0].Sign() == 0 {
			axial++
			if span.ParameterInterval[0].Sign() != 0 || span.ParameterInterval[1].RatString() != tc.axialEnd ||
				span.UVDerivative[1].RatString() != tc.ownerScale || q.Kind != "finite-cylinder-line" {
				t.Fatal("axial chart lost its original knots or source length law", id)
			}
			if tc.unit.name == "metre" && id == 666 {
				clip := span.ClippedOwnerParameters
				if clip == nil || clip[0].Sign() != 0 || clip[1].RatString() != tc.axialEnd || span.RepairedToOwner == nil ||
					span.RepairedToOwner[0].RatString() != "-1/250000000000000000000000000" || span.RepairedToOwner[1].RatString() != "1" {
					t.Fatal("metre span did not retain its distinct projected and clipped stations", span)
				}
			} else if span.ClippedOwnerParameters != nil {
				t.Fatal("clipped an in-range source owner", span)
			}
			continue
		}
		knots, shifted := phaseKnots[id]
		if !shifted {
			if q.Kind == "finite-phase-cylinder-circle" {
				t.Fatal("unexpected extra shifted angular owner", id)
			}
			continue
		}
		station := int64(0)
		if id >= 528 {
			station = int64(tc.height)
		}
		if q.Kind != "finite-phase-cylinder-circle" || span.ModelAngleBounds == nil ||
			span.ParameterInterval[0].RatString() != knots[0] || span.ParameterInterval[1].RatString() != knots[1] || span.UVStart[0].Sign() != 0 ||
			span.UVStart[1].Cmp(big.NewRat(station, 1)) != 0 || span.UVDerivative[0].RatString() != "1" || span.UVDerivative[1].Sign() != 0 {
			t.Fatal("source lost its finite shifted angular law", id)
		}
		if knots[0] == half {
			checkFiveConvertedHalfTurn(t, f, q)
		}
		delete(phaseKnots, id)
	}
	if len(phaseKnots) != 0 || axial != 10 {
		t.Fatal("lost a shifted circular owner or an axial span", phaseKnots, axial)
	}
}

func checkFiveConvertedHalfTurn(t *testing.T, f *File, q ConvexArcPCurveAgreement) {
	t.Helper()
	curve, err := simple(f, q.SurfaceCurveID, "SURFACE_CURVE", 4)
	if err != nil {
		t.Fatal(err)
	}
	model, err := simple(f, curve.Args[1].Ref, "CIRCLE", 3)
	if err != nil {
		t.Fatal(err)
	}
	cylinder, err := simple(f, q.SupportID, "CYLINDRICAL_SURFACE", 3)
	if err != nil {
		t.Fatal(err)
	}
	_, x, _, z, err := convexUVFrame(f, model.Args[1].Ref)
	if err != nil {
		t.Fatal(err)
	}
	_, cx, _, cz, err := convexUVFrame(f, cylinder.Args[1].Ref)
	if err != nil || q.SupportID != 320 || !convexUVSame(z, cz) || occtLoopDot(x, cx).RatString() != "-1" {
		t.Fatal("original half-turn owner is not antipodal", q.PCurveID, err)
	}
}

func TestRepairFiveConvertedConvexArcSource(t *testing.T) {
	for _, tc := range fiveConvertedFixtures {
		t.Run(tc.unit.name, func(t *testing.T) {
			ctx := context.Background()
			raw, err := os.ReadFile(fiveConvexArcSource + "-converted-" + tc.unit.name + ".step")
			if err != nil {
				t.Fatal(err)
			}
			m, _ := checkFiveConvertedConvexArc(t, raw, tc.groupedConvertedFixture, tc.speed, tc.projectedEnd)
			small, err := RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{MaxDistance: tc.rejectedDistance})
			if err == nil || !strings.Contains(err.Error(), "source pcurve "+tc.failingOwner+":") || small.OriginalSTEP() != "" {
				t.Fatal("tight file-unit budget accepted original owner error", err)
			}
			var record, replay bytes.Buffer
			if err := WriteRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, &record, m); err != nil {
				t.Fatal(err)
			}
			r, err := ReadRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, bytes.NewReader(record.Bytes()), IndependentMaterialJSONLimits{})
			if err != nil || r.OriginalSTEP() != string(raw) || !reflect.DeepEqual(r.CylindricalSpanOwners(), m.CylindricalSpanOwners()) ||
				!reflect.DeepEqual(r.AxialTrims(), m.AxialTrims()) || !reflect.DeepEqual(r.FiniteTrims(), m.FiniteTrims()) {
				t.Fatal("reproof lost original world or finite owner evidence", err)
			}
			if err := WriteRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, &replay, r); err != nil || !bytes.Equal(record.Bytes(), replay.Bytes()) {
				t.Fatal("five-fillet receipt changed on reproof", err)
			}
			forged := bytes.Replace(record.Bytes(), []byte(`"PCurveID":319,"ParameterInterval"`), []byte(`"PCurveID":620,"ParameterInterval"`), 1)
			if bytes.Equal(forged, record.Bytes()) {
				t.Fatal("proof omitted the first antipodal spline")
			}
			if _, err := ReadRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, bytes.NewReader(forged), IndependentMaterialJSONLimits{}); err == nil {
				t.Fatal("reproof accepted a substituted half-turn owner")
			}
		})
	}
}

func TestRepairFiveConvertedConvexArcRejectsChangedCharts(t *testing.T) {
	raw, err := os.ReadFile(fiveConvertedMetreSource + ".step")
	if err != nil {
		t.Fatal(err)
	}
	for _, edit := range [][3]string{
		{"changed half-turn knots", "(3.14159265359,4.712388980385),.PIECEWISE_BEZIER_KNOTS.);\n#327", "(3.14159365359,4.712389980385),.PIECEWISE_BEZIER_KNOTS.);\n#327"},
		{"changed half-turn basis", "#324 = DIRECTION('',(0.,1.,0.));", "#324 = DIRECTION('',(0.,-1.,0.));"},
		{"changed half-turn coefficient", "#328 = CARTESIAN_POINT('',(1.570796326795,0.));", "#328 = CARTESIAN_POINT('',(1.570796326798,0.));"},
	} {
		t.Run(edit[0], func(t *testing.T) {
			rejectChangedGroupedConversion(t, raw, groupedConvertedMetre.maxDistance, [][2]string{{edit[1], edit[2]}})
		})
	}
	negative, err := os.ReadFile(fiveConvertedInchSource + "-unscaled.step")
	if err != nil {
		t.Fatal(err)
	}
	checkFiveConvertedInchUnscaled(t, negative)
}

func checkFiveConvertedInchUnscaled(t *testing.T, raw []byte) {
	t.Helper()
	checkUnscaledInchSupports(t, raw, []roundedInchSupport{
		{390, 662, 395, "1/5000000000000000"}, {444, 689, 449, "-1/500000000000000"},
		{716, 662, 667, "1/1000000000000000"}, {716, 770, 723, "-429/1000000000000000"},
		{743, 797, 748, "-1/125000000000000"}, {824, 770, 775, "-217/500000000000000"},
		{824, 797, 804, "-3/1250000000000000"},
	})
}

func TestRepairFiveConvertedConvexArcLive(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL")
	if oracle == "" {
		t.Skip("set PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL to the pinned author helper")
	}
	for _, tc := range fiveConvertedFixtures {
		t.Run(tc.unit.name, func(t *testing.T) {
			dir := t.TempDir()
			author := func(suffix string) (string, []byte) {
				path := filepath.Join(dir, "original"+suffix+".step")
				if out, err := exec.Command(oracle, "write-five-default-tilted-converted-"+tc.unit.name+suffix, path).CombinedOutput(); err != nil {
					t.Fatalf("pinned OCCT author: %v\n%s", err, out)
				}
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				return path, raw
			}
			original, raw := author("")
			m, canonical := checkFiveConvertedConvexArc(t, raw, tc.groupedConvertedFixture, tc.speed, tc.projectedEnd)
			out, err := exec.Command(oracle, "five-converted-"+tc.unit.name+"-witness", original).CombinedOutput()
			var w struct {
				PlaneLines, PlaneCircles, CylinderLines, CylinderCircles, RationalCaps, ShiftedCylinderCircles, ShiftedSamples, HalfTurnOwners int
				MaxSourceDistance, WithoutPlanarMap, MaxCapDistance, MaxShiftedDistance, MaxHalfTurnDistance, WithoutHalfTurn                  float64
			}
			scale := tc.height / 5
			if err != nil || json.Unmarshal(out, &w) != nil || w.PlaneLines != 30 || w.PlaneCircles != 8 || w.CylinderLines != 10 ||
				w.CylinderCircles != 10 || w.RationalCaps != 2 || w.ShiftedCylinderCircles != 6 || w.ShiftedSamples != 18 || w.HalfTurnOwners != 2 ||
				w.MaxSourceDistance > tc.maxDistance || w.MaxCapDistance > tc.maxDistance || w.MaxShiftedDistance > 1.3e-13*scale ||
				w.MaxHalfTurnDistance > 2.5e-14*scale || w.WithoutHalfTurn < .43*scale || w.WithoutPlanarMap < 100 {
				t.Fatalf("pinned OCCT did not witness the converted source charts: %v\n%s", err, out)
			}
			want := rationalTiltUnitMeasurements{Volume: 39.084768347549009, Area: 68.682455982680239,
				Centroid: [3]float64{2.8653432472612699, 1.3261479966457206, .34620900365163998}, Faces: 12, Solids: 1}
			scaled := want
			scaled.Volume *= scale * scale * scale
			scaled.Area *= scale * scale
			for i := range scaled.Centroid {
				scaled.Centroid[i] *= scale
			}
			checkScaledRationalTiltUnitMeasures(t, tc.unit, measureRationalTiltUnitMaterial(t, tc.unit, m), scaled)
			for _, measured := range readRationalTiltUnitWorldSTEP(t, oracle, original, m.CorrectedModelSTEP(), canonical) {
				checkScaledRationalTiltUnitMeasures(t, tc.unit, measured, scaled)
			}
			if tc.unit.name == "inch" {
				unscaled, raw := author("-unscaled")
				checkFiveConvertedInchUnscaled(t, raw)
				out, err := exec.Command(oracle, "read", unscaled).CombinedOutput()
				var measured rationalTiltUnitMeasurements
				if err != nil || json.Unmarshal(out, &measured) != nil {
					t.Fatalf("pinned OCCT did not transfer the unscaled original: %v\n%s", err, out)
				}
				checkScaledRationalTiltUnitMeasures(t, fourConvexArcUnit, measured, want)
			}
		})
	}
}
