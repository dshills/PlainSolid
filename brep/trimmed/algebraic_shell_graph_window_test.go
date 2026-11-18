package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func graphWindow(x, y, d *big.Rat) AlgebraicGraphProjectionWindow {
	return AlgebraicGraphProjectionWindow{Min: [2]*big.Rat{new(big.Rat).Sub(x, d), new(big.Rat).Sub(y, d)}, Max: [2]*big.Rat{new(big.Rat).Add(x, d), new(big.Rat).Add(y, d)}}
}
func checkWindowSamples(t *testing.T, r AlgebraicGraphWindowDepth, w AlgebraicGraphProjectionWindow, height func(*big.Rat, *big.Rat) *big.Rat) {
	t.Helper()
	if !r.Complete || !r.FullMaterial || r.NoFootprint || r.Depth[0].Sign() <= 0 {
		t.Fatalf("not positive material: %+v", r)
	}
	for i := 0; i <= 4; i++ {
		for j := 0; j <= 4; j++ {
			x := new(big.Rat).Add(w.Min[0], new(big.Rat).Mul(new(big.Rat).Sub(w.Max[0], w.Min[0]), big.NewRat(int64(i), 4)))
			y := new(big.Rat).Add(w.Min[1], new(big.Rat).Mul(new(big.Rat).Sub(w.Max[1], w.Min[1]), big.NewRat(int64(j), 4)))
			h := height(x, y)
			if h.Cmp(r.Depth[0]) < 0 || h.Cmp(r.Depth[1]) > 0 {
				t.Fatalf("independent depth %s outside [%s,%s]", h, r.Depth[0], r.Depth[1])
			}
		}
	}
}
func TestGraphWindowNestedDepth(t *testing.T) {
	slowtest.Skip(t)
	s := graphMaterialBooleanFixture(t, true, false)
	q, e := s.PrepareGraphMaterialQuery(context.Background(), AlgebraicGraphMaterialOptions{})
	if e != nil {
		t.Fatal(e)
	}
	w := graphWindow(big.NewRat(2, 3), big.NewRat(7, 13), big.NewRat(1, 4096))
	r, e := q.BoundWindowDepth(context.Background(), w, AlgebraicGraphWindowOptions{})
	if e != nil {
		t.Fatal(e)
	}
	checkWindowSamples(t, r, w, nestedQueryHeight)
	hole := graphWindow(big.NewRat(1, 2), big.NewRat(1, 2), big.NewRat(1, 1000))
	empty, e := q.BoundWindowDepth(context.Background(), hole, AlgebraicGraphWindowOptions{})
	if e != nil || !empty.Complete || !empty.NoFootprint || empty.Depth[0] != nil {
		t.Fatalf("hole: %+v %v", empty, e)
	}
	crossing := graphWindow(big.NewRat(13, 16), big.NewRat(1, 2), big.NewRat(1, 1000))
	failed, e := q.BoundWindowDepth(context.Background(), crossing, AlgebraicGraphWindowOptions{})
	if !errors.Is(e, geom.ErrConvergence) || failed.Complete || failed.Depth[0] != nil {
		t.Fatalf("crossing: %+v %v", failed, e)
	}
	t.Logf("depth [%s,%s], cells%d tubes%d ops%d", r.Depth[0], r.Depth[1], r.RecipeCells, r.TubeTests, r.ExactOperations)
}
func TestGraphWindowPlacedNonbinary(t *testing.T) {
	slowtest.Skip(t)
	s := graphMaterialBooleanFixture(t, false, true)
	axes := [2]int{0, 2}
	q, e := s.PrepareGraphMaterialQuery(context.Background(), AlgebraicGraphMaterialOptions{Projection: &axes})
	if e != nil {
		t.Fatal(e)
	}
	w := graphWindow(big.NewRat(27, 16), big.NewRat(9, 2), big.NewRat(1, 1000))
	r, e := q.BoundWindowDepth(context.Background(), w, AlgebraicGraphWindowOptions{})
	if e != nil {
		t.Fatal(e)
	}
	checkWindowSamples(t, r, w, func(x, y *big.Rat) *big.Rat {
		dx := new(big.Rat).Sub(x, big.NewRat(27, 16))
		dy := new(big.Rat).Sub(y, big.NewRat(9, 2))
		return new(big.Rat).Sub(big.NewRat(25, 4096), new(big.Rat).Add(new(big.Rat).Mul(dx, dx), new(big.Rat).Mul(dy, dy)))
	})
	if r.ProjectionAxes != axes || r.ResidualAxis != 1 {
		t.Fatal("world axes")
	}
	for chart, g := range q.graphs {
		for _, x := range []*big.Rat{w.Min[0], w.Max[0]} {
			for _, y := range []*big.Rat{w.Min[1], w.Max[1]} {
				inv, e := g.InvertRational(context.Background(), [2]*big.Rat{x, y}, geom.SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				d := r.EnclosureDomains[chart]
				if inv.UV[0].Cmp(ratInset(d.UMin)) < 0 || inv.UV[0].Cmp(ratInset(d.UMax)) > 0 || inv.UV[1].Cmp(ratInset(d.VMin)) < 0 || inv.UV[1].Cmp(ratInset(d.VMax)) > 0 {
					t.Fatal("rounded hull shrank")
				}
			}
		}
	}
	for _, o := range []AlgebraicGraphWindowOptions{{MaxCells: 1}, {Exact: geom.ExactCurvePlaneEventOptions{MaxOperations: 128}}} {
		bad, e := q.BoundWindowDepth(context.Background(), w, o)
		if !errors.Is(e, geom.ErrConvergence) || bad.Complete || bad.Window.Min[0] != nil {
			t.Fatalf("budget partial: %+v %v", bad, e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	bad, e := q.BoundWindowDepth(ctx, w, AlgebraicGraphWindowOptions{})
	if !errors.Is(e, context.Canceled) || bad.Complete {
		t.Fatal("cancellation", e)
	}
	r.Window.Min[0].SetInt64(99)
	if w.Min[0].Cmp(big.NewRat(99, 1)) == 0 {
		t.Fatal("aliased input")
	}
}

func TestGraphWindowGuards(t *testing.T) {
	var q *AlgebraicGraphMaterialQuery
	r, e := q.BoundWindowDepth(context.Background(), AlgebraicGraphProjectionWindow{}, AlgebraicGraphWindowOptions{})
	if !errors.Is(e, geom.ErrInvalid) || r.Complete || r.Depth[0] != nil {
		t.Fatal("nil proof", e)
	}
}
