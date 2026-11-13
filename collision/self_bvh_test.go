package collision

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"plainsolid/geom"
	"plainsolid/mesh"
	"reflect"
	"sync/atomic"
	"testing"
)

func TestSelfBVHDifferentialExactContacts(t *testing.T) {
	box, e := mesh.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	fixtures := []mesh.Mesh{box}
	crossing := box
	crossing.Vertices = append([]geom.Vec3(nil), box.Vertices...)
	crossing.Vertices[6] = geom.Vec3{X: .5, Y: .5, Z: -.5}
	fixtures = append(fixtures, crossing)
	random := rand.New(rand.NewSource(48))
	for fixture := 0; fixture < 10; fixture++ {
		m := mesh.Mesh{}
		for len(m.Triangles) < 12 {
			var p [3]geom.Vec3
			for i := range p {
				p[i] = geom.Vec3{X: float64(random.Intn(9) - 4), Y: float64(random.Intn(9) - 4), Z: float64(random.Intn(9) - 4)}
			}
			if validateTriangle(p) != nil {
				continue
			}
			n := len(m.Vertices)
			m.Vertices = append(m.Vertices, p[:]...)
			m.Triangles = append(m.Triangles, [3]int{n, n + 1, n + 2})
		}
		fixtures = append(fixtures, m)
	}
	fixtures = append(fixtures, mesh.Mesh{Vertices: []geom.Vec3{{}, {X: 1}, {Y: 1}, {Z: math.SmallestNonzeroFloat64}, {X: 1, Z: math.SmallestNonzeroFloat64}, {Y: 1, Z: math.SmallestNonzeroFloat64}}, Triangles: [][3]int{{0, 1, 2}, {3, 4, 5}}})
	for i, m := range fixtures {
		sweep, e := SelfIntersections(m, Options{})
		if e != nil {
			t.Fatal(e)
		}
		tree, e := SelfIntersectionsBVHContext(context.Background(), m, Options{})
		if e != nil || !tree.Complete {
			t.Fatal(i, e)
		}
		if !reflect.DeepEqual(sweep.Contacts, tree.Contacts) {
			t.Fatal("exact contact disagreement", i, sweep.Contacts, tree.Contacts)
		}
		repeat, e := SelfIntersectionsBVHContext(context.Background(), m, Options{})
		if e != nil || !reflect.DeepEqual(tree, repeat) {
			t.Fatal("nondeterministic BVH", i, e)
		}
	}
}

func TestSelfBVHSeparatesLongThinTriangles(t *testing.T) {
	m := mesh.Mesh{}
	for i := 0; i < 500; i++ {
		y := float64((i*137)%500) * 2
		n := len(m.Vertices)
		m.Vertices = append(m.Vertices, geom.Vec3{Y: y}, geom.Vec3{X: 10000, Y: y}, geom.Vec3{Y: y + .5, Z: 1})
		m.Triangles = append(m.Triangles, [3]int{n, n + 1, n + 2})
	}
	sweep, e := SelfIntersections(m, Options{MaxPairs: 10000})
	if !errors.Is(e, ErrLimit) || sweep.Complete {
		t.Fatal("fixture does not stress sweep", sweep, e)
	}
	report, e := SelfIntersectionsBVHContext(context.Background(), m, Options{MaxPairs: 10000})
	if e != nil || !report.Complete || len(report.Contacts) != 0 || report.NarrowPhasePairs != 0 || report.BroadPhasePairs >= 3000 {
		t.Fatal(report, e)
	}
	partial, e := SelfIntersectionsBVHContext(context.Background(), m, Options{MaxPairs: 1})
	if !errors.Is(e, ErrLimit) || partial.Complete {
		t.Fatal("false completion on traversal budget", partial, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	partial, e = SelfIntersectionsBVHContext(ctx, m, Options{})
	if !errors.Is(e, context.Canceled) || partial.Complete {
		t.Fatal(e)
	}
	t.Logf("BVH %d charged visits/candidates; sweep exhausted 10000 comparisons", report.BroadPhasePairs)
}

type bvhCancelAfter struct {
	context.Context
	calls atomic.Int32
	after int32
}

func (c *bvhCancelAfter) Err() error {
	if c.calls.Add(1) > c.after {
		return context.Canceled
	}
	return nil
}
func TestSelfBVHCancellationDuringWork(t *testing.T) {
	m := mesh.Mesh{}
	for i := 0; i < 500; i++ {
		n := len(m.Vertices)
		y := float64(i) * 2
		m.Vertices = append(m.Vertices, geom.Vec3{Y: y}, geom.Vec3{X: 10000, Y: y}, geom.Vec3{Y: y + .5, Z: 1})
		m.Triangles = append(m.Triangles, [3]int{n, n + 1, n + 2})
	}
	for _, after := range []int32{2100, 2400} {
		ctx := &bvhCancelAfter{Context: context.Background(), after: after}
		report, e := SelfIntersectionsBVHContext(ctx, m, Options{})
		if !errors.Is(e, context.Canceled) || report.Complete {
			t.Fatal("ignored cancellation during work", after, report, e)
		}
		if after == 2400 && report.BroadPhasePairs == 0 {
			t.Fatal("fixture did not reach traversal")
		}
	}
}
