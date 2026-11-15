package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func orthogonalOverlapLoop(points []geom.Vec2, base int) []Use {
	var uses []Use
	for i, p := range points {
		uses = append(uses, Use{Edge: base + i, UVStart: p, UVEnd: points[(i+1)%len(points)]})
	}
	return uses
}
func TestCoincidentOrthogonalDisconnectedAndMixedContact(t *testing.T) {
	a, b := coincidenceFaceFixture(t, true, false, false)
	a.faces[0].Holes = nil
	b.faces[0].Holes = nil
	a.faces[0].Outer = orthogonalOverlapLoop([]geom.Vec2{{X: .125, Y: .125}, {X: .875, Y: .125}, {X: .875, Y: .875}, {X: .625, Y: .875}, {X: .625, Y: .375}, {X: .375, Y: .375}, {X: .375, Y: .875}, {X: .125, Y: .875}}, 10)
	for _, tc := range []struct {
		lower    float64
		area     *big.Rat
		segments int
	}{{.5, big.NewRat(1, 8), 0}, {.375, big.NewRat(3, 16), 1}} {
		// B=(2+3v,-1+5u): an independently authored horizontal shared-chart strip.
		b.faces[0].Outer = overlapRectangle(2+3*tc.lower, 4.25, -1, 4, 100, false)
		if r, e := a.IntersectCoincidentRectangularTrims(context.Background(), 0, b, 0, CoincidentRectangleOverlapOptions{}); e == nil || r.Complete {
			t.Fatal("rectangular contract broadened")
		}
		r, e := a.IntersectCoincidentOrthogonalTrims(context.Background(), 0, b, 0, CoincidentRectangleOverlapOptions{})
		if e != nil || !r.Complete || r.UVArea[0].Cmp(tc.area) != 0 || len(r.Tiles) != 2 || len(r.Segments) != tc.segments || len(r.Points) != 0 {
			t.Fatal(r, e)
		}
		inner := map[int]bool{}
		for _, tile := range r.Tiles {
			if tile.A.Min[0].Cmp(big.NewRat(3, 8)) < 0 && tile.A.Max[0].Cmp(big.NewRat(3, 8)) > 0 || tile.A.Min[0].Cmp(big.NewRat(5, 8)) < 0 && tile.A.Max[0].Cmp(big.NewRat(5, 8)) > 0 {
				t.Fatal("tile crossed notch")
			}
			for _, owners := range tile.BoundaryOwners {
				seen := map[CoincidentTrimOwner]bool{}
				for _, owner := range owners {
					if seen[owner] {
						t.Fatal("duplicate ownership")
					}
					seen[owner] = true
					if owner.Source == 0 {
						inner[owner.Edge] = true
					}
				}
			}
		}
		if !inner[13] || !inner[15] {
			t.Fatal("lost concave original walls", inner)
		}
		if tc.segments == 1 {
			segment := r.Segments[0]
			if segment.A[0][0].Cmp(big.NewRat(3, 8)) != 0 || segment.A[1][0].Cmp(big.NewRat(5, 8)) != 0 || len(segment.Owners) != 2 {
				t.Fatal("uncovered notch seam", segment)
			}
		}
	}
}
func TestCoincidentOrthogonalConcaveHole(t *testing.T) {
	a, b := coincidenceFaceFixture(t, true, false, false)
	a.faces[0].Outer = overlapRectangle(0, 1, 0, 1, 10, false)
	a.faces[0].Holes = [][]Use{orthogonalOverlapLoop([]geom.Vec2{{X: .25, Y: .25}, {X: .25, Y: .75}, {X: .5, Y: .75}, {X: .5, Y: .5}, {X: .75, Y: .5}, {X: .75, Y: .25}}, 20)}
	b.faces[0].Outer = overlapRectangle(2, 5, -1, 4, 100, false)
	b.faces[0].Holes = nil
	r, e := a.IntersectCoincidentOrthogonalTrims(context.Background(), 0, b, 0, CoincidentRectangleOverlapOptions{})
	if e != nil || !r.Complete || r.UVArea[0].Cmp(big.NewRat(13, 16)) != 0 || r.UVArea[1].Cmp(big.NewRat(195, 16)) != 0 {
		t.Fatal(r, e)
	}
	found := false
	point := [2]*big.Rat{big.NewRat(5, 8), big.NewRat(5, 8)}
	for _, tile := range r.Tiles {
		found = found || coincidentRectangleContains(tile.A, point, true)
	}
	if !found {
		t.Fatal("hole bounding rectangle replaced actual concave loop")
	}
	for _, o := range []CoincidentRectangleOverlapOptions{{MaxUses: 8}, {MaxCells: 1}} {
		r, e = a.IntersectCoincidentOrthogonalTrims(context.Background(), 0, b, 0, o)
		if e == nil || r.Complete || r.UVArea[0] != nil || len(r.Tiles) > 0 {
			t.Fatal("non-atomic quota", r, e)
		}
	}
	// Exact touching hole/outer seam must not be admitted as a separated hole.
	a.faces[0].Holes[0][0].UVStart.X = 0
	a.faces[0].Holes[0][5].UVEnd.X = 0
	a.faces[0].Holes[0][1].UVStart.X = 0
	a.faces[0].Holes[0][0].UVEnd.X = 0
	r, e = a.IntersectCoincidentOrthogonalTrims(context.Background(), 0, b, 0, CoincidentRectangleOverlapOptions{})
	if e == nil || r.Complete {
		t.Fatal("touching hole admitted", r, e)
	}
}
