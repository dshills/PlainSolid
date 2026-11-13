package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestAffineProjectedGraphPartialOverlap(t *testing.T) {
	ctx := context.Background()
	makePatch := func(target bool) *SurfacePatch {
		poles := make([]Vec3, 4)
		for i := 0; i < 2; i++ {
			for j := 0; j < 2; j++ {
				x, z := 2*float64(i), float64(j)
				if target {
					x, z = 1+2*float64(j), float64(i)
				}
				poles[2*i+j] = Vec3{x, x * z, z}
			}
		}
		s, e := NewBSplineSurface(1, 1, 2, 2, poles, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		p, e := NewSurfacePatch(s, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return p
	}
	a, b := makePatch(false), makePatch(true)
	for _, axes := range [][2]int{{0, 2}, {2, 0}} {
		r, e := CertifyAffineProjectedGraphOverlap(ctx, a, b, axes, SurfacePatchOptions{})
		if e != nil || !r.Coincident || r.Orientation != -1 || r.ResidualAxis != 1 {
			t.Fatal(r, e)
		}
		sign := 1
		if axes[0] == 2 {
			sign = -1
		}
		if r.ProjectionOrientation != [2]int{sign, -sign} {
			t.Fatal(r.ProjectionOrientation)
		}
		expected := [4][3]int64{{1, 0, 0}, {2, 0, 0}, {2, 2, 1}, {1, 1, 1}}
		for i, point := range r.ModelCorners {
			for j, q := range point {
				if q.Cmp(big.NewRat(expected[i][j], 1)) != 0 {
					t.Fatal(i, j, q)
				}
			}
		}
		if _, e := CertifyAffineProjectedGraphOverlap(ctx, a, b, axes, SurfacePatchOptions{MaxOperations: r.ExactOperations - 1}); e == nil {
			t.Fatal("cumulative limit")
		}
	}

	nonbinary, e := permuteGraphPatch(a, [3]int{0, 1, 2}, &polynomialBudget{maxOperations: 100000, maxBits: 16384})
	if e != nil {
		t.Fatal(e)
	}
	for i := range nonbinary.net {
		for _, axis := range []int{0, 1} {
			nonbinary.net[i][axis] = new(big.Rat).Mul(nonbinary.net[i][axis], big.NewRat(3, 2))
		}
	}
	exact, e := CertifyAffineProjectedGraphOverlap(ctx, nonbinary, b, [2]int{0, 2}, SurfacePatchOptions{})
	if e != nil || !exact.Coincident || exact.A[0][0].Cmp(big.NewRat(1, 3)) != 0 || exact.ModelCorners[2][1].Cmp(big.NewRat(3, 1)) != 0 {
		t.Fatal(exact, e)
	}
	if _, e := CertifyAffineProjectedGraphOverlap(ctx, a, b, [2]int{0, 0}, SurfacePatchOptions{}); e == nil {
		t.Fatal("invalid axes")
	}
	if _, e := CertifyAffineProjectedGraphOverlap(ctx, a, b, [2]int{0, 1}, SurfacePatchOptions{}); e == nil {
		t.Fatal("nonaffine projection")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := CertifyAffineProjectedGraphOverlap(cancelled, a, b, [2]int{0, 2}, SurfacePatchOptions{}); e == nil {
		t.Fatal("cancel")
	}
}
