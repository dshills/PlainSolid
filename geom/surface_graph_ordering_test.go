package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestSplineGraphStrictOrderingLooseHull(t *testing.T) {
	ctx := context.Background()
	a, c := graphContactFixture(t, true, false)
	a, e := a.Transformed(Translation(Vec3{Z: 1. / 64}))
	if e != nil {
		t.Fatal(e)
	}
	p, e := NewSurfaceHeightDifferencePatch(a, c, SurfaceHeightPatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	negative := false
	for _, h := range p.net {
		negative = negative || h[2].Sign() < 0
	}
	if !negative {
		t.Fatal("fixture does not have loose sign hull")
	}
	uv := [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}
	r, e := CertifySplineGraphOrdering(ctx, a, c, uv, GraphPointContactOptions{})
	if e != nil || !r.Proven || r.Side != 1 || r.MarginLower.Cmp(big.NewRat(1, 64)) != 0 {
		t.Fatal(r, e)
	}
	r, e = CertifySplineGraphOrdering(ctx, c, a, uv, GraphPointContactOptions{})
	if e != nil || !r.Proven || r.Side != -1 || r.MarginLower.Cmp(big.NewRat(1, 64)) != 0 {
		t.Fatal(r, e)
	}
	discovery, e := DiscoverSplineGraphOrdering(ctx, a, c, GraphPointContactDiscoveryOptions{})
	if e != nil || !discovery.Ordering.Proven {
		t.Fatal(discovery, e)
	}
}
func TestSplineGraphOrderingPositiveWeightMargin(t *testing.T) {
	poles := make([]Vec3, 9)
	s, e := NewBSplineSurface(2, 2, 3, 3, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	a, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	c, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			u, v := big.NewRat(int64(i), 2), big.NewRat(int64(j), 2)
			u2, v2 := big.NewRat(int64(i*(i-1)), 2), big.NewRat(int64(j*(j-1)), 2)
			w := new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Mul(big.NewRat(1, 2), u))
			x := new(big.Rat).Add(u, new(big.Rat).Mul(big.NewRat(1, 2), u2))
			y := new(big.Rat).Mul(v, w)
			f := new(big.Rat).Add(new(big.Rat).Sub(u2, u), new(big.Rat).Sub(v2, v))
			f.Add(f, big.NewRat(9, 16))
			k := i*3 + j
			a.net[k] = exactHomogeneous{x, y, f, w}
			c.net[k] = exactHomogeneous{new(big.Rat).Set(x), new(big.Rat).Set(y), new(big.Rat), new(big.Rat).Set(w)}
		}
	}
	for _, p := range []*SurfacePatch{a, c} {
		if e := p.computeBounds(&polynomialBudget{maxOperations: 100000, maxBits: 16384}); e != nil {
			t.Fatal(e)
		}
	}
	r, e := CertifySplineGraphOrdering(context.Background(), a, c, [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, GraphPointContactOptions{})
	if e != nil || !r.Proven || r.MarginLower.Cmp(big.NewRat(1, 24)) != 0 || r.WeightUpper.Cmp(big.NewRat(3, 2)) != 0 {
		t.Fatal(r, e)
	}
}
func TestSplineGraphOrderingDiscoveryAndRejectContact(t *testing.T) {
	ctx := context.Background()
	a, c := rationalGraphMinimum(t, big.NewRat(1, 3), big.NewRat(2, 5), false)
	a, e := a.Transformed(Translation(Vec3{Z: .125}))
	if e != nil {
		t.Fatal(e)
	}
	r, e := DiscoverSplineGraphOrdering(ctx, a, c, GraphPointContactDiscoveryOptions{})
	if e != nil || !r.Ordering.Proven || r.CandidatesTested < 2 || r.Ordering.MarginLower.Cmp(big.NewRat(1, 8)) != 0 {
		t.Fatal(r, e)
	}
	a, c = graphContactFixture(t, true, false)
	proof, e := CertifySplineGraphOrdering(ctx, a, c, [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, GraphPointContactOptions{})
	if !errors.Is(e, ErrConvergence) || proof.Proven {
		t.Fatal(proof, e)
	}
	_, e = DiscoverSplineGraphOrdering(ctx, a, c, GraphPointContactDiscoveryOptions{GraphPointContactOptions: GraphPointContactOptions{MaxOperations: 1}})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
}
