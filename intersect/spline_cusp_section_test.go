package intersect

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func cuspSectionFixture(t *testing.T) (Patch, Patch) {
	t.Helper()
	up := quarticSectionRootPower(big.NewRat(1, 4), 3)
	vp := quarticSectionRootPower(big.NewRat(1, 2), 2)
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
	var out [2]Patch
	for i := range out {
		patch, e := geom.NewExactHomogeneousBezierSurface(context.Background(), 3, 2, net[i], geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		surf, e := geom.NewExactBezierSurface(patch)
		out[i] = Patch{Surface: surf, Max: geom.Vec2{X: 1, Y: 1}}
		if e != nil {
			t.Fatal(e)
		}
	}
	return out[0], out[1]
}
func TestSplineCuspSectionSamples(t *testing.T) {
	ctx := context.Background()
	a, b := cuspSectionFixture(t)
	section, e := CertifySplinePairCuspSection(ctx, a, b, big.NewRat(1, 4), big.NewRat(1, 2), big.NewRat(1, 3), big.NewRat(1, 2), geom.SurfacePatchOptions{}, geom.GraphPointContactOptions{})
	if e != nil || !section.Complete() {
		t.Fatal(section, e)
	}
	for n := -4; n <= 4; n++ {
		q := big.NewRat(int64(n), 8)
		sample, e := section.SampleRational(ctx, q, geom.SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(n, e)
		}
		want := -1
		if n < 0 {
			want = 0
		} else if n > 0 {
			want = 1
		}
		if sample.Germ != want || sample.Terminal != (n == -4 || n == 4) {
			t.Fatal(sample)
		}
		u := new(big.Rat).Add(big.NewRat(1, 4), new(big.Rat).Mul(big.NewRat(3, 1), new(big.Rat).Mul(q, q)))
		if sample.Parameters[0][0].Cmp(u) != 0 || sample.Parameters[1][0].Cmp(u) != 0 {
			t.Fatal("original units")
		}
		for _, cap := range []int{sample.ExactOperations, sample.ExactOperations - 1} {
			got, e := section.SampleRational(ctx, q, geom.SurfacePatchEvaluationOptions{MaxOperations: cap})
			if cap == sample.ExactOperations {
				if e != nil {
					t.Fatal(e)
				}
			} else if e == nil || got.Point[0] != nil {
				t.Fatal("atomic budget", e)
			}
		}
	}
	lo, hi, _ := section.ParameterRange()
	lo.SetInt64(99)
	hi.SetInt64(99)
	lo, hi, _ = section.ParameterRange()
	if lo.Cmp(big.NewRat(-1, 2)) != 0 || hi.Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatal("range alias")
	}
	owner, e := section.Owner(1)
	if e != nil || owner != b {
		t.Fatal(owner, e)
	}
	for _, q := range []*big.Rat{big.NewRat(501, 1000), big.NewRat(-501, 1000)} {
		got, e := section.SampleRational(ctx, q, geom.SurfacePatchEvaluationOptions{})
		if e == nil || got.Point[0] != nil {
			t.Fatal("outside range")
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = section.SampleRational(cancelled, new(big.Rat), geom.SurfacePatchEvaluationOptions{}); e != context.Canceled {
		t.Fatal(e)
	}
	var forged SplineCuspSection
	if forged.Complete() {
		t.Fatal("zero result")
	}
}
