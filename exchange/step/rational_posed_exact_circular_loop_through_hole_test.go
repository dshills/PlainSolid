package step

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func rationalPosedThroughHoleCanonical(t *testing.T) (RationalPosedConvexArcThroughBore, []byte) {
	t.Helper()
	ctx := context.Background()
	raw, err := os.ReadFile(tiltedBoreSource + "through-bore.step")
	if err != nil {
		t.Fatal(err)
	}
	m, err := InspectRationalPosedConvexArcThroughBore(ctx, bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{MaxDistance: 1e-12})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReadRationalPosedExactCircularLoopThroughHoleExtrusion(ctx, bytes.NewReader(raw), Limits{}); err == nil {
		t.Fatal("strict holed reader admitted the unrepaired source")
	}
	var canonical bytes.Buffer
	if err := m.WriteCanonicalSTEP(ctx, &canonical); err != nil {
		t.Fatal(err)
	}
	return m, canonical.Bytes()
}

func TestRationalPosedExactCircularLoopThroughHoleCanonical(t *testing.T) {
	t.Parallel()
	m, raw := rationalPosedThroughHoleCanonical(t)
	ctx := context.Background()
	strict, err := ReadRationalPosedExactCircularLoopThroughHoleExtrusion(ctx, bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if strict.Exterior.LengthUnit != .001 || strict.Exterior.Height != 5 || len(strict.Native.Faces()) != 11 ||
		strict.Hole != (trimmed.ExtrusionCircle{Center: geom.Vec2{X: -1, Y: 1}, Radius: .25}) {
		t.Fatal("strict canonical reader lost the qualifying through-hole recipe")
	}
	got, err := strict.Pose.Matrix()
	if err != nil {
		t.Fatal(err)
	}
	want, err := m.Pose().Matrix()
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("strict canonical reader moved the original occurrence", err)
	}
	original, err := m.Native(ctx)
	if err != nil {
		t.Fatal(err)
	}
	a, err := prismNativeBytes(original)
	if err != nil {
		t.Fatal(err)
	}
	b, err := prismNativeBytes(strict.Native)
	if err != nil || !bytes.Equal(a, b) {
		t.Fatal("strict STEP changed native material", err)
	}
	var replay bytes.Buffer
	if err := WriteRationalPosedExactCircularLoopThroughHoleExtrusion(ctx, &replay, strict.Exterior, strict.Hole, strict.Pose); err != nil || !bytes.Equal(raw, replay.Bytes()) {
		t.Fatal("strict canonical reopen changed whole STEP bytes", err)
	}
}

func TestRationalPosedExactCircularLoopThroughHoleCanonicalRejects(t *testing.T) {
	t.Parallel()
	_, raw := rationalPosedThroughHoleCanonical(t)
	ctx := context.Background()
	for _, edit := range [][3]string{
		{"cap hole winding", "#255=FACE_BOUND('',#254,.F.);", "#255=FACE_BOUND('',#254,.T.);"},
		{"duplicated opposite cap loop", "#270=ADVANCED_FACE('',(#263,#269),#14,.T.);", "#270=ADVANCED_FACE('',(#263,#255),#14,.T.);"},
		{"outward bore wall", "#312=ADVANCED_FACE('',(#311),#44,.F.);", "#312=ADVANCED_FACE('',(#311),#44,.T.);"},
		{"missing inner wall", "#319,#326,#333));", "#319,#326));"},
	} {
		t.Run(edit[0], func(t *testing.T) {
			bad := bytes.Replace(raw, []byte(edit[1]), []byte(edit[2]), 1)
			if bytes.Equal(raw, bad) {
				t.Fatal("test changed no canonical record")
			}
			strict, err := ReadRationalPosedExactCircularLoopThroughHoleExtrusion(ctx, bytes.NewReader(bad), Limits{})
			if err == nil || len(strict.Native.Faces()) != 0 {
				t.Fatal("modified holed STEP returned material", err)
			}
		})
	}
	q, err := ReadRationalPosedExactCircularLoopThroughHoleExtrusion(ctx, bytes.NewReader(raw), Limits{MaxFaces: 10})
	if err == nil || len(q.Native.Faces()) != 0 {
		t.Fatal("strict holed STEP exceeded its face budget", err)
	}
}

func TestRationalPosedExactCircularLoopThroughHoleCanonicalLive(t *testing.T) {
	t.Parallel()
	oracle := os.Getenv("PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL")
	if oracle == "" {
		t.Skip("set PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL to the pinned author helper")
	}
	_, raw := rationalPosedThroughHoleCanonical(t)
	path := filepath.Join(t.TempDir(), "canonical-through-bore.step")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(oracle, "read", path).CombinedOutput()
	var got rationalTiltUnitMeasurements
	if err != nil || json.Unmarshal(out, &got) != nil {
		t.Fatalf("pinned OCCT canonical holed transfer: %v\n%s", err, out)
	}
	want := rationalTiltUnitMeasurements{Volume: 28.096231102869041, Area: 73.08423752236105,
		Centroid: [3]float64{2.7684492062631412, 1.028061804597145, .47540105831581114}, Faces: 11, Solids: 1}
	checkScaledRationalTiltUnitMeasures(t, fourConvexArcUnit, got, want)
}
