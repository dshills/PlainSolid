package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestEqualCylinderCommonMeasure(t *testing.T) {
	r := big.NewRat
	ctx := context.Background()
	w := [2]*big.Rat{r(-2, 1), r(2, 1)}
	q, e := MeasureEqualCylinderCommon(ctx, Vec3{X: 1}, Vec3{Z: 1}, 2, &w, &w, SurfacePatchEvaluationOptions{})
	if e != nil || !q.Complete || !q.CapsCover || q.CapTouchMask != 15 || q.VolumeBounds[0].Cmp(r(128, 3)) != 0 || q.VolumeBounds[1].Cmp(r(128, 3)) != 0 {
		t.Fatal(q, e)
	}
	area, e := q.SectionAreaSquared(ctx, r(1, 1), SurfacePatchEvaluationOptions{})
	if e != nil || area.Cmp(r(144, 1)) != 0 {
		t.Fatal(area, e)
	}
	// u=(3,4),v=(4,3), sin=7/25. Volume=16/3*25/7 for r1.
	// Raw t half-extent²=(25+24)/(25*(25-24))=49/25.
	caps := [2]*big.Rat{r(-7, 5), r(7, 5)}
	q, e = MeasureEqualCylinderCommon(ctx, Vec3{X: 3, Y: 4}, Vec3{X: 4, Y: 3}, 1, &caps, &caps, SurfacePatchEvaluationOptions{})
	if e != nil || q.VolumeBounds[0].Cmp(r(400, 21)) != 0 || q.RawAxialHalfExtentSquared.Cmp(r(49, 25)) != 0 || !q.CapsCover || q.CapTouchMask != 15 {
		t.Fatal(q, e)
	}
	caps[1] = r(1, 1)
	q, e = MeasureEqualCylinderCommon(ctx, Vec3{X: 3, Y: 4}, Vec3{X: 4, Y: 3}, 1, &caps, nil, SurfacePatchEvaluationOptions{})
	if e != nil || q.CapsCover {
		t.Fatal(q, e)
	}
	// Irrational sin: equal-length integer axes (1,1,0),(1,0,1).
	q, e = MeasureEqualCylinderCommon(ctx, Vec3{X: 1, Y: 1}, Vec3{X: 1, Z: 1}, 1, nil, nil, SurfacePatchEvaluationOptions{})
	if e != nil || q.SinSquared.Cmp(r(3, 4)) != 0 || q.VolumeSquared.Cmp(r(1024, 27)) != 0 {
		t.Fatal(q, e)
	}
	for i, v := range q.VolumeBounds {
		c := new(big.Rat).Mul(v, v).Cmp(q.VolumeSquared)
		if i == 0 && c > 0 || i == 1 && c < 0 {
			t.Fatal(q)
		}
	}
	if _, e = MeasureEqualCylinderCommon(ctx, Vec3{X: 1}, Vec3{Z: 2}, 1, nil, nil, SurfacePatchEvaluationOptions{}); e != ErrDomain {
		t.Fatal(e)
	}
	q, e = MeasureEqualCylinderCommon(ctx, Vec3{X: 1}, Vec3{Z: 1}, 1, nil, nil, SurfacePatchEvaluationOptions{MaxOperations: 1})
	if e == nil || q.Complete || q.CapsCover {
		t.Fatal(q, e)
	}
}
