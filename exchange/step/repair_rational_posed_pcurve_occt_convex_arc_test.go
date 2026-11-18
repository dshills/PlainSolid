package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"plainsolid/brep/parametric"
)

type rationalTiltFixture struct {
	suffix   string
	station  int64
	centroid [3]float64
}

var rationalTiltFixtures = []rationalTiltFixture{
	{"", 0, [3]float64{2.7610838977055483, 1.0248794567287747, .48522146972593505}},
	{"-translated", 2, [3]float64{3.7610838977055483, 1.0248794567287747, 2.4852214697259356}},
}

func rationalTiltSource(t testing.TB, pose rationalTiltFixture) []byte {
	t.Helper()
	raw, e := os.ReadFile("testdata/occt-default-tilted" + pose.suffix + "-oblique-prism-fillet.step")
	if e != nil {
		t.Fatal(e)
	}
	return raw
}

func checkRationalTiltMeasures(t testing.TB, volume, area float64, center [3]float64, expected rationalTiltFixture) {
	t.Helper()
	if math.Abs(volume-29.145041881053842) > 1e-11 || math.Abs(area-66.186284791166074) > 1e-11 {
		t.Fatal(volume, area)
	}
	for i := range center {
		if math.Abs(center[i]-expected.centroid[i]) > 1e-11 {
			t.Fatal("world centroid", center, expected.centroid)
		}
	}
}

func TestRepairRationalPosedPCurveOCCTConvexArc(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	opts := ConvexArcRepairOptions{MaxDistance: 1e-12}
	for _, tc := range rationalTiltFixtures {
		t.Run("tilted"+tc.suffix, func(t *testing.T) {
			raw := rationalTiltSource(t, tc)
			m, e := RepairRationalPosedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(raw), Limits{}, opts)
			if e != nil {
				t.Fatal(e)
			}
			if m.OriginalSTEP() != string(raw) || len(m.Repairs()) != 3 || len(m.Tangencies()) != 4 ||
				len(m.PCurves()) != 24 || len(m.FiniteLineOwners()) != 8 || m.ReferenceCircleID() != 146 {
				t.Fatal("incomplete original world evidence")
			}
			matrix, e := m.Pose().Matrix()
			if e != nil {
				t.Fatal(e)
			}
			want := [3][4]*big.Rat{
				{big.NewRat(0, 1), big.NewRat(3, 5), big.NewRat(4, 5), big.NewRat(4*tc.station, 5)},
				{big.NewRat(-1, 1), big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(0, 1)},
				{big.NewRat(0, 1), big.NewRat(-4, 5), big.NewRat(3, 5), big.NewRat(3*tc.station, 5)},
			}
			for i, row := range matrix {
				for j, q := range row {
					if q.Cmp(want[i][j]) != 0 {
						t.Fatal("source world placement", matrix)
					}
				}
			}
			f, e := Parse(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			corrected, e := Parse(bytes.NewBufferString(m.CorrectedModelSTEP()), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			if _, e = simple(f, m.Provenance().Body, "MANIFOLD_SOLID_BREP", 2); e != nil {
				t.Fatal("body lost original identity", e)
			}
			for _, s := range m.Repairs() {
				original, e := occtLoopTriple(f, s.PointID, "CARTESIAN_POINT")
				if e != nil || !convexUVSame(original, s.Original) {
					t.Fatal("source site was not copied exactly in world units", s, e)
				}
				fixed, e := occtLoopTriple(corrected, s.PointID, "CARTESIAN_POINT")
				delta := occtLoopSub(s.Repaired, s.Original)
				if e != nil || !convexUVSame(fixed, s.Repaired) || occtLoopDot(delta, delta).Cmp(s.SquaredDistance) != 0 {
					t.Fatal("world repair does not match source evidence", s, e)
				}
			}
			tol := new(big.Rat).SetFloat64(opts.MaxDistance)
			tol.Mul(tol, tol)
			finiteOwners := map[int64]FiniteConvexArcLineOwner{}
			for _, q := range m.FiniteLineOwners() {
				finiteOwners[q.PCurveID] = q
			}
			known := finiteOwners[31] // Original low-cap owner, tangent line along local X before tilt.
			if known.StartParameter == nil || known.StartParameter.Sign() != 0 || known.EndParameter.Cmp(big.NewRat(49, 16)) != 0 {
				t.Fatal("wrong original LINE parameter interval", known)
			}
			for _, q := range m.PCurves() {
				if q.DistanceSquaredUpper == nil || q.DistanceSquaredUpper.Cmp(tol) > 0 {
					t.Fatal("invalid continuous bound", q)
				}
				if _, e := simple(f, q.PCurveID, "PCURVE", 3); e != nil {
					t.Fatal("lost original pcurve", q, e)
				}
				_, finite := finiteOwners[q.PCurveID]
				if (q.Kind == "finite-plane-line") != finite || finite && q.DistanceSquaredUpper.Sign() == 0 {
					t.Fatal("finite range does not match original owner", q)
				}
			}
			known.StartParameter.SetInt64(123)
			if m.FiniteLineOwners()[0].StartParameter.Sign() != 0 {
				t.Fatal("finite range getter aliases source evidence")
			}
			p, e := m.Solid().MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-10})
			if e != nil {
				t.Fatal(e)
			}
			center, e := m.Pose().TransformPoint(ctx, occtLoopVec(p.Centroid), false, 512)
			if e != nil {
				t.Fatal(e)
			}
			var world [3]float64
			for i, q := range center {
				world[i], _ = q.Float64()
			}
			checkRationalTiltMeasures(t, p.Volume, p.Area, world, tc)
			if _, e := RepairPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(raw), Limits{}, opts); e == nil {
				t.Fatal("tilted source entered the unchanged unposed repair")
			}
		})
	}
}

func TestRepairRationalPosedPCurveOCCTConvexArcReject(t *testing.T) {
	t.Parallel()
	raw := rationalTiltSource(t, rationalTiltFixtures[0])
	ctx := context.Background()
	opts := ConvexArcRepairOptions{MaxDistance: 1e-12}
	for _, tc := range []struct {
		name, old, new string
	}{
		{"tilted circle norm", "#149 = DIRECTION('',(0.8,0.,0.6));", "#149 = DIRECTION('',(0.80000000000001,0.,0.6));"},
		{"borrowed owner", "#31 = PCURVE('',#32,#37);", "#31 = PCURVE('',#44,#37);"},
		{"changed finite speed", "#41 = DIRECTION('',(1.,2.6645352591E-17));", "#41 = DIRECTION('',(1.,1.E-6));"},
		{"changed preferred curve", "#26 = SURFACE_CURVE('',#27,(#31,#43),.PCURVE_S1.);", "#26 = SURFACE_CURVE('',#27,(#31,#43),.CURVE_3D.);"},
		{"nonneutral representation", "#12 = CARTESIAN_POINT('',(0.,0.,0.));", "#12 = CARTESIAN_POINT('',(1.,0.,0.));"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := bytes.Replace(raw, []byte(tc.old), []byte(tc.new), 1)
			if bytes.Equal(bad, raw) {
				t.Fatal("mutation missed its original source entity")
			}
			m, e := RepairRationalPosedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(bad), Limits{}, opts)
			if e == nil || m.OriginalSTEP() != "" {
				t.Fatal("invalid source was accepted or published", e)
			}
		})
	}
	if _, e := RepairRationalPosedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{MaxDistance: 1e-14}); e == nil {
		t.Fatal("original owner exceeded the smaller distance limit")
	}
	if _, e := RepairRationalPosedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(raw), Limits{MaxBytes: 64}, opts); e == nil {
		t.Fatal("source byte limit ignored")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := RepairRationalPosedPCurveOCCTConvexArcExtrusion(canceled, bytes.NewReader(raw), Limits{}, opts); !errors.Is(e, context.Canceled) {
		t.Fatal("cancellation", e)
	}
}

func TestRepairRationalPosedPCurveOCCTConvexArcLive(t *testing.T) {
	t.Parallel()
	oracle := os.Getenv("PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL")
	if oracle == "" {
		t.Skip("set PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL to the pinned author helper")
	}
	ctx := context.Background()
	for _, tc := range rationalTiltFixtures {
		t.Run("tilted"+tc.suffix, func(t *testing.T) {
			dir := t.TempDir()
			input := filepath.Join(dir, "original.step")
			if out, e := exec.Command(oracle, "write-default-tilted"+tc.suffix, input).CombinedOutput(); e != nil {
				t.Fatalf("pinned OCCT author: %v\n%s", e, out)
			}
			raw, e := os.ReadFile(input)
			if e != nil {
				t.Fatal(e)
			}
			m, e := RepairRationalPosedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{MaxDistance: 1e-12})
			if e != nil {
				t.Fatal(e)
			}
			var native bytes.Buffer
			if e = WriteRationalPosedPCurveRepairedOCCTConvexArcJSON(ctx, &native, m); e != nil {
				t.Fatal(e)
			}
			m, e = ReadRationalPosedPCurveRepairedOCCTConvexArcJSON(ctx, &native, IndependentMaterialJSONLimits{})
			if e != nil || m.OriginalSTEP() != string(raw) {
				t.Fatal("source-only reload lost independently authored STEP", e)
			}
			repaired := filepath.Join(dir, "corrected-world.step")
			if e := os.WriteFile(repaired, []byte(m.CorrectedModelSTEP()), 0600); e != nil {
				t.Fatal(e)
			}
			for _, path := range []string{input, repaired} {
				out, e := exec.Command(oracle, "read", path).CombinedOutput()
				if e != nil {
					t.Fatalf("pinned OCCT transfer of %s: %v\n%s", path, e, out)
				}
				var got struct {
					Volume, Area  float64
					Centroid      [3]float64
					Faces, Solids int
				}
				if e := json.Unmarshal([]byte(strings.TrimSpace(string(out))), &got); e != nil ||
					got.Faces != 6 || got.Solids != 1 {
					t.Fatalf("pinned OCCT transferred an invalid shape: %s (%v)", out, e)
				}
				checkRationalTiltMeasures(t, got.Volume, got.Area, got.Centroid, tc)
			}
		})
	}
}
