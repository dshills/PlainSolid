package intersect

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func affineRect(x0, y0, x1, y1 *big.Rat, hole bool) [][2]*big.Rat {
	r := [][2]*big.Rat{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}
	if hole {
		r[1], r[3] = r[3], r[1]
	}
	return r
}
func TestTransverseAffinePolygonArrangement(t *testing.T) {
	ctx := context.Background()
	d := geom.ParameterRectangle{UMax: 1, VMax: 1}
	embed := func(u, v float64) geom.Vec3 { return geom.Vec3{X: u, Y: v, Z: u + 2*v} }
	a := affineContactPatch(t, embed, d, true)
	c := affineContactPatch(t, embed, d, false)
	trims := [2]AffinePolygonTrim{{Outer: affineRingBox(big.NewRat(0, 1), big.NewRat(3, 4), false), Holes: [][][2]*big.Rat{affineRingBox(big.NewRat(1, 16), big.NewRat(1, 8), true)}}, {Outer: affineRingBox(big.NewRat(1, 4), big.NewRat(1, 1), false), Holes: [][][2]*big.Rat{affineRingBox(big.NewRat(1, 2), big.NewRat(5, 8), true)}}}
	for _, tc := range []struct {
		op     AffineRingOperation
		area   *big.Rat
		cycles int
	}{{AffineRingIntersection, big.NewRat(15, 64), 2}, {AffineRingUnion, big.NewRat(223, 256), 2}, {AffineRingDifference, big.NewRat(83, 256), 3}} {
		q, e := ArrangeTransverseAffinePolygons(ctx, a, c, trims, tc.op, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if !q.Complete() {
			t.Fatal("incomplete")
		}
		vertices, edges, cycles := q.Vertices(), q.Edges(), q.Cycles()
		if len(cycles) != tc.cycles {
			t.Fatal(tc.op, cycles)
		}
		area := new(big.Rat)
		seen := make([]bool, len(edges))
		for _, cycle := range cycles {
			area.Add(area, cycle.SignedAreaA)
			for i, ei := range cycle.Edges {
				if seen[ei] {
					t.Fatal("duplicate edge")
				}
				seen[ei] = true
				if edges[ei].End != edges[cycle.Edges[(i+1)%len(cycle.Edges)]].Start {
					t.Fatal("open cycle")
				}
			}
		}
		if area.Cmp(tc.area) != 0 {
			t.Fatal(tc.op, area, tc.area)
		}
		for i, edge := range edges {
			if !seen[i] {
				t.Fatal("omitted edge")
			}
			rings := append([][][2]*big.Rat{trims[edge.Owner].Outer}, trims[edge.Owner].Holes...)
			ring := rings[edge.Ring]
			start, end := ring[edge.Edge], ring[(edge.Edge+1)%len(ring)]
			for k, node := range []int{edge.Start, edge.End} {
				for axis := 0; axis < 2; axis++ {
					want := new(big.Rat).Add(start[axis], new(big.Rat).Mul(edge.Parameter[k], new(big.Rat).Sub(end[axis], start[axis])))
					if edge.UV[k][axis].Cmp(want) != 0 || vertices[node].A[axis].Cmp(want) != 0 {
						t.Fatal("original parameter", edge)
					}
				}
				v := vertices[node]
				wantZ := new(big.Rat).Add(v.A[0], new(big.Rat).Mul(big.NewRat(2, 1), v.A[1]))
				if v.Point[2].Cmp(wantZ) != 0 {
					t.Fatal("model source", v)
				}
			}
		}
		if tc.op == AffineRingIntersection {
			for _, cap := range []int{q.ExactOperations(), q.ExactOperations() - 1} {
				out, e := ArrangeTransverseAffinePolygons(ctx, a, c, trims, tc.op, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{MaxOperations: cap})
				if cap == q.ExactOperations() {
					if e != nil || out == nil {
						t.Fatal(e)
					}
				} else if e == nil || out != nil {
					t.Fatal("partial budget")
				}
			}
			vertices[0].A[0].SetInt64(99)
			edges[0].Parameter[0].SetInt64(99)
			cycles[0].Edges[0] = 99
			if q.Vertices()[0].A[0].Cmp(big.NewRat(99, 1)) == 0 || q.Edges()[0].Parameter[0].Cmp(big.NewRat(99, 1)) == 0 || q.Cycles()[0].Edges[0] == 99 {
				t.Fatal("mutable publication")
			}
		}
	}
}
func TestAffinePolygonArrangementReversalAndReject(t *testing.T) {
	d := geom.ParameterRectangle{UMax: 1, VMax: 1}
	a := affineContactPatch(t, func(u, v float64) geom.Vec3 { return geom.Vec3{X: u, Y: v} }, d, false)
	c := affineContactPatch(t, func(u, v float64) geom.Vec3 { return geom.Vec3{X: 1 - u, Y: v} }, d, true)
	trims := [2]AffinePolygonTrim{{Outer: affineRingBox(big.NewRat(0, 1), big.NewRat(3, 4), false)}, {Outer: affineRect(big.NewRat(0, 1), big.NewRat(1, 4), big.NewRat(3, 4), big.NewRat(1, 1), false)}}
	q, e := ArrangeTransverseAffinePolygons(context.Background(), a, c, trims, AffineRingIntersection, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cycles := q.Cycles()
	if len(cycles) != 1 || cycles[0].SignedAreaA.Cmp(big.NewRat(1, 4)) != 0 {
		t.Fatal(cycles)
	}
	seenReverse := false
	for _, edge := range q.Edges() {
		if edge.Owner == 1 {
			if edge.Parameter[0].Cmp(edge.Parameter[1]) >= 0 {
				seenReverse = true
			} else {
				t.Fatal("wrong B orientation", edge)
			}
		}
	}
	if !seenReverse {
		t.Fatal("missing reversed source")
	}
	for _, lo := range []*big.Rat{big.NewRat(3, 4), big.NewRat(0, 1)} {
		trims[1].Outer = affineRingBox(lo, big.NewRat(1, 1), false)
		out, e := ArrangeTransverseAffinePolygons(context.Background(), a, a, trims, AffineRingUnion, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{})
		if e == nil || out != nil {
			t.Fatal("touching/coincident boundary accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if out, e := ArrangeTransverseAffinePolygons(ctx, a, c, trims, AffineRingIntersection, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{}); e != context.Canceled || out != nil {
		t.Fatal(out, e)
	}
}

func TestAffinePolygonArrangementCrossesHole(t *testing.T) {
	d := geom.ParameterRectangle{UMax: 1, VMax: 1}
	a := affineContactPatch(t, func(u, v float64) geom.Vec3 { return geom.Vec3{X: u, Y: v} }, d, false)
	trims := [2]AffinePolygonTrim{{Outer: affineTrimSquare(), Holes: [][][2]*big.Rat{affineRingBox(big.NewRat(1, 4), big.NewRat(3, 4), true)}}, {Outer: affineRect(big.NewRat(1, 2), big.NewRat(1, 8), big.NewRat(7, 8), big.NewRat(7, 8), false)}}
	for _, tc := range []struct {
		op     AffineRingOperation
		area   *big.Rat
		cycles int
	}{{AffineRingIntersection, big.NewRat(5, 32), 1}, {AffineRingUnion, big.NewRat(7, 8), 2}, {AffineRingDifference, big.NewRat(19, 32), 2}} {
		q, e := ArrangeTransverseAffinePolygons(context.Background(), a, a, trims, tc.op, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{})
		if e != nil {
			t.Fatal(e)
		}
		cycles := q.Cycles()
		area := new(big.Rat)
		for _, cycle := range cycles {
			area.Add(area, cycle.SignedAreaA)
		}
		if len(cycles) != tc.cycles || area.Cmp(tc.area) != 0 {
			t.Fatal(tc.op, cycles, area)
		}
		splitHole := false
		for _, edge := range q.Edges() {
			if edge.Owner == 0 && edge.Ring == 1 && (edge.Parameter[0].Cmp(big.NewRat(1, 2)) == 0 || edge.Parameter[1].Cmp(big.NewRat(1, 2)) == 0) {
				splitHole = true
			}
		}
		if !splitHole {
			t.Fatal("missing original split hole provenance")
		}
	}
}
