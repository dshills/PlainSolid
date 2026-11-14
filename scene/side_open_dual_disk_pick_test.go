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

func TestPickSideOpeningSurvivingMaterial(t *testing.T) {
	ctx := context.Background()
	f, e := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	discs := [2]trimmed.ExtrusionCircle{{Center: geom.Vec2{X: 3.5, Y: 4}, Radius: 2.5}, {Center: geom.Vec2{X: 6.5, Y: 4}, Radius: 2.5}}
	makeCut := func(depth float64) trimmed.SideOpenDualDiskCut {
		c, e := trimmed.CutSideOpenDualDisks(ctx, f, geom.Vec2{Y: 4}, geom.Vec2{X: 10, Y: 10}, discs, 6, depth, 1e-9, trimvalidate.Options{})
		if e != nil {
			t.Fatal(e)
		}
		return c
	}
	blind, through := makeCut(3), makeCut(6)
	camera, e := Orthographic(geom.Vec3{X: 5, Y: 5, Z: -3}, geom.Vec3{X: 5, Y: 5, Z: 1}, geom.Vec3{Y: 1}, 10, 10, 1, 20)
	if e != nil {
		t.Fatal(e)
	}
	items := []SideOpenDualDiskPickItem{{ID: "miss", Boundary: through}, {ID: "z-floor", Boundary: blind},
		{ID: "a-floor", Boundary: blind}, {ID: "far", Boundary: blind, Translation: geom.Vec3{Z: 0.25}}}
	hit, ok, e := PickSideOpenDualDiskCuts(ctx, camera, geom.Vec2{}, items, geom.ToroidLineOptions{})
	if e != nil || !ok || hit.ID != "a-floor" || hit.Support != geom.BoxDualCircleBlindFloor || hit.Coincident {
		t.Fatal("opening should pick blind floor only, with stable tied instance", hit, ok, e)
	}
	for i, n := range []int64{5, 5, 3} {
		q := big.NewRat(n, 1)
		if hit.Point[i].Lower.Cmp(q) != 0 || hit.Point[i].Upper.Cmp(q) != 0 {
			t.Fatal("exact floor point", hit.Point)
		}
	}
	hit, ok, e = PickSideOpenDualDiskCuts(ctx, camera, geom.Vec2{}, items[:1], geom.ToroidLineOptions{})
	if e != nil || ok || hit.Parameter != nil {
		t.Fatal("through opening should miss", hit, ok, e)
	}
	// Shift both cuts by -1Y so the same represented camera ray follows
	// the exact shared circle generator instead of entering the void.
	seam := []SideOpenDualDiskPickItem{{ID: "seam", Boundary: through, Translation: geom.Vec3{Y: -1}}}
	hit, ok, e = PickSideOpenDualDiskCuts(ctx, camera, geom.Vec2{}, seam, geom.ToroidLineOptions{})
	want := geom.BoxDualCircleStockZ | geom.BoxDualCircleFirstWall | geom.BoxDualCircleSecondWall
	if e != nil || !ok || !hit.Coincident || hit.Support != want {
		t.Fatal("seam should retain both supports and entry cap", hit, ok, e)
	}
	if hit, ok, e = PickSideOpenDualDiskCuts(ctx, camera, geom.Vec2{}, append(items, items[0]), geom.ToroidLineOptions{}); !errors.Is(e, geom.ErrInvalid) || ok || hit.Parameter != nil {
		t.Fatal("duplicate IDs should discard earlier picks", hit, ok, e)
	}
}
