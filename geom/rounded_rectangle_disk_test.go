package geom

import (
	"context"
	"math"
	"testing"
)

func TestRoundedRectangleDiskCoverage(t *testing.T) {
	ctx := context.Background()
	frame, _ := NewFrame(Vec3{}, Vec3{1, 0, 0}, Vec3{0, 0, 1})
	rr, _ := NewPlanarRoundedRectangle(frame, 5, 4, 2)
	for _, tc := range []struct {
		x, y, r float64
		want    RoundedDiskRelation
	}{{0, 0, 1, RoundedDiskStrictlyCovered}, {0, 0, 2, RoundedDiskStrictlyCovered}, {3, 0, 2, RoundedDiskCoveredTouching}, {0, 0, 3, RoundedDiskStrictlyCovered}, {2, 1, 3, RoundedDiskCoveredTouching}, {2.01, 1, 3, RoundedDiskOverlap}, {3.75, 3, .75, RoundedDiskCoveredTouching}, {3.75, math.Nextafter(3, 4), .75, RoundedDiskOverlap}, {6, 0, 1, RoundedDiskOutsideTouching}, {7, 0, 1, RoundedDiskOutside}, {5, 0, 1, RoundedDiskOverlap}} {
		disk, _ := NewPlanarCircularProfile(Vec3{tc.x, tc.y, 0}, Vec3{0, 0, -2}, tc.r)
		r, e := CompareRoundedRectangleDisk(ctx, rr, disk, SurfacePatchEvaluationOptions{})
		if e != nil || !r.Complete || r.Relation != tc.want {
			t.Fatal(tc, r, e)
		}
	}
	// Independently sample support inequalities for covered examples, including
	// radius above corner radius. h(n)=coreX|nx|+coreY|ny|+cornerRadius.
	for _, tc := range []struct{ x, y, r float64 }{{0, 0, 2}, {2, 1, 3}, {3.75, 3, .75}} {
		for i := 0; i < 1000; i++ {
			angle := 2 * math.Pi * float64(i) / 1000
			x, y := math.Cos(angle), math.Sin(angle)
			if tc.x*x+tc.y*y+tc.r > 3*math.Abs(x)+2*math.Abs(y)+2+1e-13 {
				t.Fatal("support oracle", tc)
			}
		}
	}
	for _, corner := range []float64{0, 2} {
		shape, _ := NewPlanarRoundedRectangle(frame, 2, 2, corner)
		disk, _ := NewPlanarCircularProfile(Vec3{}, Vec3{0, 0, 1}, 2)
		r, e := CompareRoundedRectangleDisk(ctx, shape, disk, SurfacePatchEvaluationOptions{})
		if e != nil || r.Relation != RoundedDiskCoveredTouching {
			t.Fatal(r, e)
		}
	}
	d, _ := NewPlanarCircularProfile(Vec3{Z: math.SmallestNonzeroFloat64}, Vec3{0, 0, 1}, 1)
	r, e := CompareRoundedRectangleDisk(ctx, rr, d, SurfacePatchEvaluationOptions{})
	if e != nil || r.Relation != RoundedDiskNonCoplanar {
		t.Fatal(r, e)
	}
	if r, e := CompareRoundedRectangleDisk(ctx, rr, d, SurfacePatchEvaluationOptions{MaxOperations: 1}); e == nil || r.Complete {
		t.Fatal("budget")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := CompareRoundedRectangleDisk(cancelled, rr, d, SurfacePatchEvaluationOptions{}); e == nil {
		t.Fatal("cancel")
	}
	oblique, _ := NewFrame(Vec3{}, Vec3{1, 1, 0}, Vec3{0, 0, 1})
	if _, e := NewPlanarRoundedRectangle(oblique, 2, 2, 1); e == nil {
		t.Fatal("rounded frame silently accepted")
	}

}
