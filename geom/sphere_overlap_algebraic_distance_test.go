package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
)

func TestSphereOverlapAlgebraicNearest(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	for _, tc := range []struct {
		op                              SphereSetOperation
		x, y, z                         int64
		constant, coefficient, radicand float64
		rim                             bool
		owners                          BoreBoundaryMask
	}{
		{SphereSetUnion, 0, 2, 1, 21, 8, 5, true, 3},
		{SphereSetIntersection, 0, 7, 1, 66, 8, 50, true, 3},
		{SphereSetDifference, 3, 4, 1, 42, 8, 17, true, 3},
		{SphereSetIntersection, 0, 2, 1, 39, 10, 14, false, 3},
		{SphereSetUnion, 0, 7, 1, 84, 10, 59, false, 3},
		{SphereSetDifference, 0, 2, 1, 39, 10, 14, false, 2},
	} {
		s, _ := NewSpherePairMaterial(Vec3{X: -3}, 5, Vec3{X: 3}, 5, tc.op)
		p := [3]*big.Rat{r(tc.x, 1), r(tc.y, 1), r(tc.z, 1)}
		g, e := s.NearestOverlapAlgebraicBoundaryRational(ctx, p, SphereFullBoundaryDistanceOptions{})
		if e != nil || g.Owners != tc.owners || g.Candidates[2].Nearest != tc.rim {
			t.Fatal(tc, g, e)
		}
		lo, _ := g.Lower.Float64()
		hi, _ := g.Upper.Float64()
		truth := tc.constant - tc.coefficient*math.Sqrt(tc.radicand)
		if truth < lo-1e-13 || truth > hi+1e-13 {
			t.Fatal("independent formula", truth, lo, hi)
		}
		for i, w := range g.Candidates {
			if tc.rim && i < 2 && w.Retained {
				t.Fatal("buried support")
			}
			if w.Nearest {
				root, _ := w.Radicand.Float64()
				var foot [3]float64
				d := 0.
				for j := range foot {
					c, _ := w.Center[j].Float64()
					n, _ := w.Numerator[j].Float64()
					q, _ := p[j].Float64()
					foot[j] = c + n/math.Sqrt(root)
					d += math.Pow(foot[j]-q, 2)
				}
				if math.Abs(d-truth) > 1e-12 {
					t.Fatal("witness formula", foot, d, truth)
				}
				if tc.rim {
					if math.Abs(foot[0]) > 1e-14 || math.Abs(foot[1]*foot[1]+foot[2]*foot[2]-16) > 1e-12 {
						t.Fatal("exact circle identity", foot)
					}
				}
			}
		}
		for _, n := range []int{g.Operations, g.Operations - 1} {
			v, e := s.NearestOverlapAlgebraicBoundaryRational(ctx, p, SphereFullBoundaryDistanceOptions{Arithmetic: SurfacePatchOptions{MaxOperations: n}})
			if n == g.Operations {
				if e != nil {
					t.Fatal(e)
				}
			} else if e == nil || v.Lower != nil {
				t.Fatal("atomic aggregate budget", e)
			}
		}
		// Separate output ownership, original native material remains immutable.
		g.Candidates[2].Center[0].SetInt64(999)
		again, e := s.NearestOverlapAlgebraicBoundaryRational(ctx, p, SphereFullBoundaryDistanceOptions{})
		if e != nil || again.Candidates[2].Center[0].Sign() != 0 {
			t.Fatal("ownership", e)
		}
	}
	s, _ := NewSpherePairMaterial(Vec3{X: -3}, 5, Vec3{X: 3}, 5, SphereSetIntersection)
	eps := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 120))
	for _, sign := range []int64{-1, 1} {
		p := [3]*big.Rat{new(big.Rat).Mul(r(sign, 1), eps), r(2, 1), r(1, 1)}
		g, e := s.NearestOverlapAlgebraicBoundaryRational(ctx, p, SphereFullBoundaryDistanceOptions{SqrtBits: 8})
		want := SpherePairFirst
		if sign < 0 {
			want = SpherePairSecond
		}
		if e != nil || g.Owners != want {
			t.Fatal("exact subfloat order", g, e)
		}
		a, b := g.Candidates[0], g.Candidates[1]
		if a.Lower.Cmp(b.Upper) > 0 || b.Lower.Cmp(a.Upper) > 0 {
			t.Fatal("fixture must have overlapping enclosures")
		}
	}
	// Existing exact rational foot equality is preserved rather than rounded.
	p := [3]*big.Rat{r(0, 1), r(12, 5), r(16, 5)}
	g, e := s.NearestOverlapAlgebraicBoundaryRational(ctx, p, SphereFullBoundaryDistanceOptions{})
	if e != nil || g.Upper.Sign() != 0 || g.Owners != 3 {
		t.Fatal("exact seam equality", g, e)
	}
	for _, w := range g.Candidates {
		if !w.Nearest || w.Owners != 3 {
			t.Fatal("all coincident slots")
		}
	}
	for _, p := range [][3]*big.Rat{{r(0, 1), r(0, 1), r(0, 1)}, {r(-3, 1), r(0, 1), r(0, 1)}} {
		v, e := s.NearestOverlapAlgebraicBoundaryRational(ctx, p, SphereFullBoundaryDistanceOptions{})
		if !errors.Is(e, ErrDomain) || v.Lower != nil {
			t.Fatal("unsupported family", e)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	v, e := s.NearestOverlapAlgebraicBoundaryRational(cancelled, p, SphereFullBoundaryDistanceOptions{})
	if !errors.Is(e, context.Canceled) || v.Lower != nil {
		t.Fatal("cancel", e)
	}
	q, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 1}, 5, SphereSetUnion)
	v, e = q.NearestOverlapAlgebraicBoundaryRational(ctx, p, SphereFullBoundaryDistanceOptions{})
	if !errors.Is(e, ErrDomain) || v.Lower != nil {
		t.Fatal("irrational circle remains rejected", e)
	}
	signed, _ := NewSpherePairMaterial(Vec3{X: 2, Y: 1, Z: 7}, 5, Vec3{X: 2, Y: 1, Z: 1}, 5, SphereSetUnion)
	shifted, e := signed.NearestOverlapAlgebraicBoundaryRational(ctx, [3]*big.Rat{r(4, 1), r(2, 1), r(4, 1)}, SphereFullBoundaryDistanceOptions{})
	if e != nil || shifted.Owners != 3 || !shifted.Candidates[2].Nearest {
		t.Fatal("signed axis", shifted, e)
	}
	w := shifted.Candidates[2]
	if w.Radicand.Cmp(r(5, 1)) != 0 || w.Center[2].Cmp(r(4, 1)) != 0 || w.Numerator[0].Cmp(r(8, 1)) != 0 || w.Numerator[1].Cmp(r(4, 1)) != 0 || w.Numerator[2].Sign() != 0 {
		t.Fatal("exact transformed witness", w)
	}

}
