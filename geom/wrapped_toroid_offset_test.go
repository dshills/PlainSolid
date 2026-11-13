package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestOffsetWrappedCircularToroidExactNormal(t *testing.T) {
	ctx := context.Background()
	base := locationToroid(t, true)
	for _, d := range []float64{.125, -.25} {
		offset, proof, e := OffsetWrappedCircularToroid(ctx, base, d, SurfacePatchOptions{})
		want := new(big.Rat).Add(big.NewRat(1, 2), new(big.Rat).SetFloat64(d))
		if e != nil || !proof.Complete || proof.RadialMinorRadius.Cmp(want) != 0 ||
			offset.SeamCertificate().FirstPartialsMatched != ([2]bool{true, true}) {
			t.Fatal(proof, e)
		}
		for _, uv := range [][2]int64{{0, 0}, {1, 0}, {3, 5}, {5, 7}, {13, 14}} {
			u, v := big.NewRat(uv[0], 4), big.NewRat(uv[1], 4)
			a, e := base.EvaluateRational(ctx, u, v, SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			b, e := offset.EvaluateRational(ctx, u, v, SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			var displacement [3]*big.Rat
			squared := new(big.Rat)
			for i := range displacement {
				displacement[i] = new(big.Rat).Sub(b.Point[i], a.Point[i])
				squared.Add(squared, new(big.Rat).Mul(displacement[i], displacement[i]))
			}
			dot := func(x [3]*big.Rat) *big.Rat {
				p := new(big.Rat)
				for i, v := range x {
					p.Add(p, new(big.Rat).Mul(v, displacement[i]))
				}
				return p
			}
			dd := new(big.Rat).SetFloat64(d)
			if squared.Cmp(dd.Mul(dd, dd)) != 0 || dot(a.Du).Sign() != 0 || dot(a.Dv).Sign() != 0 {
				t.Fatal("not a Euclidean normal offset", u, v, squared)
			}
		}
	}
	for _, d := range []float64{-.5, 1.5, 2} {
		w, _, e := OffsetWrappedCircularToroid(ctx, base, d, SurfacePatchOptions{})
		if w != nil || !errors.Is(e, ErrDegenerate) {
			t.Fatal("topology-changing offset accepted", d, e)
		}
	}
	if w, _, e := OffsetWrappedCircularToroid(ctx, locationToroid(t, false), .125, SurfacePatchOptions{}); w != nil || !errors.Is(e, ErrConvergence) {
		t.Fatal("noncircular curve accepted", e)
	}
}
