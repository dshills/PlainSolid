//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"testing"
)

func TestUnequalSideOpenDualDiskExactLocate(t *testing.T) {
	ctx := context.Background()
	discs := [2]ExtrusionCircle{{Center: geom.Vec2{X: 20}, Radius: 13}, {Center: geom.Vec2{X: 34}, Radius: 15}}
	for _, sign := range []float64{1, -1} {
		frame, e := geom.NewFrame(geom.Vec3{X: 2, Y: 3, Z: 4}, geom.Vec3{X: 1}, geom.Vec3{Z: sign})
		if e != nil {
			t.Fatal(e)
		}
		for _, depth := range []float64{3, 6} {
			cut, e := CutUnequalSideOpenDualDisks(ctx, frame, geom.Vec2{}, geom.Vec2{X: 60, Y: 22}, discs, 6, depth, 1e-10, trimvalidate.Options{})
			if e != nil {
				t.Fatal(e)
			}
			for _, test := range []struct {
				p    geom.Vec3
				want parametric.Location
			}{{geom.Vec3{X: 34, Y: 14, Z: 1}, parametric.Outside}, {geom.Vec3{X: 20, Y: 14, Z: 1}, parametric.Inside}, {geom.Vec3{X: 25, Y: 12, Z: 1}, parametric.Boundary}, {geom.Vec3{X: 25, Y: 11, Z: 1}, parametric.Outside}, {geom.Vec3{X: 0, Y: 11, Z: 1}, parametric.Boundary}} {
				got, e := cut.Locate(ctx, frame.ToWorld(test.p))
				if e != nil || got != test.want {
					t.Fatal(sign, depth, test, got, e)
				}
			}
			// The exact two-circle junction is not widened by any proximity epsilon.
			for _, side := range []int64{-1, 1} {
				y := new(big.Rat).Add(big.NewRat(12, 1), new(big.Rat).SetFrac(big.NewInt(side), new(big.Int).Lsh(big.NewInt(1), 80)))
				world := [3]*big.Rat{big.NewRat(27, 1), new(big.Rat).Add(big.NewRat(3, 1), new(big.Rat).Mul(ratInset(sign), y)), new(big.Rat).Add(big.NewRat(4, 1), ratInset(sign))}
				want := parametric.Inside
				if side < 0 {
					want = parametric.Outside
				}
				got, e := cut.LocateRational(ctx, world)
				if e != nil || got != want {
					t.Fatal(got, e)
				}
			}
			cap := parametric.Boundary
			if depth == 6 {
				cap = parametric.Outside
			}
			got, e := cut.Locate(ctx, frame.ToWorld(geom.Vec3{X: 25, Y: 11, Z: depth}))
			if e != nil || got != cap {
				t.Fatal(got, e)
			}
		}
	}
	if _, e := (UnequalSideOpenDualDiskCut{}).Measures(); e == nil {
		t.Fatal("unbound measures")
	}
}
