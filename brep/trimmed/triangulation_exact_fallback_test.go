package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"testing"
)

func exactUVAreaTwice(r []geom.Vec2) *big.Rat {
	sum := new(big.Rat)
	for i, a := range r {
		b := r[(i+1)%len(r)]
		x := new(big.Rat).Mul(new(big.Rat).SetFloat64(a.X), new(big.Rat).SetFloat64(b.Y))
		y := new(big.Rat).Mul(new(big.Rat).SetFloat64(a.Y), new(big.Rat).SetFloat64(b.X))
		sum.Add(sum, new(big.Rat).Sub(x, y))
	}
	return sum
}
func TestUVExactFallbackPreservesBoundaryCover(t *testing.T) {
	eps := math.Ldexp(1, -45)
	outer := []geom.Vec3{{X: 0, Y: 0}, {X: .25, Y: 0}, {X: .5, Y: 0}, {X: 1, Y: eps}, {X: 1, Y: 1}, {X: 0, Y: 1}}
	// Even without the intentionally collinear subdivision, the legacy path
	// rejects the tiny but strictly nonzero corner. The fallback must retain it.
	clean := append([]geom.Vec3{outer[0]}, outer[2:]...)
	if _, e := brep.PrismWithHoles(clean, nil, geom.Vec3{Z: 1}); e == nil {
		t.Fatal("fixture no longer exercises ordinary rejection")
	}
	hole := []geom.Vec3{{X: .25, Y: .25}, {X: .25, Y: .5}, {X: .5, Y: .5}, {X: .5, Y: .25}}
	for _, holes := range [][][]geom.Vec3{nil, {hole}} {
		rings := append([][]geom.Vec3{outer}, holes...)
		triangles, e := triangulateUV(rings)
		if e != nil {
			t.Fatal(e)
		}
		want := new(big.Rat)
		boundary := map[[2]geom.Vec2]bool{}
		for _, ring := range rings {
			p := make([]geom.Vec2, len(ring))
			for i, v := range ring {
				p[i] = geom.Vec2{X: v.X, Y: v.Y}
			}
			want.Add(want, exactUVAreaTwice(p))
			for i, a := range p {
				boundary[[2]geom.Vec2{a, p[(i+1)%len(p)]}] = true
			}
		}
		got := new(big.Rat)
		uses := map[[2]geom.Vec2]int{}
		for _, tri := range triangles {
			area := exactUVAreaTwice(tri[:])
			if area.Sign() <= 0 {
				t.Fatal("nonpositive exact triangle", tri)
			}
			got.Add(got, area)
			for i, a := range tri {
				uses[[2]geom.Vec2{a, tri[(i+1)%3]}]++
			}
		}
		if got.Cmp(want) != 0 {
			t.Fatal("exact area coverage", got, want)
		}
		for edge := range boundary {
			if uses[edge] != 1 || uses[[2]geom.Vec2{edge[1], edge[0]}] != 0 {
				t.Fatal("lost original directed boundary atom", edge)
			}
		}
		for edge, n := range uses {
			if n != 1 {
				t.Fatal("duplicate triangle use")
			}
			if !boundary[edge] && uses[[2]geom.Vec2{edge[1], edge[0]}] != 1 {
				t.Fatal("unpaired internal edge", edge)
			}
		}
	}
}
func TestUVExactFallbackRejectsInvalidRings(t *testing.T) {
	for _, ring := range [][]geom.Vec3{
		{{X: 0, Y: 0}, {X: .5, Y: 0}, {X: .5, Y: 0}, {X: 1, Y: math.Ldexp(1, -45)}, {X: 1, Y: 1}, {X: 0, Y: 1}},
		{{X: 0, Y: 0}, {X: 1, Y: 1}, {X: 0, Y: 1}, {X: 1, Y: 0}},
		{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: .5, Y: 0}, {X: 1, Y: 1}, {X: 0, Y: 1}},
		{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 1, Y: 1}, {X: 0, Y: 0}, {X: 0, Y: 1}},
	} {
		if tris, e := triangulateUV([][]geom.Vec3{ring}); e == nil || len(tris) != 0 {
			t.Fatal("non-simple accepted", e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	eps := math.Ldexp(1, -45)
	ring := []geom.Vec3{{X: 0, Y: 0}, {X: .5, Y: 0}, {X: 1, Y: eps}, {X: 1, Y: 1}, {X: 0, Y: 1}}
	if tris, e := triangulateUVBounded(ctx, [][]geom.Vec3{ring}, brep.MaxHoledFaceVertices); e == nil || len(tris) != 0 {
		t.Fatal("cancelled fallback", e)
	}
}

func TestUVExactFallbackLimitsAndHoleRejection(t *testing.T) {
	outer := []geom.Vec3{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 1, Y: 1}, {X: 0, Y: 1}}
	hole := []geom.Vec3{{X: 0, Y: .25}, {X: 0, Y: .5}, {X: .5, Y: .5}, {X: .5, Y: .25}}
	if r, e := triangulateExactUVFallback(context.Background(), [][]geom.Vec3{outer, hole}, 256); e == nil || len(r) != 0 {
		t.Fatal("touching hole", e)
	}
	if r, e := triangulateExactUVFallback(context.Background(), [][]geom.Vec3{outer}, 3); e == nil || len(r) != 0 {
		t.Fatal("vertex quota", e)
	}
}
