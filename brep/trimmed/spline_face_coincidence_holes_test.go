package trimmed

import (
	"context"
	"plainsolid/geom"
	"testing"
)

func TestSplineFaceCoincidencePermutedHolesAndRefinedOuter(t *testing.T) {
	a, b := refinedCoincidenceFixture(t, true)
	a.faces[0].Holes = nil
	b.faces[0].Holes = nil
	for hole, x := range []float64{.25, .625} {
		points := []geom.Vec2{{X: x, Y: .375}, {X: x, Y: .625}, {X: x + .125, Y: .625}, {X: x + .125, Y: .375}}
		var la, lb []Use
		for i, p := range points {
			q := points[(i+1)%4]
			la = append(la, Use{Edge: 30 + hole*4 + i, UVStart: p, UVEnd: q})
			lb = append(lb, Use{Edge: 130 + hole*4 + i, UVStart: geom.Vec2{X: 2 + 3*q.Y, Y: -1 + 5*q.X}, UVEnd: geom.Vec2{X: 2 + 3*p.Y, Y: -1 + 5*p.X}})
		}
		for i, j := 0, len(lb)-1; i < j; i, j = i+1, j-1 {
			lb[i], lb[j] = lb[j], lb[i]
		}
		a.faces[0].Holes = append(a.faces[0].Holes, la)
		b.faces[0].Holes = append(b.faces[0].Holes, lb)
	}
	b.faces[0].Holes[0], b.faces[0].Holes[1] = b.faces[0].Holes[1], b.faces[0].Holes[0]
	if r, e := a.CertifySplineFaceCoincidence(context.Background(), 0, b, 0, SplineFaceCoincidenceOptions{}); e == nil || r.Complete {
		t.Fatal("ordered contract changed")
	}
	r, e := a.CertifySplineFaceCoincidence(context.Background(), 0, b, 0, SplineFaceCoincidenceOptions{MatchHolePermutation: true})
	if e != nil || !r.Complete || len(r.Uses) != 14 {
		t.Fatal(r, e)
	}
	for _, u := range r.Uses {
		if u.LoopA > 0 && u.LoopB != 3-u.LoopA {
			t.Fatal("lost original hole identity", u)
		}
	}
	// A missing/repeated hole is not an alternative valid pairing.
	b.faces[0].Holes[1] = append([]Use(nil), b.faces[0].Holes[0]...)
	if r, e = a.CertifySplineFaceCoincidence(context.Background(), 0, b, 0, SplineFaceCoincidenceOptions{MatchHolePermutation: true}); e == nil || r.Complete || len(r.Uses) != 0 {
		t.Fatal("duplicate hole", r, e)
	}
}
