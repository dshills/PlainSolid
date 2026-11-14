package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"testing"
)

func TestSideOpenDualDiskFirstHitWorldNormals(t *testing.T) {
	ctx := context.Background()
	frame, e := geom.NewFrame(geom.Vec3{X: 20, Y: 30, Z: 40}, geom.Vec3{Y: -1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	c, e := CutSideOpenDualDisks(ctx, frame, geom.Vec2{Y: 4}, geom.Vec2{X: 10, Y: 10}, [2]ExtrusionCircle{{Center: geom.Vec2{X: 3.5, Y: 4}, Radius: 2.5}, {Center: geom.Vec2{X: 6.5, Y: 4}, Radius: 2.5}}, 6, 3, 1e-10, trimvalidate.Options{})
	if e != nil {
		t.Fatal(e)
	}
	v := frame.ToWorld(geom.Vec3{X: 5, Y: 4, Z: 1})
	org := [3]*big.Rat{ratInset(v.X), ratInset(v.Y), ratInset(v.Z)}
	// Local +2Y is world +2X in this signed coordinate frame.
	dir := [3]*big.Rat{ratInset(2), ratInset(0), ratInset(0)}
	h, e := c.FirstBoundaryHit(ctx, org, dir, ratInset(0), ratInset(3), geom.ToroidLineOptions{ParameterTolerance: 1e-10})
	if e != nil || !h.HasHit || len(h.Normals) != 2 {
		t.Fatalf("hit %+v %v", h, e)
	}
	for i, n := range h.Normals {
		expected := 1.5
		if i == 1 {
			expected = -1.5
		}
		if n.Constant[1].Cmp(ratInset(expected)) != 0 || n.Linear[0].Cmp(ratInset(-2)) != 0 || n.Constant[2].Sign() != 0 || n.DirectionSign != -1 {
			t.Fatalf("normal %d %+v", i, n)
		}
	}
	p := frame.ToWorld(geom.Vec3{X: 5, Y: 6, Z: 1})
	for j, x := range []float64{p.X, p.Y, p.Z} {
		if h.PointBounds[j].Lower.Cmp(ratInset(x)) != 0 || h.PointBounds[j].Upper.Cmp(ratInset(x)) != 0 {
			t.Fatalf("point %+v", h.PointBounds)
		}
	}
}

func TestUnequalGeneralSideOpenDualDiskFirstHit(t *testing.T) {
	ctx := context.Background()
	frame, e := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	for _, general := range []bool{false, true} {
		discs := [2]ExtrusionCircle{{Center: geom.Vec2{X: 20}, Radius: 13}, {Center: geom.Vec2{X: 34}, Radius: 15}}
		joinX, joinY := 25., 12.
		var c UnequalSideOpenDualDiskCut
		if general {
			discs = [2]ExtrusionCircle{{Center: geom.Vec2{X: 20}, Radius: 5}, {Center: geom.Vec2{X: 24.5}, Radius: 8.5}}
			joinX, joinY = 17, 4
			g, err := CutGeneralSideOpenDualDisks(ctx, frame, geom.Vec2{}, geom.Vec2{X: 60, Y: 22}, discs, 6, 3, 1e-10, trimvalidate.Options{})
			e = err
			c = g.UnequalSideOpenDualDiskCut
		} else {
			c, e = CutUnequalSideOpenDualDisks(ctx, frame, geom.Vec2{}, geom.Vec2{X: 60, Y: 22}, discs, 6, 3, 1e-10, trimvalidate.Options{})
		}
		if e != nil {
			t.Fatal(e)
		}
		h, err := c.FirstBoundaryHit(ctx, [3]*big.Rat{ratInset(joinX), ratInset(0), ratInset(1)}, [3]*big.Rat{ratInset(0), ratInset(2), ratInset(0)}, ratInset(0), ratInset(10), geom.ToroidLineOptions{ParameterTolerance: 1e-10})
		if err != nil || len(h.Normals) != 2 || h.Bounds.Lower.Cmp(ratInset(joinY/2)) != 0 {
			t.Fatalf("general=%v: %+v %v", general, h, err)
		}
	}
}
