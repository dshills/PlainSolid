package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"testing"
)

func TestOffsetSideOpenDualDiskSection(t *testing.T) {
	ctx := context.Background()
	for _, swap := range []bool{false, true} {
		ds := [2]ExtrusionCircle{{Center: geom.Vec2{X: 12.5, Y: -7.5}, Radius: 65. / 8}, {Center: geom.Vec2{X: 22, Y: -7.5}, Radius: 87. / 8}}
		if swap {
			ds[0], ds[1] = ds[1], ds[0]
		}
		r, e := CertifyOffsetSideOpenDualDiskSection(ctx, geom.Vec2{}, geom.Vec2{X: 40, Y: 10}, ds, trimvalidate.Options{})
		if e != nil || r.Join[0].Cmp(big.NewRat(29, 2)) != 0 || r.Join[1].Cmp(big.NewRat(3, 8)) != 0 {
			t.Fatalf("section %+v %v", r, e)
		}
		var loop trimvalidate.Loop
		counts := [2]int{}
		for i, q := range r.Outer {
			seg, e := trimvalidate.ExactQuadratic(q.HomogeneousControls())
			if e != nil {
				t.Fatal(e)
			}
			loop = append(loop, seg)
			owner := r.Owner[i]
			if owner < 0 {
				continue
			}
			counts[owner]++
			h := q.HomogeneousControls()
			net := make([][4]*big.Rat, 3)
			for j := range h {
				net[j] = [4]*big.Rat{h[j][0], h[j][1], new(big.Rat), h[j][2]}
			}
			curve, e := geom.NewExactHomogeneousBezier(ctx, net, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			cell, e := geom.NewExactComposedBezierCell(ctx, curve, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			for id, d := range ds {
				rad := ratInset(d.Radius)
				p, e := cell.ClassifyPlanarCircle(ctx, [2]int{0, 1}, [3]*big.Rat{ratInset(d.Center.X), ratInset(d.Center.Y), new(big.Rat)}, new(big.Rat).Mul(rad, rad), geom.ExactCurvePlaneEventOptions{})
				if e != nil || !p.Complete {
					t.Fatal(p, e)
				}
				if id == owner {
					if !p.Coincident {
						t.Fatal("lost circle")
					}
				} else {
					if p.Coincident {
						t.Fatal("wrong circle")
					}
					for _, iv := range p.Intervals {
						if iv.Sign != 1 {
							t.Fatal("hidden arc", iv)
						}
					}
				}
			}
		}
		if counts[0] != 2 || counts[1] != 2 {
			t.Fatal(counts)
		}
		c, proof, e := trimvalidate.CertifyLoops(ctx, loop, nil, trimvalidate.Options{})
		if e != nil || proof.Status != trimvalidate.Certified {
			t.Fatal(proof, e)
		}
		area, e := c.BoundSignedArea(ctx, big.NewRat(1, 10000))
		if e != nil {
			t.Fatal(e)
		}
		// Independent analytic integral of cy+sqrt(r²-(x-cx)²) on each exposed interval.
		primitive := func(x, r float64) float64 { return (x*math.Sqrt(r*r-x*x) + r*r*math.Asin(x/r)) / 2 }
		removed := -7.5*(239./8-75./8) + primitive(2, 65./8) - primitive(-25./8, 65./8) + primitive(63./8, 87./8) - primitive(-7.5, 87./8)
		lo, _ := area.Lower.Float64()
		hi, _ := area.Upper.Float64()
		if lo > 400-removed || hi < 400-removed {
			t.Fatalf("area %g %g want%g", lo, hi, 400-removed)
		}
		if _, e = CertifyGeneralSideOpenDualDiskSection(ctx, geom.Vec2{}, geom.Vec2{X: 40, Y: 10}, ds, trimvalidate.Options{}); e == nil {
			t.Fatal("legacy domain widened")
		}
	}
}
func TestOffsetSideOpenDualDiskSectionRejects(t *testing.T) {
	ds := [2]ExtrusionCircle{{Center: geom.Vec2{X: 12.5, Y: -7.5}, Radius: 65. / 8}, {Center: geom.Vec2{X: 22, Y: -7.5}, Radius: 87. / 8}}
	for _, change := range []func(*[2]ExtrusionCircle){
		func(d *[2]ExtrusionCircle) { d[1].Center.Y = -7 },
		func(d *[2]ExtrusionCircle) { d[0].Center.Y = 0; d[1].Center.Y = 0 },
		func(d *[2]ExtrusionCircle) { d[0].Center.Y = -8; d[1].Center.Y = -8 },
		func(d *[2]ExtrusionCircle) { d[1].Center.X = 23.5 }, // side footprints touch
		func(d *[2]ExtrusionCircle) { d[1].Center.X = 14 },   // containment
		func(d *[2]ExtrusionCircle) { d[0].Radius = 8 },      // irrational side intercept
	} {
		bad := ds
		change(&bad)
		r, e := CertifyOffsetSideOpenDualDiskSection(context.Background(), geom.Vec2{}, geom.Vec2{X: 40, Y: 10}, bad, trimvalidate.Options{})
		if e == nil || len(r.Outer) != 0 {
			t.Fatal(r, e)
		}
	}
	r, e := CertifyOffsetSideOpenDualDiskSection(context.Background(), geom.Vec2{}, geom.Vec2{X: 40, Y: 10}, ds, trimvalidate.Options{MaxWork: 1})
	if e == nil || len(r.Outer) != 0 {
		t.Fatal(r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, e = CertifyOffsetSideOpenDualDiskSection(ctx, geom.Vec2{}, geom.Vec2{X: 40, Y: 10}, ds, trimvalidate.Options{})
	if e == nil || len(r.Outer) != 0 {
		t.Fatal(r, e)
	}
}
