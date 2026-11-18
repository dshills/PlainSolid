package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestGeneralSideOpenDiskSectionBeyondCenters(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for _, reverseRadii := range []bool{false, true} {
		for _, swap := range []bool{false, true} {
			ds := [2]ExtrusionCircle{{Center: geom.Vec2{X: 20}, Radius: 13}, {Center: geom.Vec2{X: 24}, Radius: 15}}
			join := int64(15)
			if reverseRadii {
				ds[0].Radius, ds[1].Radius = 15, 13
				join = 29
			}
			if swap {
				ds[0], ds[1] = ds[1], ds[0]
			}
			r, e := CertifyGeneralSideOpenDualDiskSection(ctx, geom.Vec2{}, geom.Vec2{X: 60, Y: 22}, ds, trimvalidate.Options{})
			if e != nil || len(r.Outer) != 8 || r.Join[0].Cmp(big.NewRat(join, 1)) != 0 || r.Join[1].Cmp(big.NewRat(12, 1)) != 0 {
				t.Fatal(r, e)
			}
			var loop trimvalidate.Loop
			counts := [2]int{}
			for i, q := range r.Outer {
				segment, e := trimvalidate.ExactQuadratic(q.HomogeneousControls())
				if e != nil {
					t.Fatal(e)
				}
				loop = append(loop, segment)
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
				for _, id := range []int{owner, 1 - owner} {
					d := ds[id]
					rad := ratInset(d.Radius)
					proof, e := cell.ClassifyPlanarCircle(ctx, [2]int{0, 1}, [3]*big.Rat{ratInset(d.Center.X), ratInset(d.Center.Y), new(big.Rat)}, new(big.Rat).Mul(rad, rad), geom.ExactCurvePlaneEventOptions{})
					if e != nil || !proof.Complete {
						t.Fatal(proof, e)
					}
					if id == owner {
						if !proof.Coincident {
							t.Fatal("source support not identical")
						}
					} else {
						if proof.Coincident {
							t.Fatal("wrong supporting circle")
						}
						for _, interval := range proof.Intervals {
							if interval.Sign != 1 {
								t.Fatal("hidden arc included", interval)
							}
						}
					}
				}
			}
			if counts[0]+counts[1] != 3 || counts[0] == 0 || counts[1] == 0 {
				t.Fatal(counts)
			}
			certificate, proof, e := trimvalidate.CertifyLoops(ctx, loop, nil, trimvalidate.Options{})
			if e != nil || proof.Status != trimvalidate.Certified {
				t.Fatal(proof, e)
			}
			area, e := certificate.BoundSignedArea(ctx, big.NewRat(1, 1000))
			if e != nil {
				t.Fatal(e)
			}
			removed := 24 + (169*(math.Pi/2-math.Asin(5./13))+225*(math.Pi/2+math.Asin(9./15)))/2
			lo, _ := area.Lower.Float64()
			hi, _ := area.Upper.Float64()
			if lo > 1320-removed || hi < 1320-removed {
				t.Fatal(lo, hi, 1320-removed)
			}
			old, e := CertifyUnequalSideOpenDualDiskSection(ctx, geom.Vec2{}, geom.Vec2{X: 60, Y: 22}, ds, trimvalidate.Options{})
			if e == nil || len(old.Outer) != 0 {
				t.Fatal("native band gate broadened")
			}
			limited, e := CertifyGeneralSideOpenDualDiskSection(ctx, geom.Vec2{}, geom.Vec2{X: 60, Y: 22}, ds, trimvalidate.Options{MaxWork: 1})
			if e == nil || len(limited.Outer) != 0 {
				t.Fatal(limited, e)
			}
		}
	}
}

func TestGeneralSideOpenDiskSectionExtremumAndRejects(t *testing.T) {
	ctx := context.Background()
	ds := [2]ExtrusionCircle{{Center: geom.Vec2{X: 10}, Radius: 3}, {Center: geom.Vec2{X: 14}, Radius: 5}}
	r, e := CertifyGeneralSideOpenDualDiskSection(ctx, geom.Vec2{}, geom.Vec2{X: 30, Y: 10}, ds, trimvalidate.Options{})
	if e != nil || r.Join[0].Cmp(big.NewRat(10, 1)) != 0 || r.Join[1].Cmp(big.NewRat(3, 1)) != 0 || len(r.Outer) != 8 {
		t.Fatal(r, e)
	}
	for _, x := range []float64{12, 18, 10, 13} {
		bad := ds
		bad[1].Center.X = x
		r, e := CertifyGeneralSideOpenDualDiskSection(ctx, geom.Vec2{}, geom.Vec2{X: 30, Y: 10}, bad, trimvalidate.Options{})
		if e == nil || len(r.Outer) != 0 {
			t.Fatal(x, r, e)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	r, e = CertifyGeneralSideOpenDualDiskSection(cancelled, geom.Vec2{}, geom.Vec2{X: 30, Y: 10}, ds, trimvalidate.Options{})
	if e == nil || len(r.Outer) != 0 {
		t.Fatal(r, e)
	}
}
