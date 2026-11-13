package geom

import (
	"context"
	"math"
	"testing"
)

func TestExactCircularProfileRelations(t *testing.T) {
	ctx := context.Background()
	makeDisk := func(x, r float64) PlanarCircularProfile {
		p, e := NewPlanarCircularProfile(Vec3{x, 0, 0}, Vec3{0, 0, 1}, r)
		if e != nil {
			t.Fatal(e)
		}
		return p
	}
	for _, test := range []struct {
		distance, a, b float64
		want           CircularProfileRelation
	}{{4, 1, 2, CircularProfilesDisjoint}, {3, 1, 2, CircularProfilesExternalTangent}, {2, 1, 2, CircularProfilesOverlap}, {.5, 1, 2, CircularProfileBStrictlyContainsA}, {1, 1, 2, CircularProfileBContainsATangent}, {.5, 2, 1, CircularProfileAStrictlyContainsB}, {1, 2, 1, CircularProfileAContainsBTangent}, {0, 2, 2, CircularProfilesCoincident}, {math.Nextafter(3, math.Inf(1)), 1, 2, CircularProfilesDisjoint}, {math.Nextafter(3, 0), 1, 2, CircularProfilesOverlap}} {
		r, e := ComparePlanarCircularProfiles(ctx, makeDisk(0, test.a), makeDisk(test.distance, test.b), SurfacePatchEvaluationOptions{})
		if e != nil || !r.Complete || !r.Coplanar || r.Relation != test.want {
			t.Fatal(test, r, e)
		}
	}
	a, _ := NewPlanarCircularProfile(Vec3{}, Vec3{1, 1, 0}, 1)
	b, _ := NewPlanarCircularProfile(Vec3{2, -2, 1}, Vec3{-3, -3, 0}, 2)
	r, e := ComparePlanarCircularProfiles(ctx, a, b, SurfacePatchEvaluationOptions{})
	if e != nil || r.Relation != CircularProfilesExternalTangent || r.NormalOrientation != -1 {
		t.Fatal(r, e)
	}
	shifted, _ := NewPlanarCircularProfile(Vec3{math.SmallestNonzeroFloat64, 0, 0}, Vec3{1, 1, 0}, 1)
	r, e = ComparePlanarCircularProfiles(ctx, a, shifted, SurfacePatchEvaluationOptions{})
	if e != nil || r.Coplanar || r.Relation != CircularProfilesNonCoplanar {
		t.Fatal("tiny plane displacement", r, e)
	}
	tiny := math.SmallestNonzeroFloat64
	r, e = ComparePlanarCircularProfiles(ctx, makeDisk(0, tiny), makeDisk(2*tiny, tiny), SurfacePatchEvaluationOptions{})
	if e != nil || r.Relation != CircularProfilesExternalTangent {
		t.Fatal(r, e)
	}
	r, e = ComparePlanarCircularProfiles(ctx, makeDisk(-1e308, 1e308), makeDisk(1e308, 1e308), SurfacePatchEvaluationOptions{})
	if e != nil || r.Relation != CircularProfilesExternalTangent {
		t.Fatal("overflow scale", r, e)
	}
	if r, e := ComparePlanarCircularProfiles(ctx, a, b, SurfacePatchEvaluationOptions{MaxOperations: 1}); e == nil || r.Complete {
		t.Fatal("budget")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := ComparePlanarCircularProfiles(cancelled, a, b, SurfacePatchEvaluationOptions{}); e == nil {
		t.Fatal("cancel")
	}
}
func TestExactAnnularFootprints(t *testing.T) {
	ctx := context.Background()
	outer, _ := NewPlanarCircularProfile(Vec3{}, Vec3{0, 0, 1}, 5)
	for _, test := range []struct {
		x, r float64
		want AnnularFootprintRelation
	}{{0, .5, AnnularFootprintDisjoint}, {0, 1, AnnularFootprintBoundaryOnly}, {.5, .5, AnnularFootprintBoundaryOnly}, {2, .5, AnnularFootprintStrictlyInside}, {2, 1, AnnularFootprintInsideTouching}, {4, 1, AnnularFootprintInsideTouching}, {4.5, 1, AnnularFootprintOverlap}, {6, 1, AnnularFootprintBoundaryOnly}, {7, 1, AnnularFootprintDisjoint}, {0, 3, AnnularFootprintOverlap}} {
		disk, _ := NewPlanarCircularProfile(Vec3{test.x, 0, 0}, Vec3{0, 0, -2}, test.r)
		r, e := CompareDiskAnnularFootprint(ctx, disk, outer, 1, SurfacePatchEvaluationOptions{})
		if e != nil || !r.Complete || r.Relation != test.want {
			t.Fatal(test, r, e)
		}
	}
}
