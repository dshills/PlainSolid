package scene

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func TestPickSideOpenDualDiskAffineSeamNormals(t *testing.T) {
	ctx := context.Background()
	f, e := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	discs := [2]trimmed.ExtrusionCircle{{Center: geom.Vec2{X: 3.5, Y: 4}, Radius: 2.5}, {Center: geom.Vec2{X: 6.5, Y: 4}, Radius: 2.5}}
	cut, e := trimmed.CutSideOpenDualDisks(ctx, f, geom.Vec2{Y: 4}, geom.Vec2{X: 10, Y: 10}, discs, 6, 6, 1e-9, trimvalidate.Options{})
	if e != nil {
		t.Fatal(e)
	}
	// The exact seam (5,6,z) maps to (-9,20,2z+1). Reflection and
	// shear make the original wall normals point in different directions.
	m := geom.Affine{{-3, 1, 0, 0}, {0, 3, 0, 2}, {0, 0, 2, 1}}
	camera, e := Orthographic(geom.Vec3{X: -9, Y: 20, Z: -3}, geom.Vec3{X: -9, Y: 20, Z: 1}, geom.Vec3{Y: 1}, 10, 10, 1, 30)
	if e != nil {
		t.Fatal(e)
	}
	far := m
	far[2][3] = 4
	items := []SideOpenDualDiskNormalItem{{"far", cut, far}, {"z-seam", cut, m}, {"a-seam", cut, m}}
	hit, ok, e := PickSideOpenDualDiskNormals(ctx, camera, geom.Vec2{}, items, geom.ToroidLineOptions{})
	h := hit.Hit
	if e != nil || !ok || hit.ID != "a-seam" || h.SpanEnd == nil || h.Tangent || len(h.Normals) != 3 {
		t.Fatal("expected cap and two wall normals at the first coincident seam", hit, ok, e)
	}
	equal := func(b geom.ExactParameterInterval, n, d int64) bool {
		r := big.NewRat(n, d)
		return b.Lower.Cmp(r) == 0 && b.Upper.Cmp(r) == 0
	}
	if !equal(h.Bounds, 3, 1) || !equal(h.SpanEnd.Interval(), 15, 1) || !equal(h.PointBounds[0], -9, 1) || !equal(h.PointBounds[1], 20, 1) || !equal(h.PointBounds[2], 1, 1) {
		t.Fatal("world camera ray intercepts were not retained exactly", h)
	}
	for i, w := range []struct {
		s       geom.BoxDualCircleSupport
		x, y, z [2]int64
		sign    int
	}{
		{geom.BoxDualCircleStockZ, [2]int64{0, 1}, [2]int64{0, 1}, [2]int64{-1, 2}, -1},
		{geom.BoxDualCircleFirstWall, [2]int64{1, 2}, [2]int64{-5, 6}, [2]int64{0, 1}, 0},
		{geom.BoxDualCircleSecondWall, [2]int64{-1, 2}, [2]int64{-1, 2}, [2]int64{0, 1}, 0},
	} {
		n := h.Normals[i]
		if n.Source != w.s || n.DirectionSign != w.sign || !equal(n.Bounds[0], w.x[0], w.x[1]) || !equal(n.Bounds[1], w.y[0], w.y[1]) || !equal(n.Bounds[2], w.z[0], w.z[1]) {
			t.Fatal("wrong world covector", n)
		}
	}
	singular := m
	singular[2] = [4]float64{}
	hit, ok, e = PickSideOpenDualDiskNormals(ctx, camera, geom.Vec2{}, append(items, SideOpenDualDiskNormalItem{"invalid", cut, singular}), geom.ToroidLineOptions{})
	if !errors.Is(e, geom.ErrDegenerate) || ok || hit.Hit.HasHit {
		t.Fatal("a late singular occurrence must discard prior hits", hit, ok, e)
	}
}
