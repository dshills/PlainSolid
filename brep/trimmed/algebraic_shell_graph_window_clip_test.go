package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestGraphWindowClipSupportBoundary(t *testing.T) {
	s := graphMaterialBooleanFixture(t, false, true)
	axes := [2]int{0, 2}
	q, e := s.PrepareGraphMaterialQuery(context.Background(), AlgebraicGraphMaterialOptions{Projection: &axes})
	if e != nil {
		t.Fatal(e)
	}
	w := AlgebraicGraphProjectionWindow{Min: [2]*big.Rat{big.NewRat(2, 3), big.NewRat(17, 4)}, Max: [2]*big.Rat{big.NewRat(7, 3), big.NewRat(19, 4)}}
	o := AlgebraicGraphWindowRefineOptions{MaxCells: 341, MaxDepth: 4}
	r, e := q.RefineClippedWindowDepth(context.Background(), w, o)
	if e != nil {
		t.Fatal(e)
	}
	if !r.HasBounds || !r.HasAreaOverlap || r.Complete || r.UnresolvedArea.Sign() <= 0 || r.ProjectionAxes != axes || r.ResidualAxis != 1 {
		t.Fatalf("invalid aggregate %+v", r)
	}
	if r.RequestedArea.Cmp(big.NewRat(5, 6)) != 0 || r.OutsideArea.Cmp(big.NewRat(11, 24)) != 0 || r.Clipped.Min[0].Cmp(big.NewRat(9, 8)) != 0 || r.Clipped.Max[0].Cmp(big.NewRat(15, 8)) != 0 {
		t.Fatal("exact clipping", r.RequestedArea, r.OutsideArea, r.Clipped)
	}
	sum := new(big.Rat).Add(r.MaterialArea, r.EmptyArea)
	sum.Add(sum, r.UnresolvedArea)
	if sum.Cmp(r.RequestedArea) != 0 {
		t.Fatal("target area conservation")
	}
	exact := math.Pi * math.Pow(5.0/64, 4)
	lo, _ := r.Volume[0].Float64()
	hi, _ := r.Volume[1].Float64()
	if lo > exact || hi < exact {
		t.Fatal("independent two-lobe integral", lo, hi, exact)
	}
	// Same interior query proves clipping adds only exact zero-volume exterior.
	inner, e := q.RefineWindowDepth(context.Background(), r.Clipped, o)
	if e != nil || inner.Volume[0].Cmp(r.Volume[0]) != 0 || inner.Volume[1].Cmp(r.Volume[1]) != 0 {
		t.Fatal("interior bound changed", e)
	}
	for _, outside := range []AlgebraicGraphProjectionWindow{{Min: [2]*big.Rat{big.NewRat(-2, 3), big.NewRat(17, 4)}, Max: [2]*big.Rat{big.NewRat(-1, 3), big.NewRat(19, 4)}}, {Min: [2]*big.Rat{big.NewRat(0, 1), big.NewRat(17, 4)}, Max: [2]*big.Rat{big.NewRat(9, 8), big.NewRat(19, 4)}}} {
		empty, e := q.RefineClippedWindowDepth(context.Background(), outside, o)
		if e != nil || !empty.HasBounds || !empty.Complete || empty.HasAreaOverlap || empty.Volume[1].Sign() != 0 || empty.OutsideArea.Cmp(empty.RequestedArea) != 0 || len(empty.Inside.Leaves) != 0 {
			t.Fatal("outside/tangent area", e, empty)
		}
	}
	// Unsupported nonrectangular trim and projected shear cannot become empty
	// merely because this particular query lies outside their bounding boxes.
	badQuery := *q
	badQuery.faces[0].Outer = badQuery.faces[0].Outer[:3]
	bad, e := badQuery.RefineClippedWindowDepth(context.Background(), w, o)
	if !errors.Is(e, geom.ErrConvergence) || bad.HasBounds {
		t.Fatal("nonrectangle", e)
	}
	shear := geom.Identity()
	shear[0][2] = .25
	p, e := q.patches[0].Transformed(shear)
	if e != nil {
		t.Fatal(e)
	}
	g, e := geom.NewAffineProjectedGraph(context.Background(), p, axes, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	badQuery = *q
	badQuery.graphs[0] = g
	bad, e = badQuery.RefineClippedWindowDepth(context.Background(), w, o)
	if !errors.Is(e, geom.ErrConvergence) || bad.HasBounds {
		t.Fatal("shear", e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	bad, e = q.RefineClippedWindowDepth(ctx, w, o)
	if !errors.Is(e, context.Canceled) || bad.HasBounds || bad.RequestedArea != nil {
		t.Fatal("cancel", e)
	}
	bad, e = q.RefineClippedWindowDepth(context.Background(), w, AlgebraicGraphWindowRefineOptions{CellOptions: AlgebraicGraphWindowOptions{Exact: geom.ExactCurvePlaneEventOptions{MaxOperations: 1}}})
	if !errors.Is(e, geom.ErrConvergence) || bad.HasBounds {
		t.Fatal("budget", e)
	}
	r.Requested.Min[0].SetInt64(99)
	if w.Min[0].Cmp(big.NewRat(2, 3)) != 0 {
		t.Fatal("input alias")
	}
	t.Logf("outside area11/24; interior unresolved%s V[%g,%g]", r.UnresolvedArea, lo, hi)
}
