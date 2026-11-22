package trimmed

import (
	"context"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"testing"
)

func TestOffsetSideOpenDualDiskNativeRejects(t *testing.T) {
	ctx := context.Background()
	f, e := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	ds := [2]ExtrusionCircle{{Center: geom.Vec2{X: 12.5, Y: -7.5}, Radius: 65. / 8}, {Center: geom.Vec2{X: 22, Y: -7.5}, Radius: 87. / 8}}
	for _, depth := range []float64{0, 7} {
		r, e := CutOffsetSideOpenDualDisks(ctx, f, geom.Vec2{}, geom.Vec2{X: 40, Y: 10}, ds, 6, depth, 1e-10, trimvalidate.Options{})
		if e == nil || len(r.Solid.Faces()) != 0 {
			t.Fatal(r, e)
		}
	}
	r, e := CutOffsetSideOpenDualDisks(ctx, f, geom.Vec2{}, geom.Vec2{X: 40, Y: 10}, ds, 6, 3, 1e-10, trimvalidate.Options{MaxWork: 1})
	if e == nil || len(r.Solid.Faces()) != 0 {
		t.Fatal(r, e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	r, e = CutOffsetSideOpenDualDisks(cancelled, f, geom.Vec2{}, geom.Vec2{X: 40, Y: 10}, ds, 6, 3, 1e-10, trimvalidate.Options{})
	if e == nil || len(r.Solid.Faces()) != 0 {
		t.Fatal(r, e)
	}
}
