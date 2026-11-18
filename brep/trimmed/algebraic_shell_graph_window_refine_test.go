package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestGraphWindowRefineRim(t *testing.T) {
	slowtest.Skip(t)
	s := graphMaterialBooleanFixture(t, false, true)
	axes := [2]int{0, 2}
	q, e := s.PrepareGraphMaterialQuery(context.Background(), AlgebraicGraphMaterialOptions{Projection: &axes})
	if e != nil {
		t.Fatal(e)
	}
	w := graphWindow(big.NewRat(27, 16), big.NewRat(9, 2), big.NewRat(3, 32))
	var previous AlgebraicGraphWindowAggregate
	for _, n := range []int{85, 341} {
		r, e := q.RefineWindowDepth(context.Background(), w, AlgebraicGraphWindowRefineOptions{MaxCells: n, MaxDepth: 4})
		if e != nil {
			t.Fatal(e)
		}
		if !r.HasBounds || r.Complete || r.UnresolvedArea.Sign() <= 0 || r.MaterialArea.Sign() <= 0 || r.EmptyArea.Sign() <= 0 {
			t.Fatalf("partition missing classified/unknown cells: %+v", r)
		}
		sum := new(big.Rat).Add(r.MaterialArea, r.EmptyArea)
		sum.Add(sum, r.UnresolvedArea)
		if sum.Cmp(big.NewRat(9, 256)) != 0 || sum.Cmp(r.ProjectedArea) != 0 {
			t.Fatal("area partition", sum)
		}
		// The whole disk lies inside this window. Independent paraboloid integral.
		volume := math.Pi * math.Pow(5.0/64, 4) / 2
		lo, _ := r.Volume[0].Float64()
		hi, _ := r.Volume[1].Float64()
		if lo > volume-1e-16 || hi < volume+1e-16 {
			t.Fatalf("analytic volume %.17g outside [%.17g,%.17g]", volume, lo, hi)
		}
		if previous.HasBounds && (r.UnresolvedArea.Cmp(previous.UnresolvedArea) >= 0 || r.Volume[0].Cmp(previous.Volume[0]) < 0 || r.Volume[1].Cmp(previous.Volume[1]) > 0) {
			t.Fatal("refinement worsened enclosure")
		}
		for _, leaf := range r.Leaves {
			if leaf.FullMaterial {
				// Exact integral of r²−dx²−dy² over this retained rectangle.
				area := new(big.Rat).Mul(new(big.Rat).Sub(leaf.Window.Max[0], leaf.Window.Min[0]), new(big.Rat).Sub(leaf.Window.Max[1], leaf.Window.Min[1]))
				mean := big.NewRat(25, 4096)
				for i, c := range []*big.Rat{big.NewRat(27, 16), big.NewRat(9, 2)} {
					a := new(big.Rat).Sub(leaf.Window.Min[i], c)
					b := new(big.Rat).Sub(leaf.Window.Max[i], c)
					m := new(big.Rat).Add(new(big.Rat).Mul(a, a), new(big.Rat).Mul(a, b))
					m.Add(m, new(big.Rat).Mul(b, b))
					m.Quo(m, big.NewRat(3, 1))
					mean.Sub(mean, m)
				}
				exact := new(big.Rat).Mul(area, mean)
				if exact.Cmp(new(big.Rat).Mul(area, leaf.Depth[0])) < 0 || exact.Cmp(new(big.Rat).Mul(area, leaf.Depth[1])) > 0 {
					t.Fatal("retained cell integral")
				}
			}
		}
		t.Logf("attempts%d leaves%d unresolved%s V[%g,%g]", r.CellAttempts, len(r.Leaves), r.UnresolvedArea, lo, hi)
		previous = r
	}
	limited, e := q.RefineWindowDepth(context.Background(), w, AlgebraicGraphWindowRefineOptions{MaxCells: 1})
	if e != nil || !limited.HasBounds || limited.Complete || limited.UnresolvedArea.Cmp(limited.ProjectedArea) != 0 {
		t.Fatal("bounded unresolved result", e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	bad, e := q.RefineWindowDepth(ctx, w, AlgebraicGraphWindowRefineOptions{})
	if !errors.Is(e, context.Canceled) || bad.HasBounds || len(bad.Leaves) != 0 {
		t.Fatal("atomic cancel", e)
	}
	outside := graphWindow(big.NewRat(100, 1), big.NewRat(9, 2), big.NewRat(1, 100))
	bad, e = q.RefineWindowDepth(context.Background(), outside, AlgebraicGraphWindowRefineOptions{})
	if !errors.Is(e, geom.ErrConvergence) || bad.HasBounds {
		t.Fatal("domain reject", e)
	}
}
