package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
)

func TestSphereTransverseNearest(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	for _, tc := range []struct {
		op             SphereSetOperation
		p              [3]*big.Rat
		constant, root *big.Rat
		rim            bool
		coefficient    int64
		owners         BoreBoundaryMask
	}{
		{SphereSetUnion, [3]*big.Rat{r(2, 1), r(0, 1), r(1, 2)}, r(99, 4), r(91, 2), true, 2, 3},
		{SphereSetIntersection, [3]*big.Rat{r(5, 1), r(-3, 1), r(1, 2)}, r(219, 4), r(728, 1), true, 2, 3},
		{SphereSetDifference, [3]*big.Rat{r(6, 1), r(-2, 1), r(1, 1)}, r(57, 1), r(728, 1), true, 2, 3},
		{SphereSetIntersection, [3]*big.Rat{r(2, 1), r(0, 1), r(1, 2)}, r(117, 4), r(17, 4), false, 10, 3},
		{SphereSetUnion, [3]*big.Rat{r(5, 1), r(-3, 1), r(1, 2)}, r(237, 4), r(137, 4), false, 10, 3},
	} {
		s, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 2, Y: 2, Z: 1}, 5, tc.op)
		g, e := s.NearestTransverseBoundaryRational(ctx, tc.p, SphereFullBoundaryDistanceOptions{})
		if e != nil || g.Owners != tc.owners || g.Candidates[2].Nearest != tc.rim {
			t.Fatal(tc, g, e)
		}
		constant, _ := tc.constant.Float64()
		root, _ := tc.root.Float64()
		truth := constant - float64(tc.coefficient)*math.Sqrt(root)
		lo, _ := g.Lower.Float64()
		hi, _ := g.Upper.Float64()
		if truth < lo-1e-13 || truth > hi+1e-13 {
			t.Fatal("independent distance", truth, lo, hi)
		}
		w := g.Candidates[2]
		if w.Center[0].Cmp(r(1, 1)) != 0 || w.Center[1].Cmp(r(1, 1)) != 0 || w.Center[2].Cmp(r(1, 2)) != 0 || w.Coefficient.Cmp(r(2, 1)) != 0 {
			t.Fatal("exact original circle center", w)
		}
		dot := new(big.Rat)
		square := new(big.Rat)
		for j, n := range w.Numerator {
			scale := int64(2)
			if j == 2 {
				scale = 1
			}
			dot.Add(dot, new(big.Rat).Mul(r(scale, 1), n))
			square.Add(square, new(big.Rat).Mul(n, n))
		}
		if dot.Sign() != 0 || new(big.Rat).Quo(square, w.Radicand).Cmp(r(91, 4)) != 0 {
			t.Fatal("symbolic whole-circle support identity")
		}
		if tc.rim {
			if g.Candidates[0].Retained || g.Candidates[1].Retained || w.Constant.Cmp(tc.constant) != 0 || w.Radicand.Cmp(tc.root) != 0 {
				t.Fatal("buried feet and exact radical", g)
			}
		}
		for _, n := range []int{g.Operations, g.Operations - 1} {
			v, e := s.NearestTransverseBoundaryRational(ctx, tc.p, SphereFullBoundaryDistanceOptions{Arithmetic: SurfacePatchOptions{MaxOperations: n}})
			if n == g.Operations {
				if e != nil {
					t.Fatal(e)
				}
			} else if e == nil || v.Lower != nil {
				t.Fatal("atomic budget", e)
			}
		}
		if _, e := s.NearestOverlapAlgebraicBoundaryRational(ctx, tc.p, SphereFullBoundaryDistanceOptions{}); !errors.Is(e, ErrDomain) {
			t.Fatal("legacy gate changed", e)
		}
	}
	s, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 2, Y: 2, Z: 1}, 5, SphereSetIntersection)
	eps := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 120))
	for _, sign := range []int64{-1, 1} {
		p := [3]*big.Rat{r(2, 1), r(0, 1), r(1, 2)}
		for j := range p {
			scale := int64(2)
			if j == 2 {
				scale = 1
			}
			p[j].Add(p[j], new(big.Rat).Mul(r(sign*scale, 1), eps))
		}
		g, e := s.NearestTransverseBoundaryRational(ctx, p, SphereFullBoundaryDistanceOptions{SqrtBits: 8})
		want := SpherePairFirst
		if sign < 0 {
			want = SpherePairSecond
		}
		if e != nil || g.Owners != want {
			t.Fatal("oblique subfloat ordering", g, e)
		}
	}
	p := [3]*big.Rat{r(1, 1), r(1, 1), r(1, 2)}
	v, e := s.NearestTransverseBoundaryRational(ctx, p, SphereFullBoundaryDistanceOptions{})
	if !errors.Is(e, ErrDomain) || v.Lower != nil {
		t.Fatal("axis family must stay explicit", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	v, e = s.NearestTransverseBoundaryRational(cancelled, p, SphereFullBoundaryDistanceOptions{})
	if !errors.Is(e, context.Canceled) || v.Lower != nil {
		t.Fatal(e)
	}
	// Different symbolic radicals describe the same attained irrational foot.
	unequal, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 2, Y: 2, Z: 1}, 4, SphereSetUnion)
	shared, e := unequal.NearestTransverseBoundaryRational(ctx, [3]*big.Rat{r(3, 1), r(1, 1), r(1, 1)}, SphereFullBoundaryDistanceOptions{SqrtBits: 8})
	if e != nil || shared.Owners != 3 || shared.Candidates[0].Nearest || !shared.Candidates[1].Nearest || !shared.Candidates[2].Nearest || shared.Candidates[1].Radicand.Cmp(r(2, 1)) != 0 || shared.Candidates[2].Radicand.Cmp(r(32, 1)) != 0 {
		t.Fatal("exact cross-formula seam tie", shared, e)
	}
	shared.Candidates[2].Center[0].SetInt64(999)
	again, e := unequal.NearestTransverseBoundaryRational(ctx, [3]*big.Rat{r(3, 1), r(1, 1), r(1, 1)}, SphereFullBoundaryDistanceOptions{})
	if e != nil || again.Candidates[2].Center[0].Cmp(r(2, 1)) != 0 {
		t.Fatal("owned source result", e)
	}

}
