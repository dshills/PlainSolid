package boolean

import (
	"context"
	"errors"
	"testing"

	"plainsolid/brep"
	"plainsolid/geom"
)

// TestPassVertex covers X1, X2 and X4 on pairs of planar solids.
func TestPassVertex(t *testing.T) {
	whole := box(t, 0, 0, 0, 2, 2, 2)
	t.Run("X1 merges coincident vertices", func(t *testing.T) {
		a, err := arranged(t, context.Background(), whole, box(t, 2, 2, 2, 3, 3, 3), nil)
		if err != nil {
			t.Fatal(err)
		}
		shared := 0
		for _, n := range a.nodes {
			if n.into < 0 && len(n.verts[0]) == 1 && len(n.verts[1]) == 1 {
				shared++
				if n.pos != (geom.Vec3{X: 2, Y: 2, Z: 2}) || n.radius != a.t.eps {
					t.Fatalf("shared node %+v", n)
				}
			}
		}
		if shared != 1 {
			t.Fatalf("%d shared nodes, want 1", shared)
		}
	})
	t.Run("X1 filters by the margin, not by the tolerance", func(t *testing.T) {
		// Two vertices 1.5ε apart, each already in a node of radius 2ε
		// whose position is ε away from it: the nodes are 3.5ε apart.
		const eps = 1e-7
		a := &arrangement{ctx: context.Background(), t: tolerances{eps: eps, angular: 1e-9, margin: 4 * eps}, limits: Options{MaxNodes: 8, MaxPairs: 8}}
		for i, xs := range [2][2]float64{{0, 2 * eps}, {3.5 * eps, 5.5 * eps}} {
			first, _ := a.add(geom.Vec3{X: xs[0]})
			second, _ := a.add(geom.Vec3{X: xs[1]})
			n, err := a.merge(first, second)
			if err != nil {
				t.Fatal(err)
			}
			a.vnode[i] = []int{n}
		}
		if err := a.vertexWithVertex(); err != nil {
			t.Fatal(err)
		}
		if a.find(a.vnode[0][0]) != a.find(a.vnode[1][0]) || a.tests != 1 {
			t.Fatalf("the nodes were not found coincident (%d tests)", a.tests)
		}
	})
	t.Run("X2 puts a vertex on an edge", func(t *testing.T) {
		a, err := arranged(t, context.Background(), whole, box(t, 1, 0, 2, 3, 2, 3), nil)
		if err != nil {
			t.Fatal(err)
		}
		split := 0
		for _, s := range a.spans {
			if s.op == 0 && s.t1-s.t0 == 1 {
				split++
			}
		}
		if split != 4 {
			t.Fatalf("%d spans of A of length 1, want 4", split)
		}
	})
	t.Run("X4 records a touching vertex", func(t *testing.T) {
		// A square pyramid standing on its apex inside the top face of A.
		apex := polyhedron(t, []geom.Vec3{{X: 1, Y: 1, Z: 2}, {X: 0.5, Y: 0.5, Z: 3}, {X: 1.5, Y: 0.5, Z: 3}, {X: 1.5, Y: 1.5, Z: 3}, {X: 0.5, Y: 1.5, Z: 3}},
			[][]int{{1, 2, 3, 4}, {0, 2, 1}, {0, 3, 2}, {0, 4, 3}, {0, 1, 4}})
		a, err := arranged(t, context.Background(), whole, apex, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(a.contacts) != 1 || a.contacts[0].point < 0 || len(a.contacts[0].faces[0]) != 1 || len(a.contacts[0].faces[1]) != 4 {
			t.Fatalf("contacts %+v", a.contacts)
		}
		if top := a.contacts[0].faces[0][0]; len(a.free[0][top]) != 1 || a.points[a.free[0][top][0]].pos != (geom.Vec3{X: 1, Y: 1, Z: 2}) {
			t.Fatalf("the apex is not recorded in face %d of A", top)
		}
	})
	t.Run("X4 does not call a flush corner touching", func(t *testing.T) {
		a, err := arranged(t, context.Background(), whole, box(t, 0, 0, 0, 1, 1, 1), nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(a.contacts) != 0 {
			t.Fatalf("contacts %+v", a.contacts)
		}
	})
}

// TestPassVertexTolerance: a vertex between ε and 4ε from an edge is
// tolerance-scale and may not be settled silently.
func TestPassVertexTolerance(t *testing.T) {
	const eps = 1e-7
	var solids [2]brep.Solid
	solids[0] = box(t, 0, 0, 0, 2, 2, 2)
	solids[1] = box(t, 1, 0, 2+2*eps, 3, 2, 3)
	a, err := arranged(t, context.Background(), solids[0], solids[1], nil)
	if err != nil && !errors.Is(err, ErrTolerance) {
		t.Fatalf("got %v, want a sealed arrangement or ErrTolerance", err)
	}
	if err == nil && len(a.points) > 16 {
		t.Fatalf("%d nodes", len(a.points))
	}
}
