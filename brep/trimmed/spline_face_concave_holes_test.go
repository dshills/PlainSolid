package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestCoincidentSimpleConcaveSlantedHole(t *testing.T) {
	a, b := coincidenceFaceFixture(t, true, false, false)
	a.faces[0].Outer = orthogonalOverlapLoop([]geom.Vec2{{}, {X: 1}, {X: 1, Y: 1}, {X: .5, Y: .5}, {Y: 1}}, 10)
	b.faces[0].Outer = overlapRectangle(2, 5, -1, 4, 100, false)
	points := []geom.Vec2{{X: .25, Y: .125}, {X: .125, Y: .25}, {X: .25, Y: .375}, {X: .375, Y: .25}}
	a.faces[0].Holes = [][]Use{orthogonalOverlapLoop(points, 20)}
	var mapped []geom.Vec2
	for i := len(points) - 1; i >= 0; i-- {
		p := points[i]
		mapped = append(mapped, geom.Vec2{X: 2 + 3*p.Y, Y: -1 + 5*p.X})
	}
	b.faces[0].Holes = [][]Use{orthogonalOverlapLoop(mapped, 120)}
	r, e := a.BuildCoincidentSimpleHoledRegions(context.Background(), 0, b, 0, CoincidentSimpleRegionOptions{})
	if e != nil || !r.Complete || len(r.Components) != 1 || len(r.Components[0].Holes) != 1 || r.UVArea[0].Cmp(big.NewRat(23, 32)) != 0 || r.UVArea[1].Cmp(big.NewRat(345, 32)) != 0 {
		t.Fatal("concave diamond hole", e)
	}
	if r.Components[0].Holes[0].SignedAreaA.Cmp(big.NewRat(-1, 32)) != 0 {
		t.Fatal("hole winding")
	}
	seen := map[[2]int]bool{}
	for _, edge := range r.Components[0].Holes[0].Edges {
		if edge.CurveA == nil || edge.CurveB == nil {
			t.Fatal("missing native curves")
		}
		for _, s := range edge.Sources {
			if s.Owner.Loop != 1 {
				t.Fatal("bridge diagonal leaked")
			}
			seen[[2]int{s.Owner.Source, s.Owner.Use}] = true
			face, pts := a.faces[0], edge.A
			if s.Owner.Source == 1 {
				face, pts = b.faces[0], edge.B
			}
			u := face.Holes[0][s.Owner.Use]
			for i, v := range s.Range {
				for k, ends := range [][2]float64{{u.UVStart.X, u.UVEnd.X}, {u.UVStart.Y, u.UVEnd.Y}} {
					x := new(big.Rat).Add(ratInset(ends[0]), new(big.Rat).Mul(v, new(big.Rat).Sub(ratInset(ends[1]), ratInset(ends[0]))))
					if x.Cmp(pts[i][k]) != 0 {
						t.Fatal("hole original range")
					}
				}
			}
		}
	}
	if len(seen) != 8 {
		t.Fatal("missing original hole owners", seen)
	}
	if q, e := a.BuildCoincidentSimpleRegions(context.Background(), 0, b, 0, CoincidentSimpleRegionOptions{}); e == nil || q.Complete {
		t.Fatal("legacy widened")
	}
	if q, e := a.BuildCoincidentSimpleHoledRegions(context.Background(), 0, b, 0, CoincidentSimpleRegionOptions{MaxWork: 2}); e == nil || q.Complete || len(q.Components) > 0 {
		t.Fatal("atomic quota")
	}
	b.faces[0].Holes = nil
	b.faces[0].Outer = overlapRectangle(3.5, 5, -1, 4, 100, false)
	if q, e := a.BuildCoincidentSimpleHoledRegions(context.Background(), 0, b, 0, CoincidentSimpleRegionOptions{}); e == nil || q.Complete {
		t.Fatal("combined notch pinch accepted")
	}
	// Original hole touches the concave notch: source trim certificate rejects.
	a.faces[0].Holes = [][]Use{orthogonalOverlapLoop([]geom.Vec2{{X: .5, Y: .25}, {X: .375, Y: .375}, {X: .5, Y: .5}, {X: .625, Y: .375}}, 20)}
	if q, e := a.BuildCoincidentSimpleHoledRegions(context.Background(), 0, b, 0, CoincidentSimpleRegionOptions{}); e == nil || q.Complete {
		t.Fatal("touching notch accepted")
	}
}
