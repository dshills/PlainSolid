package geom

import (
	"context"
	"math"
	"testing"
)

func TestSpherePairPlaneRings(t *testing.T) {
	for _, normal := range []Vec3{{X: 1, Y: 1, Z: 1}, {X: -1, Y: -1, Z: -1}} {
		for _, x := range []float64{1, 20} {
			for _, op := range []SphereSetOperation{SphereSetUnion, SphereSetIntersection, SphereSetDifference} {
				s, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: x}, 30, op)
				// For the disjoint case choose equal heights relative to the raw plane.
				if x == 20 {
					s, _ = NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 20, Y: -20}, 3, op)
				}
				r, e := s.PlaneBoundaryRings(context.Background(), Vec3{}, normal, SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(x, op, e)
				}
				want := SphereSectionDisk
				owner := SpherePairSecond
				if x == 1 {
					if op == SphereSetIntersection {
						owner = SpherePairFirst
					}
					if op == SphereSetDifference {
						want = SphereSectionEmpty
					}
				} else {
					owner = SpherePairFirst
					if op == SphereSetUnion {
						want = SphereSectionDisjointDisks
					}
					if op == SphereSetIntersection {
						want = SphereSectionEmpty
					}
				}
				if r.Region != want {
					t.Fatal(r)
				}
				if want == SphereSectionDisk && (len(r.Rings) != 1 || r.Rings[0].Source != owner) {
					t.Fatal(r)
				}
			}
		}
		s, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 1}, 2, SphereSetDifference)
		r, e := s.PlaneBoundaryRings(context.Background(), Vec3{}, normal, SurfacePatchEvaluationOptions{})
		if e != nil || r.Region != SphereSectionAnnulus || !r.Rings[0].CCW || r.Rings[1].CCW {
			t.Fatal(r, e)
		}
	}
	// Physical distance is used, not Euclidean length of nonorthogonal UV.
	s, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 8}, 3, SphereSetUnion)
	if _, e := s.PlaneBoundaryRings(context.Background(), Vec3{}, Vec3{Y: 1, Z: 1}, SurfacePatchEvaluationOptions{}); e != ErrDomain {
		t.Fatal(e)
	}
	s, _ = NewSpherePairMaterial(Vec3{}, 5, Vec3{X: math.Nextafter(8, 9)}, 3, SphereSetUnion)
	r, e := s.PlaneBoundaryRings(context.Background(), Vec3{}, Vec3{Y: 1, Z: 1}, SurfacePatchEvaluationOptions{})
	if e != nil || r.Region != SphereSectionDisjointDisks {
		t.Fatal(r, e)
	}
	r, e = s.PlaneBoundaryRings(context.Background(), Vec3{}, Vec3{Z: 1}, SurfacePatchEvaluationOptions{MaxOperations: 1})
	if e == nil || r.Chart.NormalSquared != nil {
		t.Fatal("atomic", r, e)
	}
}
