package trimmed

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"os/exec"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestCoincidentRoofBoxPointLocation(t *testing.T) {
	ctx := context.Background()
	a, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{2, 2, 2}, 0)
	share, _ := makeOpenSplineBox(t, [3]float64{1, 0, 0}, [3]float64{3, 2, 2}, 0)
	adjacent, _ := makeOpenSplineBox(t, [3]float64{2, 1, 1}, [3]float64{4, 3, 3}, 0)
	pts := [3][4][3]int64{
		{{3, 2, 4}, {2, 2, 2}, {0, 2, 4}, {4, 4, 4}},
		{{4, 3, 3}, {4, 2, 3}, {4, 4, 4}, {4, 1, 3}},
		{{0, 0, 0}, {2, 2, 4}, {2, 2, 2}, {-2, 2, 2}},
	}
	const O, B, I = parametric.Outside, parametric.Boundary, parametric.Inside
	want := [3][3][4]parametric.Location{
		{{B, B, O, B}, {B, I, B, B}, {O, B, B, O}},
		{{O, O, O, O}, {I, B, B, B}, {B, B, B, B}},
		{{B, B, I, O}, {B, B, I, O}, {O, O, O, O}},
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_COINCIDENT_ROOF_AREA"); oracle != "" {
		data, err := exec.CommandContext(ctx, oracle, "point").Output()
		// Location's uint8 representation would make JSON decode byte strings.
		var refs [3][3][4]int
		if err != nil || json.Unmarshal(data, &refs) != nil {
			t.Fatal("independent regularized box locations", err, string(data))
		}
		for i := range want {
			for j := range want[i] {
				for k, location := range want[i][j] {
					if parametric.Location(refs[i][j][k]) != location {
						t.Fatal("independent regularized Boolean classification", i, j, k, refs[i][j][k], location)
					}
				}
			}
		}
	}
	for i, b := range []Solid{share, adjacent, a} {
		for j, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference} {
			q, err := PrepareRoofBoxPointQuery(ctx, a, b, op, MixedRoofQueryOptions{})
			if err != nil {
				t.Fatal(err)
			}
			for k, p := range pts[i] {
				point := [3]*big.Rat{big.NewRat(p[0], 2), big.NewRat(p[1], 2), big.NewRat(p[2], 2)}
				out, err := q.LocateRational(ctx, point, geom.SurfacePatchEvaluationOptions{})
				if err != nil || out.Location != want[i][j][k] {
					t.Fatal("regularized source incidence", i, op, k, out, err)
				}
				if i == 1 && k == 0 && out.Operands != ([2]parametric.Location{B, B}) {
					t.Fatal("shared wall retains both source contacts", out)
				}
			}
		}
	}
	q, err := PrepareRoofBoxPointQuery(ctx, a, a, parametric.Difference, MixedRoofQueryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	point := [3]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 1), big.NewRat(2, 1)}
	out, err := q.LocateRational(ctx, point, geom.SurfacePatchEvaluationOptions{})
	if err != nil || out.ExactOperations < 2 {
		t.Fatal("exact cancellation proof", out, err)
	}
	if bad, err := q.LocateRational(ctx, point, geom.SurfacePatchEvaluationOptions{MaxOperations: out.ExactOperations - 1}); !errors.Is(err, geom.ErrConvergence) || bad != (MixedRoofMaterialPoint{}) {
		t.Fatal("one-less operation quota must fail atomically", bad, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if bad, err := q.LocateRational(cancelled, point, geom.SurfacePatchEvaluationOptions{}); !errors.Is(err, context.Canceled) || bad != (MixedRoofMaterialPoint{}) {
		t.Fatal("cancelled proof leaked a classification", bad, err)
	}
}

func TestRoofBoxNonlinearPointGerms(t *testing.T) {
	ctx := context.Background()
	// The cutter and stock roofs meet at a wall. One side of their local
	// difference is a positive-volume wedge, even though both heights agree.
	a, _ := crossingFootprintRoofShell(t, false)
	b, _ := crossingFootprintRoofShell(t, true)
	// hB-hA = x²-x+3/16 + (y-y²)/64. At y=0 the exact root
	// is x=1/4, lying on A's wall and in B's footprint.
	p := [3]*big.Rat{big.NewRat(1, 4), new(big.Rat), new(big.Rat)}
	for _, tc := range []struct {
		op   parametric.BooleanOp
		want parametric.Location
	}{{parametric.Intersection, parametric.Boundary}, {parametric.Union, parametric.Boundary}, {parametric.Difference, parametric.Boundary}} {
		q, err := PrepareRoofBoxPointQuery(ctx, a, b, tc.op, MixedRoofQueryOptions{})
		if err != nil {
			t.Fatal(err)
		}
		out, err := q.LocateRational(ctx, p, geom.SurfacePatchEvaluationOptions{})
		if err != nil || out.Location != tc.want || out.Operands != ([2]parametric.Location{parametric.Boundary, parametric.Boundary}) {
			t.Fatal("exact wedge at original roof/wall contact", tc.op, out, err)
		}
	}
	// Identical nonplanar roofs cancel, even when the surviving support is
	// rational with a nonconstant denominator or at a genuine roof crease.
	for _, tc := range []struct {
		b Solid
		z *big.Rat
	}{{weightedIdentityRoofStock(t, false, 2), big.NewRat(3, 2)}, {weightedIdentityRoofStockMode(t, false, 2, false, true), big.NewRat(5, 2)}} {
		q, err := PrepareRoofBoxPointQuery(ctx, tc.b, tc.b, parametric.Difference, MixedRoofQueryOptions{})
		if err != nil {
			t.Fatal(err)
		}
		out, err := q.LocateRational(ctx, [3]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2), tc.z}, geom.SurfacePatchEvaluationOptions{})
		if err != nil || out.Location != parametric.Outside || out.Operands != ([2]parametric.Location{parametric.Boundary, parametric.Boundary}) {
			t.Fatal("nonlinear coincident roof has no remaining germ", out, err)
		}
	}
}

func TestRoofBoxTangentialPointGerm(t *testing.T) {
	// At the true tangent (Y-X)^2-X^3, strict negative material occurs only
	// after cancellation along the diagonal and strict positive material
	// occurs along unequal approaches. Replacing -X^3 by +X^3 does not allow
	// this bounded germ proof to rule out a negative region; it must stay open.
	for _, c := range []struct {
		coefficient  int64
		wantPossible uint8
		wantProof    uint8
	}{{-1, 3, 3}, {1, 3, 2}} {
		w := mixedRoofLineWork{o: MixedRoofLineOptions{SurfacePatchEvaluationOptions: geom.SurfacePatchEvaluationOptions{MaxOperations: 1000, MaxCoefficientBits: 128}}}
		r := func(n int64) *big.Rat { return big.NewRat(n, 1) }
		p := roofPowerGrid{{r(0), r(0), r(1)}, {r(0), r(-2)}, {r(1)}, {r(c.coefficient)}}
		proved, possible, err := roofGermSigns(p, &w)
		if err != nil || proved != c.wantProof || possible != c.wantPossible {
			t.Fatal("exact tangent germ proof", c, proved, possible, err)
		}
	}
}
