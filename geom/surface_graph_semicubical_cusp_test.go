package geom

import (
	"context"
	"math/big"
	"testing"
)

func semicubicalFixture(t *testing.T) (*SurfacePatch, *SurfacePatch) {
	t.Helper()
	up := rootPower(t, big.NewRat(1, 4), 3)
	vp := rootPower(t, big.NewRat(1, 2), 2)
	bern := func(power []*big.Rat, i, n int) *big.Rat {
		q := new(big.Rat)
		for j := 0; j <= i; j++ {
			q.Add(q, new(big.Rat).Mul(power[j], new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(j)), new(big.Int).Binomial(int64(n), int64(j)))))
		}
		return q
	}
	var net [2][][4]*big.Rat
	for i := 0; i < 4; i++ {
		for j := 0; j < 3; j++ {
			u, v := big.NewRat(int64(i), 3), big.NewRat(int64(j), 2)
			base := new(big.Rat).Quo(new(big.Rat).Mul(u, v), big.NewRat(64, 1))
			z := new(big.Rat).Add(base, new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(36, 1), bern(vp, j, 2)), new(big.Rat).Mul(big.NewRat(12, 1), bern(up, i, 3))))
			net[0] = append(net[0], [4]*big.Rat{u, v, z, big.NewRat(1, 1)})
			net[1] = append(net[1], [4]*big.Rat{u, v, base, big.NewRat(1, 1)})
		}
	}
	var out [2]*SurfacePatch
	for i := range out {
		var e error
		out[i], e = NewExactHomogeneousBezierSurface(context.Background(), 3, 2, net[i], SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
	}
	return out[0], out[1]
}
func TestCompleteSemicubicalCusp(t *testing.T) {
	ctx := context.Background()
	a, b := semicubicalFixture(t)
	r, s, k, tau := big.NewRat(1, 4), big.NewRat(1, 2), big.NewRat(1, 3), big.NewRat(1, 2)
	proof, e := CertifyGraphSemicubicalCusp(ctx, a, b, r, s, k, tau, GraphPointContactOptions{})
	if e != nil || !proof.Proven {
		t.Fatal(proof, e)
	}
	if proof.Terminals[0][1].Cmp(big.NewRat(1, 8)) != 0 || proof.Terminals[1][1].Cmp(big.NewRat(7, 8)) != 0 {
		t.Fatal(proof.Terminals)
	}
	// Independent rational parameter samples satisfy both original supports;
	// dx/dt=2t/k is nonzero off cusp; both germs have the same limiting ray.
	for n := -4; n <= 4; n++ {
		q := big.NewRat(int64(n), 8)
		x := new(big.Rat).Quo(new(big.Rat).Mul(q, q), k)
		y := new(big.Rat).Mul(q, x)
		u, v := new(big.Rat).Add(r, x), new(big.Rat).Add(s, y)
		aj, e := a.EvaluateRational(ctx, u, v, SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		bj, e := b.EvaluateRational(ctx, u, v, SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for j := range aj.Point {
			if aj.Point[j].Cmp(bj.Point[j]) != 0 {
				t.Fatal("support residual", n)
			}
		}
		if n != 0 && x.Sign() != 1 {
			t.Fatal("wrong cusp germ")
		}
	}
	rev, e := CertifyGraphSemicubicalCusp(ctx, b, a, r, s, k, tau, GraphPointContactOptions{})
	if e != nil || rev.Side != -proof.Side {
		t.Fatal(rev, e)
	}
	for _, cap := range []int{proof.ExactOperations, proof.ExactOperations - 1} {
		got, e := CertifyGraphSemicubicalCusp(ctx, a, b, r, s, k, tau, GraphPointContactOptions{MaxOperations: cap})
		if cap == proof.ExactOperations {
			if e != nil {
				t.Fatal(e)
			}
		} else if e == nil || got.Proven || got.Point[0] != nil {
			t.Fatal("atomic quota", e)
		}
	}
	for _, args := range [][4]*big.Rat{{r, s, k, big.NewRat(1, 4)}, {r, s, big.NewRat(-1, 3), tau}, {r, big.NewRat(1, 4), k, tau}, {big.NewRat(1, 3), s, k, tau}} {
		got, e := CertifyGraphSemicubicalCusp(ctx, a, b, args[0], args[1], args[2], args[3], GraphPointContactOptions{})
		if e == nil || got.Proven {
			t.Fatal("invalid proposal", args)
		}
	}
}

func TestSemicubicalCuspRejectsPerturbedIdentity(t *testing.T) {
	ctx := context.Background()
	a, b := semicubicalFixture(t)
	for i := range a.net {
		a.net[i][2] = new(big.Rat).Add(a.net[i][2], big.NewRat(1, 1024))
	}
	got, e := CertifyGraphSemicubicalCusp(ctx, a, b, big.NewRat(1, 4), big.NewRat(1, 2), big.NewRat(1, 3), big.NewRat(1, 2), GraphPointContactOptions{})
	if e == nil || got.Proven || got.Tau != nil {
		t.Fatal("perturbed numerator accepted", got, e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	got, e = CertifyGraphSemicubicalCusp(cancelled, a, b, big.NewRat(1, 4), big.NewRat(1, 2), big.NewRat(1, 3), big.NewRat(1, 2), GraphPointContactOptions{})
	if e != context.Canceled || got.Proven {
		t.Fatal(e)
	}
}
