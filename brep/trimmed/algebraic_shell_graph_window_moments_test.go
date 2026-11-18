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

func TestGraphWindowExactMoments(t *testing.T) {
	slowtest.Skip(t)
	s := graphMaterialBooleanFixture(t, false, true)
	axes := [2]int{0, 2}
	q, e := s.PrepareGraphMaterialQuery(context.Background(), AlgebraicGraphMaterialOptions{Projection: &axes})
	if e != nil {
		t.Fatal(e)
	}
	w := graphWindow(big.NewRat(27, 16), big.NewRat(9, 2), big.NewRat(1, 1000))
	o := AlgebraicGraphWindowMomentOptions{Polynomial: AlgebraicGraphPolynomialWindowOptions{Refinement: AlgebraicGraphWindowRefineOptions{MaxCells: 1}}}
	r, e := q.IntegrateWindowMoments(context.Background(), w, o)
	if e != nil {
		t.Fatal(e)
	}
	// Independent symmetric rectangle moments of h=r²−x²−y²:
	// mean h=r²−2d²/3; mean h²=r⁴−4r²d²/3+28d⁴/45.
	radius2, d2 := big.NewRat(25, 4096), big.NewRat(1, 1000000)
	mean := new(big.Rat).Sub(radius2, new(big.Rat).Mul(big.NewRat(2, 3), d2))
	square := new(big.Rat).Sub(new(big.Rat).Mul(radius2, radius2), new(big.Rat).Mul(big.NewRat(4, 3), new(big.Rat).Mul(radius2, d2)))
	square.Add(square, new(big.Rat).Mul(big.NewRat(28, 45), new(big.Rat).Mul(d2, d2)))
	y := new(big.Rat).Add(big.NewRat(-3, 1), new(big.Rat).Quo(square, new(big.Rat).Mul(big.NewRat(2, 1), mean)))
	want := [3]*big.Rat{big.NewRat(27, 16), y, big.NewRat(9, 2)}
	volume := polynomialLobeCellVolume(w)
	if !r.HasCentroid || len(r.ExactCells) != 1 {
		t.Fatal("exact cell centroid missing")
	}
	for axis, c := range want {
		m := new(big.Rat).Mul(volume, c)
		for end := 0; end < 2; end++ {
			if r.Centroid[axis][end].Cmp(c) != 0 || r.FirstMoments[axis][end].Cmp(m) != 0 {
				t.Fatalf("axis%d: centroid%v vs%s, moment%v vs%s", axis, r.Centroid[axis], c, r.FirstMoments[axis], m)
			}
		}
	}
	// Both lobes, original signed world placement, and unresolved rim measure.
	both := AlgebraicGraphProjectionWindow{Min: [2]*big.Rat{big.NewRat(9, 8), big.NewRat(17, 4)}, Max: [2]*big.Rat{big.NewRat(15, 8), big.NewRat(19, 4)}}
	globalOptions := AlgebraicGraphWindowMomentOptions{Polynomial: AlgebraicGraphPolynomialWindowOptions{Refinement: AlgebraicGraphWindowRefineOptions{MaxCells: 341, MaxDepth: 4}}}
	g, e := q.IntegrateWindowMoments(context.Background(), both, globalOptions)
	if e != nil {
		t.Fatal(e)
	}
	if !g.HasCentroid || g.Volume.Bounds.UnresolvedArea.Sign() <= 0 {
		t.Fatal("global certified remainder")
	}
	centroid := [3]*big.Rat{big.NewRat(3, 2), new(big.Rat).Add(big.NewRat(-3, 1), big.NewRat(25, 12288)), big.NewRat(9, 2)}
	v := math.Pi * math.Pow(5.0/64, 4)
	for axis, c := range centroid {
		if c.Cmp(g.Centroid[axis][0]) < 0 || c.Cmp(g.Centroid[axis][1]) > 0 {
			t.Fatal("global centroid", axis, g.Centroid[axis], c)
		}
		f, _ := c.Float64()
		lo, _ := g.FirstMoments[axis][0].Float64()
		hi, _ := g.FirstMoments[axis][1].Float64()
		if v*f < lo || v*f > hi {
			t.Fatal("global independent first moment", axis)
		}
	}
	unknown, e := q.IntegrateWindowMoments(context.Background(), both, o)
	if e != nil || unknown.HasCentroid || unknown.Volume.Bounds.Volume[0].Sign() != 0 {
		t.Fatal("unproved denominator", e)
	}
	emptyWindow := graphWindow(big.NewRat(3, 2), big.NewRat(9, 2), big.NewRat(1, 1000))
	empty, e := q.IntegrateWindowMoments(context.Background(), emptyWindow, o)
	if e != nil || empty.HasCentroid || !empty.Volume.Bounds.Complete || empty.Volume.Bounds.Volume[1].Sign() != 0 {
		t.Fatal("empty footprint moments", e)
	}
	for _, interval := range empty.FirstMoments {
		if interval[0].Sign() != 0 || interval[1].Sign() != 0 {
			t.Fatal("nonzero empty moment")
		}
	}
	limited := o
	limited.MomentCellOperations = 1
	bad, e := q.IntegrateWindowMoments(context.Background(), w, limited)
	if !errors.Is(e, geom.ErrConvergence) || bad.HasCentroid || bad.Volume.Bounds.HasBounds || len(bad.ExactCells) != 0 {
		t.Fatal("atomic moment limit", e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	bad, e = q.IntegrateWindowMoments(ctx, w, o)
	if !errors.Is(e, context.Canceled) || bad.HasCentroid {
		t.Fatal("cancel", e)
	}
	t.Logf("exact local centroid (%s,%s,%s); global moments %v centroid %v", want[0], want[1], want[2], g.FirstMoments, g.Centroid)
}
