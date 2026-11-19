//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestCoincidentConvexHoleMaterial(t *testing.T) {
	a, b := coincidenceFaceFixture(t, false, false, false)
	b.faces[0].Surface = a.faces[0].Surface
	a.faces[0].Outer = orthogonalOverlapLoop([]geom.Vec2{{}, {X: 1}, {X: 1, Y: 1}, {Y: 1}}, 10)
	b.faces[0].Outer = append([]Use(nil), a.faces[0].Outer...)
	a.faces[0].Holes = [][]Use{orthogonalOverlapLoop([]geom.Vec2{{X: .5, Y: .25}, {X: .25, Y: .5}, {X: .5, Y: .75}, {X: .75, Y: .5}}, 20)}
	b.faces[0].Holes = nil
	r, e := a.IntersectCoincidentConvexHoleMaterial(context.Background(), 0, b, 0, CoincidentConvexHoleOptions{})
	if e != nil || !r.Complete || r.UVArea[0].Cmp(big.NewRat(7, 8)) != 0 || len(r.Cells) != 4 {
		t.Fatalf("area partition: cells=%d err=%v", len(r.Cells), e)
	}
	seen := map[int]bool{}
	for _, c := range r.Cells {
		for _, s := range c.Boundary {
			if s.Owner.Source == 0 && s.Owner.Loop == 1 {
				seen[s.Owner.Use] = true
			}
		}
	}
	if len(seen) != 4 {
		t.Fatal("lost hole owners", seen)
	}
	// Same authored hole in both operands is removed once, not twice.
	b.faces[0].Holes = a.faces[0].Holes
	r, e = a.IntersectCoincidentConvexHoleMaterial(context.Background(), 0, b, 0, CoincidentConvexHoleOptions{})
	if e != nil || r.UVArea[0].Cmp(big.NewRat(7, 8)) != 0 {
		t.Fatal("duplicate hole", e)
	}
	if q, e := a.IntersectCoincidentConvexHoleMaterial(context.Background(), 0, b, 0, CoincidentConvexHoleOptions{MaxCells: 1}); e == nil || q.Complete || len(q.Cells) != 0 {
		t.Fatal("non atomic quota")
	}
	// A hole touching its outer boundary is outside this strict source domain.
	a.faces[0].Holes = [][]Use{orthogonalOverlapLoop([]geom.Vec2{{X: .5}, {X: .25, Y: .5}, {X: .5, Y: .75}, {X: .75, Y: .5}}, 20)}
	if q, e := a.IntersectCoincidentConvexHoleMaterial(context.Background(), 0, b, 0, CoincidentConvexHoleOptions{}); e == nil || q.Complete {
		t.Fatal("touch admitted")
	}
}

func TestCoincidentConvexHoleMappedOwners(t *testing.T) {
	a, b := coincidenceFaceFixture(t, true, false, false)
	for i := range a.faces[0].Outer {
		a.faces[0].Outer[i].Curve = nil
	}
	for i := range b.faces[0].Outer {
		b.faces[0].Outer[i].Curve = nil
	}
	points := []geom.Vec2{{X: .5, Y: .375}, {X: .375, Y: .5}, {X: .5, Y: .625}, {X: .625, Y: .5}}
	a.faces[0].Holes = [][]Use{orthogonalOverlapLoop(points, 20)}
	var mapped []geom.Vec2
	for i := len(points) - 1; i >= 0; i-- {
		p := points[i]
		mapped = append(mapped, geom.Vec2{X: 2 + 3*p.Y, Y: -1 + 5*p.X})
	}
	b.faces[0].Holes = [][]Use{orthogonalOverlapLoop(mapped, 120)}
	r, e := a.IntersectCoincidentConvexHoleMaterial(context.Background(), 0, b, 0, CoincidentConvexHoleOptions{})
	if e != nil || !r.Complete || r.UVArea[0].Cmp(big.NewRat(17, 32)) != 0 || r.UVArea[1].Cmp(big.NewRat(255, 32)) != 0 {
		t.Fatal("mapped hole area", e)
	}
	owners := map[[2]int]bool{}
	for _, c := range r.Cells {
		for _, s := range c.Boundary {
			if s.Owner.Loop == 1 {
				owners[[2]int{s.Owner.Source, s.Owner.Use}] = true
			}
			face := a.faces[0]
			poly := c.A
			if s.Owner.Source == 1 {
				face = b.faces[0]
				poly = c.B
			}
			loop := face.Outer
			if s.Owner.Loop == 1 {
				loop = face.Holes[0]
			}
			u := loop[s.Owner.Use]
			for _, v := range s.Range {
				p := [2]*big.Rat{new(big.Rat).Add(ratInset(u.UVStart.X), new(big.Rat).Mul(v, new(big.Rat).Sub(ratInset(u.UVEnd.X), ratInset(u.UVStart.X)))), new(big.Rat).Add(ratInset(u.UVStart.Y), new(big.Rat).Mul(v, new(big.Rat).Sub(ratInset(u.UVEnd.Y), ratInset(u.UVStart.Y))))}
				on := false
				for i, x := range poly {
					y := poly[(i+1)%len(poly)]
					cross := new(big.Rat).Sub(new(big.Rat).Mul(new(big.Rat).Sub(y[0], x[0]), new(big.Rat).Sub(p[1], x[1])), new(big.Rat).Mul(new(big.Rat).Sub(y[1], x[1]), new(big.Rat).Sub(p[0], x[0])))
					if cross.Sign() == 0 {
						on = true
					}
				}
				if !on {
					t.Fatal("source range off cell")
				}
			}
		}
	}
	if len(owners) != 8 {
		t.Fatal("mapped hole owner loss", owners)
	}
}

func TestCoincidentConvexIntersectingHoles(t *testing.T) {
	a, b := coincidenceFaceFixture(t, false, false, false)
	b.faces[0].Surface = a.faces[0].Surface
	a.faces[0].Outer = orthogonalOverlapLoop([]geom.Vec2{{}, {X: 1}, {X: 1, Y: 1}, {Y: 1}}, 10)
	b.faces[0].Outer = append([]Use(nil), a.faces[0].Outer...)
	a.faces[0].Holes = [][]Use{overlapRectangle(.125, .625, .25, .75, 20, true)}
	b.faces[0].Holes = [][]Use{overlapRectangle(.375, .875, .25, .75, 120, true)}
	r, e := a.IntersectCoincidentConvexHoleMaterial(context.Background(), 0, b, 0, CoincidentConvexHoleOptions{})
	if e != nil || !r.Complete || r.UVArea[0].Cmp(big.NewRat(5, 8)) != 0 {
		t.Fatal("overlapping hole inclusion exclusion", e)
	}
	// No removed overlap edge can be returned as a positive boundary interval.
	for _, c := range r.Cells {
		for _, s := range c.Boundary {
			if s.Owner.Loop == 1 && ((s.Owner.Source == 0 && s.Owner.Use == 2) || (s.Owner.Source == 1 && s.Owner.Use == 0)) {
				t.Fatal("buried hole side retained", s)
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if q, e := a.IntersectCoincidentConvexHoleMaterial(ctx, 0, b, 0, CoincidentConvexHoleOptions{}); e == nil || q.Complete {
		t.Fatal("cancelled certificate")
	}
}
