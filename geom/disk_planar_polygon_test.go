package geom

import (
	"context"
	"math"
	"testing"
)

func TestExactDiskPolygon(t *testing.T) {
	ctx := context.Background()
	square := []Vec3{{-2, -2, 0}, {2, -2, 0}, {2, 2, 0}, {-2, 2, 0}}
	hole := []Vec3{{-.5, -.5, 0}, {.5, -.5, 0}, {.5, .5, 0}, {-.5, .5, 0}}
	for _, tc := range []struct {
		x, y, r float64
		holes   [][]Vec3
		want    DiskPolygonRelation
	}{{0, 0, 1, nil, DiskPolygonStrictlyCovered}, {0, 0, 2, nil, DiskPolygonCoveredTouching}, {0, 0, math.Nextafter(2, 3), nil, DiskPolygonOverlap}, {3, 0, 1, nil, DiskPolygonOutsideTouching}, {3, 0, .5, nil, DiskPolygonOutside}, {2, 0, .1, nil, DiskPolygonOverlap}, {0, 0, .25, [][]Vec3{hole}, DiskPolygonOutside}, {0, 0, .5, [][]Vec3{hole}, DiskPolygonOutsideTouching}, {0, 0, .75, [][]Vec3{hole}, DiskPolygonOverlap}, {1, 0, .5, [][]Vec3{hole}, DiskPolygonCoveredTouching}, {1, 0, .25, [][]Vec3{hole}, DiskPolygonStrictlyCovered}} {
		d, _ := NewPlanarCircularProfile(Vec3{tc.x, tc.y, 0}, Vec3{0, 0, 1}, tc.r)
		r, e := ClassifyDiskPlanarPolygon(ctx, d, square, tc.holes, DiskPolygonOptions{})
		if e != nil || !r.Complete || r.Relation != tc.want {
			t.Fatal(tc, r, e)
		}
	}
	// Tilted raw plane x+y=0; projection distances must remain 3D Euclidean.
	tilted := []Vec3{{-2, 2, -2}, {2, -2, -2}, {2, -2, 2}, {-2, 2, 2}}
	d, _ := NewPlanarCircularProfile(Vec3{}, Vec3{1, 1, 0}, 2)
	r, e := ClassifyDiskPlanarPolygon(ctx, d, tilted, nil, DiskPolygonOptions{})
	if e != nil || r.Relation != DiskPolygonCoveredTouching {
		t.Fatal(r, e)
	}
	bad := append([]Vec3(nil), square...)
	bad[0].Z = math.SmallestNonzeroFloat64
	d, _ = NewPlanarCircularProfile(Vec3{}, Vec3{0, 0, 1}, 1)
	if _, e := ClassifyDiskPlanarPolygon(ctx, d, bad, nil, DiskPolygonOptions{}); e == nil {
		t.Fatal("noncoplanar")
	}
	for _, loop := range [][]Vec3{{{-2, -2, 0}, {2, 2, 0}, {2, -2, 0}, {-2, 2, 0}}, {{-2, -2, 0}, {2, -2, 0}, {0, -2, 0}, {2, 2, 0}, {-2, 2, 0}}} {
		if _, e := ClassifyDiskPlanarPolygon(ctx, d, loop, nil, DiskPolygonOptions{}); e == nil {
			t.Fatal("invalid loop")
		}
	}
	nested := []Vec3{{-.25, -.25, 0}, {.25, -.25, 0}, {.25, .25, 0}, {-.25, .25, 0}}
	if _, e := ClassifyDiskPlanarPolygon(ctx, d, square, [][]Vec3{hole, nested}, DiskPolygonOptions{}); e == nil {
		t.Fatal("nested holes")
	}
	if r, e := ClassifyDiskPlanarPolygon(ctx, d, square, nil, DiskPolygonOptions{MaxOperations: 1}); e == nil || r.Complete {
		t.Fatal("limit")
	}
}
