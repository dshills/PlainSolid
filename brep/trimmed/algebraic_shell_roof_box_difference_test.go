package trimmed

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"os/exec"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestOrderedRoofBoxDifferenceNative(t *testing.T) {
	ctx := context.Background()
	stock, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 4}, 0)
	cutter := weightedIdentityRoofStock(t, false, 2)
	part, h, err := SubtractOrderedRoofBoxes(ctx, stock, cutter, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{})
	if err != nil || part.Topology().Faces != 6 || part.Topology().EulerCharacteristic != 2 || len(h.Faces) != 6 || h.Faces[1] != (MixedRoofFaceOwner{1, 1}) || !h.Reversed[1] {
		t.Fatal("native roof-band difference and reversed original cutter", part.Topology(), h, err)
	}
	for i, owner := range h.Faces {
		if i != 1 && (h.Reversed[i] || owner.Operand != 0) {
			t.Fatal("incorrect unchanged stock side ledger", i, owner)
		}
	}
	mass, err := part.MassProperties(ctx, parametric.IntegrationOptions{})
	want := 4.5 - 3*math.Log(2)
	if err != nil || math.Abs(mass.Volume-want) > 1e-8 {
		t.Fatal("independent exact band integral", mass, want, err)
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_ORDERED_ROOF_COMMON"); oracle != "" {
		data, err := exec.CommandContext(ctx, oracle, "weighted-ordered-difference").Output()
		var ref struct {
			Volume, Area  float64
			Solids, Euler int
		}
		if err != nil || json.Unmarshal(data, &ref) != nil || ref.Solids != 1 || ref.Euler != 2 || math.Abs(ref.Volume-mass.Volume) > 1e-8 || math.Abs(ref.Area-mass.Area) > 1e-8 {
			t.Fatal("independent pinned OCCT weighted Boolean Cut", mass, ref, err, string(data))
		}
	}
	for _, tc := range []struct {
		max, min  float64
		wantFaces int
	}{{5, -1, 0}, {0, -2, 6}} {
		other, _ := makeOpenSplineBox(t, [3]float64{-1, -1, tc.min}, [3]float64{2, 2, tc.max}, 0)
		out, h, err := SubtractOrderedRoofBoxes(ctx, stock, other, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{})
		if err != nil || len(h.Faces) != tc.wantFaces || out.Topology().Faces != tc.wantFaces {
			t.Fatal("covered or boundary-only stock result", tc, out.Topology(), h, err)
		}
	}
	bad, _ := makeOpenSplineBox(t, [3]float64{.125, 0, 1}, [3]float64{1, 1, 2}, 0)
	if out, h, err := SubtractOrderedRoofBoxes(ctx, stock, bad, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{}); !errors.Is(err, geom.ErrUnrepresentable) || out.Topology().Faces != 0 || len(h.Faces) != 0 {
		t.Fatal("unqualified two-band result leaked output", out.Topology(), h, err)
	}
}

func TestOrderedRoofBoxDifferenceFloatingCutter(t *testing.T) {
	ctx := context.Background()
	stock, _ := makeOpenSplineBox(t, [3]float64{0, 0, -1}, [3]float64{1, 1, 4}, 0)
	cutter := weightedIdentityRoofStock(t, false, 2)
	out, h, err := SubtractOrderedRoofBoxes(ctx, stock, cutter, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{})
	if err != nil || len(h.Faces) != 12 || len(h.Reversed) != 12 || h.Faces[0] != (MixedRoofFaceOwner{1, 0}) || h.Faces[7] != (MixedRoofFaceOwner{1, 1}) {
		t.Fatal("original cutter floor and roof ownership", h, err)
	}
	for i, reversed := range h.Reversed {
		if reversed != (i == 0 || i == 7) {
			t.Fatal("only the two retained cutter caps reverse", h)
		}
	}
	if top := out.Topology(); len(top.Shells) != 2 || top.Faces != 12 || top.Edges != 24 || top.EulerCharacteristic != 4 {
		t.Fatal("separate closed components on either side of the cutter", top)
	}
	mass, err := out.MassProperties(ctx, parametric.IntegrationOptions{})
	want := 5.5 - 3*math.Log(2)
	if err != nil || math.Abs(mass.Volume-want) > 1e-8 {
		t.Fatal("independent volume of both retained bands", mass, want, err)
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_ORDERED_ROOF_COMMON"); oracle != "" {
		data, err := exec.CommandContext(ctx, oracle, "weighted-ordered-split").Output()
		var ref struct {
			Volume, Area  float64
			Centroid      [3]float64
			Solids, Euler int
		}
		if err != nil || json.Unmarshal(data, &ref) != nil || ref.Solids != 2 || ref.Euler != 4 || math.Abs(ref.Volume-mass.Volume) > 1e-8 || math.Abs(ref.Area-mass.Area) > 1e-8 || mass.Centroid.Sub(geom.Vec3{X: ref.Centroid[0], Y: ref.Centroid[1], Z: ref.Centroid[2]}).Length() > 1e-8 {
			t.Fatal("independent pinned OCCT weighted split", mass, ref, err, string(data))
		}
	}
	_, ledger, err := CombineSplineShells(ctx, stock, cutter, parametric.Difference, SplineShellCombineOptions{OrderedRoofCut: &SplineShellRoofOptions{}})
	if err != nil || ledger.Method != "ordered-roof-cut" || len(ledger.FacesA[0]) != 2 || len(ledger.FacesB[0]) != 1 || ledger.FacesB[0][0] != 0 || len(ledger.FacesB[1]) != 1 || ledger.FacesB[1][0] != 7 {
		t.Fatal("split ancestry survives opt-in dispatcher", ledger, err)
	}
	if s, h, err := SubtractOrderedRoofBoxes(ctx, stock, cutter, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{MaxOperations: 1}); !errors.Is(err, geom.ErrConvergence) || s.Topology().Faces != 0 || len(h.Faces) != 0 {
		t.Fatal("failure never returns half of the split", s.Topology(), h, err)
	}
}
