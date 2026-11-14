package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestSphereOverlapNearestRetainedFeet(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	for _, tc := range []struct {
		op       SphereSetOperation
		x, y     *big.Rat
		distance *big.Rat
		retained [2]bool
		owners   BoreBoundaryMask
	}{
		{SphereSetUnion, r(0, 1), r(9, 4), r(49, 16), [2]bool{false, false}, 3},
		{SphereSetIntersection, r(0, 1), r(36, 5), r(256, 25), [2]bool{false, false}, 3},
		{SphereSetDifference, r(3, 1), r(9, 2), r(37, 4), [2]bool{false, false}, 3},
		{SphereSetIntersection, r(0, 1), r(9, 4), r(25, 16), [2]bool{true, true}, 3},
		{SphereSetDifference, r(0, 1), r(9, 4), r(25, 16), [2]bool{false, true}, 2},
		{SphereSetUnion, r(0, 1), r(36, 5), r(196, 25), [2]bool{true, true}, 3},
	} {
		s, _ := NewSpherePairMaterial(Vec3{X: -3}, 5, Vec3{X: 3}, 5, tc.op)
		p := [3]*big.Rat{tc.x, tc.y, r(0, 1)}
		g, e := s.NearestOverlapBoundaryRational(ctx, p, SurfacePatchOptions{})
		if e != nil || g.Owners != tc.owners || g.SquaredDistance.Cmp(tc.distance) != 0 {
			t.Fatal(tc.op, g, e)
		}
		for i := 0; i < 2; i++ {
			if g.Candidates[i].Retained != tc.retained[i] {
				t.Fatal("buried foot", tc.op, i, g)
			}
			want := 1
			if i == 1 && tc.op == SphereSetDifference {
				want = -1
			}
			if g.MaterialNormalSigns[i] != want {
				t.Fatal("material normals")
			}
		}
		rim := g.Candidates[2]
		if rim.Point[0].Sign() != 0 || rim.Point[1].Cmp(r(4, 1)) != 0 || rim.Point[2].Sign() != 0 || rim.Owners != 3 {
			t.Fatal("exact paired seam", rim)
		}
		if !tc.retained[0] && !tc.retained[1] && !rim.Nearest {
			t.Fatal("rim minimum")
		}
		// Every retained point is independently classified on the material boundary.
		for _, w := range g.Candidates {
			if !w.Retained {
				continue
			}
			q := Vec3{}
			q.X, _ = w.Point[0].Float64()
			q.Y, _ = w.Point[1].Float64()
			q.Z, _ = w.Point[2].Float64()
			if tc.y.Denom().Int64() == 4 {
				loc, mask, e := s.Classify(q)
				if e != nil || loc != BoreBoundary || mask != w.Owners {
					t.Fatal("independent incidence", loc, mask, e)
				}
			}
		}
		for _, n := range []int{g.Operations, g.Operations - 1} {
			v, e := s.NearestOverlapBoundaryRational(ctx, p, SurfacePatchOptions{MaxOperations: n})
			if n == g.Operations {
				if e != nil {
					t.Fatal(e)
				}
			} else if e == nil || v.SquaredDistance != nil {
				t.Fatal("atomic quota", e)
			}
		}
		g.Candidates[2].Point[1].SetInt64(999)
		v, e := s.NearestOverlapBoundaryRational(ctx, p, SurfacePatchOptions{})
		if e != nil || v.Candidates[2].Point[1].Cmp(r(4, 1)) != 0 {
			t.Fatal("owned results")
		}
	}
	s, _ := NewSpherePairMaterial(Vec3{X: -3}, 5, Vec3{X: 3}, 5, SphereSetUnion)
	for _, p := range [][3]*big.Rat{{r(0, 1), r(0, 1), r(0, 1)}, {r(0, 1), r(1, 1), r(0, 1)}, {r(-3, 1), r(0, 1), r(0, 1)}} {
		v, e := s.NearestOverlapBoundaryRational(ctx, p, SurfacePatchOptions{})
		if !errors.Is(e, ErrDomain) || v.SquaredDistance != nil {
			t.Fatal("unsupported radical/family", v, e)
		}
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	v, e := s.NearestOverlapBoundaryRational(cancelCtx, [3]*big.Rat{r(0, 1), r(9, 4), r(0, 1)}, SurfacePatchOptions{})
	if !errors.Is(e, context.Canceled) || v.SquaredDistance != nil {
		t.Fatal(e)
	}
	// Signed Z axis and translated common plane preserve the same exact feet.
	placed, _ := NewSpherePairMaterial(Vec3{X: 2, Y: 1, Z: 7}, 5, Vec3{X: 2, Y: 1, Z: 1}, 5, SphereSetUnion)
	pg, e := placed.NearestOverlapBoundaryRational(ctx, [3]*big.Rat{r(17, 4), r(1, 1), r(4, 1)}, SurfacePatchOptions{})
	if e != nil || pg.SquaredDistance.Cmp(r(49, 16)) != 0 || pg.Candidates[2].Point[0].Cmp(r(6, 1)) != 0 || pg.Candidates[2].Point[2].Cmp(r(4, 1)) != 0 {
		t.Fatal("signed translated axis", pg, e)
	}
	// Boundary query retains all stable candidate slots, sharing the same foot.
	boundary, e := s.NearestOverlapBoundaryRational(ctx, [3]*big.Rat{r(0, 1), r(4, 1), r(0, 1)}, SurfacePatchOptions{})
	if e != nil || boundary.SquaredDistance.Sign() != 0 {
		t.Fatal(e)
	}
	for _, w := range boundary.Candidates {
		if !w.Nearest || w.Owners != 3 || w.Point[1].Cmp(r(4, 1)) != 0 {
			t.Fatal("stable equal witnesses", w)
		}
	}
	for _, q := range []SpherePairMaterial{func() SpherePairMaterial {
		v, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 1}, 5, SphereSetUnion)
		return v
	}(), func() SpherePairMaterial {
		v, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 10}, 5, SphereSetUnion)
		return v
	}()} {
		v, e := q.NearestOverlapBoundaryRational(ctx, [3]*big.Rat{r(0, 1), r(4, 1), r(0, 1)}, SurfacePatchOptions{})
		if !errors.Is(e, ErrDomain) || v.SquaredDistance != nil {
			t.Fatal("irrational seam/tangent accepted", e)
		}
	}

	unequal, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 4}, 3, SphereSetIntersection)
	u, e := unequal.NearestOverlapBoundaryRational(ctx, [3]*big.Rat{r(4, 1), r(16, 3), r(0, 1)}, SurfacePatchOptions{})
	if e != nil || u.SquaredDistance.Cmp(r(49, 9)) != 0 || u.Owners != 3 || u.Candidates[0].Retained || !u.Candidates[2].Nearest {
		t.Fatal("unequal retained rim", u, e)
	}

}
