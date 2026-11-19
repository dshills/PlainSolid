//go:build research

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

func TestGraphWindowExactInertia(t *testing.T) {
	slowtest.Skip(t)
	s := graphMaterialBooleanFixture(t, false, true)
	axes := [2]int{0, 2}
	q, e := s.PrepareGraphMaterialQuery(context.Background(), AlgebraicGraphMaterialOptions{Projection: &axes})
	if e != nil {
		t.Fatal(e)
	}
	w := graphWindow(big.NewRat(27, 16), big.NewRat(9, 2), big.NewRat(1, 1000))
	o := AlgebraicGraphWindowInertiaOptions{Moments: AlgebraicGraphWindowMomentOptions{Polynomial: AlgebraicGraphPolynomialWindowOptions{Refinement: AlgebraicGraphWindowRefineOptions{MaxCells: 1}}}}
	got, e := q.IntegrateWindowInertia(context.Background(), w, o)
	if e != nil {
		t.Fatal(e)
	}
	if !got.HasCentroidal || len(got.ExactCells) != 1 {
		t.Fatal("exact inertia unavailable")
	}
	add := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Add(a, b) }
	sub := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Sub(a, b) }
	mul := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Mul(a, b) }
	r, d := big.NewRat(25, 4096), big.NewRat(1, 1000000)
	area := mul(big.NewRat(4, 1), d)
	r2, d2 := mul(r, r), mul(d, d)
	h := sub(r, mul(big.NewRat(2, 3), d))
	h2 := add(sub(r2, mul(big.NewRat(4, 3), mul(r, d))), mul(big.NewRat(28, 45), d2))
	h3 := sub(add(sub(mul(r2, r), mul(big.NewRat(2, 1), mul(r2, d))), mul(big.NewRat(28, 15), mul(r, d2))), mul(big.NewRat(24, 35), mul(d2, d)))
	volume := mul(area, h)
	my := mul(area, add(mul(big.NewRat(-3, 1), h), mul(big.NewRat(1, 2), h2)))
	a2h := sub(mul(big.NewRat(1, 3), mul(r, d)), mul(big.NewRat(14, 45), d2))
	cx, cz := big.NewRat(27, 16), big.NewRat(9, 2)
	raw := [6]*big.Rat{add(mul(mul(cx, cx), volume), mul(area, a2h)), mul(cx, my), mul(mul(cx, cz), volume), mul(area, add(sub(mul(big.NewRat(9, 1), h), mul(big.NewRat(3, 1), h2)), mul(big.NewRat(1, 3), h3))), mul(cz, my), add(mul(mul(cz, cz), volume), mul(area, a2h))}
	vx := mul(area, a2h)
	vy := sub(raw[3], new(big.Rat).Quo(mul(my, my), volume))
	central := [6]*big.Rat{add(vx, vy), new(big.Rat), new(big.Rat), mul(big.NewRat(2, 1), vx), new(big.Rat), add(vx, vy)}
	origin := [6]*big.Rat{add(raw[3], raw[5]), new(big.Rat).Neg(raw[1]), new(big.Rat).Neg(raw[2]), add(raw[0], raw[5]), new(big.Rat).Neg(raw[4]), add(raw[0], raw[3])}
	for i := 0; i < 6; i++ {
		for end := 0; end < 2; end++ {
			if got.RawSecondMoments[i][end].Cmp(raw[i]) != 0 || got.OriginInertia[i][end].Cmp(origin[i]) != 0 || got.CentroidalInertia[i][end].Cmp(central[i]) != 0 {
				t.Fatalf("tensor%d mismatch: raw%v expected%s; central%v expected%s", i, got.RawSecondMoments[i], raw[i], got.CentroidalInertia[i], central[i])
			}
		}
	}
	both := AlgebraicGraphProjectionWindow{Min: [2]*big.Rat{big.NewRat(9, 8), big.NewRat(17, 4)}, Max: [2]*big.Rat{big.NewRat(15, 8), big.NewRat(19, 4)}}
	globalOptions := o
	globalOptions.Moments.Polynomial.Refinement = AlgebraicGraphWindowRefineOptions{MaxCells: 341, MaxDepth: 4}
	g, e := q.IntegrateWindowInertia(context.Background(), both, globalOptions)
	if e != nil {
		t.Fatal(e)
	}
	if !g.HasCentroidal || g.Moments.Volume.Bounds.UnresolvedArea.Sign() <= 0 {
		t.Fatal("global remainder missing")
	}
	rr := 25.0 / 4096
	v := math.Pi * rr * rr
	meanY := -3 + rr/3
	expectedRaw := [6]float64{v * ((math.Pow(27.0/16, 2)+math.Pow(21.0/16, 2))/2 + rr/6), v * 1.5 * meanY, v * 1.5 * 4.5, v * (9 - 2*rr + rr*rr/6), v * 4.5 * meanY, v * (4.5*4.5 + rr/6)}
	expectedCentral := [6]float64{v * (rr*rr/18 + rr/6), 0, 0, v * (9.0/256 + rr/3), 0, v * (9.0/256 + rr/6 + rr*rr/18)}
	for i := 0; i < 6; i++ {
		a, b := g.RawSecondMoments[i][0], g.RawSecondMoments[i][1]
		lo, _ := a.Float64()
		hi, _ := b.Float64()
		if expectedRaw[i] < lo || expectedRaw[i] > hi {
			t.Fatal("global raw moment", i, lo, hi, expectedRaw[i])
		}
		lo, _ = g.CentroidalInertia[i][0].Float64()
		hi, _ = g.CentroidalInertia[i][1].Float64()
		if expectedCentral[i] < lo || expectedCentral[i] > hi {
			t.Fatal("global centroidal inertia", i)
		}
	}
	unknown, e := q.IntegrateWindowInertia(context.Background(), both, o)
	if e != nil || unknown.HasCentroidal || unknown.Moments.Volume.Bounds.Volume[0].Sign() != 0 {
		t.Fatal("positive denominator proof missing", e)
	}
	limited := o
	limited.SecondCellOperations = 1
	bad, e := q.IntegrateWindowInertia(context.Background(), w, limited)
	if !errors.Is(e, geom.ErrConvergence) || bad.HasCentroidal || bad.Moments.Volume.Bounds.HasBounds || len(bad.ExactCells) != 0 {
		t.Fatal("second quota atomicity", e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	bad, e = q.IntegrateWindowInertia(ctx, w, o)
	if !errors.Is(e, context.Canceled) || bad.HasCentroidal {
		t.Fatal("cancel", e)
	}
	t.Logf("exact nonbinary raw XX%s, YY%s; centroidal YY%s; global retained%d", raw[0], raw[3], central[3], len(g.ExactCells))
}
