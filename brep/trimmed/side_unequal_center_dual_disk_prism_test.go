package trimmed

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"testing"
)

func TestUnequalCenterSideOpenDualDiskNativeOwnership(t *testing.T) {
	ctx := context.Background()
	f, e := geom.NewFrame(geom.Vec3{}, geom.Vec3{Y: -1}, geom.Vec3{Z: -1})
	if e != nil {
		t.Fatal(e)
	}
	ds := [2]ExtrusionCircle{{Center: geom.Vec2{X: 7, Y: -3}, Radius: 5}, {Center: geom.Vec2{X: 30, Y: -20}, Radius: 29}}
	join := [2]*big.Rat{big.NewRat(10, 1), big.NewRat(1, 1)}
	cut, e := CutUnequalCenterSideOpenDualDisks(ctx, f, geom.Vec2{}, geom.Vec2{X: 60, Y: 10}, ds, join, 6, 3, 1e-10, trimvalidate.Options{})
	if e != nil {
		t.Fatal(e)
	}
	join[0].SetInt64(0)
	var stream bytes.Buffer
	if e = WriteJSON(&stream, cut.Solid); e != nil {
		t.Fatal(e)
	}
	back, e := ReadJSONContext(ctx, &stream, JSONLimits{})
	if e != nil || len(back.Faces()) != len(cut.Solid.Faces()) {
		t.Fatal(e)
	}
	for _, faces := range cut.WallFaces {
		for _, id := range faces {
			a, e := cut.Solid.Faces()[id].Surface.Eval(.5, .5)
			if e != nil {
				t.Fatal(e)
			}
			b, e := back.Faces()[id].Surface.Eval(.5, .5)
			if e != nil || a != b {
				t.Fatal("stored support changed", a, b, e)
			}
		}
	}
	bad, e := CutUnequalCenterSideOpenDualDisks(ctx, f, geom.Vec2{}, geom.Vec2{X: 60, Y: 10}, ds, join, 6, 3, 1e-10, trimvalidate.Options{})
	if e == nil || len(bad.Solid.Faces()) != 0 {
		t.Fatal("bad join published", e)
	}
}
