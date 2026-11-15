package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func concaveOriginalHoleFixture(t *testing.T) (Solid, Solid) {
	a, b := coincidenceFaceFixture(t, true, false, false)
	a.faces[0].Outer = orthogonalOverlapLoop([]geom.Vec2{{}, {X: 1}, {X: 1, Y: 1}, {X: .5, Y: .5}, {Y: 1}}, 10)
	b.faces[0].Outer = overlapRectangle(2, 5, -1, 4, 100, false)
	// Clockwise concave pentagon, area -(3/4*1/2-3/4*1/4/2)=-9/32.
	p := []geom.Vec2{{X: .125, Y: .125}, {X: .125, Y: .625}, {X: .5, Y: .375}, {X: .875, Y: .625}, {X: .875, Y: .125}}
	a.faces[0].Holes = [][]Use{orthogonalOverlapLoop(p, 20)}
	var mapped []geom.Vec2
	for i := len(p) - 1; i >= 0; i-- {
		mapped = append(mapped, geom.Vec2{X: 2 + 3*p[i].Y, Y: -1 + 5*p[i].X})
	}
	b.faces[0].Holes = [][]Use{orthogonalOverlapLoop(mapped, 120)}
	return a, b
}
func TestCoincidentGenuineConcaveHole(t *testing.T) {
	a, b := concaveOriginalHoleFixture(t)
	r, e := a.BuildCoincidentSimpleHoledRegions(context.Background(), 0, b, 0, CoincidentSimpleRegionOptions{})
	if e != nil || !r.Complete || len(r.Components) != 1 || len(r.Components[0].Holes) != 1 || r.UVArea[0].Cmp(big.NewRat(15, 32)) != 0 || r.UVArea[1].Cmp(big.NewRat(225, 32)) != 0 {
		t.Fatal("concave outer and hole", e)
	}
	h := r.Components[0].Holes[0]
	if h.SignedAreaA.Cmp(big.NewRat(-9, 32)) != 0 {
		t.Fatal("hole analytic area")
	}
	seen := map[[2]int]bool{}
	for _, edge := range h.Edges {
		if len(edge.Sources) != 2 {
			t.Fatal("concave bridge leaked")
		}
		for _, s := range edge.Sources {
			if s.Owner.Loop != 1 {
				t.Fatal("original hole lost")
			}
			seen[[2]int{s.Owner.Source, s.Owner.Use}] = true
			f, points := a.faces[0], edge.A
			if s.Owner.Source == 1 {
				f, points = b.faces[0], edge.B
			}
			u := f.Holes[0][s.Owner.Use]
			for i, v := range s.Range {
				for k, ends := range [][2]float64{{u.UVStart.X, u.UVEnd.X}, {u.UVStart.Y, u.UVEnd.Y}} {
					p := new(big.Rat).Add(ratInset(ends[0]), new(big.Rat).Mul(v, new(big.Rat).Sub(ratInset(ends[1]), ratInset(ends[0]))))
					if p.Cmp(points[i][k]) != 0 {
						t.Fatal("concave original range")
					}
				}
			}
		}
	}
	if len(seen) != 10 {
		t.Fatal("concave source ownership", seen)
	}
	b.faces[0].Holes = nil
	b.faces[0].Outer = overlapRectangle(2.75, 2.9375, -1, 4, 100, false)
	r, e = a.BuildCoincidentSimpleHoledRegions(context.Background(), 0, b, 0, CoincidentSimpleRegionOptions{})
	if e != nil || !r.Complete || len(r.Components) != 2 || r.UVArea[0].Cmp(big.NewRat(1, 64)) != 0 {
		t.Fatal("concave hole splits clipped material", e)
	}
	for _, component := range r.Components {
		if len(component.Holes) != 0 || component.Outer.SignedAreaA.Cmp(big.NewRat(1, 128)) != 0 {
			t.Fatal("independent end strip")
		}
	}
}
func TestCoincidentConcaveHoleRejectsUnprovedTopology(t *testing.T) {
	a, b := concaveOriginalHoleFixture(t)
	// Raising the reentrant hole vertex to the stock notch creates a true
	// source boundary touch; it cannot be converted to a welded hole.
	a.faces[0].Holes[0][1].UVEnd = geom.Vec2{X: .5, Y: .5}
	a.faces[0].Holes[0][2].UVStart = geom.Vec2{X: .5, Y: .5}
	if q, e := a.BuildCoincidentSimpleHoledRegions(context.Background(), 0, b, 0, CoincidentSimpleRegionOptions{}); e == nil || q.Complete {
		t.Fatal("touch accepted")
	}
	a, b = concaveOriginalHoleFixture(t)
	if q, e := a.BuildCoincidentSimpleHoledRegions(context.Background(), 0, b, 0, CoincidentSimpleRegionOptions{MaxCells: 1}); e == nil || q.Complete || len(q.Components) > 0 {
		t.Fatal("partial cells on quota")
	}
}
