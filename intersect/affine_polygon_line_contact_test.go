package intersect

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func affineHole(x0, y0, x1, y1 *big.Rat) [][2]*big.Rat {
	return [][2]*big.Rat{{x0, y0}, {x0, y1}, {x1, y1}, {x1, y0}}
}
func affinePolygonLineFixture(t *testing.T, y float64) (Patch, Patch) {
	d := geom.ParameterRectangle{UMax: 1, VMax: 1}
	return affineContactPatch(t, func(u, v float64) geom.Vec3 { return geom.Vec3{X: u, Y: v} }, d, true), affineContactPatch(t, func(u, v float64) geom.Vec3 { return geom.Vec3{X: u, Y: y, Z: v - .5} }, d, false)
}
func TestAffinePolygonLineHoles(t *testing.T) {
	a, b := affinePolygonLineFixture(t, .5)
	trims := [2]AffinePolygonTrim{{Outer: affineTrimSquare(), Holes: [][][2]*big.Rat{affineHole(big.NewRat(1, 3), big.NewRat(1, 4), big.NewRat(2, 3), big.NewRat(3, 4))}}, {Outer: affineTrimSquare()}}
	out, e := IntersectPolygonAffinePatches(context.Background(), a, b, trims, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	ev, sp := out.Events(), out.Spans()
	if !out.Complete() || len(ev) != 4 || len(sp) != 2 {
		t.Fatal(ev, sp)
	}
	if ev[0].Before != AffineTrimLineNoSide || ev[1].Before != AffineTrimLineInterior || ev[1].After != AffineTrimLineOutside || ev[2].Before != AffineTrimLineOutside || ev[2].After != AffineTrimLineInterior || ev[3].After != AffineTrimLineNoSide {
		t.Fatal("wrong finite germs", ev)
	}
	length := new(big.Rat)
	for _, s := range sp {
		if s.BoundaryOwners != 0 {
			t.Fatal(s)
		}
		length.Add(length, new(big.Rat).Sub(ev[s.End].Parameter, ev[s.Start].Parameter))
	}
	if length.Cmp(big.NewRat(2, 3)) != 0 {
		t.Fatal(length)
	}
	for _, i := range []int{1, 2} {
		if ev[i].Isolated || len(ev[i].Sides) != 1 || ev[i].Sides[0].Owner != 0 || ev[i].Sides[0].Ring != 1 {
			t.Fatal(ev[i])
		}
	}
	ev[0].Vertex.Point[0].SetInt64(99)
	ev[1].Sides[0].Edge = 99
	sp[0].End = 99
	if out.Events()[0].Vertex.Point[0].Cmp(big.NewRat(99, 1)) == 0 || out.Events()[1].Sides[0].Edge == 99 || out.Spans()[0].End == 99 {
		t.Fatal("mutable result")
	}
	for _, cap := range []int{out.ExactOperations(), out.ExactOperations() - 1} {
		q, e := IntersectPolygonAffinePatches(context.Background(), a, b, trims, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{MaxOperations: cap})
		if cap == out.ExactOperations() {
			if e != nil || q == nil {
				t.Fatal(e)
			}
		} else if e == nil || q != nil {
			t.Fatal("partial budget")
		}
	}
	// A second, independently authored hole removes another interval in B's UV.
	trims[1].Holes = [][][2]*big.Rat{affineHole(big.NewRat(1, 8), big.NewRat(1, 4), big.NewRat(1, 4), big.NewRat(3, 4))}
	q, e := IntersectPolygonAffinePatches(context.Background(), a, b, trims, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{})
	if e != nil || len(q.Spans()) != 3 {
		t.Fatal(q, e)
	}
}
func TestAffinePolygonLineBoundaryAndIsolated(t *testing.T) {
	a, b := affinePolygonLineFixture(t, .25)
	trims := [2]AffinePolygonTrim{{Outer: affineTrimSquare(), Holes: [][][2]*big.Rat{affineHole(big.NewRat(1, 3), big.NewRat(1, 4), big.NewRat(2, 3), big.NewRat(3, 4))}}, {Outer: affineTrimSquare()}}
	q, e := IntersectPolygonAffinePatches(context.Background(), a, b, trims, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	sp := q.Spans()
	if len(sp) != 3 || sp[1].BoundaryOwners != 1 || len(sp[1].Sides) != 1 || sp[1].Sides[0].Ring != 1 {
		t.Fatal(sp)
	}
	a, b = affinePolygonLineFixture(t, .5)
	trims[0] = AffinePolygonTrim{Outer: [][2]*big.Rat{{big.NewRat(1, 2), big.NewRat(1, 2)}, affineTrimUV(1, 1), affineTrimUV(0, 1)}}
	q, e = IntersectPolygonAffinePatches(context.Background(), a, b, trims, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	ev := q.Events()
	if len(ev) != 1 || len(q.Spans()) != 0 || !ev[0].Isolated || len(ev[0].Sides) != 2 || ev[0].Vertex.Point[0].Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatal(ev, q.Spans())
	}
}
func TestAffinePolygonLineInvalidAndNonconvex(t *testing.T) {
	a, b := affinePolygonLineFixture(t, .5)
	// Positive simple nonconvex U: its horizontal section has two components.
	uv := func(x, y int64) [2]*big.Rat { return [2]*big.Rat{big.NewRat(x, 4), big.NewRat(y, 4)} }
	outer := [][2]*big.Rat{uv(0, 0), uv(4, 0), uv(4, 4), uv(3, 4), uv(3, 1), uv(1, 1), uv(1, 4), uv(0, 4)}
	trims := [2]AffinePolygonTrim{{Outer: outer}, {Outer: affineTrimSquare()}}
	q, e := IntersectPolygonAffinePatches(context.Background(), a, b, trims, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{})
	if e != nil || len(q.Spans()) != 2 {
		t.Fatal(q, e)
	}
	trims[0] = AffinePolygonTrim{Outer: affineTrimSquare(), Holes: [][][2]*big.Rat{affineHole(big.NewRat(0, 1), big.NewRat(1, 4), big.NewRat(1, 2), big.NewRat(3, 4))}}
	if q, e := IntersectPolygonAffinePatches(context.Background(), a, b, trims, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{}); e == nil || q != nil {
		t.Fatal("touching hole accepted")
	}
	trims[0] = AffinePolygonTrim{Outer: affineTrimSquare()}
	if q, e := IntersectPolygonAffinePatches(context.Background(), a, a, trims, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{}); e == nil || q != nil {
		t.Fatal("coplanar accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if q, e := IntersectPolygonAffinePatches(ctx, a, b, trims, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{}); e != context.Canceled || q != nil {
		t.Fatal(q, e)
	}
}
