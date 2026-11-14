package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestRationalProjectedJacobianZero(t *testing.T) {
	ctx := context.Background()
	q := big.NewRat
	// (x,y,z) = (2u/(1+u), 4u²/(1+u)², v) is a ruled cylinder
	// over a planar parabola. One exact homogeneous tensor net is
	// (X,Y,W)=(0,0,1),(1,0,2),(4,4,4), with Z=vW.
	net := make([][4]*big.Rat, 0, 6)
	for _, p := range [][3]int64{{0, 0, 1}, {1, 0, 2}, {4, 4, 4}} {
		for v := int64(0); v < 2; v++ {
			net = append(net, [4]*big.Rat{q(p[0], 1), q(p[1], 1), q(v*p[2], 1), q(p[2], 1)})
		}
	}
	patch, e := NewExactHomogeneousBezierSurface(ctx, 2, 1, net, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, item := range []struct {
		axis [2]int
		zero bool
	}{{[2]int{0, 1}, true}, {[2]int{1, 0}, true}, {[2]int{0, 2}, false}, {[2]int{1, 2}, false}} {
		zero, work, e := patch.RationalProjectedJacobianZero(ctx, item.axis, SurfacePatchEvaluationOptions{})
		if e != nil || work < 1 || zero != item.zero {
			t.Fatal("projection rank of exact weighted extrusion", item, zero, work, e)
		}
	}
	// A minute, exact off-sweep perturbation creates a two-dimensional
	// projection even when its effect is below a binary64 ulp.
	tiny := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 120))
	net[5][1].Add(net[5][1], tiny)
	perturbed, e := NewExactHomogeneousBezierSurface(ctx, 2, 1, net, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if yes, _, e := perturbed.RationalProjectedJacobianZero(ctx, [2]int{0, 1}, SurfacePatchEvaluationOptions{}); e != nil || yes {
		t.Fatal("an exact non-vertical perturbation must not be classified as zero", yes, e)
	}
	if yes, _, e := patch.RationalProjectedJacobianZero(ctx, [2]int{0, 1}, SurfacePatchEvaluationOptions{MaxOperations: 1}); yes || !errors.Is(e, ErrConvergence) {
		t.Fatal("small budget must fail without claiming zero", yes, e)
	}
}
