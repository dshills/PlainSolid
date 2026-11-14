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
	bad, _ := makeOpenSplineBox(t, [3]float64{0, 0, 1}, [3]float64{1, 1, 2}, 0)
	if out, h, err := SubtractOrderedRoofBoxes(ctx, stock, bad, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{}); !errors.Is(err, geom.ErrUnrepresentable) || out.Topology().Faces != 0 || len(h.Faces) != 0 {
		t.Fatal("unqualified two-band result leaked output", out.Topology(), h, err)
	}
}
