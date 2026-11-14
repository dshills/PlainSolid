package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func TestSideOpenDualDiskExactLineBoundary(t *testing.T) {
	ctx := context.Background()
	frame, e := geom.NewFrame(geom.Vec3{X: 20, Y: 30, Z: 40}, geom.Vec3{Y: -1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	p := func(x, y, z float64) [3]*big.Rat { return [3]*big.Rat{ratInset(x), ratInset(y), ratInset(z)} }
	point := func(x, y, z float64) [3]*big.Rat {
		v := frame.ToWorld(geom.Vec3{X: x, Y: y, Z: z})
		return p(v.X, v.Y, v.Z)
	}
	discs := [2]ExtrusionCircle{{Center: geom.Vec2{X: 3.5, Y: 4}, Radius: 2.5}, {Center: geom.Vec2{X: 6.5, Y: 4}, Radius: 2.5}}
	o := geom.ToroidLineOptions{ParameterTolerance: 1e-10}
	for _, depth := range []float64{3, 6} {
		c, e := CutSideOpenDualDisks(ctx, frame, geom.Vec2{Y: 4}, geom.Vec2{X: 10, Y: 10}, discs, 6, depth, 1e-10, trimvalidate.Options{})
		if e != nil {
			t.Fatal(e)
		}
		line, e := c.IntersectLineBoundary(ctx, point(5, 6, -1), p(0, 0, 1), ratInset(0), ratInset(8), o)
		wall := geom.BoxDualCircleFirstWall | geom.BoxDualCircleSecondWall
		var stop = geom.BoxDualCircleStockZ
		if depth < 6 {
			stop = geom.BoxDualCircleBlindFloor
		}
		if e != nil || len(line.Intervals) != 1 || line.Intervals[0].Support != wall ||
			line.Intervals[0].StartBounds.Lower.Cmp(ratInset(1)) != 0 || line.Intervals[0].EndBounds.Upper.Cmp(ratInset(depth+1)) != 0 ||
			line.Intervals[0].StartSupport != wall|geom.BoxDualCircleStockZ || line.Intervals[0].EndSupport != wall|stop {
			t.Fatal("shared cylindrical generator and its end incidences", depth, line, e)
		}
		if depth == 6 && len(line.Hits) != 0 || depth == 3 && (len(line.Hits) != 1 || line.Hits[0].Support != geom.BoxDualCircleStockZ) {
			t.Fatal("seam interval duplicated as isolated hits", depth, line)
		}
		// This line starts strictly in material and crosses the entire removed
		// overlap. The individual-circle roots hidden by the union do not appear.
		line, e = c.IntersectLineBoundary(ctx, point(0.5, 5, 1), p(0, -2, 0), ratInset(0), ratInset(4.5), o)
		if e != nil || len(line.Intervals) != 0 || len(line.Hits) != 2 ||
			line.Hits[0].Support != geom.BoxDualCircleFirstWall || line.Hits[1].Support != geom.BoxDualCircleSecondWall {
			t.Fatal("hidden roots or artificial clip endpoints surfaced", depth, line, e)
		}
		for _, hit := range line.Hits {
			if hit.Bounds.Lower.Cmp(hit.Bounds.Upper) >= 0 || hit.Parameter == nil || hit.Multiplicity != 1 {
				t.Fatal("nonbinary exact transverse parameter not retained", hit)
			}
		}
		line, e = c.IntersectLineBoundary(ctx, point(0, 6.5, 1), p(0, -1, 0), ratInset(1), ratInset(9), o)
		if e != nil || len(line.Hits) != 2 || len(line.Intervals) != 0 || line.Hits[0].Multiplicity != 2 || line.Hits[1].Multiplicity != 2 {
			t.Fatal("tangent arc tips not retained", depth, line, e)
		}
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		if line, e = c.IntersectLineBoundary(cancelled, point(0, 5, 1), p(0, -1, 0), ratInset(0), ratInset(10), o); !errors.Is(e, context.Canceled) || len(line.Hits) != 0 || len(line.Intervals) != 0 {
			t.Fatal("cancel must discard all hits", line, e)
		}
		small := o
		small.Surface.MaxOperations = 1
		if line, e = c.IntersectLineBoundary(ctx, point(0, 5, 1), p(0, -1, 0), ratInset(0), ratInset(10), small); e == nil || len(line.Hits) != 0 || len(line.Intervals) != 0 {
			t.Fatal("exhaustion must discard all hits", line, e)
		}
	}
}

func TestUnequalSideOpenDualDiskExactLineBoundary(t *testing.T) {
	ctx := context.Background()
	frame, e := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	discs := [2]ExtrusionCircle{{Center: geom.Vec2{X: 34}, Radius: 15}, {Center: geom.Vec2{X: 20}, Radius: 13}}
	c, e := CutUnequalSideOpenDualDisks(ctx, frame, geom.Vec2{}, geom.Vec2{X: 60, Y: 22}, discs, 6, 3, 1e-10, trimvalidate.Options{})
	if e != nil {
		t.Fatal(e)
	}
	x := big.NewRat(25, 1)
	line, e := c.IntersectLineBoundary(ctx, [3]*big.Rat{x, new(big.Rat), big.NewRat(1, 1)}, [3]*big.Rat{new(big.Rat), big.NewRat(1, 1), new(big.Rat)},
		big.NewRat(0, 1), big.NewRat(22, 1), geom.ToroidLineOptions{ParameterTolerance: 1e-10})
	if e != nil || len(line.Intervals) != 0 || len(line.Hits) != 2 ||
		line.Hits[0].Support != geom.BoxDualCircleFirstWall|geom.BoxDualCircleSecondWall ||
		line.Hits[1].Support != geom.BoxDualCircleStockY {
		t.Fatal("unequal radical seam or exterior stock wall lost", line, e)
	}
	root, _, e := geom.NewExactRationalRoot(ctx, big.NewRat(12, 1), geom.ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	order, e := line.Hits[0].Parameter.Compare(ctx, root, geom.ExactCurvePlaneEventOptions{})
	if e != nil || !order.Proven || order.Order != 0 {
		t.Fatal("unequal circle/circle root did not compare exactly with ordinate 12", order, e)
	}
}
