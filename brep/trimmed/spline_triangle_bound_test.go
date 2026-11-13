package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func TestExactFilledTriangleDistanceIndependent(t *testing.T) {
	tri := newExactBoundTriangle([3]geom.Vec3{{}, {X: 1}, {Y: 1}})
	for _, tc := range []struct {
		p    geom.Vec3
		want *big.Rat
	}{
		{geom.Vec3{X: .25, Y: .25, Z: 2}, big.NewRat(4, 1)},
		{geom.Vec3{X: 1, Y: 1}, big.NewRat(1, 2)},
		{geom.Vec3{X: -1, Y: -1, Z: 1}, big.NewRat(3, 1)},
		{geom.Vec3{X: .5}, big.NewRat(0, 1)},
	} {
		if got := tri.distance(exactBoundPoint(tc.p)); got.Cmp(tc.want) != 0 {
			t.Fatal(tc.p, got, tc.want)
		}
	}
}

func TestSplineTriangleDeviationIndependent(t *testing.T) {
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	surface, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1, Z: 1}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	patch, e := geom.NewSurfacePatch(surface, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	uv := [3]geom.Vec2{{}, {X: 1}, {Y: 1}}
	model := [3]geom.Vec3{{}, {X: 1}, {Y: 1}}
	bound, e := BoundSplineTriangleDeviation(ctx, patch, uv, model)
	if e != nil {
		t.Fatal(e)
	}
	// On u+v<=1 the exact surface S=(u,v,uv) projects inside the target
	// triangle; the maximum true squared deviation is (1/4)^2.
	if bound.SquaredDistance.Cmp(big.NewRat(1, 16)) < 0 {
		t.Fatal("false interior certificate", bound)
	}
	for i := 0; i <= 20; i++ {
		for j := 0; j <= 20-i; j++ {
			u, v := big.NewRat(int64(i), 20), big.NewRat(int64(j), 20)
			z := new(big.Rat).Mul(u, v)
			squared := new(big.Rat).Mul(z, z)
			if squared.Cmp(bound.SquaredDistance) > 0 {
				t.Fatal("independent exact interior sample")
			}
		}
	}
	halfUV := [3]geom.Vec2{{}, {X: .5}, {Y: .5}}
	halfModel := [3]geom.Vec3{{}, {X: .5}, {Y: .5}}
	small, e := BoundSplineTriangleDeviation(ctx, patch, halfUV, halfModel)
	if e != nil || small.SquaredDistance.Cmp(bound.SquaredDistance) >= 0 {
		t.Fatal("subdivision enclosure", e)
	}
	// A displaced target triangle must never inherit a zero endpoint assumption.
	for i := range model {
		model[i].Z = 2
	}
	shifted, e := BoundSplineTriangleDeviation(ctx, patch, uv, model)
	if e != nil || shifted.SquaredDistance.Cmp(big.NewRat(4, 1)) < 0 {
		t.Fatal("shifted target", e)
	}
	bad, e := BoundSplineTriangleDeviation(ctx, patch, [3]geom.Vec2{}, model)
	if !errors.Is(e, geom.ErrDegenerate) || bad.SquaredDistance != nil {
		t.Fatal("UV degeneracy", e)
	}
	bad, e = BoundSplineTriangleDeviation(ctx, patch, uv, [3]geom.Vec3{})
	if !errors.Is(e, geom.ErrDegenerate) || bad.SquaredDistance != nil {
		t.Fatal("model degeneracy", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	bad, e = BoundSplineTriangleDeviation(canceled, patch, uv, model)
	if !errors.Is(e, context.Canceled) || bad.SquaredDistance != nil {
		t.Fatal("cancellation", e)
	}
}
