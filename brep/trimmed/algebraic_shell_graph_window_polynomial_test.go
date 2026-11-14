package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func polynomialLobeCellVolume(w AlgebraicGraphProjectionWindow) *big.Rat {
	mean := big.NewRat(25, 4096)
	area := big.NewRat(1, 1)
	for i, center := range []*big.Rat{big.NewRat(27, 16), big.NewRat(9, 2)} {
		a := new(big.Rat).Sub(w.Min[i], center)
		b := new(big.Rat).Sub(w.Max[i], center)
		m := new(big.Rat).Add(new(big.Rat).Mul(a, a), new(big.Rat).Mul(a, b))
		m.Add(m, new(big.Rat).Mul(b, b))
		m.Quo(m, big.NewRat(3, 1))
		mean.Sub(mean, m)
		area.Mul(area, new(big.Rat).Sub(w.Max[i], w.Min[i]))
	}
	return new(big.Rat).Mul(mean, area)
}
func TestGraphWindowPolynomialIntegral(t *testing.T) {
	s := graphMaterialBooleanFixture(t, false, true)
	axes := [2]int{0, 2}
	q, e := s.PrepareGraphMaterialQuery(context.Background(), AlgebraicGraphMaterialOptions{Projection: &axes})
	if e != nil {
		t.Fatal(e)
	}
	w := graphWindow(big.NewRat(27, 16), big.NewRat(9, 2), big.NewRat(1, 1000))
	one := AlgebraicGraphPolynomialWindowOptions{Refinement: AlgebraicGraphWindowRefineOptions{MaxCells: 1}}
	r, e := q.IntegratePolynomialWindow(context.Background(), w, one)
	if e != nil {
		t.Fatal(e)
	}
	expected := polynomialLobeCellVolume(w)
	if !r.Bounds.Complete || len(r.ExactCells) != 1 || r.Bounds.Volume[0].Cmp(expected) != 0 || r.Bounds.Volume[1].Cmp(expected) != 0 {
		t.Fatal("nonbinary exact rectangle integral", r.Bounds.Volume, expected)
	}
	rim := graphWindow(big.NewRat(27, 16), big.NewRat(9, 2), big.NewRat(3, 32))
	opts := AlgebraicGraphPolynomialWindowOptions{Refinement: AlgebraicGraphWindowRefineOptions{MaxCells: 85, MaxDepth: 3}}
	baseline, e := q.RefineWindowDepth(context.Background(), rim, opts.Refinement)
	if e != nil {
		t.Fatal(e)
	}
	improved, e := q.IntegratePolynomialWindow(context.Background(), rim, opts)
	if e != nil {
		t.Fatal(e)
	}
	if improved.Bounds.UnresolvedArea.Cmp(baseline.UnresolvedArea) != 0 || improved.Bounds.Volume[0].Cmp(baseline.Volume[0]) <= 0 || improved.Bounds.Volume[1].Cmp(baseline.Volume[1]) >= 0 {
		t.Fatal("integral enclosure did not tighten", baseline.Volume, improved.Bounds.Volume)
	}
	for _, cell := range improved.ExactCells {
		want := polynomialLobeCellVolume(improved.Bounds.Leaves[cell.Leaf].Window)
		if cell.Volume.Cmp(want) != 0 {
			t.Fatal("independent exact leaf integral", cell.Volume, want)
		}
	}
	exact := math.Pi * math.Pow(5.0/64, 4) / 2
	lo, _ := improved.Bounds.Volume[0].Float64()
	hi, _ := improved.Bounds.Volume[1].Float64()
	if lo > exact || hi < exact {
		t.Fatal("analytic whole-disk integral")
	}
	for _, o := range []AlgebraicGraphPolynomialWindowOptions{{Refinement: opts.Refinement, MaxPolynomialCells: 1}, {Refinement: one.Refinement, CellOperations: 1}} {
		bad, e := q.IntegratePolynomialWindow(context.Background(), w, o)
		if o.MaxPolynomialCells == 1 {
			bad, e = q.IntegratePolynomialWindow(context.Background(), rim, o)
		}
		if !errors.Is(e, geom.ErrConvergence) || bad.Bounds.HasBounds || len(bad.ExactCells) != 0 {
			t.Fatal("atomic polynomial quota", e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	bad, e := q.IntegratePolynomialWindow(ctx, w, one)
	if !errors.Is(e, context.Canceled) || bad.Bounds.HasBounds {
		t.Fatal("cancel", e)
	}
	t.Logf("exact nonbinary cell V%s; %d retained cells; rim bounds %v -> %v", expected, len(improved.ExactCells), baseline.Volume, improved.Bounds.Volume)
}

func TestGraphPolynomialRejectsRationalDenominatorAndDegree(t *testing.T) {
	for _, degree := range []int{2, 7} {
		controls := make([][4]*big.Rat, 0, (degree+1)*2)
		for i := 0; i <= degree; i++ {
			for j := 0; j < 2; j++ {
				x, y, z, weight := big.NewRat(int64(i), int64(degree)), big.NewRat(int64(j), 1), new(big.Rat), big.NewRat(1, 1)
				if degree == 2 {
					weight = big.NewRat(int64(2+i), 2)
					x = []*big.Rat{big.NewRat(0, 1), big.NewRat(1, 2), big.NewRat(2, 1)}[i]
					y = new(big.Rat).Mul(y, weight)
					z.SetInt64(1)
				} else if i == degree {
					z.SetInt64(1)
				}
				controls = append(controls, [4]*big.Rat{x, y, z, weight})
			}
		}
		p, e := geom.NewExactHomogeneousBezierSurface(context.Background(), degree, 1, controls, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		g, e := geom.NewAffineProjectedGraph(context.Background(), p, [2]int{0, 1}, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		q := AlgebraicGraphMaterialQuery{patches: [2]*geom.SurfacePatch{p, p}, graphs: [2]*geom.AffineProjectedGraph{g, g}, axes: [2]int{0, 1}, residual: 2, regions: []graphMaterialRegion{{faces: [2]int{0, 1}, upper: 1}}}
		leaf := AlgebraicGraphWindowLeaf{Faces: [2]int{0, 1}, FullMaterial: true, Window: graphWindow(big.NewRat(3, 8), big.NewRat(3, 8), big.NewRat(1, 8))}
		value, e := q.integrateGraphPolynomialCell(context.Background(), leaf, 65536, 16384)
		if !errors.Is(e, geom.ErrConvergence) || value != nil {
			t.Fatal("unsupported exact support integrated", degree, value, e)
		}
	}
}
