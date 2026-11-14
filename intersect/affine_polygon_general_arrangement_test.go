package intersect

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func generalAffinePlane(t *testing.T) Patch {
	return affineContactPatch(t, func(u, v float64) geom.Vec3 { return geom.Vec3{X: u, Y: v, Z: u + 2*v} }, geom.ParameterRectangle{UMax: 1, VMax: 1}, true)
}
func generalAffineArea(q *AffineGeneralPolygonArrangement) *big.Rat {
	a := new(big.Rat)
	for _, c := range q.Area().Cycles() {
		a.Add(a, c.SignedAreaA)
	}
	return a
}
func TestGeneralAffineSharedEdgeAndTJunction(t *testing.T) {
	a := generalAffinePlane(t)
	zero, one, half := big.NewRat(0, 1), big.NewRat(1, 1), big.NewRat(1, 2)
	for _, partial := range []bool{false, true} {
		lo, hi := zero, one
		if partial {
			lo, hi = big.NewRat(1, 4), big.NewRat(3, 4)
		}
		trims := [2]AffinePolygonTrim{{Outer: affineRect(zero, zero, half, one, false)}, {Outer: affineRect(half, lo, one, hi, false)}}
		for _, op := range []AffineRingOperation{AffineRingIntersection, AffineRingUnion, AffineRingDifference} {
			q, e := ArrangeGeneralAffinePolygons(context.Background(), a, a, trims, op, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{})
			if e != nil {
				t.Fatal(partial, op, e)
			}
			want := new(big.Rat)
			if op == AffineRingUnion {
				want.SetInt64(1)
				if partial {
					want.SetFrac64(3, 4)
				}
			} else if op == AffineRingDifference {
				want.SetFrac64(1, 2)
			}
			if generalAffineArea(q).Cmp(want) != 0 {
				t.Fatal(op, generalAffineArea(q), want)
			}
			shared := q.SharedBoundarySpans()
			if len(shared) != 1 || shared[0].IntersectionSides != [2]bool{} || len(shared[0].Sources) != 2 {
				t.Fatal(shared)
			}
			if len(q.IsolatedCommonPoints()) != 0 {
				t.Fatal("span endpoints are not isolated")
			}
			if op == AffineRingUnion {
				for _, cap := range []int{q.Area().ExactOperations(), q.Area().ExactOperations() - 1} {
					r, e := ArrangeGeneralAffinePolygons(context.Background(), a, a, trims, op, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{MaxOperations: cap})
					if cap == q.Area().ExactOperations() {
						if e != nil || r == nil {
							t.Fatal(e)
						}
					} else if e == nil || r != nil {
						t.Fatal("partial budget")
					}
				}
			}
		}
		if q, e := ArrangeTransverseAffinePolygons(context.Background(), a, a, trims, AffineRingUnion, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{}); e == nil || q != nil {
			t.Fatal("strict API widened")
		}
	}
}
func TestGeneralAffineCoincidentAndIsolated(t *testing.T) {
	a := generalAffinePlane(t)
	trims := [2]AffinePolygonTrim{{Outer: affineTrimSquare()}, {Outer: affineTrimSquare()}}
	for _, op := range []AffineRingOperation{AffineRingIntersection, AffineRingUnion, AffineRingDifference} {
		q, e := ArrangeGeneralAffinePolygons(context.Background(), a, a, trims, op, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{})
		if e != nil {
			t.Fatal(e)
		}
		want := big.NewRat(1, 1)
		if op == AffineRingDifference {
			want.SetInt64(0)
		}
		if generalAffineArea(q).Cmp(want) != 0 {
			t.Fatal(op, q.Area().Cycles())
		}
		if len(q.SharedBoundarySpans()) != 4 {
			t.Fatal(q.SharedBoundarySpans())
		}
		for _, uses := range q.EdgeSources() {
			if len(uses) != 2 {
				t.Fatal("lost duplicate owner", uses)
			}
		}
		if op == AffineRingUnion {
			sources := q.EdgeSources()
			sources[0][0].Parameter[0].SetInt64(99)
			shared := q.SharedBoundarySpans()
			shared[0].Vertices[0].A[0].SetInt64(99)
			if q.EdgeSources()[0][0].Parameter[0].Cmp(big.NewRat(99, 1)) == 0 || q.SharedBoundarySpans()[0].Vertices[0].A[0].Cmp(big.NewRat(99, 1)) == 0 {
				t.Fatal("mutable provenance")
			}
		}
	}
	trims = [2]AffinePolygonTrim{{Outer: affineRingBox(big.NewRat(0, 1), big.NewRat(1, 2), false)}, {Outer: affineRingBox(big.NewRat(1, 2), big.NewRat(1, 1), false)}}
	q, e := ArrangeGeneralAffinePolygons(context.Background(), a, a, trims, AffineRingIntersection, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	points := q.IsolatedCommonPoints()
	if len(points) != 1 || len(q.Area().Edges()) != 0 || len(q.SharedBoundarySpans()) != 0 || points[0].A[0].Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatal(points)
	}
	if q, e := ArrangeGeneralAffinePolygons(context.Background(), a, a, trims, AffineRingUnion, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{}); e == nil || q != nil {
		t.Fatal("nonmanifold point union accepted")
	}
}
func TestGeneralAffineCrossingAndFilledHole(t *testing.T) {
	a := generalAffinePlane(t)
	trims := [2]AffinePolygonTrim{{Outer: affineRingBox(big.NewRat(0, 1), big.NewRat(3, 4), false)}, {Outer: affineRingBox(big.NewRat(1, 4), big.NewRat(1, 1), false)}}
	for _, op := range []AffineRingOperation{AffineRingIntersection, AffineRingUnion, AffineRingDifference} {
		strict, e := ArrangeTransverseAffinePolygons(context.Background(), a, a, trims, op, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{})
		if e != nil {
			t.Fatal(e)
		}
		q, e := ArrangeGeneralAffinePolygons(context.Background(), a, a, trims, op, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{})
		if e != nil {
			t.Fatal(e)
		}
		area := new(big.Rat)
		for _, c := range strict.Cycles() {
			area.Add(area, c.SignedAreaA)
		}
		if generalAffineArea(q).Cmp(area) != 0 {
			t.Fatal(op, area, generalAffineArea(q))
		}
	}
	hole := affineRingBox(big.NewRat(1, 4), big.NewRat(3, 4), true)
	trims = [2]AffinePolygonTrim{{Outer: affineTrimSquare(), Holes: [][][2]*big.Rat{hole}}, {Outer: affineRingBox(big.NewRat(1, 4), big.NewRat(3, 4), false)}}
	q, e := ArrangeGeneralAffinePolygons(context.Background(), a, a, trims, AffineRingUnion, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{})
	if e != nil || len(q.Area().Cycles()) != 1 || generalAffineArea(q).Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal(q, e)
	}
	q, e = ArrangeGeneralAffinePolygons(context.Background(), a, a, trims, AffineRingIntersection, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{})
	if e != nil || len(q.SharedBoundarySpans()) != 4 || len(q.Area().Edges()) != 0 {
		t.Fatal(q, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if q, e := ArrangeGeneralAffinePolygons(ctx, a, a, trims, AffineRingUnion, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{}); e != context.Canceled || q != nil {
		t.Fatal(q, e)
	}
}

func TestGeneralAffineCoincidentReversedOriginalChart(t *testing.T) {
	a := generalAffinePlane(t)
	c := affineContactPatch(t, func(u, v float64) geom.Vec3 { return geom.Vec3{X: 1 - u, Y: v, Z: 1 - u + 2*v} }, geom.ParameterRectangle{UMax: 1, VMax: 1}, false)
	trims := [2]AffinePolygonTrim{{Outer: affineTrimSquare()}, {Outer: affineTrimSquare()}}
	q, e := ArrangeGeneralAffinePolygons(context.Background(), a, c, trims, AffineRingUnion, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{})
	if e != nil || !q.Complete() || generalAffineArea(q).Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal(q, e)
	}
	for _, uses := range q.EdgeSources() {
		if len(uses) != 2 || uses[0].Owner != 0 || uses[1].Owner != 1 || uses[0].Parameter[0].Cmp(uses[0].Parameter[1]) >= 0 || uses[1].Parameter[0].Cmp(uses[1].Parameter[1]) <= 0 {
			t.Fatal("lost reversed use", uses)
		}
	}
}
