package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestProjectiveContactArcEvaluationRetainsBothOriginalUV(t *testing.T) {
	ctx := context.Background()
	a, b := projectiveTinyLinearHeightCharts(t)
	g, err := TraceProjectiveSurfaceContacts(ctx, a, b, [2]int{0, 1}, ProjectiveSurfaceContactGraphOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if axis, err := g.ArcFreeAxis(0); err != nil || axis != 1 {
		t.Fatal("unexpected independent clock", axis, err)
	}
	// Alter returned inspection data. It cannot change the sealed proof.
	g.Cover.Atlas.Overlap.BStationRatios[1].SetInt64(5)
	g.Cover.Atlas.Cells[2].Common.V.Upper.SetInt64(7)
	g.Cover.Isolations[2].Arcs[0].Certificate.FixedParameterAxis = 1
	g.Arcs[0].Cell = 0
	station := big.NewRat(3, 7)
	point, used, err := g.EvaluateArcRational(ctx, 0, station, SurfacePatchEvaluationOptions{})
	if err != nil || used == 0 || point.OnOverlapBoundary {
		t.Fatal("original nonbinary interior contact was lost", point, used, err)
	}
	for _, check := range []struct {
		root *ExactAlgebraicRoot
		want *big.Rat
	}{{point.Common[0], big.NewRat(1, 2)}, {point.Common[1], station},
		{point.OriginalUV[0][0], big.NewRat(1, 2)}, {point.OriginalUV[0][1], big.NewRat(5, 7)},
		{point.OriginalUV[1][0], big.NewRat(-22, 19)}, {point.OriginalUV[1][1], big.NewRat(32, 5)}} {
		assertProjectiveBoundaryRoot(t, check.root, check.want)
	}
	if _, _, err := g.EvaluateArcRational(ctx, 0, big.NewRat(19, 20), SurfacePatchEvaluationOptions{}); !errors.Is(err, ErrDomain) {
		t.Fatal("the original first arc may not extrapolate into the next cell", err)
	}
	if q, _, err := g.EvaluateArcRational(ctx, 0, station, SurfacePatchEvaluationOptions{MaxOperations: used - 1}); !errors.Is(err, ErrConvergence) || q.Common[0] != nil {
		t.Fatal("incomplete work must not publish algebraic parameters", q, err)
	}
	forged := ProjectiveSurfaceContactGraph{Complete: true, Cover: g.Cover, Arcs: g.Arcs}
	if _, _, err := forged.EvaluateArcRational(ctx, 0, station, SurfacePatchEvaluationOptions{}); !errors.Is(err, ErrInvalid) {
		t.Fatal("public contact metadata is not an evaluable geometry proof", err)
	}
}

func TestProjectiveContactArcEvaluationRetainsAnIrrationalCoordinate(t *testing.T) {
	ctx := context.Background()
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	b, err := NewSurfacePatch(quadraticContactSurface(t, -.125), SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	g, err := TraceProjectiveSurfaceContacts(ctx, a, b, [2]int{0, 1}, ProjectiveSurfaceContactGraphOptions{})
	if err != nil {
		t.Fatal(err)
	}
	station := big.NewRat(1, 2)
	for i := range g.Arcs {
		q, _, err := g.EvaluateArcRational(ctx, i, station, SurfacePatchEvaluationOptions{})
		if errors.Is(err, ErrDomain) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		free, _ := g.ArcFreeAxis(i)
		assertProjectiveBoundaryRoot(t, q.Common[free], station)
		root := q.Common[1-free]
		poly := []*big.Rat{big.NewRat(1, 8), big.NewRat(-1, 1), big.NewRat(1, 1)}
		sign, err := root.SignPolynomial(ctx, poly, ExactCurvePlaneEventOptions{})
		if err != nil || sign.Sign != 0 || root.Interval().Lower.Cmp(root.Interval().Upper) == 0 {
			t.Fatal("expected the exact irrational circle intersection", root.Interval(), sign, err)
		}
		for _, owner := range q.OriginalUV {
			order, err := root.Compare(ctx, owner[1-free], ExactCurvePlaneEventOptions{})
			if err != nil || !order.Proven || order.Order != 0 {
				t.Fatal("both original surfaces must retain the same algebraic station", order, err)
			}
		}
		return
	}
	t.Fatal("the contour must meet the original midpoint sections")
}
