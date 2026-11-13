package collision

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"testing"
)

func dbox(t *testing.T, lo, hi geom.Vec3) brep.Solid {
	t.Helper()
	s, e := brep.Box(lo, hi)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func checkWitness(t *testing.T, r DistanceResult, a, b [3]geom.Vec3) {
	t.Helper()
	pa, pb := point(r.ExactPointA), point(r.ExactPointB)
	d := minus(pa, pb)
	if dot(d, d).Cmp(r.SquaredDistance) != 0 {
		t.Fatal("exact witness distance mismatch")
	}
	for i, p := range []point{pa, pb} {
		tr := a
		if i == 1 {
			tr = b
		}
		q, ok := faceProjection(p, exactTriangle(tr))
		if !ok || !same(q, p) {
			t.Fatal("exact witness outside source triangle")
		}
	}
}
func TestTriangleDistanceCandidates(t *testing.T) {
	a := [3]geom.Vec3{{}, {X: 2}, {Y: 2}}
	for _, c := range []struct {
		name string
		b    [3]geom.Vec3
		want float64
	}{{"face", [3]geom.Vec3{{X: .2, Y: .2, Z: 3}, {X: .3, Y: .2, Z: 4}, {X: .2, Y: .3, Z: 4}}, 3}, {"edge", [3]geom.Vec3{{X: 3, Y: 0}, {X: 3, Y: 1}, {X: 4, Y: 0}}, 1}, {"pierce", [3]geom.Vec3{{X: .5, Y: .5, Z: -1}, {X: .5, Y: .5, Z: 1}, {X: 1, Y: .5, Z: 0}}, 0}, {"coplanar", [3]geom.Vec3{{X: .2, Y: .2}, {X: 1, Y: .2}, {X: .2, Y: 1}}, 0}} {
		t.Run(c.name, func(t *testing.T) {
			r, e := TriangleDistance(a, c.b)
			if e != nil {
				t.Fatal(e)
			}
			if math.Abs(r.Distance-c.want) > 1e-13 {
				t.Fatal(r.Distance, c.want)
			}
			checkWitness(t, r, a, c.b)
			rr, e := TriangleDistance(c.b, a)
			if e != nil || rr.SquaredDistance.Cmp(r.SquaredDistance) != 0 {
				t.Fatal(rr, e)
			}
		})
	}
	// Closest points occur strictly within two skew edges, with neither input
	// vertex projecting inside the opposite triangle.
	a = [3]geom.Vec3{{X: -1}, {X: 1}, {Y: -.5}}
	b := [3]geom.Vec3{{Y: -1, Z: 2}, {Y: 1, Z: 2}, {X: 1, Y: 0, Z: 3}}
	r, e := TriangleDistance(a, b)
	if e != nil || r.SquaredDistance.Cmp(big.NewRat(4, 1)) != 0 {
		t.Fatal(r, e)
	}
	checkWitness(t, r, a, b)
}
func TestBoundaryDistanceBoxes(t *testing.T) {
	a := dbox(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 2})
	for _, c := range []struct {
		lo, hi geom.Vec3
		want   float64
	}{{geom.Vec3{X: 3, Y: 4, Z: 5}, geom.Vec3{X: 4, Y: 5, Z: 6}, math.Sqrt(14)}, {geom.Vec3{X: .5, Y: .5, Z: .5}, geom.Vec3{X: 1.5, Y: 1.5, Z: 1.5}, .5}, {geom.Vec3{X: 2}, geom.Vec3{X: 3, Y: 1, Z: 1}, 0}, {geom.Vec3{X: 1, Y: 1, Z: 1}, geom.Vec3{X: 3, Y: 3, Z: 3}, 0}} {
		b := dbox(t, c.lo, c.hi)
		r, e := BoundaryDistance(context.Background(), a, b, DistanceOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if math.Abs(r.Distance-c.want) > 1e-13 {
			t.Fatal(r.Distance, c.want)
		}
		if r.FaceA != a.TriangleFaces()[r.TriangleA] || r.FaceB != b.TriangleFaces()[r.TriangleB] {
			t.Fatal("ancestry")
		}
		am, bm := a.Tessellate(), b.Tessellate()
		at, bt := am.Triangles[r.TriangleA], bm.Triangles[r.TriangleB]
		checkWitness(t, r, [3]geom.Vec3{am.Vertices[at[0]], am.Vertices[at[1]], am.Vertices[at[2]]}, [3]geom.Vec3{bm.Vertices[bt[0]], bm.Vertices[bt[1]], bm.Vertices[bt[2]]})
	}
}
func TestDistanceConcaveAndHoleFaces(t *testing.T) {
	outer := []geom.Vec3{{}, {X: 4}, {X: 4, Y: 1}, {X: 1, Y: 1}, {X: 1, Y: 4}, {Y: 4}}
	a, e := brep.Prism(outer, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	b := dbox(t, geom.Vec3{X: 2, Y: 2, Z: .25}, geom.Vec3{X: 3, Y: 3, Z: .75})
	r, e := BoundaryDistance(context.Background(), a, b, DistanceOptions{})
	if e != nil || r.Distance != 1 {
		t.Fatal(r.Distance, e)
	}
	a, e = brep.PrismWithHoles([]geom.Vec3{{}, {X: 6}, {X: 6, Y: 6}, {Y: 6}}, [][]geom.Vec3{{{X: 2, Y: 2}, {X: 2, Y: 4}, {X: 4, Y: 4}, {X: 4, Y: 2}}}, geom.Vec3{Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	b = dbox(t, geom.Vec3{X: 2.5, Y: 2.5, Z: .5}, geom.Vec3{X: 3.5, Y: 3.5, Z: 1.5})
	r, e = BoundaryDistance(context.Background(), a, b, DistanceOptions{})
	if e != nil || r.Distance != .5 {
		t.Fatal(r.Distance, e)
	}
}
func TestDistanceNearContactAndTransforms(t *testing.T) {
	a := dbox(t, geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	next := math.Nextafter(1, 2)
	b := dbox(t, geom.Vec3{X: next}, geom.Vec3{X: 2, Y: 1, Z: 1})
	r, e := BoundaryDistance(context.Background(), a, b, DistanceOptions{})
	if e != nil || r.Distance != next-1 {
		t.Fatal(r.Distance, e)
	}
	a = dbox(t, geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	b = dbox(t, geom.Vec3{X: 2}, geom.Vec3{X: 3, Y: 1, Z: 1})
	xf := geom.Affine{{0, -3, 0, 12}, {3, 0, 0, -4}, {0, 0, 3, 7}}
	a, e = a.Transformed(xf)
	if e != nil {
		t.Fatal(e)
	}
	b, e = b.Transformed(xf)
	if e != nil {
		t.Fatal(e)
	}
	r, e = BoundaryDistance(context.Background(), a, b, DistanceOptions{})
	if e != nil || r.Distance != 3 {
		t.Fatal(r.Distance, e)
	}
	ta := [3]geom.Vec3{{}, {X: 1}, {Y: 1}}
	tb := [3]geom.Vec3{{Z: math.SmallestNonzeroFloat64}, {X: 1, Z: math.SmallestNonzeroFloat64}, {Y: 1, Z: math.SmallestNonzeroFloat64}}
	r, e = TriangleDistance(ta, tb)
	if e != nil || r.Distance != math.SmallestNonzeroFloat64 {
		t.Fatal(r.Distance, e)
	}
	checkWitness(t, r, ta, tb)
}
func TestDistanceLimits(t *testing.T) {
	a := dbox(t, geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	b := dbox(t, geom.Vec3{X: 2}, geom.Vec3{X: 3, Y: 1, Z: 1})
	for _, o := range []DistanceOptions{{MaxPairs: 1}, {MaxPairs: -1}} {
		r, e := BoundaryDistance(context.Background(), a, b, o)
		if e == nil || r.SquaredDistance != nil {
			t.Fatal("partial result", e)
		}
	}
	if _, e := BoundaryDistance(context.Background(), a, brep.Solid{}, DistanceOptions{}); !errors.Is(e, ErrEmpty) {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := BoundaryDistance(ctx, a, b, DistanceOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e := BoundaryDistance(nil, a, b, DistanceOptions{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	if _, e := TriangleDistance([3]geom.Vec3{}, [3]geom.Vec3{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}

func FuzzTriangleDistanceWitness(f *testing.F) {
	f.Add([]byte{0, 0, 0, 4, 0, 0, 0, 4, 0, 1, 1, 3, 2, 1, 3, 1, 2, 3})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 18 {
			return
		}
		var a, b [3]geom.Vec3
		for i := 0; i < 3; i++ {
			a[i] = geom.Vec3{X: float64(int8(data[3*i])) / 8, Y: float64(int8(data[3*i+1])) / 8, Z: float64(int8(data[3*i+2])) / 8}
			b[i] = geom.Vec3{X: float64(int8(data[9+3*i])) / 8, Y: float64(int8(data[10+3*i])) / 8, Z: float64(int8(data[11+3*i])) / 8}
		}
		if validateTriangle(a) != nil || validateTriangle(b) != nil {
			return
		}
		r, e := TriangleDistance(a, b)
		if e != nil {
			t.Fatal(e)
		}
		checkWitness(t, r, a, b)
		rr, e := TriangleDistance(b, a)
		if e != nil || r.SquaredDistance.Cmp(rr.SquaredDistance) != 0 {
			t.Fatal("asymmetry", e)
		}
		kind, _ := triangleIntersection(a, b)
		if (kind == Disjoint) != (r.SquaredDistance.Sign() > 0) {
			t.Fatal("intersection inconsistency")
		}
		for i := range a {
			a[i] = geom.Vec3{X: 2*a[i].Z + 4, Y: 2*a[i].X - 2, Z: 2*a[i].Y + 1}
			b[i] = geom.Vec3{X: 2*b[i].Z + 4, Y: 2*b[i].X - 2, Z: 2*b[i].Y + 1}
		}
		rr, e = TriangleDistance(a, b)
		if e != nil || rr.SquaredDistance.Cmp(mul(r.SquaredDistance, big.NewRat(4, 1))) != 0 {
			t.Fatal("affine covariance", e)
		}
	})
}

func TestDistanceFloatRangeAndInputBudget(t *testing.T) {
	a := [3]geom.Vec3{{X: -math.MaxFloat64}, {X: -math.MaxFloat64, Y: 1}, {X: -math.MaxFloat64, Z: 1}}
	b := [3]geom.Vec3{{X: math.MaxFloat64}, {X: math.MaxFloat64, Y: 1}, {X: math.MaxFloat64, Z: 1}}
	if _, e := TriangleDistance(a, b); !errors.Is(e, ErrDistanceRange) {
		t.Fatal(e)
	}
	eps := math.SmallestNonzeroFloat64
	a = [3]geom.Vec3{{Y: .25, Z: .5}, {X: -eps, Y: .25, Z: .5}, {Y: .5, Z: .5}}
	b = [3]geom.Vec3{{}, {X: eps, Z: 1}, {Y: 1}}
	if _, e := TriangleDistance(a, b); !errors.Is(e, ErrDistanceRange) {
		t.Fatal(e)
	}
	s := dbox(t, geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	if _, e := BoundaryDistance(context.Background(), s, s, DistanceOptions{MaxTriangles: 1}); !errors.Is(e, ErrLimit) {
		t.Fatal(e)
	}
	if _, e := BoundaryDistance(context.Background(), s, s, DistanceOptions{MaxTriangles: -1}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}

func TestSkewInteriorRationalDistance(t *testing.T) {
	a := [3]geom.Vec3{{X: -1}, {X: 1}, {Y: .1}}
	b := [3]geom.Vec3{{Y: -2, Z: 1}, {Y: 2, Z: 3}, {Z: 100}}
	r, e := TriangleDistance(a, b)
	if e != nil || r.SquaredDistance.Cmp(big.NewRat(16, 5)) != 0 {
		t.Fatal(r.SquaredDistance, e)
	}
	checkWitness(t, r, a, b)
	if r.ExactPointA[0].Sign() != 0 || r.ExactPointA[1].Sign() != 0 || r.ExactPointB[1].Cmp(big.NewRat(-4, 5)) != 0 || r.ExactPointB[2].Cmp(big.NewRat(8, 5)) != 0 {
		t.Fatal("wrong interior witnesses")
	}
}
