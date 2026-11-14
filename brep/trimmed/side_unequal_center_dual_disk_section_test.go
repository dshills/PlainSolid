package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"testing"
)

func TestUnequalCenterSideOpenDualDiskSection(t *testing.T) {
	ctx := context.Background()
	for _, swap := range []bool{false, true} {
		ds := [2]ExtrusionCircle{{Center: geom.Vec2{X: 7, Y: -3}, Radius: 5}, {Center: geom.Vec2{X: 30, Y: -20}, Radius: 29}}
		if swap {
			ds[0], ds[1] = ds[1], ds[0]
		}
		r, e := CertifyUnequalCenterSideOpenDualDiskSection(ctx, geom.Vec2{}, geom.Vec2{X: 60, Y: 10}, ds, [2]*big.Rat{big.NewRat(10, 1), big.NewRat(1, 1)}, trimvalidate.Options{})
		if e != nil || r.Join[0].Cmp(big.NewRat(10, 1)) != 0 || r.Join[1].Cmp(big.NewRat(1, 1)) != 0 {
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
		area, e := c.BoundSignedArea(ctx, big.NewRat(1, 100))
		if e != nil {
			t.Fatal(e)
		}
		removed := 216.5*math.Pi - 409
		lo, _ := area.Lower.Float64()
		hi, _ := area.Upper.Float64()
		if lo > 600-removed || hi < 600-removed {
			t.Fatalf("area %g %g want%g", lo, hi, 600-removed)
		}

	}
}

func TestUnequalCenterSideOpenDualDiskJoinProof(t *testing.T) {
	ctx := context.Background()
	ds := [2]ExtrusionCircle{{Center: geom.Vec2{X: 7, Y: -3}, Radius: 5}, {Center: geom.Vec2{X: 30, Y: -20}, Radius: 29}}
	join := [2]*big.Rat{big.NewRat(10, 1), big.NewRat(1, 1)}
	r, e := CertifyUnequalCenterSideOpenDualDiskSection(ctx, geom.Vec2{}, geom.Vec2{X: 60, Y: 10}, ds, join, trimvalidate.Options{})
	if e != nil {
		t.Fatal(e)
	}
	if r.LowerJoin[0].Cmp(big.NewRat(1659, 409)) != 0 || r.LowerJoin[1].Cmp(big.NewRat(-2880, 409)) != 0 {
		t.Fatal(r.LowerJoin)
	}
	for _, d := range ds {
		dx := new(big.Rat).Sub(r.LowerJoin[0], ratInset(d.Center.X))
		dy := new(big.Rat).Sub(r.LowerJoin[1], ratInset(d.Center.Y))
		radius := ratInset(d.Radius)
		if new(big.Rat).Add(new(big.Rat).Mul(dx, dx), new(big.Rat).Mul(dy, dy)).Cmp(new(big.Rat).Mul(radius, radius)) != 0 {
			t.Fatal("excluded root off support")
		}
	}
	join[0].SetInt64(99)
	if r.Join[0].Cmp(big.NewRat(10, 1)) != 0 {
		t.Fatal("proposal alias")
	}
	for _, p := range [][2]*big.Rat{{big.NewRat(10, 1), new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 80)))}, r.LowerJoin, {nil, big.NewRat(1, 1)}} {
		bad, e := CertifyUnequalCenterSideOpenDualDiskSection(ctx, geom.Vec2{}, geom.Vec2{X: 60, Y: 10}, ds, p, trimvalidate.Options{})
		if e == nil || len(bad.Outer) != 0 {
			t.Fatal(bad, e)
		}
	}
	for _, opts := range []trimvalidate.Options{{MaxWork: 1}, {MaxBits: 8}} {
		bad, e := CertifyUnequalCenterSideOpenDualDiskSection(ctx, geom.Vec2{}, geom.Vec2{X: 60, Y: 10}, ds, [2]*big.Rat{big.NewRat(10, 1), big.NewRat(1, 1)}, opts)
		if e == nil || len(bad.Outer) != 0 {
			t.Fatal(bad, e)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	bad, e := CertifyUnequalCenterSideOpenDualDiskSection(cancelled, geom.Vec2{}, geom.Vec2{X: 60, Y: 10}, ds, [2]*big.Rat{big.NewRat(10, 1), big.NewRat(1, 1)}, trimvalidate.Options{})
	if e == nil || len(bad.Outer) != 0 {
		t.Fatal(bad, e)
	}
}
