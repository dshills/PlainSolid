package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func TestSideOpenDualDiskExactAxialStations(t *testing.T) {
	ctx := context.Background()
	frame, err := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	// Input order is opposite the increasing-X boundary order.
	discs := [2]ExtrusionCircle{{Center: geom.Vec2{X: 6.5, Y: 4}, Radius: 2.5}, {Center: geom.Vec2{X: 3.5, Y: 4}, Radius: 2.5}}
	for _, depth := range []float64{3, 6} {
		c, err := CutSideOpenDualDisks(ctx, frame, geom.Vec2{Y: 4}, geom.Vec2{X: 10, Y: 10}, discs, 6, depth, 1e-10, trimvalidate.Options{})
		if err != nil {
			t.Fatal(err)
		}
		tiny := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 120))
		stations := []struct {
			z                     *big.Rat
			empty, floor, notched bool
		}{
			{big.NewRat(-1, 1), true, false, false},
			{new(big.Rat), false, false, true},
			{new(big.Rat).Sub(ratInset(depth), tiny), false, false, true},
			{ratInset(depth), false, depth == 3, depth == 6},
			{ratInset(6), false, false, depth == 6},
			{new(big.Rat).Add(ratInset(6), tiny), true, false, false},
		}
		for _, p := range stations {
			section, e := c.SectionAtStation(ctx, p.z)
			if e != nil || p.empty != section.Empty || p.floor != section.Floor || len(section.Outer) != len(section.CutterOwner) {
				t.Fatal("incorrect station metadata", depth, p, section, e)
			}
			if p.empty {
				if len(section.Outer) != 0 {
					t.Fatal("empty station emitted boundaries")
				}
				continue
			}
			var owners []int
			var loop trimvalidate.Loop
			for i, q := range section.Outer {
				piece, e := trimvalidate.ExactQuadratic(q.HomogeneousControls())
				if e != nil {
					t.Fatal(e)
				}
				loop = append(loop, piece)
				if section.CutterOwner[i] >= 0 {
					owners = append(owners, section.CutterOwner[i])
				}
			}
			if p.notched {
				if len(owners) != 2 || owners[0] != 1 || owners[1] != 0 {
					t.Fatal("wrong input cutter ancestry", owners)
				}
			} else if len(owners) != 0 || len(loop) != 4 {
				t.Fatal("intact rectangle section changed", owners)
			}
			check := [2]*big.Rat{big.NewRat(5, 1), big.NewRat(5, 1)}
			inSection, proof, e := trimvalidate.LocateRationalInLoops(ctx, loop, nil, check, trimvalidate.Options{})
			inSolid, qe := c.LocateRational(ctx, [3]*big.Rat{check[0], check[1], p.z})
			if e != nil || qe != nil || proof.Status != trimvalidate.Certified || (inSection == trimvalidate.Outside) != (inSolid == parametric.Outside) {
				t.Fatal("section and actual material disagree", depth, p.z, inSection, inSolid, e, qe)
			}
		}
		original, _ := c.SectionAtStation(ctx, new(big.Rat))
		original.CutterOwner[1] = -1
		original.Outer[1] = ExactQuadraticUV{}
		again, _ := c.SectionAtStation(ctx, new(big.Rat))
		if again.CutterOwner[1] != 1 {
			t.Fatal("caller mutation changed the original section")
		}
		a, _ := again.Outer[1].Endpoints()
		if a[1][0].Cmp(big.NewRat(5, 1)) != 0 || a[1][1].Cmp(big.NewRat(6, 1)) != 0 {
			t.Fatal("exact seam was not retained", a)
		}
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		if p, e := c.SectionAtStation(cancelled, new(big.Rat)); !errors.Is(e, context.Canceled) || len(p.Outer) != 0 {
			t.Fatal("cancel did not discard station", p, e)
		}
	}
}
