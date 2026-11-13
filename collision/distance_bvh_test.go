package collision

import (
	"context"
	"math"
	"math/rand"
	"plainsolid/brep"
	"plainsolid/geom"
	"testing"
)

func triangleItems(ts [][3]geom.Vec3) []distanceItem {
	out := make([]distanceItem, len(ts))
	for i, t := range ts {
		out[i].t = t
		et := exactTriangle(t)
		for k := 0; k < 3; k++ {
			out[i].lo[k], out[i].hi[k] = et[0][k], et[0][k]
			for j := 1; j < 3; j++ {
				if et[j][k].Cmp(out[i].lo[k]) < 0 {
					out[i].lo[k] = et[j][k]
				}
				if et[j][k].Cmp(out[i].hi[k]) > 0 {
					out[i].hi[k] = et[j][k]
				}
			}
		}
	}
	return out
}
func bruteDistance(aa, bb []distanceItem) (closest, int, int) {
	var best closest
	ai, bi := 0, 0
	for i, a := range aa {
		for j, b := range bb {
			if best.squared != nil && separation(a, b).Cmp(best.squared) >= 0 {
				continue
			}
			c := closestTriangles(a.t, b.t)
			if best.squared == nil || c.squared.Cmp(best.squared) < 0 {
				best, ai, bi = c, i, j
			}
			if best.squared.Sign() == 0 {
				return best, ai, bi
			}
		}
	}
	return best, ai, bi
}
func compareSearch(t *testing.T, aa, bb []distanceItem) {
	t.Helper()
	want, wi, wj := bruteDistance(aa, bb)
	got, ai, bi, pairs, narrow, e := searchDistanceTrees(context.Background(), aa, bb, 100000)
	if e != nil {
		t.Fatal(e)
	}
	if ai != wi || bi != wj || got.squared.Cmp(want.squared) != 0 || !same(got.a, want.a) || !same(got.b, want.b) {
		t.Fatalf("BVH changed lexicographic witness: got (%d,%d), want (%d,%d)", ai, bi, wi, wj)
	}
	again, i, j, p2, n2, e := searchDistanceTrees(context.Background(), aa, bb, 100000)
	if e != nil || i != ai || j != bi || p2 != pairs || n2 != narrow || !same(again.a, got.a) || !same(again.b, got.b) {
		t.Fatal("nondeterministic BVH", e)
	}
}
func TestBVHDifferentialWitnesses(t *testing.T) {
	random := rand.New(rand.NewSource(604))
	for trial := 0; trial < 60; trial++ {
		var a, b [][3]geom.Vec3
		for group := 0; group < 2; group++ {
			for i := 0; i < 9; i++ {
				p := geom.Vec3{X: float64(random.Intn(12)-6) / 2, Y: float64(random.Intn(12)-6) / 2, Z: float64(random.Intn(4) + group*2)}
				tr := [3]geom.Vec3{p, p.Add(geom.Vec3{X: 1}), p.Add(geom.Vec3{Y: 1})}
				if group == 0 {
					a = append(a, tr)
				} else {
					b = append(b, tr)
				}
			}
		}
		compareSearch(t, triangleItems(a), triangleItems(b))
	}
}
func TestBVHExtremeCoordinatesAndTies(t *testing.T) {
	var a, b [][3]geom.Vec3
	for _, x := range []float64{math.MaxFloat64, -math.MaxFloat64, 0, math.MaxFloat64 / 2} {
		a = append(a, [3]geom.Vec3{{X: x}, {X: x, Y: 1}, {X: x, Z: 1}})
		b = append(b, [3]geom.Vec3{{X: x, Y: 2}, {X: x, Y: 3}, {X: x, Y: 2, Z: 1}})
	}
	compareSearch(t, triangleItems(a), triangleItems(b))
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	compareSearch(t, triangleItems(a), triangleItems(b))
}
func TestBoundaryDistanceManySeparatedComponents(t *testing.T) {
	var aa, bb []brep.Solid
	for i := 0; i < 100; i++ {
		x := float64(10 * i)
		aa = append(aa, dbox(t, geom.Vec3{X: x}, geom.Vec3{X: x + 1, Y: 1, Z: 1}))
		bb = append(bb, dbox(t, geom.Vec3{X: 10000 + x}, geom.Vec3{X: 10001 + x, Y: 1, Z: 1}))
	}
	a, b := locationJoin(t, aa...), locationJoin(t, bb...)
	r, e := BoundaryDistance(context.Background(), a, b, DistanceOptions{MaxPairs: 1000})
	if e != nil {
		t.Fatal(e)
	}
	if r.Distance != 9009 {
		t.Fatal(r.Distance)
	}
	if r.Pairs >= 1000 || r.NarrowPhasePairs >= 100 {
		t.Fatal("BVH failed to prune separated components", r.Pairs, r.NarrowPhasePairs)
	}
	t.Logf("1200 x 1200 triangles: %d exact node bounds, %d triangle predicates", r.Pairs, r.NarrowPhasePairs)
	if r.FaceA != a.TriangleFaces()[r.TriangleA] || r.FaceB != b.TriangleFaces()[r.TriangleB] {
		t.Fatal("face ancestry changed")
	}
}
func TestBVHContactTieMatchesFlat(t *testing.T) {
	a := dbox(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 2})
	b := dbox(t, geom.Vec3{X: 1, Y: 1, Z: 1}, geom.Vec3{X: 3, Y: 3, Z: 3})
	aa, e := distanceItems(context.Background(), a, 4096)
	if e != nil {
		t.Fatal(e)
	}
	bb, e := distanceItems(context.Background(), b, 4096)
	if e != nil {
		t.Fatal(e)
	}
	compareSearch(t, aa, bb)
	compareSearch(t, aa, aa)
}

func FuzzBVHMatchesFlatWitness(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 12 {
			return
		}
		var a, b [][3]geom.Vec3
		for i := 0; i < 4; i++ {
			p := geom.Vec3{X: float64(int8(data[3*i])) / 8, Y: float64(int8(data[3*i+1])) / 8, Z: float64(data[3*i+2] % 4)}
			tr := [3]geom.Vec3{p, p.Add(geom.Vec3{X: 1}), p.Add(geom.Vec3{Y: 1})}
			a = append(a, tr)
			q := geom.Vec3{X: p.Y, Y: p.X, Z: p.Z + float64(i%2)}
			b = append(b, [3]geom.Vec3{q, q.Add(geom.Vec3{X: 1}), q.Add(geom.Vec3{Y: 1})})
		}
		compareSearch(t, triangleItems(a), triangleItems(b))
	})
}
