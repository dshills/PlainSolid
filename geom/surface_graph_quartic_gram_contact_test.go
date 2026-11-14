package geom

import (
	"context"
	"math"
	"math/big"
	"testing"
)

// Independent source polynomial in X=3u-1,Y=5v-2. Every resulting Bezier
// pole is explicitly checked for lossless float representation before using
// the represented BSpline constructor; the contact (1/3,2/5) stays rational.
func representedQuarticGramFixture(t *testing.T, coeff [5]int64) (*SurfacePatch, *SurfacePatch) {
	t.Helper()
	power := map[[2]int]*big.Rat{}
	for j, c := range coeff {
		up, vp := rootPower(t, big.NewRat(1, 3), 4-j), rootPower(t, big.NewRat(2, 5), j)
		scale := new(big.Int).Mul(new(big.Int).Exp(big.NewInt(3), big.NewInt(int64(4-j)), nil), new(big.Int).Exp(big.NewInt(5), big.NewInt(int64(j)), nil))
		for u, x := range up {
			for v, y := range vp {
				q := new(big.Rat).Mul(x, y)
				q.Mul(q, new(big.Rat).SetInt(scale))
				q.Mul(q, big.NewRat(c, 1))
				key := [2]int{u, v}
				if power[key] == nil {
					power[key] = new(big.Rat)
				}
				power[key].Add(power[key], q)
			}
		}
	}
	var poles [2][]Vec3
	for i := range 5 {
		for j := range 5 {
			z := new(big.Rat)
			for ij, q := range power {
				if ij[0] > i || ij[1] > j {
					continue
				}
				u := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(ij[0])), new(big.Int).Binomial(4, int64(ij[0])))
				v := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(j), int64(ij[1])), new(big.Int).Binomial(4, int64(ij[1])))
				z.Add(z, new(big.Rat).Mul(q, new(big.Rat).Mul(u, v)))
			}
			base := big.NewRat(int64(i*j), 1024)
			z.Add(z, base)
			f, err := RepresentedExactScalar(z)
			if err != nil {
				t.Fatal("source is not dyadic", i, j, z)
			}
			poles[0] = append(poles[0], Vec3{float64(i) / 4, float64(j) / 4, f})
			poles[1] = append(poles[1], Vec3{float64(i) / 4, float64(j) / 4, float64(i*j) / 1024})
		}
	}
	k := []float64{0, 0, 0, 0, 0, 1, 1, 1, 1, 1}
	var out [2]*SurfacePatch
	for i := range out {
		s, e := NewBSplineSurface(4, 4, 5, 5, poles[i], k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		out[i], e = NewSurfacePatch(s, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
	}
	return out[0], out[1]
}

func TestQuarticGramIsolatedOddMixedContact(t *testing.T) {
	ctx := context.Background()
	a, b := representedQuarticGramFixture(t, [5]int64{2, 4, 6, 4, 2})
	uv := [2]*big.Rat{big.NewRat(1, 3), big.NewRat(2, 5)}
	for i, pair := range [][2]*SurfacePatch{{a, b}, {b, a}} {
		r, e := CertifyQuarticGramGraphPointContact(ctx, pair[0], pair[1], uv, new(big.Rat), GraphPointContactOptions{})
		if e != nil || !r.Proven || r.Order != 4 || r.Side != 1-2*i {
			t.Fatal(r, e)
		}
		want := []int64{162, 540, 1350, 1500, 1250}
		for j, c := range want {
			if r.Coefficients[j].Cmp(big.NewRat(c*int64(1-2*i), 1)) != 0 {
				t.Fatal(r.Coefficients)
			}
		}
		if r.Parameters[0].Cmp(uv[0]) != 0 || r.Parameters[1].Cmp(uv[1]) != 0 || r.Point[2].Cmp(big.NewRat(1, 480)) != 0 || r.Tangent[0][2].Cmp(big.NewRat(1, 160)) != 0 || r.Tangent[1][2].Cmp(big.NewRat(1, 192)) != 0 {
			t.Fatal("original witness/jet", r)
		}
		for j, want := range []int64{162, 145800, 91125000} {
			if r.PrincipalMinors[j].Cmp(big.NewRat(want, 1)) != 0 {
				t.Fatal("independent Sylvester minors", r.PrincipalMinors)
			}
		}
		for _, m := range r.PrincipalMinors {
			if m.Sign() <= 0 {
				t.Fatal("not definite")
			}
		}
		if _, e = CertifyQuarticGramGraphPointContact(ctx, pair[0], pair[1], uv, new(big.Rat), GraphPointContactOptions{MaxOperations: r.ExactOperations}); e != nil {
			t.Fatal(e)
		}
		bad, e := CertifyQuarticGramGraphPointContact(ctx, pair[0], pair[1], uv, new(big.Rat), GraphPointContactOptions{MaxOperations: r.ExactOperations - 1})
		if e == nil || bad.Proven || bad.Parameters[0] != nil || bad.Gram[0][0] != nil {
			t.Fatal("partial publication", e)
		}
	}
	other, e := CertifyQuarticGramGraphPointContact(ctx, a, b, uv, big.NewRat(1, 1), GraphPointContactOptions{})
	if e != nil || !other.Proven || other.Gram[0][2].Cmp(big.NewRat(1, 1)) != 0 || new(big.Rat).Add(other.Gram[1][1], new(big.Rat).Mul(big.NewRat(2, 1), other.Gram[0][2])).Cmp(other.Coefficients[2]) != 0 {
		t.Fatal("nonzero Gram split", e)
	}
	other.Gram[0][1].SetInt64(999)
	if other.Gram[1][0].Cmp(big.NewRat(270, 1)) != 0 || other.Coefficients[1].Cmp(big.NewRat(540, 1)) != 0 {
		t.Fatal("aliased exact output")
	}
	if r, e := CertifyCoupledQuarticGraphPointContact(ctx, a, b, uv, GraphPointContactOptions{}); e == nil || r.Proven {
		t.Fatal("fixture already in even-mixed family")
	}
	if r, e := ClassifySplineGraphCriticalContact(ctx, a, b, uv, GraphPointContactOptions{}); e == nil || r.Proven {
		t.Fatal("degenerate Hessian claimed Morse")
	}
	// Independent numerical reference only: evaluate source against a direct
	// sum-of-fourth-powers expression, not against recovered coefficients.
	for _, u := range []float64{0, .125, .375, .75, 1} {
		for _, v := range []float64{0, .25, .5, .875, 1} {
			ja, e := a.EvaluateRational(ctx, new(big.Rat).SetFloat64(u), new(big.Rat).SetFloat64(v), SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			actual, _ := ja.Point[2].Float64()
			x, y := 3*u-1, 5*v-2
			want := math.Pow(x+y, 4) + math.Pow(x, 4) + math.Pow(y, 4) + u*v/64
			if math.Abs(actual-want) > 1e-10*math.Max(1, math.Abs(want)) {
				t.Fatal("independent source reference", actual, want)
			}
		}
	}
}

func TestQuarticGramContactRejectsExtraZeros(t *testing.T) {
	ctx := context.Background()
	uv := [2]*big.Rat{big.NewRat(1, 3), big.NewRat(2, 5)}
	for _, coeff := range [][5]int64{{1, 4, 6, 4, 1}, {1, 0, -3, 0, 1}} {
		a, b := representedQuarticGramFixture(t, coeff)
		r, e := CertifyQuarticGramGraphPointContact(ctx, a, b, uv, new(big.Rat), GraphPointContactOptions{})
		if e == nil || r.Proven || r.Coefficients[0] != nil {
			t.Fatal("extra zero line/saddle", e)
		}
	}
	a, b := representedQuarticGramFixture(t, [5]int64{2, 4, 6, 4, 2})
	for _, item := range []struct {
		uv    [2]*big.Rat
		cross *big.Rat
	}{{uv, big.NewRat(10000, 1)}, {[2]*big.Rat{big.NewRat(1, 2), uv[1]}, new(big.Rat)}, {[2]*big.Rat{new(big.Rat), uv[1]}, new(big.Rat)}} {
		r, e := CertifyQuarticGramGraphPointContact(ctx, a, b, item.uv, item.cross, GraphPointContactOptions{})
		if e == nil || r.Proven {
			t.Fatal("bad proposal", e)
		}
	}
	for _, h := range a.net {
		h[2].Add(h[2], big.NewRat(1, 1024))
	}
	if e := a.computeBounds(&polynomialBudget{maxOperations: 100000, maxBits: 4096}); e != nil {
		t.Fatal(e)
	}
	if r, e := CertifyQuarticGramGraphPointContact(ctx, a, b, uv, new(big.Rat), GraphPointContactOptions{}); e == nil || r.Proven {
		t.Fatal("extra constant", e)
	}
	a, b = representedQuarticGramFixture(t, [5]int64{2, 4, 6, 4, 2})
	for _, p := range []*SurfacePatch{a, b} {
		for i := range 5 {
			for j := range 5 {
				p.net[i*5+j][0] = new(big.Rat).Add(new(big.Rat).Sub(big.NewRat(int64(i*(i-1)), 12), big.NewRat(int64(i), 4)), big.NewRat(1, 4))
			}
		}
		if e := p.computeBounds(&polynomialBudget{maxOperations: 100000, maxBits: 4096}); e != nil {
			t.Fatal(e)
		}
	}
	if r, e := CertifyQuarticGramGraphPointContact(ctx, a, b, uv, new(big.Rat), GraphPointContactOptions{}); e == nil || r.Proven {
		t.Fatal("folded projection", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := CertifyQuarticGramGraphPointContact(cancelled, a, b, uv, new(big.Rat), GraphPointContactOptions{}); e != context.Canceled || r.Proven {
		t.Fatal(e)
	}
}

func TestQuarticGramPositiveRationalWeights(t *testing.T) {
	ctx := context.Background()
	a, b := representedQuarticGramFixture(t, [5]int64{2, 4, 6, 4, 2})
	var pair [2]*SurfacePatch
	for owner, p := range []*SurfacePatch{a, b} {
		var net [][4]*big.Rat
		for i := range 5 {
			for j := range 5 {
				w := big.NewRat(int64(4+i), 4)
				x := new(big.Rat).Add(big.NewRat(int64(i), 4), big.NewRat(int64(i*(i-1)), 12))
				y := new(big.Rat).Mul(big.NewRat(int64(j), 4), w)
				net = append(net, [4]*big.Rat{x, y, new(big.Rat).Set(p.net[i*5+j][2]), w})
			}
		}
		var e error
		pair[owner], e = NewExactHomogeneousBezierSurface(ctx, 4, 4, net, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
	}
	uv := [2]*big.Rat{big.NewRat(1, 3), big.NewRat(2, 5)}
	r, e := CertifyQuarticGramGraphPointContact(ctx, pair[0], pair[1], uv, new(big.Rat), GraphPointContactOptions{})
	if e != nil || !r.Proven || r.Point[2].Cmp(big.NewRat(1, 640)) != 0 {
		t.Fatal("exact rational numerator/positive denominator", r, e)
	}
	huge := new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 200))
	if bad, e := CertifyQuarticGramGraphPointContact(ctx, pair[0], pair[1], uv, huge, GraphPointContactOptions{MaxCoefficientBits: 64}); e == nil || bad.Proven || bad.Parameters[0] != nil {
		t.Fatal("input bit bound", e)
	}
}
