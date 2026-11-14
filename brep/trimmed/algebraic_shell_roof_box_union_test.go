package trimmed

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"os"
	"os/exec"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestOrderedRoofBoxFuseNative(t *testing.T) {
	ctx := context.Background()
	a, _ := makeOpenSplineBox(t, [3]float64{0, 0, -1}, [3]float64{1, 1, 1}, 0)
	b := weightedIdentityRoofStock(t, false, 2)
	var ref struct {
		Volume, Area  float64
		Centroid      [3]float64
		Solids, Euler int
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_ORDERED_ROOF_COMMON"); oracle != "" {
		data, err := exec.CommandContext(ctx, oracle, "weighted-ordered-union").Output()
		if err != nil || json.Unmarshal(data, &ref) != nil || ref.Solids != 1 || ref.Euler != 2 {
			t.Fatal("independent OCCT rational Fuse", ref, err, string(data))
		}
	}
	for i, src := range [][2]Solid{{a, b}, {b, a}} {
		out, h, err := FuseOrderedRoofBoxes(ctx, src[0], src[1], MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{})
		if err != nil || len(h.Faces) != 6 || len(h.Faces[0]) != 1 || h.Faces[0][0] != (MixedRoofFaceOwner{1 - i, 1}) || len(h.Faces[1]) != 1 || h.Faces[1][0] != (MixedRoofFaceOwner{i, 4}) {
			t.Fatal("native roof and floor ancestry", out.Topology(), h, err)
		}
		for _, f := range h.Faces[2:] {
			if len(f) != 2 || f[0].Operand != 0 || f[1].Operand != 1 {
				t.Fatal("merged wall lost its original region ancestry", f)
			}
		}
		if topo := out.Topology(); len(topo.Shells) != 1 || topo.EulerCharacteristic != 2 || topo.Edges != 12 {
			t.Fatal("one closed canonical output band", topo)
		}
		roof := out.Faces()[0].Surface.(*geom.ExactBezierSurface)
		p, err := roof.ExactPatch().EvaluateRational(ctx, big.NewRat(1, 2), big.NewRat(1, 2), geom.SurfacePatchEvaluationOptions{})
		if err != nil || p.Point[2].Cmp(big.NewRat(3, 2)) != 0 {
			t.Fatal("exact original rational roof", p, err)
		}
		mass, err := out.MassProperties(ctx, parametric.IntegrationOptions{})
		if err != nil || math.Abs(mass.Volume-(3*math.Log(2)+.5)) > 1e-8 {
			t.Fatal("independent complete band integral", mass, err)
		}
		if ref.Solids != 0 && (math.Abs(mass.Volume-ref.Volume) > 1e-8 || math.Abs(mass.Area-ref.Area) > 1e-8 || mass.Centroid.Sub(geom.Vec3{X: ref.Centroid[0], Y: ref.Centroid[1], Z: ref.Centroid[2]}).Length() > 1e-8) {
			t.Fatal("rational Fuse differs from pinned OCCT", mass, ref)
		}
	}
	_, ledger, err := CombineSplineShells(ctx, a, b, parametric.Union, SplineShellCombineOptions{OrderedRoofFuse: &SplineShellRoofOptions{}})
	if err != nil || ledger.Method != "ordered-roof-fuse" || len(ledger.FacesB[1]) != 1 || ledger.FacesB[1][0] != 0 || len(ledger.FacesA[4]) != 1 || ledger.FacesA[4][0] != 1 {
		t.Fatal("optional native Fuse dispatch and ancestry", ledger, err)
	}
}

func TestOrderedRoofBoxFuseContactAndGap(t *testing.T) {
	ctx := context.Background()
	a := orderedRoofTangentSource(t)
	b, _ := makeOpenSplineBox(t, [3]float64{0, -.125, 1}, [3]float64{1, 1.125, 2}, 0)
	out, h, err := FuseOrderedRoofBoxes(ctx, a, b, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{})
	if err != nil || out.Topology().Faces != 6 || len(h.Faces) != 6 {
		t.Fatal("tangential roof to floor meeting does not leave a gap", out.Topology(), h, err)
	}
	mass, err := out.MassProperties(ctx, parametric.IntegrationOptions{})
	if err != nil || math.Abs(mass.Volume-3.125) > 1e-10 {
		t.Fatal("the internal contact must disappear in regularized Fuse", mass, err)
	}
	for _, tc := range []struct {
		min, max [3]float64
	}{
		{[3]float64{0, -.125, 1.0625}, [3]float64{1, 1.125, 2}}, // gap on a positive-area subregion
		{[3]float64{0, -.125, 0}, [3]float64{1, 1.125, 1.125}},  // roof order switches
		{[3]float64{0, -.125, 0}, [3]float64{2, 1.125, 2}},      // different footprints
	} {
		other, _ := makeOpenSplineBox(t, tc.min, tc.max, 0)
		if s, h, err := FuseOrderedRoofBoxes(ctx, a, other, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{}); !errors.Is(err, geom.ErrUnrepresentable) || s.Topology().Faces != 0 || len(h.Faces) != 0 {
			t.Fatal("unqualified single-band result escaped", tc, s.Topology(), h, err)
		}
	}
	if s, h, err := FuseOrderedRoofBoxes(ctx, a, b, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{MaxOperations: 1}); !errors.Is(err, geom.ErrConvergence) || s.Topology().Faces != 0 || len(h.Faces) != 0 {
		t.Fatal("budget exhaustion leaked topology", s.Topology(), h, err)
	}
}
