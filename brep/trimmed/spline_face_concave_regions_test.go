//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestCoincidentSimpleConcaveRegions(t *testing.T) {
	a, b := coincidenceFaceFixture(t, true, false, false)
	a.faces[0].Holes = nil
	b.faces[0].Holes = nil
	a.faces[0].Outer = orthogonalOverlapLoop([]geom.Vec2{{}, {X: 1}, {X: 1, Y: 1}, {X: .5, Y: .5}, {Y: 1}}, 10)
	b.faces[0].Outer = overlapRectangle(2, 5, -1, 4, 100, false)
	r, e := a.BuildCoincidentSimpleRegions(context.Background(), 0, b, 0, CoincidentSimpleRegionOptions{})
	if e != nil || !r.Complete || len(r.Components) != 1 || r.UVArea[0].Cmp(big.NewRat(3, 4)) != 0 || r.UVArea[1].Cmp(big.NewRat(45, 4)) != 0 {
		t.Fatal("concave full original", e)
	}
	if r.CanceledSeams == 0 {
		t.Fatal("triangulation diagonals not canceled")
	}
	seen := map[int]bool{}
	for _, edge := range r.Components[0].Outer.Edges {
		if len(edge.Sources) == 0 {
			t.Fatal("invented diagonal")
		}
		for _, owner := range edge.Sources {
			if owner.Owner.Source == 0 {
				seen[owner.Owner.Use] = true
			}
		}
	}
	if len(seen) != 5 {
		t.Fatal("lost concave source uses", seen)
	}
	b.faces[0].Outer = overlapRectangle(4.25, 5, -1, 4, 100, false)
	r, e = a.BuildCoincidentSimpleRegions(context.Background(), 0, b, 0, CoincidentSimpleRegionOptions{})
	if e != nil || !r.Complete || len(r.Components) != 2 || r.UVArea[0].Cmp(big.NewRat(1, 16)) != 0 {
		t.Fatal("two independently triangular components", e)
	}
	// Lowering the strip to the notch makes the two positive components
	// meet at one vertex. Reject the pinched boundary, rather than joining it.
	b.faces[0].Outer = overlapRectangle(3.5, 5, -1, 4, 100, false)
	if q, e := a.BuildCoincidentSimpleRegions(context.Background(), 0, b, 0, CoincidentSimpleRegionOptions{}); e == nil || q.Complete {
		t.Fatal("pinch accepted")
	}
	b.faces[0].Outer = overlapRectangle(2, 5, -1, 4, 100, false)
	if q, e := a.BuildCoincidentSimpleRegions(context.Background(), 0, b, 0, CoincidentSimpleRegionOptions{MaxWork: 1}); e == nil || q.Complete || len(q.Components) != 0 {
		t.Fatal("quota result")
	}
	a.faces[0].Outer = orthogonalOverlapLoop([]geom.Vec2{{}, {X: 1, Y: 1}, {Y: 1}, {X: 1}}, 10)
	if q, e := a.BuildCoincidentSimpleRegions(context.Background(), 0, b, 0, CoincidentSimpleRegionOptions{}); e == nil || q.Complete {
		t.Fatal("crossed original accepted")
	}
}
