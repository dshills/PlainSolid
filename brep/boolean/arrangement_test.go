package boolean

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// taken takes two planar solids through intake.
func taken(t *testing.T, x, y brep.Solid, change func(*Options)) ([2]*operand, tolerances, Options) {
	t.Helper()
	o, err := matrixOptions.normalized()
	if err != nil {
		t.Fatal(err)
	}
	if change != nil {
		change(&o)
	}
	sa, sb := lifted(t, x), lifted(t, y)
	tol, err := working(o, sa, sb, [2]bool{})
	if err != nil {
		t.Fatal(err)
	}
	var ops [2]*operand
	for i, s := range [2]trimmed.Solid{sa, sb} {
		if ops[i], err = intake(i, s, tol); err != nil {
			t.Fatal(err)
		}
	}
	return ops, tol, o
}

// arranged takes two planar solids through intake and the passes below
// face with face, and seals the arrangement.
func arranged(t *testing.T, ctx context.Context, x, y brep.Solid, change func(*Options)) (*arrangement, error) {
	t.Helper()
	ops, tol, o := taken(t, x, y, change)
	a, err := newArrangement(ctx, ops, tol, o)
	if err != nil {
		return nil, err
	}
	if err := a.lower(); err != nil {
		return nil, err
	}
	return a, a.seal()
}

// TestArrangement counts the nodes, spans and bundles of the box pairs of
// C4 to C8, and what each face of A holds.
func TestArrangement(t *testing.T) {
	whole := box(t, 0, 0, 0, 2, 2, 2)
	for _, c := range []struct {
		name                         string
		b                            brep.Solid
		nodes, spans, bundles        int
		lying, loose, along, touches int // totals over the faces of both operands
	}{
		{"C4 whole face shared", box(t, 0, 0, 2, 2, 2, 3), 12, 24, 4, 0, 0, 16, 0},
		{"C5 part of a face shared", box(t, 1, 0, 2, 3, 2, 3), 16, 28, 2, 2, 0, 8, 0},
		{"C6 three faces flush", box(t, 0, 0, 0, 1, 1, 1), 15, 27, 3, 6, 3, 12, 0},
		{"C7 one edge shared", box(t, 2, 0, 2, 3, 2, 3), 14, 24, 1, 0, 0, 4, 0},
		{"C8 one vertex shared", box(t, 2, 2, 2, 3, 3, 3), 15, 24, 0, 0, 0, 0, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			a, err := arranged(t, context.Background(), whole, c.b, nil)
			if err != nil {
				t.Fatal(err)
			}
			total := func(lists [2][][]int) int {
				n := 0
				for _, side := range lists {
					for _, list := range side {
						n += len(list)
					}
				}
				return n
			}
			got := [7]int{len(a.points), len(a.spans), len(a.bundles), total(a.inner), total(a.free), total(a.along), len(a.contacts)}
			if want := [7]int{c.nodes, c.spans, c.bundles, c.lying, c.loose, c.along, c.touches}; got != want {
				t.Fatalf("nodes, spans, bundles, lying, loose, along, touching: got %v, want %v", got, want)
			}
			// Every element joins two points, and every face boundary is
			// a closed chain of hops.
			for i := range a.rims {
				for f, loops := range a.rims[i] {
					for _, loop := range loops {
						for k, h := range loop {
							next := cyclic(loop, k+1)
							end, start := a.elems[h.elem].b, a.elems[next.elem].a
							if h.reversed {
								end = a.elems[h.elem].a
							}
							if next.reversed {
								start = a.elems[next.elem].b
							}
							if end != start {
								t.Fatalf("operand %d face %d: the boundary is open after hop %d", i, f, k)
							}
						}
					}
				}
			}
		})
	}
}

// TestArrangementNodes covers merging (T2), the growth bound (T4) and
// the node limit.
func TestArrangementNodes(t *testing.T) {
	const eps = 1e-7
	fresh := func(max int) *arrangement {
		return &arrangement{t: tolerances{eps: eps, angular: 1e-9, margin: 4 * eps}, limits: Options{MaxNodes: max}}
	}
	a := fresh(8)
	var ids []int
	for _, x := range []float64{0, 1.9 * eps, 3.8 * eps, 5.7 * eps, 7.6 * eps} {
		n, err := a.add(geom.Vec3{X: x})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, n)
	}
	n, err := a.merge(ids[0], ids[1])
	if err != nil {
		t.Fatal(err)
	}
	if got := a.at(n); math.Abs(got.pos.X-0.95*eps) > 1e-20 || math.Abs(got.radius-1.95*eps) > 1e-20 || a.find(ids[1]) != n || a.live != 4 {
		t.Fatalf("merged node %+v, live %d", got, a.live)
	}
	if n, err = a.merge(n, ids[2]); err != nil {
		t.Fatal(err)
	}
	if n, err = a.merge(n, ids[3]); err != nil {
		t.Fatal(err)
	}
	if _, err := a.merge(n, ids[4]); !errors.Is(err, ErrTolerance) {
		t.Fatalf("a chain of five near coincidences: got %v, want ErrTolerance", err)
	}
	b := fresh(2)
	for i := 0; i < 2; i++ {
		if _, err := b.add(geom.Vec3{}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := b.add(geom.Vec3{}); !errors.Is(err, ErrBudget) {
		t.Fatalf("a third node with MaxNodes 2: got %v, want ErrBudget", err)
	}
}

// TestArrangementLimits covers the work limits that the passes check and
// the precedence of a cancelled context over them.
func TestArrangementLimits(t *testing.T) {
	x, y := box(t, 0, 0, 0, 2, 2, 2), box(t, 1, 0, 2, 3, 2, 3)
	if _, err := arranged(t, context.Background(), x, y, func(o *Options) { o.MaxPairs = 3 }); !errors.Is(err, ErrBudget) {
		t.Fatalf("MaxPairs 3: got %v, want ErrBudget", err)
	}
	if _, err := arranged(t, context.Background(), x, y, func(o *Options) { o.MaxNodes = 15 }); !errors.Is(err, ErrBudget) {
		t.Fatalf("MaxNodes 15: got %v, want ErrBudget", err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := arranged(t, cancelled, x, y, func(o *Options) { o.MaxPairs = 3 }); err != context.Canceled {
		t.Fatalf("cancelled with MaxPairs 3: got %v, want the context error", err)
	}
}
