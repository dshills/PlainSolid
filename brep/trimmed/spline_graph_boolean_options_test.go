package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestAutomaticDisjointEllipseLimitsAndLegacy(t *testing.T) {
	ctx := context.Background()
	base, a, b, _ := disjointEllipseGraphFixture(t)
	ap, _ := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
	ap, e := ap.InsertUKnot(.125, 1, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	bp, _ := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
	r, e := geom.DiscoverSplineGraphDisjointEllipses(ctx, ap, bp, geom.DisjointEllipseDiscoveryOptions{})
	if e != nil || !r.Complete {
		t.Fatal(e)
	}
	for _, o := range []geom.DisjointEllipseDiscoveryOptions{{MaxSeparators: 1}, {MaxSeparators: 65}, {GraphPointContactOptions: geom.GraphPointContactOptions{MaxOperations: r.ExactOperations - 1}}, {GraphPointContactOptions: geom.GraphPointContactOptions{MaxCells: 1}}} {
		q, e := geom.DiscoverSplineGraphDisjointEllipses(ctx, ap, bp, o)
		if e == nil || q.Complete || q.Section.Complete {
			t.Fatal("partial discovery", e)
		}
	}
	if q, e := geom.DiscoverSplineGraphDisjointEllipses(ctx, ap, bp, geom.DisjointEllipseDiscoveryOptions{GraphPointContactOptions: geom.GraphPointContactOptions{MaxOperations: r.ExactOperations}}); e != nil || !q.Complete {
		t.Fatal("exact cumulative quota", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if q, h, e := CombineSplineGraphSolidsWithOptions(cancelled, base, a, b, parametric.Difference, 1e-9, SplineGraphBooleanOptions{DisjointEllipses: &geom.DisjointEllipseDiscoveryOptions{}}); e == nil || len(q.faces) != 0 || h.DisjointEllipses != nil {
		t.Fatal("cancelled publication", e)
	}
	// Nil mode delegates verbatim to the previous automatic single-loop route.
	base, a, b = automaticEllipseGraphs(t)
	q, h, e := CombineSplineGraphSolidsWithOptions(ctx, base, a, b, parametric.Difference, 1e-9, SplineGraphBooleanOptions{})
	if e != nil || h.RationalEllipseArcs == nil || h.DisjointEllipses != nil || len(q.faces) != 2 {
		t.Fatal("legacy default", e)
	}
	if q, h, e := CombineSplineGraphSolidsWithOptions(ctx, base, a, b, parametric.Difference, 1e-9, SplineGraphBooleanOptions{DisjointEllipses: &geom.DisjointEllipseDiscoveryOptions{}}); e == nil || len(q.faces) != 0 || h.DisjointEllipses != nil {
		t.Fatal("selected unsupported mode", e)
	}
}

func TestAutomaticDisjointEllipseRejectsThirdActualLoop(t *testing.T) {
	base, _, b, _ := disjointEllipseGraphFixture(t)
	us := []float64{0, .125, .25, .375, .5, .75, 1}
	terms := []float64{1, -1, 1, -1, 1, -3, 9}
	var poles []geom.Vec3
	for i, u := range us {
		for j, v := range []float64{0, .5, 1} {
			z := 3 + u*v/64 + 1.0/1024 - terms[i]/64 - []float64{.25, -.25, .25}[j]
			poles = append(poles, geom.Vec3{X: u, Y: v, Z: z})
		}
	}
	a, e := geom.NewBSplineSurface(2, 2, 7, 3, poles, []float64{0, 0, 0, .25, .25, .5, .5, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	// Independent witnesses on all three closed contacts; no two-slab hint may
	// silently publish only two of these genuine original branches.
	ap, _ := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
	bp, _ := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
	for _, center := range []float64{.125, .375, .625} {
		u, v := new(big.Rat).SetFloat64(center+1.0/32), big.NewRat(1, 2)
		pa, ea := ap.EvaluateRational(context.Background(), u, v, geom.SurfacePatchEvaluationOptions{})
		pb, eb := bp.EvaluateRational(context.Background(), u, v, geom.SurfacePatchEvaluationOptions{})
		if ea != nil || eb != nil {
			t.Fatal(ea, eb)
		}
		for axis := range pa.Point {
			if pa.Point[axis].Cmp(pb.Point[axis]) != 0 {
				t.Fatal("third actual branch witness")
			}
		}
	}

	q, h, e := CombineSplineGraphSolidsWithOptions(context.Background(), base, a, b, parametric.Difference, 1e-9, SplineGraphBooleanOptions{DisjointEllipses: &geom.DisjointEllipseDiscoveryOptions{}})
	if e == nil || len(q.faces) != 0 || h.DisjointEllipses != nil {
		t.Fatal("omitted actual branch", e)
	}
}
