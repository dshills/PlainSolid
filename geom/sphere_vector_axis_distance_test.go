package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
)

func TestSphereVectorAxisFamilies(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	for _, tc := range []struct {
		op                          SphereSetOperation
		x                           int64
		constant, coefficient, root float64
		kind                        SphereAxisMinimumKind
		slot                        int
		owners                      BoreBoundaryMask
	}{
		{SphereSetUnion, 0, 25, 0, 0, SphereAxisRetainedCap, 0, 3},
		{SphereSetUnion, 2, 25, 0, 0, SphereAxisRetainedCap, 1, 3},
		{SphereSetUnion, 1, 22, 0, 0, SphereAxisSharedCircle, 2, 3},
		{SphereSetUnion, -2, 37, 10, 12, SphereAxisPole, 0, 1},
		{SphereSetIntersection, 1, 28, 10, 3, SphereAxisPole, 0, 3},
		{SphereSetIntersection, 0, 37, 10, 12, SphereAxisPole, 1, 2},
		{SphereSetDifference, 2, 25, 0, 0, SphereAxisRetainedCap, 1, 3},
		{SphereSetDifference, 0, 37, 10, 12, SphereAxisPole, 1, 2},
	} {
		s, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 2, Y: 2, Z: 2}, 5, tc.op)
		p := [3]*big.Rat{r(tc.x, 1), r(tc.x, 1), r(tc.x, 1)}
		g, e := s.NearestTransverseAxisBoundaryRational(ctx, p, SphereFullBoundaryDistanceOptions{})
		if e != nil || g.Owners != tc.owners || g.Families[tc.slot].Kind != tc.kind || !g.Families[tc.slot].Nearest {
			t.Fatal(tc, g, e)
		}
		truth := tc.constant - tc.coefficient*math.Sqrt(tc.root)
		lo, _ := g.Lower.Float64()
		hi, _ := g.Upper.Float64()
		if truth < lo-1e-13 || truth > hi+1e-13 {
			t.Fatal("independent formula", truth, lo, hi)
		}
		for i, f := range g.Families {
			if i < 2 && f.SourceOwner != 1<<uint(i) || i == 2 && f.SourceOwner != 0 {
				t.Fatal("original source slot")
			}
			if f.Kind == SphereAxisPole {
				if f.Pole == nil || f.Pole.Radicand.Sign() <= 0 {
					t.Fatal("missing algebraic pole")
				}
				sum := new(big.Rat)
				for _, n := range f.Pole.Numerator {
					sum.Add(sum, new(big.Rat).Mul(n, n))
				}
				if new(big.Rat).Quo(sum, f.Pole.Radicand).Cmp(r(25, 1)) != 0 {
					t.Fatal("exact pole on original sphere")
				}
			} else {
				if f.Pole != nil {
					t.Fatal("guessed family representative")
				}
				for j := 0; j < 3; j++ {
					if f.Normal[j].Cmp(r(2, 1)) != 0 || f.PlanePoint[j].Cmp(r(1, 1)) != 0 {
						t.Fatal("original exact vector plane")
					}
				}
				if f.Kind == SphereAxisRetainedCap {
					side := 1
					if i == 1 {
						side = -1
					}
					if tc.op == SphereSetUnion || tc.op == SphereSetDifference && i == 0 {
						side = -side
					}
					if f.CapSide != side || f.RimOwners != 3 || f.Owners != 1<<uint(i) || f.RadiusSquared.Cmp(r(25, 1)) != 0 {
						t.Fatal("cap family ownership/halfspace")
					}
				} else if f.RadiusSquared.Cmp(r(22, 1)) != 0 || f.Owners != 3 {
					t.Fatal("whole circle")
				}
			}
		}
		for _, n := range []int{g.Operations, g.Operations - 1} {
			v, e := s.NearestTransverseAxisBoundaryRational(ctx, p, SphereFullBoundaryDistanceOptions{Arithmetic: SurfacePatchOptions{MaxOperations: n}})
			if n == g.Operations {
				if e != nil {
					t.Fatal(e)
				}
			} else if e == nil || v.Lower != nil {
				t.Fatal("atomic quota", e)
			}
		}
		g.Families[2].Normal[0].SetInt64(999)
		again, e := s.NearestTransverseAxisBoundaryRational(ctx, p, SphereFullBoundaryDistanceOptions{})
		if e != nil || again.Families[2].Normal[0].Cmp(r(2, 1)) != 0 {
			t.Fatal("owned family")
		}
		if _, e := s.NearestOverlapAxisBoundaryRational(ctx, p, SurfacePatchOptions{}); !errors.Is(e, ErrDomain) {
			t.Fatal("legacy axis gate changed", e)
		}
	}
	signed, _ := NewSpherePairMaterial(Vec3{X: 7, Y: -2, Z: 4}, 5, Vec3{X: 5, Y: -4, Z: 2}, 5, SphereSetUnion)
	p := [3]*big.Rat{r(7, 1), r(-2, 1), r(4, 1)}
	g, e := signed.NearestTransverseAxisBoundaryRational(ctx, p, SphereFullBoundaryDistanceOptions{})
	if e != nil || g.Upper.Cmp(r(25, 1)) != 0 || g.Families[0].Kind != SphereAxisRetainedCap || g.Families[0].CapSide != -1 {
		t.Fatal("signed placed cap", g, e)
	}
	for j, v := range []int64{6, -3, 3} {
		if g.Families[0].Normal[j].Cmp(r(-2, 1)) != 0 || g.Families[0].PlanePoint[j].Cmp(r(v, 1)) != 0 {
			t.Fatal("signed original plane")
		}
	}
	unequal, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 2, Y: 2, Z: 2}, 3, SphereSetDifference)
	u, e := unequal.NearestTransverseAxisBoundaryRational(ctx, [3]*big.Rat{r(2, 1), r(2, 1), r(2, 1)}, SphereFullBoundaryDistanceOptions{})
	if e != nil || u.Upper.Cmp(r(9, 1)) != 0 || u.Families[1].Kind != SphereAxisRetainedCap || u.Families[1].CapSide != -1 || u.Families[2].RadiusSquared.Cmp(r(26, 3)) != 0 {
		t.Fatal("unequal nonbinary cap", u, e)
	}
	for _, v := range u.Families[1].PlanePoint {
		if v.Cmp(r(7, 3)) != 0 {
			t.Fatal("exact plane coefficient")
		}
	}
	eps := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 120))
	p[0].Add(p[0], eps)
	v, e := signed.NearestTransverseAxisBoundaryRational(ctx, p, SphereFullBoundaryDistanceOptions{})
	if !errors.Is(e, ErrDomain) || v.Lower != nil {
		t.Fatal("off-axis subfloat", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	v, e = signed.NearestTransverseAxisBoundaryRational(cancelled, p, SphereFullBoundaryDistanceOptions{})
	if !errors.Is(e, context.Canceled) || v.Lower != nil {
		t.Fatal(e)
	}
}
