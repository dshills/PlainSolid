package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func multiHoleCoincidentFixture(t *testing.T) (Solid, Solid) {
	a, b := coincidenceFaceFixture(t, true, false, false)
	a.faces[0].Outer = orthogonalOverlapLoop([]geom.Vec2{{}, {X: 1}, {X: 1, Y: 1}, {X: .5, Y: .5}, {Y: 1}}, 10)
	b.faces[0].Outer = overlapRectangle(2, 5, -1, 4, 100, false)
	a.faces[0].Holes = nil
	b.faces[0].Holes = nil
	for i, c := range []float64{.25, .75} {
		p := []geom.Vec2{{X: c, Y: .125}, {X: c - .125, Y: .25}, {X: c, Y: .375}, {X: c + .125, Y: .25}}
		a.faces[0].Holes = append(a.faces[0].Holes, orthogonalOverlapLoop(p, 20+10*i))
		var mapped []geom.Vec2
		for j := len(p) - 1; j >= 0; j-- {
			mapped = append(mapped, geom.Vec2{X: 2 + 3*p[j].Y, Y: -1 + 5*p[j].X})
		}
		b.faces[0].Holes = append(b.faces[0].Holes, orthogonalOverlapLoop(mapped, 120+10*i))
	}
	b.faces[0].Holes[0], b.faces[0].Holes[1] = b.faces[0].Holes[1], b.faces[0].Holes[0]
	return a, b
}
func TestCoincidentConcaveMultipleSlantedHoles(t *testing.T) {
	a, b := multiHoleCoincidentFixture(t)
	r, e := a.BuildCoincidentMultiHoledRegions(context.Background(), 0, b, 0, 2, CoincidentSimpleRegionOptions{})
	if e != nil || !r.Complete || len(r.Components) != 1 || len(r.Components[0].Holes) != 2 || r.UVArea[0].Cmp(big.NewRat(11, 16)) != 0 || r.UVArea[1].Cmp(big.NewRat(165, 16)) != 0 {
		t.Fatal("two diamond holes", e)
	}
	seen := map[[3]int]bool{}
	for _, h := range r.Components[0].Holes {
		if h.SignedAreaA.Cmp(big.NewRat(-1, 32)) != 0 {
			t.Fatal("independent hole area")
		}
		for _, edge := range h.Edges {
			if len(edge.Sources) != 2 {
				t.Fatal("original dual owners")
			}
			for _, s := range edge.Sources {
				if s.Owner.Loop < 1 || s.Owner.Loop > 2 {
					t.Fatal("bridge leaked")
				}
				seen[[3]int{s.Owner.Source, s.Owner.Loop, s.Owner.Use}] = true
				f, points := a.faces[0], edge.A
				if s.Owner.Source == 1 {
					f, points = b.faces[0], edge.B
				}
				u := f.Holes[s.Owner.Loop-1][s.Owner.Use]
				if u.Edge != s.Owner.Edge {
					t.Fatal("permuted hole ID")
				}
				for i, v := range s.Range {
					for k, ends := range [][2]float64{{u.UVStart.X, u.UVEnd.X}, {u.UVStart.Y, u.UVEnd.Y}} {
						p := new(big.Rat).Add(ratInset(ends[0]), new(big.Rat).Mul(v, new(big.Rat).Sub(ratInset(ends[1]), ratInset(ends[0]))))
						if p.Cmp(points[i][k]) != 0 {
							t.Fatal("original ranged hole identity")
						}
					}
				}
			}
		}
	}
	if len(seen) != 16 {
		t.Fatal("lost original hole use", seen)
	}
	// Source hole order changes identities, never material or loop areas.
	b.faces[0].Holes[0], b.faces[0].Holes[1] = b.faces[0].Holes[1], b.faces[0].Holes[0]
	q, e := a.BuildCoincidentMultiHoledRegions(context.Background(), 0, b, 0, 2, CoincidentSimpleRegionOptions{})
	if e != nil || q.UVArea[0].Cmp(r.UVArea[0]) != 0 || len(q.Components[0].Holes) != 2 {
		t.Fatal("hole order changed material", e)
	}
}
func TestCoincidentMultiHoleGuards(t *testing.T) {
	a, b := multiHoleCoincidentFixture(t)
	for _, n := range []int{0, 1, 9} {
		if r, e := a.BuildCoincidentMultiHoledRegions(context.Background(), 0, b, 0, n, CoincidentSimpleRegionOptions{}); e == nil || r.Complete || len(r.Components) > 0 {
			t.Fatal("hole quota", n)
		}
	}
	if r, e := a.BuildCoincidentSimpleHoledRegions(context.Background(), 0, b, 0, CoincidentSimpleRegionOptions{}); e == nil || r.Complete {
		t.Fatal("one-hole API changed")
	}
	o := CoincidentSimpleRegionOptions{}
	o.Regions.Material.Outer.MaxUses = 24
	if r, e := a.BuildCoincidentMultiHoledRegions(context.Background(), 0, b, 0, 2, o); e == nil || r.Complete {
		t.Fatal("aggregate use quota")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r, e := a.BuildCoincidentMultiHoledRegions(ctx, 0, b, 0, 2, CoincidentSimpleRegionOptions{}); e == nil || r.Complete {
		t.Fatal("cancelled result")
	}
	a.faces[0].Holes[1] = a.faces[0].Holes[0]
	if r, e := a.BuildCoincidentMultiHoledRegions(context.Background(), 0, b, 0, 2, CoincidentSimpleRegionOptions{}); e == nil || r.Complete {
		t.Fatal("coincident original holes")
	}
	a, b = multiHoleCoincidentFixture(t)
	a.faces[0].Holes[1] = orthogonalOverlapLoop([]geom.Vec2{{X: .5, Y: .125}, {X: .375, Y: .25}, {X: .5, Y: .375}, {X: .625, Y: .25}}, 30)
	if r, e := a.BuildCoincidentMultiHoledRegions(context.Background(), 0, b, 0, 2, CoincidentSimpleRegionOptions{}); e == nil || r.Complete {
		t.Fatal("point touching original holes")
	}
}

func TestCoincidentMultiHoleDistinctParents(t *testing.T) {
	a, b := multiHoleCoincidentFixture(t)
	// A contributes two radius1/8 diamonds; B contributes a distinct radius1/16
	// diamond between them. Exact material area=3/4-2/32-1/128=87/128.
	p := []geom.Vec2{{X: .5, Y: .1875}, {X: .4375, Y: .25}, {X: .5, Y: .3125}, {X: .5625, Y: .25}}
	var mapped []geom.Vec2
	for i := len(p) - 1; i >= 0; i-- {
		mapped = append(mapped, geom.Vec2{X: 2 + 3*p[i].Y, Y: -1 + 5*p[i].X})
	}
	b.faces[0].Holes = [][]Use{orthogonalOverlapLoop(mapped, 140)}
	r, e := a.BuildCoincidentMultiHoledRegions(context.Background(), 0, b, 0, 2, CoincidentSimpleRegionOptions{})
	if e != nil || !r.Complete || len(r.Components) != 1 || len(r.Components[0].Holes) != 3 || r.UVArea[0].Cmp(big.NewRat(87, 128)) != 0 {
		t.Fatal("distinct parent holes", e)
	}
	owners := map[[2]int]bool{}
	for _, h := range r.Components[0].Holes {
		for _, edge := range h.Edges {
			if len(edge.Sources) != 1 {
				t.Fatal("invented shared source")
			}
			s := edge.Sources[0]
			owners[[2]int{s.Owner.Source, s.Owner.Loop}] = true
		}
	}
	if len(owners) != 3 || !owners[[2]int{0, 1}] || !owners[[2]int{0, 2}] || !owners[[2]int{1, 1}] {
		t.Fatal("wrong original parent loops", owners)
	}
}
