package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
)

func TestSphereCavityNearestExactOwners(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	s, e := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 1}, 1, SphereSetDifference)
	if e != nil {
		t.Fatal(e)
	}
	tieP := [3]*big.Rat{r(7, 2), r(0, 1), r(0, 1)}
	got, e := s.NearestCavityBoundaryRational(ctx, tieP, SphereCavityDistanceOptions{})
	if e != nil || got.Owners != SpherePairFirst|SpherePairSecond || got.Comparison != 0 || got.Lower.Cmp(r(9, 4)) != 0 || got.Upper.Cmp(got.Lower) != 0 {
		t.Fatal("complete two-owner tie", got, e)
	}
	for i, w := range got.Candidates {
		if w.WholeSphere || w.Owner != 1<<uint(i) || w.MaterialNormalSign != 1-2*i {
			t.Fatal("original owner/normal")
		}
		x := new(big.Rat).Add(w.Center[0], new(big.Rat).Quo(w.Numerator[0], w.Root[0]))
		want := r(5, 1)
		if i == 1 {
			want = r(2, 1)
		}
		if x.Cmp(want) != 0 {
			t.Fatal("distinct attained feet")
		}
		loc, owner, e := s.Classify(Vec3{X: float64(5 - 3*i)})
		if e != nil || loc != BoreBoundary || owner != w.Owner {
			t.Fatal("original exposed support", e)
		}
	}
	eps := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 120))
	for _, sign := range []int64{-1, 1} {
		p := tieP
		p[0] = new(big.Rat).Add(tieP[0], new(big.Rat).Mul(r(sign, 1), eps))
		g, e := s.NearestCavityBoundaryRational(ctx, p, SphereCavityDistanceOptions{SqrtBits: 8})
		want := SpherePairFirst
		if sign < 0 {
			want = SpherePairSecond
		}
		if e != nil || g.Owners != want {
			t.Fatal("subfloat exact order", g, e)
		}
	}
	// Two irrational radial norms and an independently calculated minimum.
	for _, p := range [][3]*big.Rat{{r(7, 2), r(1, 2), r(0, 1)}, {r(13, 4), r(1, 2), r(0, 1)}} {
		g, e := s.NearestCavityBoundaryRational(ctx, p, SphereCavityDistanceOptions{})
		if e != nil {
			t.Fatal(e)
		}
		x, _ := p[0].Float64()
		outer := math.Pow(math.Hypot(x, .5)-5, 2)
		inner := math.Pow(math.Hypot(x-1, .5)-1, 2)
		want := SpherePairFirst
		if inner < outer {
			want = SpherePairSecond
		}
		lo, _ := g.Lower.Float64()
		hi, _ := g.Upper.Float64()
		truth := math.Min(outer, inner)
		if g.Owners != want || truth < lo-1e-14 || truth > hi+1e-14 {
			t.Fatal("independent irrational minima", g)
		}
	}
	family, e := s.NearestCavityBoundaryRational(ctx, [3]*big.Rat{r(1, 1), r(0, 1), r(0, 1)}, SphereCavityDistanceOptions{})
	if e != nil || family.Owners != SpherePairSecond || !family.Candidates[1].WholeSphere || family.Candidates[1].Radicand.Sign() != 0 || family.Lower.Cmp(r(1, 1)) != 0 {
		t.Fatal("explicit center family", family, e)
	}
	// Arbitrary off-axis source centers are supported without frame rounding.
	off, e := NewSpherePairMaterial(Vec3{X: 1, Y: 2, Z: 3}, 5, Vec3{X: 2, Y: 3, Z: 3}, 1, SphereSetDifference)
	if e != nil {
		t.Fatal(e)
	}
	f, e := off.NearestCavityBoundaryRational(ctx, [3]*big.Rat{r(2, 1), r(3, 1), r(3, 1)}, SphereCavityDistanceOptions{})
	if e != nil || f.Owners != SpherePairSecond || !f.Candidates[1].WholeSphere {
		t.Fatal("off-axis cavity", e)
	}
	// One ULP of strict containment is accepted exactly; equality and outside
	// are rejected even though represented centers can appear visually tangent.
	for _, x := range []float64{math.Nextafter(4, 0), 4, math.Nextafter(4, 5)} {
		q, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: x}, 1, SphereSetDifference)
		v, e := q.NearestCavityBoundaryRational(ctx, [3]*big.Rat{new(big.Rat).SetFloat64(x), r(0, 1), r(0, 1)}, SphereCavityDistanceOptions{})
		if x < 4 {
			if e != nil || v.Owners != SpherePairSecond {
				t.Fatal("exact near containment", e)
			}
		} else if !errors.Is(e, ErrDomain) || v.Lower != nil {
			t.Fatal("tangent/crossing accepted", e)
		}
	}
	// A shared total budget includes original-domain validation and ordering.
	exactBudget := SphereCavityDistanceOptions{Arithmetic: SurfacePatchOptions{MaxOperations: got.Operations}}
	if _, e = s.NearestCavityBoundaryRational(ctx, tieP, exactBudget); e != nil {
		t.Fatal("exact quota", e)
	}
	exactBudget.Arithmetic.MaxOperations--
	if v, e := s.NearestCavityBoundaryRational(ctx, tieP, exactBudget); e == nil || v.Lower != nil {
		t.Fatal("atomic shared quota")
	}
	cc, cancel := context.WithCancel(ctx)
	cancel()
	if v, e := s.NearestCavityBoundaryRational(cc, tieP, SphereCavityDistanceOptions{}); e == nil || v.Lower != nil {
		t.Fatal("atomic cancellation")
	}
	got.Candidates[0].Center[0].SetInt64(99)
	again, e := s.NearestCavityBoundaryRational(ctx, tieP, SphereCavityDistanceOptions{})
	if e != nil || again.Candidates[0].Center[0].Sign() != 0 {
		t.Fatal("source mutation through result")
	}
}
