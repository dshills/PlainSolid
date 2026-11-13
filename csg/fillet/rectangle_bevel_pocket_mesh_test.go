package fillet

import (
	"context"
	"math"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestCertifiedBeveledRectanglePocket(t *testing.T) {
	for _, cap := range []int{0, 1} {
		r, e := PrismRectangleTopBeveledBoredPocketCertified(context.Background(), geom.Vec2{}, geom.Vec2{X: 8, Y: 5}, -1, 2, .75, .25, .5, .375, RectanglePocket{Center: geom.Vec2{X: 2, Y: 2}, Radius: .25, Depth: .75, EntryCap: cap}, RectangleMeshOptions{Deflection: .05})
		if e != nil {
			t.Fatal(cap, e)
		}
		if e = r.Mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		if !r.Collision.Complete || len(r.Collision.Contacts) != 0 || r.IdealBoundarySquaredDistance.Cmp(num(.0025)) > 0 {
			t.Fatal("certificate")
		}
		area := func(t float64) float64 { return (8-2*t)*(5-2*t) - (4-math.Pi)*(.75-t)*(.75-t) }
		expected := area(0)*2.5 + .5/6*(area(0)+4*area(.125)+area(.25)) - math.Pi*.375*.375*3 - math.Pi*.25*.25*.75
		if math.Abs(r.Mesh.SignedVolume()-expected) > .12 {
			t.Fatal("independent volume", r.Mesh.SignedVolume(), expected)
		}

		// Independent rational-circle sample on the first conical band at
		// half height; compare its same-parameter diagonal interpolant.
		lower, upper := r.Caps[0].Region.Loops[0].Boundary, r.Caps[1].Region.Loops[0].Boundary
		for i, seg := range lower.Segments {
			if seg.Source != 1 {
				continue
			}
			other := upper.Segments[i]
			t0 := (seg.Start + seg.End) / 2
			x := 7.25 + .625*(2*t0/(1+t0*t0))
			y := .75 - .625*((1-t0*t0)/(1+t0*t0))
			a, b := lower.Points[seg.Vertices[0]], upper.Points[other.Vertices[1]]
			dx, dy := x-(a.X+b.X)/2, y-(a.Y+b.Y)/2
			limit, _ := r.IdealBoundarySquaredDistance.Float64()
			if dx*dx+dy*dy > limit {
				t.Fatal("independent conical correspondence", dx*dx+dy*dy, limit)
			}
		}
		faces := map[int]int{}
		edges := map[[2]int]int{}
		for i, tr := range r.Mesh.Triangles {
			faces[r.OriginalFace[i]]++
			for j, a := range tr {
				b := tr[(j+1)%3]
				if a > b {
					a, b = b, a
				}
				edges[[2]int{a, b}]++
			}
		}
		if len(faces) != len(r.Model.Solid.Faces()) || len(r.Mesh.Vertices)-len(edges)+len(r.Mesh.Triangles) != 0 {
			t.Fatal("ancestry/genus")
		}
		for _, n := range edges {
			if n != 2 {
				t.Fatal("unshared rim")
			}
		}
		t.Logf("cap%d vertices%d triangles%d squared-bound%g volume%g", cap, len(r.Mesh.Vertices), len(r.Mesh.Triangles), func() float64 { x, _ := r.IdealBoundarySquaredDistance.Float64(); return x }(), r.Mesh.SignedVolume())
	}
}
func TestCertifiedBeveledRectanglePocketLimit(t *testing.T) {
	r, e := PrismRectangleTopBeveledBoredPocketCertified(context.Background(), geom.Vec2{}, geom.Vec2{X: 8, Y: 5}, -1, 2, .75, .25, .5, .375, RectanglePocket{Center: geom.Vec2{X: 2, Y: 2}, Radius: .25, Depth: .75}, RectangleMeshOptions{Deflection: .05, MaxTriangles: 1})
	if e == nil || len(r.Mesh.Vertices) != 0 {
		t.Fatal("partial result")
	}
}

func TestCertifiedBeveledPocketCollisionLimit(t *testing.T) {
	r, e := PrismRectangleTopBeveledBoredPocketCertified(context.Background(), geom.Vec2{}, geom.Vec2{X: 8, Y: 5}, -1, 2, .75, .25, .5, .375, RectanglePocket{Center: geom.Vec2{X: 2, Y: 2}, Radius: .25, Depth: .75}, RectangleMeshOptions{Deflection: .05, Collision: collision.Options{MaxPairs: 1}})
	if e == nil || len(r.Mesh.Vertices) != 0 {
		t.Fatal("partial contact certificate")
	}
}
