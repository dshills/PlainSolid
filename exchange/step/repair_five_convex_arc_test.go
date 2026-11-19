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
	"slices"
	"strings"
	"testing"
)

const fiveConvexArcSource = "testdata/occt-five-default-tilted-pentagonal-prism-fillet"
const fiveConvexArcDistance = 1e-12

func checkFiveConvexArcSource(t *testing.T, raw []byte) (RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion, []byte) {
	t.Helper()
	ctx := context.Background()
	if _, err := ReadOCCTExactCircularLoopExtrusion(ctx, bytes.NewReader(raw), Limits{}); err == nil {
		t.Fatal("strict STEP accepted the unrepaired five-fillet owner graph")
	}
	m, err := RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{MaxDistance: fiveConvexArcDistance})
	if err != nil {
		t.Fatal(err)
	}
	d := m.LocalDescriptor()
	if m.OriginalSTEP() != string(raw) || d.LengthUnit != .001 || d.Height != 5 || len(d.Segments) != 10 ||
		len(m.Solid().Faces()) != 12 || len(m.Repairs()) != 10 || len(m.Tangencies()) != 20 || len(m.Reversals()) != 2 ||
		len(m.PCurves()) != 58 || len(m.FiniteLineOwners()) != 12 || len(m.CylindricalSpanOwners()) != 0 {
		t.Fatal("five-fillet source lost geometry or original owner evidence")
	}
	var radius []float64
	for i, segment := range d.Segments {
		if (segment.Radius == 0) == (d.Segments[(i+1)%len(d.Segments)].Radius == 0) {
			t.Fatal("missing a straight segment between rounds", i)
		}
		if segment.Radius > 0 {
			radius = append(radius, segment.Radius)
		}
	}
	slices.Sort(radius)
	if !slices.Equal(radius, []float64{7. / 32, 15. / 64, 1. / 4, 5. / 16, 3. / 8}) {
		t.Fatal("five independently authored round radii changed", radius)
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
	if err != nil || len(f.Order) != 836 {
		t.Fatal("lost the original 836-entity graph", err)
	}
	checkFiveConvexArcPhaseOwners(t, f, m.PCurves())

	trims, caps, finite := m.AxialTrims(), m.RationalCaps(), m.FiniteTrims()
	if len(trims) != 2 || len(caps) != 2 || len(finite) != 2 {
		t.Fatal("lost original axial or cap trim proof")
	}
	for i, trim := range trims {
		if trim.EdgeID != [2]int64{395, 667}[i] || trim.OriginalParameters[0].Sign() != 0 ||
			trim.RepairedParameters[0].RatString() != [2]string{"-3/500000000000000000000000000000", "1/250000000000000000000000000000"}[i] ||
			trim.RepairedParameters[1].Cmp(trim.OriginalParameters[1]) != 0 {
			t.Fatal("unequal axial stations changed", trim.EdgeID)
		}
		cap, pair := caps[i], finite[i]
		if cap.Owner.PCurveID != [2]int64{372, 432}[i] || !cap.PositiveAngularVelocity || !cap.ClosedOneTurn ||
			pair.CapOwner.PCurveID != cap.Owner.PCurveID || pair.CylinderOwner.PCurveID != [2]int64{383, 425}[i] || pair.EndStation == nil ||
			pair.SourceStart.Sign() != 0 || pair.SourceEnd.Lower.Cmp(pair.SourceEnd.Upper) >= 0 {
			t.Fatal("changed the original rational cap proof", cap.Owner.PCurveID)
		}
		curve, err := simple(f, cap.Owner.SurfaceCurveID, "SURFACE_CURVE", 4)
		if err != nil {
			t.Fatal(err)
		}
		order, err := listRefs(curve.Args[2], 2)
		if err != nil || !slices.Equal(order, [2][]int64{{372, 383}, {425, 432}}[i]) {
			t.Fatal("changed the original cap owner order", order, err)
		}
	}
	return m, checkRationalTiltUnitCanonical(t, fourConvexArcUnit, m, new(big.Rat), new(big.Rat))
}

func checkFiveConvexArcPhaseOwners(t *testing.T, f *File, owners []ConvexArcPCurveAgreement) {
	t.Helper()
	const quarter = "-314159265359/200000000000"
	const oblique = "-2498091544797/1000000000000"
	const half = "-314159265359/100000000000"
	wantPhases := map[int64][2]string{210: {quarter, "0"}, 267: {oblique, "0"}, 324: {half, "0"},
		539: {quarter, "5"}, 586: {oblique, "5"}, 633: {half, "5"}}
	wantKinds := map[string]int{"plane-line": 18, "finite-plane-line": 12, "cylinder-line": 10, "plane-circle": 8,
		"phase-cylinder-circle": 6, "cylinder-circle": 4}
	shiftLimit, _ := new(big.Rat).SetString("2.7e-26")
	halfLimit, _ := new(big.Rat).SetString("4.2e-27")
	for _, owner := range owners {
		wantKinds[owner.Kind]--
		if owner.Kind != "phase-cylinder-circle" {
			continue
		}
		want, ok := wantPhases[owner.PCurveID]
		uv, speed, err := convexUVLine(f, owner.GeometryID, 2)
		if !ok || err != nil || uv[0].RatString() != want[0] || uv[1].RatString() != want[1] ||
			speed[0].RatString() != "1" || speed[1].Sign() != 0 || owner.DistanceSquaredUpper.Sign() <= 0 ||
			owner.DistanceSquaredUpper.Cmp(shiftLimit) > 0 {
			t.Fatal("lost original shifted cylinder chart or full-span bound", owner.PCurveID)
		}
		delete(wantPhases, owner.PCurveID)
		if want[0] == half {
			sc, err := simple(f, owner.SurfaceCurveID, "SURFACE_CURVE", 4)
			if err != nil {
				t.Fatal(err)
			}
			model, err := simple(f, sc.Args[1].Ref, "CIRCLE", 3)
			if err != nil {
				t.Fatal(err)
			}
			cylinder, err := simple(f, owner.SupportID, "CYLINDRICAL_SURFACE", 3)
			if err != nil {
				t.Fatal(err)
			}
			_, x, _, z, err := convexUVFrame(f, model.Args[1].Ref)
			if err != nil {
				t.Fatal(err)
			}
			_, cx, _, cz, err := convexUVFrame(f, cylinder.Args[1].Ref)
			if err != nil || !convexUVSame(z, cz) || occtLoopDot(x, cx).RatString() != "-1" ||
				owner.DistanceSquaredUpper.Cmp(halfLimit) > 0 {
				t.Fatal("half-turn charts lost the exact antipodal frame or bound", owner.PCurveID, err)
			}
		}
	}
	if len(wantPhases) != 0 {
		t.Fatal("missing original shifted owner", wantPhases)
	}
	for kind, missing := range wantKinds {
		if missing != 0 {
			t.Fatal("incorrect original chart count", kind, missing)
		}
	}
}

func TestRepairFiveConvexArcSource(t *testing.T) {
	slowtest.Skip(t)
	raw, err := os.ReadFile(fiveConvexArcSource + ".step")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	m, _ := checkFiveConvexArcSource(t, raw)
	tooSmall, err := RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{MaxDistance: 1e-13})
	if err == nil || !strings.Contains(err.Error(), "source pcurve #267:") || tooSmall.OriginalSTEP() != "" {
		t.Fatal("tight budget did not reject the original oblique chart", err)
	}
	var receipt bytes.Buffer
	if err := WriteRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, &receipt, m); err != nil {
		t.Fatal(err)
	}
	read, err := ReadRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, bytes.NewReader(receipt.Bytes()), IndependentMaterialJSONLimits{})
	if err != nil || read.OriginalSTEP() != string(raw) || !reflect.DeepEqual(read.PCurves(), m.PCurves()) ||
		!reflect.DeepEqual(read.FiniteLineOwners(), m.FiniteLineOwners()) || !reflect.DeepEqual(read.AxialTrims(), m.AxialTrims()) ||
		!reflect.DeepEqual(read.FiniteTrims(), m.FiniteTrims()) {
		t.Fatal("five-fillet native proof lost original owner evidence", err)
	}
	var again bytes.Buffer
	if err := WriteRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, &again, read); err != nil || !bytes.Equal(again.Bytes(), receipt.Bytes()) {
		t.Fatal("five-fillet proof bytes changed on reproof", err)
	}
	forged := bytes.Replace(receipt.Bytes(), []byte(`"PCurveID":324`), []byte(`"PCurveID":633`), 1)
	if bytes.Equal(forged, receipt.Bytes()) {
		t.Fatal("native record omitted the original half-turn owner")
	}
	if _, err := ReadRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, bytes.NewReader(forged), IndependentMaterialJSONLimits{}); err == nil {
		t.Fatal("native reproof accepted a substituted half-turn owner")
	}
}

func TestRepairFiveConvexArcRejectsChangedCharts(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(fiveConvexArcSource + ".step")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range [][3]string{
		{"shortened half-turn phase", "#332 = CARTESIAN_POINT('',(-3.14159265359,0.));", "#332 = CARTESIAN_POINT('',(-3.14159,0.));"},
		{"phase outside bounded range", "#636 = CARTESIAN_POINT('',(-3.14159265359,5.));", "#636 = CARTESIAN_POINT('',(-9.424777960769,5.));"},
		{"non-antipodal frame", "#329 = DIRECTION('',(0.,1.,0.));", "#329 = DIRECTION('',(0.,-1.,0.));"},
		{"reversed chart", "#334 = DIRECTION('',(1.,0.));", "#334 = DIRECTION('',(-1.,0.));"},
		{"borrowed opposite-cap owner", "#311 = SURFACE_CURVE('',#312,(#317,#324),.PCURVE_S1.);", "#311 = SURFACE_CURVE('',#312,(#317,#633),.PCURVE_S1.);"},
	} {
		t.Run(test[0], func(t *testing.T) {
			rejectChangedGroupedConversion(t, raw, fiveConvexArcDistance, [][2]string{{test[1], test[2]}})
		})
	}
	if m, err := RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion(context.Background(), bytes.NewReader(raw), Limits{MaxEntities: 835},
		ConvexArcRepairOptions{MaxDistance: fiveConvexArcDistance}); err == nil || m.OriginalSTEP() != "" {
		t.Fatal("source entity limit must reject before publishing material")
	}
}

func checkFiveConvexArcNondyadic(t *testing.T, raw []byte) {
	t.Helper()
	f, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	cylinder, err := occtLoopReadFace(f, 824)
	if err != nil || cylinder.kind != "CYLINDRICAL_SURFACE" || cylinder.radius.RatString() != "3/16" {
		t.Fatal("negative source changed its original cylinder", err)
	}
	side, err := occtLoopReadFace(f, 797)
	if err != nil || side.kind != "PLANE" || !convexUVUnit(side.z[:]) || !convexUVUnit(cylinder.z[:]) ||
		occtLoopDot(side.z, cylinder.z).Sign() != 0 {
		t.Fatal("negative source lost its exact oblique plane", err)
	}
	for _, face := range []occtLoopFace{cylinder, side} {
		if !slices.ContainsFunc(face.uses, func(use analyticUse) bool { return use.id == 804 }) {
			t.Fatal("source faces lost their original shared axial edge", face.id)
		}
	}
	d := occtLoopDot(occtLoopSub(cylinder.origin, side.origin), side.z)
	if new(big.Rat).Abs(d).Cmp(cylinder.radius) != 0 {
		t.Fatal("negative control lost exact authored tangency", d)
	}
	var foot [3]*big.Rat
	for i := range foot {
		foot[i] = new(big.Rat).Sub(cylinder.origin[i], new(big.Rat).Mul(d, side.z[i]))
	}
	local := [2]*big.Rat{new(big.Rat).Neg(foot[1]), occtLoopDot(foot, [3]*big.Rat{big.NewRat(3, 5), new(big.Rat), big.NewRat(-4, 5)})}
	for i, value := range local {
		if _, exact := value.Float64(); exact || value.RatString() != [2]string{"-217/80", "41/20"}[i] {
			t.Fatal("negative control lost its original non-binary tangent coordinate", value)
		}
	}
	checkGroupedConvexArcUnrepresentable(t, raw, fiveConvexArcDistance)
}

func TestRepairFiveConvexArcRejectsNondyadicTangencies(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(fiveConvexArcSource + "-nondyadic.step")
	if err != nil {
		t.Fatal(err)
	}
	checkFiveConvexArcNondyadic(t, raw)
}

func TestRepairFiveConvexArcLive(t *testing.T) {
	t.Parallel()
	oracle := os.Getenv("PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL")
	if oracle == "" {
		t.Skip("set PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL to the pinned author helper")
	}
	dir := t.TempDir()
	author := func(suffix string) (string, []byte) {
		path := filepath.Join(dir, "original"+suffix+".step")
		if out, err := exec.Command(oracle, "write-five-default-tilted"+suffix, path).CombinedOutput(); err != nil {
			t.Fatalf("pinned OCCT author: %v\n%s", err, out)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return path, raw
	}
	original, raw := author("")
	m, canonical := checkFiveConvexArcSource(t, raw)
	evaluate := func(mode, file string, report any) {
		out, err := exec.Command(oracle, mode, file).CombinedOutput()
		if err != nil || json.Unmarshal(out, report) != nil {
			t.Fatalf("pinned OCCT %s: %v\n%s", mode, err, out)
		}
	}
	var owners struct {
		PlaneLines, PlaneCircles, CylinderLines, CylinderCircles, RationalCaps int
		MaxSourceDistance, MaxCapDistance                                      float64
	}
	evaluate("five-owner-witness", original, &owners)
	if owners.PlaneLines != 30 || owners.PlaneCircles != 8 || owners.CylinderLines != 10 || owners.CylinderCircles != 10 ||
		owners.RationalCaps != 2 || owners.MaxSourceDistance > fiveConvexArcDistance || owners.MaxCapDistance > fiveConvexArcDistance {
		t.Fatal("independent evaluator lost original owner coverage", owners)
	}
	var phase struct {
		Owners, Samples, HalfTurnOwners                                       int
		MaxSourceDistance, WithoutPhase, MaxHalfTurnDistance, WithoutHalfTurn float64
	}
	evaluate("five-shifted-cylinder-witness", original, &phase)
	if phase.Owners != 6 || phase.Samples != 18 || phase.HalfTurnOwners != 2 || phase.MaxSourceDistance > 1.2e-13 ||
		phase.WithoutPhase < .4 || phase.MaxHalfTurnDistance > 7e-14 || phase.WithoutHalfTurn < .43 {
		t.Fatal("independent evaluator lost the original half-turn phases", phase)
	}
	want := rationalTiltUnitMeasurements{Volume: 39.084768347549009, Area: 68.682455982680239,
		Centroid: [3]float64{2.8653432472612699, 1.3261479966457206, .34620900365163998}, Faces: 12, Solids: 1}
	for _, measured := range readRationalTiltUnitWorldSTEP(t, oracle, original, m.CorrectedModelSTEP(), canonical) {
		checkScaledRationalTiltUnitMeasures(t, fourConvexArcUnit, measured, want)
	}
	negative, raw := author("-nondyadic")
	checkFiveConvexArcNondyadic(t, raw)
	var rounded rationalTiltUnitMeasurements
	evaluate("read", negative, &rounded)
	if rounded.Faces != 12 || rounded.Solids != 1 || rounded.Volume <= 0 {
		t.Fatal("OCCT did not transfer the actual negative original", rounded)
	}
}
