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
	"strings"
	"testing"
)

const tiltedBoreSource = "testdata/occt-both-default-tilted-oblique-prism-fillet-"

func checkRationalPosedConvexArcBore(t *testing.T, raw []byte) RationalPosedConvexArcThroughBore {
	t.Helper()
	ctx := context.Background()
	if _, err := ReadOCCTExactCircularLoopExtrusion(ctx, bytes.NewReader(raw), Limits{}); err == nil {
		t.Fatal("strict one-loop reader accepted the original through bore")
	}
	if _, err := RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{MaxDistance: 1e-12}); err == nil {
		t.Fatal("existing exterior-only repair accepted a multi-loop cap")
	}
	m, err := InspectRationalPosedConvexArcThroughBore(ctx, bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{MaxDistance: 1e-12})
	if err != nil {
		t.Fatal(err)
	}
	if m.OriginalSTEP() != string(raw) || m.CorrectedModelSTEP() == string(raw) || len(m.ExteriorRepairs()) != 8 ||
		len(m.outer.PCurves()) != 28 || len(m.outer.FiniteTrims()) != 2 {
		t.Fatal("lost restored bore or existing original exterior evidence")
	}
	bore := m.Bore
	if bore.WallFaceID != 492 || bore.WallSupportID != 267 || bore.WallBoundID != 493 || bore.WallLoopID != 494 ||
		bore.SeamEdgeID != 497 || bore.SeamModelID != 499 || bore.Center[0].RatString() != "-1" ||
		bore.Center[1].RatString() != "1" || bore.Radius.RatString() != "1/4" {
		t.Fatal("through bore lost its exact original cylinder geometry", bore)
	}
	for i, rim := range bore.Rims {
		actual := [6]int64{rim.CapFaceID, rim.BoundID, rim.LoopID, rim.EdgeID, rim.VertexID, rim.ModelID}
		if actual != [2][6]int64{{137, 247, 248, 250, 251, 254}, {382, 432, 433, 435, 436, 439}}[i] ||
			rim.AxialStation.RatString() != [2]string{"1", "6"}[i] || bore.SeamParameters[i].Cmp(rim.AxialStation) != 0 {
			t.Fatal("through bore lost an original cap and rim incidence", rim)
		}
		if rim.Owners[0].PCurveID != [2]int64{259, 444}[i] || rim.Owners[1].PCurveID != [2]int64{266, 451}[i] ||
			rim.Owners[0].Kind != "plane-circle" || rim.Owners[1].Kind != "cylinder-circle" ||
			rim.Owners[0].DistanceSquaredUpper.Sign() != 0 || rim.Owners[1].DistanceSquaredUpper.Sign() != 0 {
			t.Fatal("original circular owners lost their exact whole-rim lift", rim)
		}
	}
	lower, _ := new(big.Rat).SetString("1e-26")
	upper, _ := new(big.Rat).SetString("1.1e-26")
	if bore.SeamOwners[0].PCurveID != 503 || bore.SeamOwners[1].PCurveID != 510 || bore.SeamAngles[0].RatString() != "314159265359/50000000000" ||
		bore.SeamAngles[1].Sign() != 0 || bore.SeamOwners[0].DistanceSquaredUpper.Cmp(lower) <= 0 ||
		bore.SeamOwners[0].DistanceSquaredUpper.Cmp(upper) >= 0 || bore.SeamOwners[1].DistanceSquaredUpper.Sign() != 0 ||
		bore.TurnDefect[0].Sign() <= 0 || bore.TurnDefect[0].Cmp(bore.TurnDefect[1]) >= 0 {
		t.Fatal("lost the original distinct seam branches or certified phase defect")
	}
	pose, err := m.Pose().Matrix()
	if err != nil {
		t.Fatal(err)
	}
	want := [3][4]string{{"0", "3/5", "4/5", "0"}, {"-1", "0", "0", "0"}, {"0", "-4/5", "3/5", "0"}}
	for i := range pose {
		for j := range pose[i] {
			if pose[i][j].RatString() != want[i][j] {
				t.Fatal("source occurrence lost the original tilt", pose)
			}
		}
	}
	source, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	corrected, err := Parse(strings.NewReader(m.CorrectedModelSTEP()), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	for _, root := range []int64{bore.WallFaceID, bore.Rims[0].BoundID, bore.Rims[1].BoundID} {
		if _, err := equalCircularExtrusionGraph(source, root, corrected, root); err != nil {
			t.Fatal("restoring corrected exterior altered the attached bore topology or owners", root, err)
		}
	}
	return m
}

func TestInspectRationalPosedConvexArcThroughBore(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(tiltedBoreSource + "through-bore.step")
	if err != nil {
		t.Fatal(err)
	}
	m := checkRationalPosedConvexArcBore(t, raw)
	ctx := context.Background()
	replay, err := InspectRationalPosedConvexArcThroughBore(ctx, bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{MaxDistance: 1e-12})
	if err != nil || replay.CorrectedModelSTEP() != m.CorrectedModelSTEP() {
		t.Fatal("same source produced different corrected exchange bytes", err)
	}
	outside := m.Bore
	outside.Radius = big.NewRat(3, 2)
	if err := proveConvexArcBoreSeparation(ctx, m.outer.Solid(), outside); err == nil {
		t.Fatal("bore overlap with the exact exterior passed separation")
	}
}

func TestInspectRationalPosedConvexArcThroughBoreRejects(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	raw, err := os.ReadFile(tiltedBoreSource + "through-bore.step")
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range [][3]string{
		{"wrong cap winding", "#249 = ORIENTED_EDGE('',*,*,#250,.F.);", "#249 = ORIENTED_EDGE('',*,*,#250,.T.);"},
		{"swapped rim owner roles", "(#259,#266),.PCURVE_S1.", "(#266,#259),.PCURVE_S1."},
		{"same seam branch twice", "#513 = CARTESIAN_POINT('',(0.,-0.));", "#513 = CARTESIAN_POINT('',(6.28318530718,-0.));"},
		{"incorrect work-plane rim height", "#274 = CARTESIAN_POINT('',(0.,1.));", "#274 = CARTESIAN_POINT('',(0.,1.01));"},
		{"seam phase exceeds request budget", "#506 = CARTESIAN_POINT('',(6.28318530718,-0.));", "#506 = CARTESIAN_POINT('',(6.283185307187,-0.));"},
		{"outward instead of inward wall", "#492 = ADVANCED_FACE('',(#493),#267,.F.);", "#492 = ADVANCED_FACE('',(#493),#267,.T.);"},
	} {
		t.Run(q[0], func(t *testing.T) {
			bad := bytes.Replace(raw, []byte(q[1]), []byte(q[2]), 1)
			if bytes.Equal(raw, bad) {
				t.Fatal("test changed no source record")
			}
			m, err := InspectRationalPosedConvexArcThroughBore(ctx, bytes.NewReader(bad), Limits{}, ConvexArcRepairOptions{MaxDistance: 1e-12})
			if err == nil || !strings.Contains(err.Error(), "through-bore owners:") || m.OriginalSTEP() != "" {
				t.Fatal("changed original bore returned material", err)
			}
		})
	}
	if m, err := InspectRationalPosedConvexArcThroughBore(ctx, bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{MaxDistance: 1e-13}); err == nil || m.OriginalSTEP() != "" {
		t.Fatal("insufficient source budget published a result", err)
	}
	if m, err := InspectRationalPosedConvexArcThroughBore(ctx, bytes.NewReader(raw), Limits{MaxEntities: 523}, ConvexArcRepairOptions{MaxDistance: 1e-12}); err == nil || m.OriginalSTEP() != "" {
		t.Fatal("entity ceiling published a result", err)
	}
	blind, err := os.ReadFile(tiltedBoreSource + "blind-bore.step")
	if err != nil {
		t.Fatal(err)
	}
	checkRationalPosedConvexArcBlindControl(t, blind)
}

func checkRationalPosedConvexArcBlindControl(t *testing.T, raw []byte) {
	t.Helper()
	ctx := context.Background()
	m, err := InspectRationalPosedConvexArcThroughBore(ctx, bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{MaxDistance: 1e-12})
	if err == nil || !strings.Contains(err.Error(), "through-bore topology:") || m.OriginalSTEP() != "" {
		t.Fatal("blind bore passed through-hole topology", err)
	}
}

func TestInspectRationalPosedConvexArcThroughBoreLive(t *testing.T) {
	t.Parallel()
	oracle := os.Getenv("PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL")
	if oracle == "" {
		t.Skip("set PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL to the pinned author helper")
	}
	dir := t.TempDir()
	author := func(kind string) (string, []byte) {
		path := filepath.Join(dir, kind+".step")
		if out, err := exec.Command(oracle, "write-both-default-tilted-"+kind, path).CombinedOutput(); err != nil {
			t.Fatalf("pinned OCCT author: %v\n%s", err, out)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return path, raw
	}
	read := func(path string) rationalTiltUnitMeasurements {
		out, err := exec.Command(oracle, "read", path).CombinedOutput()
		var m rationalTiltUnitMeasurements
		if err != nil || json.Unmarshal(out, &m) != nil {
			t.Fatalf("pinned OCCT transfer: %v\n%s", err, out)
		}
		return m
	}
	original, raw := author("through-bore")
	m := checkRationalPosedConvexArcBore(t, raw)
	corrected := filepath.Join(dir, "corrected.step")
	if err := os.WriteFile(corrected, []byte(m.CorrectedModelSTEP()), 0600); err != nil {
		t.Fatal(err)
	}
	want := rationalTiltUnitMeasurements{Volume: 28.096231102869041, Area: 73.08423752236105,
		Centroid: [3]float64{2.7684492062631412, 1.028061804597145, .47540105831581114}, Faces: 8, Solids: 1}
	for _, name := range []string{original, corrected} {
		checkScaledRationalTiltUnitMeasures(t, fourConvexArcUnit, read(name), want)
	}
	blind, raw := author("blind-bore")
	checkRationalPosedConvexArcBlindControl(t, raw)
	if control := read(blind); control.Faces != 9 || control.Solids != 1 || math.Abs(control.Volume-want.Volume-2*math.Pi*.25*.25) > 1e-10 {
		t.Fatal("independently authored blind control lost its finite floor", control)
	}
}
