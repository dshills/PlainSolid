package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestSphereOverlapAxisFamilies(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	for _, tc := range []struct {
		op     SphereSetOperation
		x, d   int64
		kind   [3]SphereAxisMinimumKind
		owners BoreBoundaryMask
	}{
		{SphereSetUnion, -3, 25, [3]SphereAxisMinimumKind{SphereAxisRetainedCap, SphereAxisSharedCircle, SphereAxisSharedCircle}, 3},
		{SphereSetUnion, 0, 16, [3]SphereAxisMinimumKind{SphereAxisSharedCircle, SphereAxisSharedCircle, SphereAxisSharedCircle}, 3},
		{SphereSetUnion, 3, 25, [3]SphereAxisMinimumKind{SphereAxisSharedCircle, SphereAxisRetainedCap, SphereAxisSharedCircle}, 3},
		{SphereSetUnion, -10, 4, [3]SphereAxisMinimumKind{SphereAxisPole, SphereAxisSharedCircle, SphereAxisSharedCircle}, 1},
		{SphereSetIntersection, 0, 4, [3]SphereAxisMinimumKind{SphereAxisPole, SphereAxisPole, SphereAxisSharedCircle}, 3},
		{SphereSetIntersection, -3, 1, [3]SphereAxisMinimumKind{SphereAxisRetainedCap, SphereAxisPole, SphereAxisSharedCircle}, 2},
		{SphereSetDifference, 3, 25, [3]SphereAxisMinimumKind{SphereAxisSharedCircle, SphereAxisRetainedCap, SphereAxisSharedCircle}, 3},
		{SphereSetDifference, -3, 1, [3]SphereAxisMinimumKind{SphereAxisRetainedCap, SphereAxisPole, SphereAxisSharedCircle}, 2},
	} {
		s, _ := NewSpherePairMaterial(Vec3{X: -3}, 5, Vec3{X: 3}, 5, tc.op)
		p := [3]*big.Rat{r(tc.x, 1), r(0, 1), r(0, 1)}
		g, e := s.NearestOverlapAxisBoundaryRational(ctx, p, SurfacePatchOptions{})
		if e != nil || g.SquaredDistance.Cmp(r(tc.d, 1)) != 0 || g.Owners != tc.owners {
			t.Fatal(tc, g, e)
		}
		for i, f := range g.Families {
			if f.Kind != tc.kind[i] || f.Axis != 0 {
				t.Fatal("complete family type", i, f)
			}
			if i < 2 && f.SourceOwner != 1<<uint(i) || i == 2 && f.SourceOwner != 0 {
				t.Fatal("original source slots")
			}
			switch f.Kind {
			case SphereAxisSharedCircle:
				if f.Center[0].Sign() != 0 || f.Center[1].Sign() != 0 || f.Center[2].Sign() != 0 || f.RadiusSquared.Cmp(r(16, 1)) != 0 || f.Owners != 3 || f.RimOwners != 0 {
					t.Fatal("entire exact original circle", f)
				}
			case SphereAxisRetainedCap:
				outside := tc.op == SphereSetUnion || tc.op == SphereSetDifference && i == 0
				side := 1
				if i == 1 {
					side = -side
				}
				if outside {
					side = -side
				}
				if f.RadiusSquared.Cmp(r(25, 1)) != 0 || f.Center[0].Cmp(r(int64(-3+6*i), 1)) != 0 || f.CapCoordinate.Sign() != 0 || f.CapSide != side || f.Owners != 1<<uint(i) || f.RimOwners != 3 {
					t.Fatal("complete cap inequality/owners", f)
				}
			case SphereAxisPole:
				if f.Point[1].Sign() != 0 || f.Point[2].Sign() != 0 {
					t.Fatal("axis pole")
				}
				source := r(int64(-3+6*i), 1)
				v := new(big.Rat).Sub(f.Point[0], source)
				if new(big.Rat).Mul(v, v).Cmp(r(25, 1)) != 0 {
					t.Fatal("original pole")
				}
			}
			if f.Kind != SphereAxisPole {
				for _, v := range f.Point {
					if v != nil {
						t.Fatal("family has guessed representative")
					}
				}
			}
		}
		if tc.op == SphereSetDifference && g.MaterialNormalSigns[1] != -1 {
			t.Fatal("cutter normal")
		}
		for _, n := range []int{g.Operations, g.Operations - 1} {
			v, e := s.NearestOverlapAxisBoundaryRational(ctx, p, SurfacePatchOptions{MaxOperations: n})
			if n == g.Operations {
				if e != nil {
					t.Fatal(e)
				}
			} else if e == nil || v.SquaredDistance != nil {
				t.Fatal("atomic budget", e)
			}
		}
		g.Families[2].RadiusSquared.SetInt64(999)
		again, e := s.NearestOverlapAxisBoundaryRational(ctx, p, SurfacePatchOptions{})
		if e != nil || again.Families[2].RadiusSquared.Cmp(r(16, 1)) != 0 {
			t.Fatal("owned family", e)
		}
	}
	// Irrational radius sqrt(3) is preserved symbolically, never approximated.
	s, _ := NewSpherePairMaterial(Vec3{X: -1}, 2, Vec3{X: 1}, 2, SphereSetUnion)
	p := [3]*big.Rat{r(0, 1), r(0, 1), r(0, 1)}
	g, e := s.NearestOverlapAxisBoundaryRational(ctx, p, SurfacePatchOptions{})
	if e != nil || g.SquaredDistance.Cmp(r(3, 1)) != 0 || g.Families[2].RadiusSquared.Cmp(r(3, 1)) != 0 || g.Owners != 3 {
		t.Fatal("irrational circle family", g, e)
	}
	// Exact translated, reversed axis preserves whole-cap inequality in world Z.
	placed, _ := NewSpherePairMaterial(Vec3{X: 2, Y: 1, Z: 7}, 5, Vec3{X: 2, Y: 1, Z: 1}, 5, SphereSetUnion)
	w, e := placed.NearestOverlapAxisBoundaryRational(ctx, [3]*big.Rat{r(2, 1), r(1, 1), r(7, 1)}, SurfacePatchOptions{})
	f := w.Families[0]
	if e != nil || w.SquaredDistance.Cmp(r(25, 1)) != 0 || !f.Nearest || f.Kind != SphereAxisRetainedCap || f.Axis != 2 || f.CapSide != 1 || f.CapCoordinate.Cmp(r(4, 1)) != 0 {
		t.Fatal("signed placed family", w, e)
	}
	eps := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 120))
	p[1] = eps
	v, e := s.NearestOverlapAxisBoundaryRational(ctx, p, SurfacePatchOptions{})
	if !errors.Is(e, ErrDomain) || v.SquaredDistance != nil {
		t.Fatal("off-axis subfloat accepted", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	v, e = s.NearestOverlapAxisBoundaryRational(cancelled, p, SurfacePatchOptions{})
	if !errors.Is(e, context.Canceled) || v.SquaredDistance != nil {
		t.Fatal(e)
	}
	tangent, _ := NewSpherePairMaterial(Vec3{}, 2, Vec3{X: 4}, 2, SphereSetUnion)
	p[1] = r(0, 1)
	v, e = tangent.NearestOverlapAxisBoundaryRational(ctx, p, SurfacePatchOptions{})
	if !errors.Is(e, ErrDomain) || v.SquaredDistance != nil {
		t.Fatal("nontransverse accepted")
	}
	// Unequal source radii give a nonbinary radical plane and irrational rim.
	unequal, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 3}, 3, SphereSetDifference)
	u, e := unequal.NearestOverlapAxisBoundaryRational(ctx, [3]*big.Rat{r(3, 1), r(0, 1), r(0, 1)}, SurfacePatchOptions{})
	if e != nil || u.Owners != 3 || u.SquaredDistance.Cmp(r(9, 1)) != 0 || u.Families[1].Kind != SphereAxisRetainedCap || !u.Families[1].Nearest || u.Families[1].CapSide != -1 || u.Families[1].CapCoordinate.Cmp(r(25, 6)) != 0 || u.Families[2].RadiusSquared.Cmp(r(275, 36)) != 0 {
		t.Fatal("nonbinary retained family", u, e)
	}

}
