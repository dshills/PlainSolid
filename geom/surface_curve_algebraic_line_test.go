package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestFiniteLineAlgebraicIsolatedClipping(t *testing.T) {
	p, uv, _, _ := algebraicSeamFixture(t)
	cells, e := p.BuildSurfaceCurveCompositionCells(context.Background(), uv, SurfaceCurveAgreementOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, cell := range cells.Cells {
		r, e := cell.IntersectFiniteLine(context.Background(), Vec3{X: .5, Z: 1.5}, Vec3{Y: 1, Z: -1}, [2]*big.Rat{big.NewRat(1, 2), big.NewRat(3, 4)}, ComposedCurveLineOptions{})
		if e != nil || !r.Complete || r.Coincident || len(r.Hits) != 1 {
			t.Fatal(r, e)
		}
		bound := r.Hits[0].LineParameter
		if new(big.Rat).Mul(bound.Lower, bound.Lower).Cmp(big.NewRat(1, 2)) > 0 || new(big.Rat).Mul(bound.Upper, bound.Upper).Cmp(big.NewRat(1, 2)) < 0 {
			t.Fatal("line parameter enclosure")
		}
		r, e = cell.IntersectFiniteLine(context.Background(), Vec3{X: .5, Z: 1.5}, Vec3{Y: 1, Z: -1}, [2]*big.Rat{new(big.Rat), big.NewRat(2, 3)}, ComposedCurveLineOptions{})
		if e != nil || !r.Complete || len(r.Hits) != 0 {
			t.Fatal(r, e)
		}
		r, e = cell.IntersectFiniteLine(context.Background(), Vec3{X: .5, Z: 1.5}, Vec3{Y: -2, Z: 2}, [2]*big.Rat{big.NewRat(-1, 2), big.NewRat(-1, 3)}, ComposedCurveLineOptions{})
		if e != nil || !r.Complete || len(r.Hits) != 1 || r.Hits[0].LineParameter.Upper.Sign() >= 0 {
			t.Fatal(r, e)
		}
	}
	// Rational model contact is exactly on a finite cap, not tolerance-snapped.
	cell := cells.Cells[0]
	r, e := cell.IntersectFiniteLine(context.Background(), Vec3{Y: .5, Z: .375}, Vec3{X: 1}, [2]*big.Rat{big.NewRat(1, 4), big.NewRat(1, 2)}, ComposedCurveLineOptions{})
	if e != nil || !r.Complete || len(r.Hits) != 1 || r.Hits[0].BoundaryMask != 1 || r.Hits[0].LineParameter.Lower.Cmp(big.NewRat(1, 4)) != 0 {
		t.Fatal(r, e)
	}
	lower := new(big.Rat).Add(big.NewRat(1, 4), new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100)))
	r, e = cell.IntersectFiniteLine(context.Background(), Vec3{Y: .5, Z: .375}, Vec3{X: 1}, [2]*big.Rat{lower, big.NewRat(1, 2)}, ComposedCurveLineOptions{})
	if e != nil || !r.Complete || len(r.Hits) != 0 {
		t.Fatal(r, e)
	}
}
func TestFiniteLineCoincidentPortionsAndTouch(t *testing.T) {
	s, _ := NewBSplineSurface(1, 1, 2, 2, []Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	p, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	// x=1/4+(t-1/2)^2 is a backtracking line curve entirely in the patch.
	uv, e := NewExactHomogeneousBezier(context.Background(), [][4]*big.Rat{{big.NewRat(1, 2), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, {new(big.Rat), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(1, 2), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cells, e := p.BuildSurfaceCurveCompositionCells(context.Background(), uv, SurfaceCurveAgreementOptions{})
	if e != nil || len(cells.Cells) != 1 {
		t.Fatal(cells, e)
	}
	cell := cells.Cells[0]
	r, e := cell.IntersectFiniteLine(context.Background(), Vec3{}, Vec3{X: 1}, [2]*big.Rat{big.NewRat(3, 10), big.NewRat(2, 5)}, ComposedCurveLineOptions{})
	if e != nil || !r.Complete || !r.Coincident || len(r.Portions) != 2 || len(r.Hits) != 0 {
		t.Fatal(r, e)
	}
	for _, portion := range r.Portions {
		order, e := portion.Start.CompareParameter(context.Background(), portion.End, ExactCurvePlaneEventOptions{})
		if e != nil || !order.Proven || order.Order != -1 {
			t.Fatal(order, e)
		}
	}
	r, e = cell.IntersectFiniteLine(context.Background(), Vec3{}, Vec3{X: 1}, [2]*big.Rat{new(big.Rat), big.NewRat(1, 4)}, ComposedCurveLineOptions{})
	if e != nil || !r.Complete || len(r.Portions) != 0 || len(r.Hits) != 1 || r.Hits[0].BoundaryMask != 2 {
		t.Fatal(r, e)
	}
	r, e = cell.IntersectFiniteLine(context.Background(), Vec3{}, Vec3{X: 1}, [2]*big.Rat{big.NewRat(1, 4), big.NewRat(1, 4)}, ComposedCurveLineOptions{})
	if e != nil || !r.Complete || len(r.Portions) != 0 || len(r.Hits) != 1 || r.Hits[0].BoundaryMask != 3 {
		t.Fatal(r, e)
	}
	r, e = cell.IntersectFiniteLine(context.Background(), Vec3{}, Vec3{X: 1}, [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}, ComposedCurveLineOptions{ExactCurvePlaneEventOptions: ExactCurvePlaneEventOptions{MaxOperations: 1}})
	if e == nil || r.Complete {
		t.Fatal(r, e)
	}
}
