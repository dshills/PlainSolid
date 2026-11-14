package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
)

func TestSphereSeparatedUnionNearest(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	s, _ := NewSpherePairMaterial(Vec3{}, 2, Vec3{X: 6}, 1, SphereSetUnion)
	p := [3]*big.Rat{r(7, 2), r(0, 1), r(0, 1)}
	g, e := s.NearestSeparatedUnionBoundaryRational(ctx, p, SphereFullBoundaryDistanceOptions{})
	if e != nil || g.Owners != 3 || g.Comparison != 0 || g.Lower.Cmp(r(9, 4)) != 0 || g.Upper.Cmp(g.Lower) != 0 {
		t.Fatal(g, e)
	}
	for i, w := range g.Candidates {
		x := new(big.Rat).Add(w.Center[0], new(big.Rat).Quo(w.Numerator[0], w.Root[0]))
		if x.Cmp(r(int64(2+3*i), 1)) != 0 || w.MaterialNormalSign != 1 {
			t.Fatal("foot/normal", w)
		}
	}
	eps := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 120))
	for _, sign := range []int64{-1, 1} {
		q := p
		q[0] = new(big.Rat).Add(p[0], new(big.Rat).Mul(r(sign, 1), eps))
		v, e := s.NearestSeparatedUnionBoundaryRational(ctx, q, SphereFullBoundaryDistanceOptions{SqrtBits: 8})
		want := SpherePairFirst
		if sign > 0 {
			want = SpherePairSecond
		}
		if e != nil || v.Owners != want {
			t.Fatal("exact subfloat", v, e)
		}
	}
	for i, x := range []int64{0, 6} {
		v, e := s.NearestSeparatedUnionBoundaryRational(ctx, [3]*big.Rat{r(x, 1), r(0, 1), r(0, 1)}, SphereFullBoundaryDistanceOptions{})
		if e != nil || v.Owners != 1<<uint(i) || !v.Candidates[i].WholeSphere {
			t.Fatal("center family", v, e)
		}
	}
	tangent, _ := NewSpherePairMaterial(Vec3{}, 2, Vec3{X: 4}, 2, SphereSetUnion)
	for _, y := range []int64{0, 1} {
		v, e := tangent.NearestSeparatedUnionBoundaryRational(ctx, [3]*big.Rat{r(2, 1), r(y, 1), r(0, 1)}, SphereFullBoundaryDistanceOptions{})
		if e != nil || v.Owners != 3 || v.Comparison != 0 {
			t.Fatal("tangent tie", v, e)
		}
		if y == 0 {
			if v.Upper.Sign() != 0 {
				t.Fatal("contact distance")
			}
			for _, w := range v.Candidates {
				x := new(big.Rat).Add(w.Center[0], new(big.Rat).Quo(w.Numerator[0], w.Root[0]))
				if x.Cmp(r(2, 1)) != 0 {
					t.Fatal("shared foot")
				}
			}
		}
	}
	for _, x := range []float64{math.Nextafter(3, 0), 3, math.Nextafter(3, 4)} {
		q, _ := NewSpherePairMaterial(Vec3{}, 2, Vec3{X: x}, 1, SphereSetUnion)
		v, e := q.NearestSeparatedUnionBoundaryRational(ctx, p, SphereFullBoundaryDistanceOptions{})
		if x < 3 {
			if !errors.Is(e, ErrDomain) || v.Lower != nil {
				t.Fatal("overlap accepted", e)
			}
		} else if e != nil {
			t.Fatal(e)
		}
	}
	for _, n := range []int{g.Operations, g.Operations - 1} {
		v, e := s.NearestSeparatedUnionBoundaryRational(ctx, p, SphereFullBoundaryDistanceOptions{Arithmetic: SurfacePatchOptions{MaxOperations: n}})
		if n == g.Operations {
			if e != nil {
				t.Fatal(e)
			}
		} else if e == nil || v.Lower != nil {
			t.Fatal("atomic quota", e)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	v, e := s.NearestSeparatedUnionBoundaryRational(cancelled, p, SphereFullBoundaryDistanceOptions{})
	if !errors.Is(e, context.Canceled) || v.Lower != nil {
		t.Fatal("cancel", e)
	}
	for _, tc := range []struct {
		b  Vec3
		r  float64
		op SphereSetOperation
	}{{Vec3{}, 1, SphereSetUnion}, {Vec3{}, 2, SphereSetUnion}, {Vec3{X: 6}, 1, SphereSetDifference}} {
		q, _ := NewSpherePairMaterial(Vec3{}, 2, tc.b, tc.r, tc.op)
		v, e := q.NearestSeparatedUnionBoundaryRational(ctx, p, SphereFullBoundaryDistanceOptions{})
		if !errors.Is(e, ErrDomain) || v.Lower != nil {
			t.Fatal("unsupported full supports", e)
		}
	}
	off, _ := NewSpherePairMaterial(Vec3{}, 2, Vec3{X: 4, Y: 4}, 1, SphereSetUnion)
	v, e = off.NearestSeparatedUnionBoundaryRational(ctx, [3]*big.Rat{r(4, 1), r(4, 1), r(0, 1)}, SphereFullBoundaryDistanceOptions{})
	if e != nil || v.Owners != SpherePairSecond || !v.Candidates[1].WholeSphere {
		t.Fatal("oblique centers", e)
	}
	g.Candidates[0].Center[0].SetInt64(999)
	again, e := s.NearestSeparatedUnionBoundaryRational(ctx, p, SphereFullBoundaryDistanceOptions{})
	if e != nil || again.Candidates[0].Center[0].Sign() != 0 {
		t.Fatal("owned rationals", e)
	}

}
