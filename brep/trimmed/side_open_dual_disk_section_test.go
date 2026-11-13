package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func TestExactSameSideDualDiskSection(t *testing.T) {
	ctx := context.Background()
	min, max := geom.Vec2{Y: 4}, geom.Vec2{X: 10, Y: 10}
	discs := [2]ExtrusionCircle{{Center: geom.Vec2{X: 6.5, Y: 4}, Radius: 2.5}, {Center: geom.Vec2{X: 3.5, Y: 4}, Radius: 2.5}}
	r, e := CertifySideOpenDualDiskSection(ctx, min, max, discs, trimvalidate.Options{})
	if e != nil || len(r.Outer) != 9 || r.Join[0].Cmp(big.NewRat(5, 1)) != 0 || r.Join[1].Cmp(big.NewRat(6, 1)) != 0 {
		t.Fatal(r, e)
	}
	var loop trimvalidate.Loop
	for i, q := range r.Outer {
		a, e := trimvalidate.ExactQuadratic(q.HomogeneousControls())
		if e != nil {
			t.Fatal(e)
		}
		loop = append(loop, a)
		if id := r.Owner[i]; id >= 0 {
			for j := int64(1); j < 11; j++ {
				p, _, e := q.EvaluateRational(big.NewRat(j, 11))
				if e != nil {
					t.Fatal(e)
				}
				d2 := func(c ExtrusionCircle) *big.Rat {
					x := new(big.Rat).Sub(p[0], ratInset(c.Center.X))
					y := new(big.Rat).Sub(p[1], ratInset(c.Center.Y))
					return new(big.Rat).Add(new(big.Rat).Mul(x, x), new(big.Rat).Mul(y, y))
				}
				r2 := big.NewRat(25, 4)
				if d2(discs[id]).Cmp(r2) != 0 || d2(discs[1-id]).Cmp(r2) <= 0 || p[1].Cmp(big.NewRat(4, 1)) <= 0 {
					t.Fatal("wrong exact exposed circular arc", i, p)
				}
			}
		}
	}
	c, proof, e := trimvalidate.CertifyLoops(ctx, loop, nil, trimvalidate.Options{})
	if e != nil || proof.Status != trimvalidate.Certified {
		t.Fatal(proof, e)
	}
	a, e := c.BoundSignedArea(ctx, big.NewRat(1, 1000))
	if e != nil {
		t.Fatal(e)
	}
	l, _ := a.Lower.Float64()
	u, _ := a.Upper.Float64()
	// Independent two overlapping disks, cut in half by their centerline.
	want := 60 - (25./4*(math.Pi-math.Acos(3./5)) + 3)
	if want < l || u < want {
		t.Fatal("remaining section area", l, u, want)
	}
	bad := discs
	bad[0].Center.X = 6
	if r, e := CertifySideOpenDualDiskSection(ctx, min, max, bad, trimvalidate.Options{}); !errors.Is(e, geom.ErrUnrepresentable) || len(r.Outer) != 0 {
		t.Fatal("irrational circle crossing rounded", r, e)
	}
}
