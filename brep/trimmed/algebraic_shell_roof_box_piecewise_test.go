package trimmed

import (
	"context"
	"encoding/json"
	"math/big"
	"os"
	"os/exec"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestOriginalCreasedRoofMaterial(t *testing.T) {
	ctx := context.Background()
	a := weightedIdentityRoofStockMode(t, false, 2, false, true)
	b := weightedIdentityRoofStock(t, true, 2)
	sec, err := SectionSplineShellsAlgebraic(ctx, a, b, AlgebraicShellSectionOptions{})
	if err != nil || !sec.Complete {
		t.Fatal("original creased shells", sec.Coverage, err)
	}
	m, err := sec.CombineAlgebraic(ctx, parametric.Difference, AlgebraicShellBooleanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	q, err := m.PrepareRoofMaterialQuery(ctx, MixedRoofQueryOptions{})
	if err != nil || len(q.box[0].roofSpans) != 2 {
		t.Fatal("two distinct original roof spans", err)
	}
	r := big.NewRat
	for _, c := range []struct {
		z     *big.Rat
		roots []*big.Rat
		spans int
	}{{r(9, 4), []*big.Rat{r(1, 4), r(3, 4)}, 1}, {r(5, 2), []*big.Rat{r(1, 2)}, 0}} {
		for _, sign := range []int64{1, -1} {
			x := r((1-sign)/2, 1)
			line, err := q.IntersectLineRational(ctx, [3]*big.Rat{x, r(1, 2), c.z}, [3]*big.Rat{r(sign, 1), r(0, 1), r(0, 1)}, r(-1, 8), r(9, 8), MixedRoofLineOptions{})
			if err != nil || len(line.Events) != len(c.roots) || len(line.Spans) != c.spans || line.Start != parametric.Outside || line.End != parametric.Outside {
				t.Fatal("piecewise original line", c, sign, line, err)
			}
			for i, event := range line.Events {
				root, err := event.Parameter.Classify(ctx, geom.ExactCurvePlaneEventOptions{})
				if err != nil || root.Rational == nil || root.Rational.Cmp(c.roots[i]) != 0 || len(event.Owners) != 1 || event.Owners[0] != (MixedRoofFaceOwner{0, 1}) {
					t.Fatal("knot is not a new face and roof hits stay exact", root, event, err)
				}
			}
		}
	}
	for _, clip := range [][2]*big.Rat{{r(1, 2), r(1, 2)}, {r(1, 2), r(7, 8)}, {r(1, 8), r(1, 2)}} {
		line, err := q.IntersectLineRational(ctx,
			[3]*big.Rat{r(0, 1), r(1, 2), r(5, 2)}, [3]*big.Rat{r(1, 1), r(0, 1), r(0, 1)},
			clip[0], clip[1], MixedRoofLineOptions{})
		if err != nil || len(line.Events) != 1 || line.Events[0].Before != parametric.Outside || line.Events[0].After != parametric.Outside {
			t.Fatal("crease contact at an exact clip end", clip, line, err)
		}
	}
	line, err := q.IntersectLineRational(ctx,
		[3]*big.Rat{r(0, 1), r(1, 2), r(2, 1)}, [3]*big.Rat{r(1, 1), r(0, 1), r(1, 1)},
		r(-1, 8), r(9, 8), MixedRoofLineOptions{})
	if err != nil || len(line.Events) != 2 || len(line.Spans) != 1 || line.Spans[0].Location != parametric.Boundary ||
		len(line.Spans[0].Owners) != 1 || line.Spans[0].Owners[0] != (MixedRoofFaceOwner{0, 1}) {
		t.Fatal("coincident run ending at a roof crease", line, err)
	}
	normals, err := q.BoundaryNormalsRational(ctx, [3]*big.Rat{r(1, 2), r(1, 2), r(5, 2)}, geom.SurfacePatchEvaluationOptions{})
	if err != nil || len(normals.Normals) != 2 {
		t.Fatal("crease exposes both one-sided normals", normals, err)
	}
	for i, n := range normals.Normals {
		if n.Owner != (MixedRoofFaceOwner{0, 1}) || n.Direction[0].Cmp(r(int64(2*i-1), 1)) != 0 || n.Direction[1].Sign() != 0 || n.Direction[2].Cmp(r(1, 1)) != 0 {
			t.Fatal("wrong one-sided crease normal or owner", n)
		}
	}
	// Integrating 2+x on the left and 3-x on the right gives 9/4.
	// The original buried tool removes 27/64.
	volume, err := q.BoundVolume(ctx, MixedRoofVolumeOptions{AbsoluteGap: r(1, 30), SurfacePatchEvaluationOptions: geom.SurfacePatchEvaluationOptions{MaxOperations: 4000000}})
	if err != nil || volume.Volume.Lower.Cmp(r(117, 64)) > 0 || volume.Volume.Upper.Cmp(r(117, 64)) < 0 {
		t.Fatal("creased original cavity volume", volume, err)
	}
	if program := os.Getenv("PLAINSOLID_OCCT_CREASED_ROOF"); program != "" {
		data, err := exec.CommandContext(ctx, program, "creased-mass").Output()
		var ref struct {
			Name     string
			Volume   float64
			Centroid [3]float64
			Inertia  [3][3]float64
		}
		if err != nil || json.Unmarshal(data, &ref) != nil || ref.Name != "difference" {
			t.Fatal("missing independent creased cavity", ref, err)
		}
		mass, err := q.BoundMass(ctx, MixedRoofMassOptions{MaxDepth: 8})
		contains := func(v geom.ExactParameterInterval, x float64) bool {
			p := new(big.Rat).SetFloat64(x)
			return p != nil && v.Lower != nil && v.Lower.Cmp(p) <= 0 && v.Upper.Cmp(p) >= 0
		}
		if err != nil || !contains(mass.Volume, ref.Volume) {
			t.Fatal("original creased mass excludes OCCT", mass, ref, err)
		}
		for i := range 3 {
			if !contains(mass.Centroid[i], ref.Centroid[i]) {
				t.Fatal("original creased centroid excludes OCCT", i, mass.Centroid[i], ref.Centroid[i])
			}
			for j := range 3 {
				if !contains(mass.Inertia[i][j], ref.Inertia[i][j]) {
					t.Fatal("original creased tensor excludes OCCT", i, j, mass.Inertia[i][j], ref.Inertia[i][j])
				}
			}
		}
	}
}
