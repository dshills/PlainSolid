package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestSurfaceOrientedFluxEnclosesAnalyticIntegral(t *testing.T) {
	// S(u,v)=(u,v,1+uv), so S dot (Su x Sv)/3=(1-uv)/3
	// and the exact oriented flux over the unit square is 1/4.
	k := []float64{0, 0, 1, 1}
	s, e := NewBSplineSurface(1, 1, 2, 2,
		[]Vec3{{Z: 1}, {Y: 1, Z: 1}, {X: 1, Z: 1}, {X: 1, Y: 1, Z: 2}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	want := big.NewRat(1, 4)
	var previous *big.Rat
	for _, depth := range []int{0, 2, 4, 6} {
		a, b, e := BoundSurfaceOrientedVolumeFlux(ctx, p, Vec3{}, depth, SurfacePatchEvaluationOptions{})
		if e != nil || a.Cmp(want) > 0 || b.Cmp(want) < 0 {
			t.Fatal(depth, a, b, e)
		}
		w := new(big.Rat).Sub(b, a)
		if previous != nil && w.Cmp(previous) >= 0 {
			t.Fatal("flux bounds failed to refine", previous, w)
		}
		previous = w
	}
}

func TestSurfaceSupportingQuadricProvesCompleteSidePattern(t *testing.T) {
	// S(u,v)=(u,v,u²+v). The shared quadric z-x²=0 has
	// the entire v-min side and strictly positive residual elsewhere.
	s, e := NewBSplineSurface(2, 1, 3, 2,
		[]Vec3{{}, {Y: 1, Z: 1}, {X: .5}, {X: .5, Y: 1, Z: 1}, {X: 1, Z: 1}, {X: 1, Y: 1, Z: 2}},
		[]float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var q ExactQuadric
	q[0], q[8] = big.NewRat(-1, 1), big.NewRat(1, 1)
	got, e := CertifySurfaceSupportingQuadric(context.Background(), p, q, SurfaceSupportingPlaneOptions{})
	if e != nil || !got.Complete || got.Coincident || got.HalfspaceSign != 1 || got.SideMask != 1<<SurfaceEdgeVMin || got.CornerMask != 0 {
		t.Fatal(got, e)
	}
}
