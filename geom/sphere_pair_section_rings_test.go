package geom

import (
	"context"
	"math"
	"math/big"
	"testing"
)

func TestSpherePairSectionDisjointRings(t *testing.T) {
	for _, op := range []SphereSetOperation{SphereSetUnion, SphereSetIntersection, SphereSetDifference} {
		s, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 10}, 3, op)
		r, e := s.HorizontalBoundaryRings(context.Background(), 1, SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		want, count := SphereSectionDisjointDisks, 2
		if op == SphereSetIntersection {
			want, count = SphereSectionEmpty, 0
		} else if op == SphereSetDifference {
			want, count = SphereSectionDisk, 1
		}
		if r.Region != want || len(r.Rings) != count {
			t.Fatal(op, r)
		}
		for i, v := range r.Rings {
			if !v.CCW || v.Source != 1<<uint(i) {
				t.Fatal(v)
			}
		}
	}
}
func TestSpherePairSectionNestedRings(t *testing.T) {
	for _, swap := range []bool{false, true} {
		for _, op := range []SphereSetOperation{SphereSetUnion, SphereSetIntersection, SphereSetDifference} {
			a, b, ra, rb := Vec3{}, Vec3{X: 1}, 5.0, 2.0
			if swap {
				a, b, ra, rb = b, a, rb, ra
			}
			s, _ := NewSpherePairMaterial(a, ra, b, rb, op)
			r, e := s.HorizontalBoundaryRings(context.Background(), 1, SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(swap, op, e)
			}
			outer := SpherePairFirst
			inner := SpherePairSecond
			if swap {
				outer, inner = inner, outer
			}
			switch op {
			case SphereSetUnion:
				if r.Region != SphereSectionDisk || len(r.Rings) != 1 || r.Rings[0].Source != outer || r.Rings[0].RadiusSquared.Cmp(big.NewRat(24, 1)) != 0 {
					t.Fatal(r)
				}
			case SphereSetIntersection:
				if r.Region != SphereSectionDisk || len(r.Rings) != 1 || r.Rings[0].Source != inner || r.Rings[0].RadiusSquared.Cmp(big.NewRat(3, 1)) != 0 {
					t.Fatal(r)
				}
			case SphereSetDifference:
				if swap {
					if r.Region != SphereSectionEmpty || len(r.Rings) != 0 {
						t.Fatal(r)
					}
				} else if r.Region != SphereSectionAnnulus || len(r.Rings) != 2 || !r.Rings[0].CCW || r.Rings[1].CCW {
					t.Fatal("cavity orientation", r)
				}
			}
		}
	}
	// Strictly concentric unequal circles have no division-by-distance issue.
	s, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{}, 2, SphereSetDifference)
	r, e := s.HorizontalBoundaryRings(context.Background(), 0, SurfacePatchEvaluationOptions{})
	if e != nil || r.Region != SphereSectionAnnulus {
		t.Fatal(r, e)
	}
}
func TestSpherePairSectionRingGuards(t *testing.T) {
	for _, c := range []struct{ x, r, h float64 }{{8, 3, 0}, {2, 3, 0}, {0, 5, 0}, {4, 3, 0}, {10, 3, 3}} {
		s, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: c.x}, c.r, SphereSetUnion)
		r, e := s.HorizontalBoundaryRings(context.Background(), c.h, SurfacePatchEvaluationOptions{})
		if e != ErrDomain || r.Height != nil || len(r.Rings) != 0 {
			t.Fatal(c, r, e)
		}
	}
	// One ULP outward from tangency proves separation with no tolerance gate.
	s, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: math.Nextafter(8, 9)}, 3, SphereSetUnion)
	r, e := s.HorizontalBoundaryRings(context.Background(), 0, SurfacePatchEvaluationOptions{})
	if e != nil || r.Region != SphereSectionDisjointDisks {
		t.Fatal(r, e)
	}
	r, e = s.HorizontalBoundaryRings(context.Background(), 0, SurfacePatchEvaluationOptions{MaxOperations: 1})
	if e == nil || r.Height != nil {
		t.Fatal("atomic budget", r, e)
	}
}

// Globally transverse spheres can still have strictly nested section disks.
func TestSpherePairSectionRingsGlobalTransverse(t *testing.T) {
	for _, op := range []SphereSetOperation{SphereSetUnion, SphereSetIntersection, SphereSetDifference} {
		s, err := NewSpherePairMaterial(Vec3{}, 5, Vec3{Z: 4}, 3, op)
		if err != nil {
			t.Fatal(err)
		}
		r, err := s.HorizontalBoundaryRings(context.Background(), 2, SurfacePatchEvaluationOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if op == SphereSetDifference {
			if r.Region != SphereSectionAnnulus || len(r.Rings) != 2 || r.Rings[1].CCW {
				t.Fatal(r)
			}
		} else {
			want := SpherePairFirst
			if op == SphereSetIntersection {
				want = SpherePairSecond
			}
			if r.Region != SphereSectionDisk || len(r.Rings) != 1 || r.Rings[0].Source != want {
				t.Fatal(r)
			}
		}
	}
}
