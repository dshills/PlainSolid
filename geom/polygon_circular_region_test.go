package geom

import (
	"context"
	"math"
	"testing"
)

func TestPolygonCircularRegionCoverage(t *testing.T) {
	ctx := context.Background()
	disk, _ := NewPlanarCircularProfile(Vec3{}, Vec3{0, 0, 1}, 5)
	square := []Vec3{{-3, -4, 0}, {3, -4, 0}, {3, 4, 0}, {-3, 4, 0}}
	r, e := ClassifyPolygonCircularRegion(ctx, square, nil, disk, nil, DiskPolygonOptions{})
	if e != nil || !r.Covered || !r.Touching {
		t.Fatal(r, e)
	}
	outside := append([]Vec3(nil), square...)
	outside[0].X = math.Nextafter(-3, math.Inf(-1))
	r, e = ClassifyPolygonCircularRegion(ctx, outside, nil, disk, nil, DiskPolygonOptions{})
	if e != nil || !r.Complete || r.Covered || r.OuterVertexOutside != 0 {
		t.Fatal(r, e)
	}
	bore, _ := NewPlanarCircularProfile(Vec3{1, 0, 0}, Vec3{0, 0, -2}, .5)
	inner := []Vec3{{.5, -.5, 0}, {.5, .5, 0}, {1.5, .5, 0}, {1.5, -.5, 0}}
	r, e = ClassifyPolygonCircularRegion(ctx, square, [][]Vec3{inner}, disk, []PlanarCircularProfile{bore}, DiskPolygonOptions{})
	if e != nil || !r.Covered || !r.Touching || len(r.HoleRelations) != 1 {
		t.Fatal(r, e)
	}
	r, e = ClassifyPolygonCircularRegion(ctx, square, nil, disk, []PlanarCircularProfile{bore}, DiskPolygonOptions{})
	if e != nil || !r.Complete || r.Covered {
		t.Fatal("filled bore covered", r, e)
	}
	// The bore lies outside an offset source rectangle, with exact edge tangency.
	offset := []Vec3{{1.5, -1, 0}, {3, -1, 0}, {3, 1, 0}, {1.5, 1, 0}}
	r, e = ClassifyPolygonCircularRegion(ctx, offset, nil, disk, []PlanarCircularProfile{bore}, DiskPolygonOptions{})
	if e != nil || !r.Covered || !r.Touching {
		t.Fatal(r, e)
	}
	if r, e := ClassifyPolygonCircularRegion(ctx, square, nil, disk, []PlanarCircularProfile{disk}, DiskPolygonOptions{}); e == nil || r.Complete {
		t.Fatal("invalid hole")
	}
	if r, e := ClassifyPolygonCircularRegion(ctx, square, nil, disk, nil, DiskPolygonOptions{MaxOperations: 1}); e == nil || r.Complete {
		t.Fatal("budget")
	}
}
