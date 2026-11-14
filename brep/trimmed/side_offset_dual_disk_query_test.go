package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"testing"
)

func TestOffsetSideOpenDualDiskExactPoint(t *testing.T) {
	ctx := context.Background()
	f, e := geom.NewFrame(geom.Vec3{X: 10, Y: 20, Z: 30}, geom.Vec3{Y: -1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	ds := [2]ExtrusionCircle{{Center: geom.Vec2{X: 12.5, Y: -7.5}, Radius: 65. / 8}, {Center: geom.Vec2{X: 22, Y: -7.5}, Radius: 87. / 8}}
	for _, depth := range []float64{3, 6} {
		c, e := CutOffsetSideOpenDualDisks(ctx, f, geom.Vec2{}, geom.Vec2{X: 40, Y: 10}, ds, 6, depth, 1e-10, trimvalidate.Options{})
		if e != nil {
			t.Fatal(e)
		}
		world := func(x, y, z float64) [3]*big.Rat {
			p := f.ToWorld(geom.Vec3{X: x, Y: y, Z: z})
			return [3]*big.Rat{ratInset(p.X), ratInset(p.Y), ratInset(p.Z)}
		}
		seam := geom.BoxDualCircleFirstWall | geom.BoxDualCircleSecondWall
		for _, v := range []struct {
			x, y, z float64
			loc     parametric.Location
			mask    geom.BoxDualCircleSupport
		}{{14.5, .375, 1, parametric.Boundary, seam}, {14.5, .25, 1, parametric.Outside, 0}, {14.5, .5, 1, parametric.Inside, 0}, {0, 0, 1, parametric.Boundary, geom.BoxDualCircleStockX | geom.BoxDualCircleStockY}, {75. / 8, 0, 1, parametric.Boundary, geom.BoxDualCircleStockY | geom.BoxDualCircleFirstWall}} {
			r, e := c.ClassifyRational(ctx, world(v.x, v.y, v.z))
			if e != nil || r.Location != v.loc || r.Support != v.mask {
				t.Fatal(v, r, e)
			}
		}
		// World X is local Y; sub-ULP perturbations cannot collapse to seam.
		for _, sign := range []int64{-1, 1} {
			p := world(14.5, .375, 1)
			p[0].Add(p[0], new(big.Rat).SetFrac(big.NewInt(sign), new(big.Int).Lsh(big.NewInt(1), 90)))
			r, e := c.ClassifyRational(ctx, p)
			want := parametric.Inside
			if sign < 0 {
				want = parametric.Outside
			}
			if e != nil || r.Location != want || r.Support != 0 {
				t.Fatal(sign, r, e)
			}
		}
		r, e := c.ClassifyRational(ctx, world(14.5, .25, depth))
		if e != nil {
			t.Fatal(e)
		}
		if depth == 3 {
			if r.Location != parametric.Boundary || r.Support != geom.BoxDualCircleBlindFloor {
				t.Fatal(r)
			}
		} else if r.Location != parametric.Outside || r.Support != 0 {
			t.Fatal(r)
		}
		line, err := c.IntersectLineBoundary(ctx, world(14.5, 0, 1), [3]*big.Rat{ratInset(2), ratInset(0), ratInset(0)}, ratInset(0), ratInset(1), geom.ToroidLineOptions{ParameterTolerance: 1e-10})
		if err != nil || len(line.Hits) != 1 || line.Hits[0].Support != seam || line.Hits[0].Bounds.Lower.Cmp(big.NewRat(3, 16)) != 0 {
			t.Fatal("offset line", line, err)
		}
		line, err = c.IntersectLineBoundary(ctx, world(14.5, .375, -1), [3]*big.Rat{ratInset(0), ratInset(0), ratInset(1)}, ratInset(0), ratInset(8), geom.ToroidLineOptions{ParameterTolerance: 1e-10})
		if err != nil || len(line.Intervals) != 1 || line.Intervals[0].Support != seam || line.Intervals[0].StartBounds.Lower.Cmp(ratInset(1)) != 0 || line.Intervals[0].EndBounds.Upper.Cmp(ratInset(depth+1)) != 0 {
			t.Fatal("offset generator", line, err)
		}
		// Public result topology assignment cannot replace the immutable source.
		c.Solid = Solid{}
		r, e = c.ClassifyRational(ctx, world(14.5, .375, 1))
		if e != nil || r.Support != seam {
			t.Fatal(r, e)
		}
	}
}
