package fillet

import (
	"context"
	"math"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestCertifiedBoredRectanglePocket(t *testing.T) {
	for _, cap := range []int{0, 1} {
		r, e := PrismRectangleCornersBoredPocketCertified(context.Background(), geom.Vec2{}, geom.Vec2{X: 6, Y: 4}, -1, 1, .375, []CornerRadius{{Corner: 0, Radius: .25}, {Corner: 2, Radius: .5}}, RectanglePocket{Center: geom.Vec2{X: 1.5, Y: 1.75}, Radius: .25, Depth: .75, EntryCap: cap}, RectangleMeshOptions{Deflection: .05})
		if e != nil {
			t.Fatal(cap, e)
		}
		if e = r.Mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		if !r.Collision.Complete || len(r.Collision.Contacts) != 0 || r.IdealBoundarySquaredDistance.Cmp(num(.0025)) > 0 {
			t.Fatal("certificate")
		}
		expected := 2*(24-(1-math.Pi/4)*(.25*.25+.5*.5)-math.Pi*.375*.375) - math.Pi*.25*.25*.75
		if math.Abs(r.Mesh.SignedVolume()-expected) > .08 {
			t.Fatal("independent volume", r.Mesh.SignedVolume(), expected)
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
func TestCertifiedBoredRectanglePocketLimit(t *testing.T) {
	r, e := PrismRectangleCornersBoredPocketCertified(context.Background(), geom.Vec2{}, geom.Vec2{X: 6, Y: 4}, -1, 1, .375, []CornerRadius{{Corner: 0, Radius: .25}}, RectanglePocket{Center: geom.Vec2{X: 1.5, Y: 1.75}, Radius: .25, Depth: .75}, RectangleMeshOptions{Deflection: .05, MaxTriangles: 1})
	if e == nil || len(r.Mesh.Vertices) != 0 {
		t.Fatal("partial result")
	}
}

func TestCertifiedBoredPocketCollisionLimit(t *testing.T) {
	r, e := PrismRectangleCornersBoredPocketCertified(context.Background(), geom.Vec2{}, geom.Vec2{X: 6, Y: 4}, -1, 1, .375, []CornerRadius{{Corner: 0, Radius: .25}}, RectanglePocket{Center: geom.Vec2{X: 1.5, Y: 1.75}, Radius: .25, Depth: .75}, RectangleMeshOptions{Deflection: .05, Collision: collision.Options{MaxPairs: 1}})
	if e == nil || len(r.Mesh.Vertices) != 0 {
		t.Fatal("partial contact certificate")
	}
}
