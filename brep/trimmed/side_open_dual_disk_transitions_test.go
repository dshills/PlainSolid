//go:build research

package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"testing"
)

func TestSideOpenDualDiskTransitionsWorld(t *testing.T) {
	ctx := context.Background()
	frame, e := geom.NewFrame(geom.Vec3{X: 20, Y: 30, Z: 40}, geom.Vec3{Y: -1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	type query func(context.Context, [3]*big.Rat, [3]*big.Rat, *big.Rat, *big.Rat, geom.ToroidLineOptions) (geom.BoxDualCircleBoundaryTransitions, error)
	for _, kind := range []string{"equal", "unequal", "general"} {
		t.Run(kind, func(t *testing.T) {
			var run query
			var joinX, joinY float64
			switch kind {
			case "equal":
				c, err := CutSideOpenDualDisks(ctx, frame, geom.Vec2{Y: 4}, geom.Vec2{X: 10, Y: 10}, [2]ExtrusionCircle{{Center: geom.Vec2{X: 3.5, Y: 4}, Radius: 2.5}, {Center: geom.Vec2{X: 6.5, Y: 4}, Radius: 2.5}}, 6, 3, 1e-10, trimvalidate.Options{})
				if err != nil {
					t.Fatal(err)
				}
				run = c.IntersectLineBoundaryTransitions
				joinX, joinY = 5, 6
			case "unequal":
				c, err := CutUnequalSideOpenDualDisks(ctx, frame, geom.Vec2{}, geom.Vec2{X: 60, Y: 22}, [2]ExtrusionCircle{{Center: geom.Vec2{X: 20}, Radius: 13}, {Center: geom.Vec2{X: 34}, Radius: 15}}, 6, 3, 1e-10, trimvalidate.Options{})
				if err != nil {
					t.Fatal(err)
				}
				run = c.IntersectLineBoundaryTransitions
				joinX, joinY = 25, 12
			default:
				c, err := CutGeneralSideOpenDualDisks(ctx, frame, geom.Vec2{}, geom.Vec2{X: 60, Y: 22}, [2]ExtrusionCircle{{Center: geom.Vec2{X: 20}, Radius: 5}, {Center: geom.Vec2{X: 24.5}, Radius: 8.5}}, 6, 3, 1e-10, trimvalidate.Options{})
				if err != nil {
					t.Fatal(err)
				}
				run = c.IntersectLineBoundaryTransitions
				joinX, joinY = 17, 4
			}
			// World X advances local Y. Place origin exactly 1/3 below the seam,
			// and use speed 2/3: exact event parameter must be 1/2, not rounded.
			p := frame.ToWorld(geom.Vec3{X: joinX, Y: joinY, Z: 1})
			org := [3]*big.Rat{new(big.Rat).Sub(ratInset(p.X), big.NewRat(1, 3)), ratInset(p.Y), ratInset(p.Z)}
			dir := [3]*big.Rat{big.NewRat(2, 3), new(big.Rat), new(big.Rat)}
			opts := geom.ToroidLineOptions{ParameterTolerance: 1e-10}
			r, err := run(ctx, org, dir, big.NewRat(0, 1), big.NewRat(1, 1), opts)
			if err != nil || len(r.Events) != 1 {
				t.Fatalf("events %+v %v", r, err)
			}
			ev := r.Events[0]
			if ev.Parameter.Interval().Lower.Cmp(big.NewRat(1, 2)) != 0 || ev.Support != geom.BoxDualCircleFirstWall|geom.BoxDualCircleSecondWall || !ev.Before.Known || !ev.After.Known || ev.Before.Location != geom.BoreOutside || ev.After.Location != geom.BoreInside {
				t.Fatalf("event %+v", ev)
			}
			// Reverse the exact same physical segment, preserving both ownership bits.
			org[0].Add(org[0], dir[0])
			dir[0].Neg(dir[0])
			r, err = run(ctx, org, dir, big.NewRat(0, 1), big.NewRat(1, 1), opts)
			if err != nil || len(r.Events) != 1 || r.Events[0].Before.Location != geom.BoreInside || r.Events[0].After.Location != geom.BoreOutside {
				t.Fatalf("reverse %+v %v", r, err)
			}
			cancelled, cancel := context.WithCancel(ctx)
			cancel()
			r, err = run(cancelled, org, dir, big.NewRat(0, 1), big.NewRat(1, 1), opts)
			if !errors.Is(err, context.Canceled) || len(r.Events) != 0 {
				t.Fatalf("cancel %+v %v", r, err)
			}
			opts.Surface.MaxOperations = 1
			r, err = run(ctx, org, dir, big.NewRat(0, 1), big.NewRat(1, 1), opts)
			if err == nil || len(r.Events) != 0 || len(r.Boundary.Hits) != 0 {
				t.Fatalf("budget %+v %v", r, err)
			}
		})
	}
}
