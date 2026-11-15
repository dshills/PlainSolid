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
	"testing"

	"plainsolid/brep/parametric"
)

func rationalTiltGroupedSource(t testing.TB) []byte {
	t.Helper()
	raw, e := os.ReadFile("testdata/occt-both-default-tilted-oblique-prism-fillet.step")
	if e != nil {
		t.Fatal(e)
	}
	return raw
}

func checkRationalTiltGroupedMeasures(t testing.TB, volume, area float64, center [3]float64) {
	t.Helper()
	if math.Abs(volume-29.077978807115539) > 1e-11 || math.Abs(area-65.622954970084379) > 1e-11 {
		t.Fatal(volume, area)
	}
	want := [3]float64{2.7627619257740741, 1.0271143655601034, .48298409896790118}
	for i, got := range center {
		if math.Abs(got-want[i]) > 1e-11 {
			t.Fatal("world centroid", center, want)
		}
	}
}

func TestRepairRationalPosedGroupedPCurveOCCTConvexArc(t *testing.T) {
	ctx := context.Background()
	raw := rationalTiltGroupedSource(t)
	opts := ConvexArcRepairOptions{MaxDistance: 1e-12}
	m, e := RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(raw), Limits{}, opts)
	if e != nil {
		t.Fatal(e)
	}
	if m.OriginalSTEP() != string(raw) || len(m.Repairs()) != 8 || len(m.Tangencies()) != 8 ||
		len(m.Reversals()) != 2 || len(m.PCurves()) != 28 || len(m.RationalCaps()) != 2 ||
		len(m.FiniteTrims()) != 2 || len(m.FiniteLineOwners()) != 8 || len(m.AxialTrims()) != 1 ||
		m.ReferenceCircleID() != 146 || len(m.Solid().Faces()) != 7 {
		t.Fatal("incomplete original grouped evidence")
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	fixed, e := Parse(bytes.NewBufferString(m.CorrectedModelSTEP()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, q := range m.Repairs() {
		original, e := occtLoopTriple(f, q.PointID, "CARTESIAN_POINT")
		if e != nil || !convexUVSame(original, q.Original) {
			t.Fatal("wrong original world token", q, e)
		}
		repaired, e := occtLoopTriple(fixed, q.PointID, "CARTESIAN_POINT")
		delta := occtLoopSub(q.Repaired, q.Original)
		if e != nil || !convexUVSame(repaired, q.Repaired) || occtLoopDot(delta, delta).Cmp(q.SquaredDistance) != 0 {
			t.Fatal("wrong corrected world token", q, e)
		}
	}
	axial := m.AxialTrims()[0]
	if axial.EdgeID != 252 || axial.LineID != 256 || axial.OriginPointID != 257 ||
		axial.VertexIDs != ([2]int64{195, 253}) || axial.OriginalParameters[0].Sign() != 0 ||
		axial.RepairedParameters[0].Sign() >= 0 || axial.ProjectionDistanceSquared[1].Sign() <= 0 ||
		axial.StationDistanceSquared[0].Sign() <= 0 || axial.StationDistanceSquared[1].Sign() != 0 {
		t.Fatal("missing source axial phase and support evidence", axial)
	}
	before, e := occtLoopTriple(f, axial.OriginPointID, "CARTESIAN_POINT")
	if e != nil {
		t.Fatal(e)
	}
	after, e := occtLoopTriple(fixed, axial.OriginPointID, "CARTESIAN_POINT")
	if e != nil {
		t.Fatal(e)
	}
	direction := [3]*big.Rat{big.NewRat(4, 5), new(big.Rat), big.NewRat(3, 5)}
	shift := occtLoopSub(after, before)
	if occtLoopDot(shift, direction).Sign() != 0 || occtLoopDot(shift, shift).Cmp(axial.SupportDistanceSquared) != 0 {
		t.Fatal("world support shift changed axial speed or station", axial)
	}
	tol := new(big.Rat).SetFloat64(opts.MaxDistance)
	tol2 := new(big.Rat).Mul(tol, tol)
	for i, id := range axial.VertexIDs {
		v, e := simple(f, id, "VERTEX_POINT", 2)
		if e != nil {
			t.Fatal(e)
		}
		p, e := occtLoopTriple(f, v.Args[1].Ref, "CARTESIAN_POINT")
		if e != nil {
			t.Fatal(e)
		}
		q, e := occtLoopTriple(fixed, v.Args[1].Ref, "CARTESIAN_POINT")
		if e != nil {
			t.Fatal(e)
		}
		src := occtLoopSub(p, before)
		srcT := occtLoopDot(src, direction)
		dstT := occtLoopDot(occtLoopSub(q, after), direction)
		var perpendicular [3]*big.Rat
		for axis := range src {
			perpendicular[axis] = new(big.Rat).Sub(src[axis], new(big.Rat).Mul(srcT, direction[axis]))
		}
		drift := new(big.Rat).Sub(srcT, dstT)
		drift.Mul(drift, drift)
		finiteMovement := new(big.Rat).Add(drift, axial.SupportDistanceSquared)
		if srcT.Cmp(axial.OriginalParameters[i]) != 0 || dstT.Cmp(axial.RepairedParameters[i]) != 0 ||
			drift.Cmp(axial.StationDistanceSquared[i]) != 0 ||
			occtLoopDot(perpendicular, perpendicular).Cmp(axial.ProjectionDistanceSquared[i]) != 0 ||
			finiteMovement.Cmp(tol2) > 0 {
			t.Fatal("finite original and repaired line station mismatch", i, axial)
		}
	}
	axial.RepairedParameters[0].SetInt64(0)
	if m.AxialTrims()[0].RepairedParameters[0].Sign() >= 0 {
		t.Fatal("axial trim getter aliases proof")
	}

	var sourceCapRepaired bool
	for _, trim := range m.FiniteTrims() {
		if !trim.StartsAtSeam || trim.Wraps || trim.EndStation == nil || trim.CapCoedgeID == 0 || trim.CylinderCoedgeID == 0 {
			t.Fatal("incomplete cap/cylinder finite correspondence", trim)
		}
		for i, id := range []int64{trim.StartVertexID, trim.EndVertexID} {
			if id == 195 {
				sourceCapRepaired = true
			}
			if trim.EndpointDistanceUpper[i].Sign() <= 0 || trim.EndpointDistanceUpper[i].Cmp(tol) > 0 {
				t.Fatal("original cap endpoint exceeded budget", trim)
			}
		}
	}
	if !sourceCapRepaired {
		t.Fatal("unequal axial vertex must also own its original rational cap")
	}
	for _, cap := range m.RationalCaps() {
		if !cap.PositiveAngularVelocity || !cap.ClosedOneTurn || cap.WitnessDistanceSquaredLower.Sign() <= 0 {
			t.Fatal("missing source radial reparameterization proof", cap)
		}
	}
	props, e := m.Solid().MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-10})
	if e != nil {
		t.Fatal(e)
	}
	center, e := m.Pose().TransformPoint(ctx, occtLoopVec(props.Centroid), false, 512)
	if e != nil {
		t.Fatal(e)
	}
	var world [3]float64
	for i, q := range center {
		world[i], _ = q.Float64()
	}
	checkRationalTiltGroupedMeasures(t, props.Volume, props.Area, world)
	if _, e := RepairPosedGroupedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(raw), Limits{}, opts); e == nil {
		t.Fatal("tilted source entered the existing signed-coordinate reader")
	}
	if _, e := RepairRationalPosedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(raw), Limits{}, opts); e == nil {
		t.Fatal("grouped source entered the existing single reader")
	}
}

func TestRepairRationalPosedGroupedPCurveOCCTConvexArcReject(t *testing.T) {
	raw := rationalTiltGroupedSource(t)
	ctx := context.Background()
	opts := ConvexArcRepairOptions{MaxDistance: 1e-12}
	for _, tc := range []struct {
		name string
		edit func(*File)
	}{
		{"unbounded endpoint station", func(f *File) {
			f.Entities[254].Records[0].Args[1].Items[0].Text = "4.000000008"
			f.Entities[254].Records[0].Args[1].Items[2].Text = "3.000000006"
		}},
		{"unbounded original line support", func(f *File) {
			f.Entities[257].Records[0].Args[1].Items[1].Text = "0.250000001"
		}},
		{"unpaired line sense", func(f *File) {
			f.Entities[252].Records[0].Args[4].Text = "F"
		}},
		{"cap weights", func(f *File) {
			representation := f.Entities[229].Records[0].Args[2].Ref
			curve := f.Entities[representation].Records[0].Args[1].Items[0].Ref
			for _, rec := range f.Entities[curve].Records {
				if rec.Type == "RATIONAL_B_SPLINE_CURVE" {
					rec.Args[0].Items[0].Text = "1.01"
				}
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, e := Parse(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			tc.edit(f)
			bad, e := mappedPlanarSourceBytes(f)
			if e != nil {
				t.Fatal(e)
			}
			m, e := RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(bad), Limits{}, opts)
			if e == nil || m.OriginalSTEP() != "" {
				t.Fatal("invalid source published material", e)
			}
		})
	}
	if _, e := RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{MaxDistance: 1e-14}); e == nil {
		t.Fatal("smaller displacement budget should fail")
	}
}

func TestRepairRationalPosedGroupedPCurveOCCTConvexArcLive(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL")
	if oracle == "" {
		t.Skip("set PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL to the pinned author helper")
	}
	dir := t.TempDir()
	input := filepath.Join(dir, "original.step")
	if out, e := exec.Command(oracle, "write-both-default-tilted", input).CombinedOutput(); e != nil {
		t.Fatalf("pinned OCCT author: %v\n%s", e, out)
	}
	raw, e := os.ReadFile(input)
	if e != nil {
		t.Fatal(e)
	}
	m, e := RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion(context.Background(), bytes.NewReader(raw), Limits{}, ConvexArcRepairOptions{MaxDistance: 1e-12})
	if e != nil {
		t.Fatal(e)
	}
	repaired := filepath.Join(dir, "corrected-world.step")
	if e = os.WriteFile(repaired, []byte(m.CorrectedModelSTEP()), 0600); e != nil {
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
		if e = json.Unmarshal(bytes.TrimSpace(out), &got); e != nil || got.Faces != 7 || got.Solids != 1 {
			t.Fatalf("pinned OCCT transferred an invalid shape: %s (%v)", out, e)
		}
		checkRationalTiltGroupedMeasures(t, got.Volume, got.Area, got.Centroid)
	}
}
