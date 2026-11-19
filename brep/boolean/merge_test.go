package boolean

import (
	"context"
	"math"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// tiles is an arrangement of unit squares of one plane face on an n by n
// grid, with every listed cell a kept piece.
func tiles(t *testing.T, n int, cells [][2]int) (*arrangement, []piece, []kept) {
	t.Helper()
	frame, err := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	a := &arrangement{ctx: context.Background(), t: tolerances{eps: 1e-7, angular: 1e-9, margin: 4e-7, size: float64(n)}}
	a.ops = [2]*operand{{faces: []face{{s: plane{frame}}}}, {}}
	at := func(i, j int) int { return i + j*(n+1) }
	across, up := map[[2]int]int{}, map[[2]int]int{}
	for j := 0; j <= n; j++ {
		for i := 0; i <= n; i++ {
			a.points = append(a.points, point{geom.Vec3{X: float64(i), Y: float64(j)}, a.t.eps})
		}
	}
	for j := 0; j <= n; j++ {
		for i := 0; i <= n; i++ {
			if i < n {
				across[[2]int{i, j}] = len(a.elems)
				a.elems = append(a.elems, element{line{geom.Vec3{X: float64(i), Y: float64(j)}, geom.Vec3{X: 1}}, 0, 1, at(i, j), at(i+1, j)})
			}
			if j < n {
				up[[2]int{i, j}] = len(a.elems)
				a.elems = append(a.elems, element{line{geom.Vec3{X: float64(i), Y: float64(j)}, geom.Vec3{Y: 1}}, 0, 1, at(i, j), at(i, j+1)})
			}
		}
	}
	var pieces []piece
	var keep []kept
	for k, c := range cells {
		i, j := c[0], c[1]
		loop := []hop{{across[[2]int{i, j}], false}, {up[[2]int{i + 1, j}], false}, {across[[2]int{i, j + 1}], true}, {up[[2]int{i, j}], true}}
		pieces = append(pieces, piece{loops: [][]hop{loop}, mate: -1})
		keep = append(keep, kept{piece: k})
	}
	return a, pieces, keep
}

// TestMerge covers M1, M3 and M4 on unit squares of one plane.
func TestMerge(t *testing.T) {
	for _, c := range []struct {
		name   string
		n      int
		cells  [][2]int
		sheets int
		loops  []int // hops of the loops of the first sheet
		edges  int
	}{
		{"two squares become one face", 2, [][2]int{{0, 0}, {1, 0}}, 1, []int{4}, 4},
		{"an L keeps its corner", 2, [][2]int{{0, 0}, {1, 0}, {1, 1}}, 1, []int{6}, 6},
		{"four squares become one", 2, [][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}}, 1, []int{4}, 4},
		{"a ring keeps its hole", 3, [][2]int{{0, 0}, {1, 0}, {2, 0}, {2, 1}, {2, 2}, {1, 2}, {0, 2}, {0, 1}}, 1, []int{4, 4}, 8},
		{"squares that share only a corner stay apart", 2, [][2]int{{0, 0}, {1, 1}}, 2, []int{4}, 8},
		// A ring without one corner cell: merged whole, its boundary
		// would pass twice through the grid point (1,1). M3 keeps the
		// edge of the merge that would close it.
		{"M3", 3, [][2]int{{1, 0}, {2, 0}, {2, 1}, {2, 2}, {1, 2}, {0, 2}, {0, 1}}, 2, nil, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			a, pieces, keep := tiles(t, c.n, c.cells)
			edges, sheets, err := a.coalesce(pieces, keep)
			if err != nil {
				t.Fatal(err)
			}
			if len(sheets) != c.sheets || c.edges != 0 && len(edges) != c.edges {
				t.Fatalf("%d sheets and %d edges, want %d and %d", len(sheets), len(edges), c.sheets, c.edges)
			}
			for k, n := range c.loops {
				if len(sheets[0].loops) != len(c.loops) || len(sheets[0].loops[k]) != n {
					t.Fatalf("loops of the first sheet: %v, want lengths %v", sheets[0].loops, c.loops)
				}
			}
			area := 0.
			for _, sh := range sheets {
				area += chartArea(sh.s.chart(), legsOf(edges, sh.loops))
				// No point is visited twice by the loops of a face, and
				// consecutive hops join.
				seen := map[int]bool{}
				for _, loop := range sh.loops {
					for k, h := range loop {
						from, to := edges[h.elem].a, edges[h.elem].b
						if h.reversed {
							from, to = to, from
						}
						next := cyclic(loop, k+1)
						start := edges[next.elem].a
						if next.reversed {
							start = edges[next.elem].b
						}
						if seen[from] || to != start {
							t.Fatalf("loop %v visits point %d twice or is open", loop, from)
						}
						seen[from] = true
					}
				}
			}
			if math.Abs(area-float64(len(c.cells))) > 1e-12 {
				t.Fatalf("area %g, want %d", area, len(c.cells))
			}
		})
	}
}

// TestMergeRemovesRepeatedCorner covers the other half of M3: a piece
// whose loop passes twice through a point is merged with the neighbour
// that fills the corner between its two parts, and the face that results
// passes through the point once and needs no divider.
func TestMergeRemovesRepeatedCorner(t *testing.T) {
	a, pieces, keep := tiles(t, 2, [][2]int{{0, 0}, {1, 1}, {1, 0}})
	low, high := pieces[0].loops[0], pieces[1].loops[0]
	// The squares (0,0) and (1,1) as one piece: its loop leaves the
	// first square at the point (1,1), goes round the second and
	// comes back.
	pinched := append(append(append([]hop(nil), low[:2]...), high...), low[2:]...)
	pieces = []piece{{loops: [][]hop{pinched}, mate: -1}, pieces[2]}
	if _, repeated, err := a.cornersOf(a.elems, sheet{s: a.ops[0].faces[0].s, loops: pieces[0].loops}); err != nil || !repeated {
		t.Fatalf("the piece does not touch itself: %v", err)
	}
	edges, sheets, err := a.coalesce(pieces, keep[:2])
	if err != nil {
		t.Fatal(err)
	}
	if len(sheets) != 1 || len(sheets[0].loops) != 1 || len(sheets[0].loops[0]) != 6 || len(edges) != 6 {
		t.Fatalf("%d sheets and %d edges, want one face with six edges", len(sheets), len(edges))
	}
	if area := chartArea(sheets[0].s.chart(), legsOf(edges, sheets[0].loops)); math.Abs(area-3) > 1e-12 {
		t.Fatalf("area %g, want 3", area)
	}
}

// TestMergeResult covers M5 through Combine: no two adjacent faces of a
// result lie on one surface, and no vertex is left on a straight edge.
func TestMergeResult(t *testing.T) {
	cube := lifted(t, box(t, 0, 0, 0, 2, 2, 2))
	for _, c := range []struct {
		name                   string
		b                      [6]float64
		op                     Operation
		faces, edges, vertices int
	}{
		{"C6 union is the box again", [6]float64{0, 0, 0, 1, 1, 1}, Union, 6, 12, 8},
		{"C5 cut is the box again", [6]float64{1, 0, 2, 3, 2, 3}, Cut, 6, 12, 8},
		{"C5 union", [6]float64{1, 0, 2, 3, 2, 3}, Union, 10, 24, 16},
		{"C6 cut", [6]float64{0, 0, 0, 1, 1, 1}, Cut, 9, 21, 14},
		{"corner overlap union", [6]float64{1, 1, 1, 3, 3, 3}, Union, 12, 30, 20},
	} {
		t.Run(c.name, func(t *testing.T) {
			b := lifted(t, box(t, c.b[0], c.b[1], c.b[2], c.b[3], c.b[4], c.b[5]))
			r, err := Combine(context.Background(), cube, b, c.op, matrixOptions)
			if err != nil {
				t.Fatal(err)
			}
			if top := r.Solid.Topology(); top.Faces != c.faces || top.Edges != c.edges || top.Vertices != c.vertices || top.EulerCharacteristic != 2 {
				t.Fatalf("%d faces, %d edges, %d vertices, Euler %d; want %d, %d, %d, 2", top.Faces, top.Edges, top.Vertices, top.EulerCharacteristic, c.faces, c.edges, c.vertices)
			}
		})
	}
}

// TestMergeSeam covers M2 and the conditioning of B2 on results with a
// cylinder face.
func TestMergeSeam(t *testing.T) {
	// seams lists, for each face of a solid, the edges it uses twice.
	seams := func(s trimmed.Solid) (total int) {
		for _, f := range s.Faces() {
			seen := map[int]bool{}
			for _, loop := range append([][]trimmed.Use{f.Outer}, f.Holes...) {
				for _, u := range loop {
					if seen[u.Edge] {
						total++
					}
					seen[u.Edge] = true
				}
			}
		}
		return total
	}
	run := func(a, b trimmed.Solid, op Operation) Result {
		t.Helper()
		r, err := Combine(context.Background(), a, b, op, matrixOptions)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	t.Run("a whole cylinder keeps one seam and has quarter arcs", func(t *testing.T) {
		for _, form := range []string{"sf2", "sf3"} {
			r := run(formed(t, form, 0, 0, 0, 1, 2), trimmed.Solid{}, Union)
			top := r.Solid.Topology()
			// Two rims of four arcs each, and the seam between them.
			if top.Faces != 3 || top.Edges != 9 || top.Vertices != 8 || seams(r.Solid) != 1 {
				t.Fatalf("%s: %d faces, %d edges, %d vertices, %d seams", form, top.Faces, top.Edges, top.Vertices, seams(r.Solid))
			}
			for i, e := range r.Solid.Edges() {
				if _, ok := e.Curve.(geom.Ellipse); ok && e.Range.Max-e.Range.Min > math.Pi/2+1e-9 {
					t.Fatalf("%s: edge %d spans %g of a turn", form, i, (e.Range.Max-e.Range.Min)/(2*math.Pi))
				}
			}
		}
	})
	t.Run("seams at different angles are joined by a cut", func(t *testing.T) {
		// Coaxial cylinders of heights [0,2] and [1,3], the second
		// turned about the axis: its seam is elsewhere. The union is one
		// cylinder of three faces, whose cut steps from one seam to the
		// other.
		turn, err := geom.Rotation(geom.Vec3{Z: 1}, 1)
		if err != nil {
			t.Fatal(err)
		}
		b, err := tube(t, 0, 0, 1, 1, 2).Transformed(turn)
		if err != nil {
			t.Fatal(err)
		}
		r := run(tube(t, 0, 0, 0, 1, 2), b, Union)
		if top := r.Solid.Topology(); top.Faces != 3 || math.Abs(r.Volume-3*math.Pi) > 1e-6 || seams(r.Solid) == 0 {
			t.Fatalf("%d faces, volume %g, %d seams", top.Faces, r.Volume, seams(r.Solid))
		}
	})
	t.Run("a face that no longer winds has no seam", func(t *testing.T) {
		// Half of a cylinder: the box takes away the side x < 0, and the
		// seam of the cylinder runs down the middle of what is left.
		r := run(tube(t, 0, 0, 0, 1, 2), lifted(t, box(t, -2, -2, -1, 0, 2, 3)), Cut)
		if top := r.Solid.Topology(); top.Faces != 4 || math.Abs(r.Volume-math.Pi) > 1e-6 || seams(r.Solid) != 0 {
			t.Fatalf("%d faces, volume %g, %d seams", top.Faces, r.Volume, seams(r.Solid))
		}
	})
	t.Run("a bore wall in four stored faces is one face", func(t *testing.T) {
		r := run(cube(t), formed(t, "sf3", 1, 1, 0, 0.5, 2), Cut)
		if top := r.Solid.Topology(); top.Faces != 7 || seams(r.Solid) != 1 {
			t.Fatalf("%d faces, %d seams", top.Faces, seams(r.Solid))
		}
	})
	t.Run("a wall with a window is cut past the window", func(t *testing.T) {
		// A box through one side of a cylinder: the wall keeps winding
		// around the axis, and the seam is interrupted by the box. The
		// cut runs from the bottom rim to the window and from the window
		// to the top rim.
		r := run(lifted(t, box(t, 0.5, 0, 1, 1.5, 4, 1.5)), tube(t, 0.5, 1, 0.5, 0.5, 1.5), Union)
		want := 2 + 0.375*math.Pi - 0.0625*math.Pi
		if top := r.Solid.Topology(); top.Faces != 10 || math.Abs(r.Volume-want) > 1e-6 || seams(r.Solid) != 2 {
			t.Fatalf("%d faces, volume %g (want %g), %d seams", top.Faces, r.Volume, want, seams(r.Solid))
		}
	})
	t.Run("a polar cap under a face", func(t *testing.T) {
		// The cylinder stands under the box. The interior point of the
		// part of the box face over the cap falls on the radial edge of
		// the polar cap, which is not a boundary of the cap (K1).
		r := run(lifted(t, box(t, 0, 0, 2, 3, 4, 2.5)), tube(t, 0.5, 0.5, 0, 0.5, 2), Union)
		if math.Abs(r.Volume-6-0.5*math.Pi) > 1e-6 || r.Solids != 1 {
			t.Fatalf("volume %g, %d solids", r.Volume, r.Solids)
		}
	})
	t.Run("a result is an operand", func(t *testing.T) {
		// Bore a box, then fill the bore again with the same cylinder.
		bored := run(cube(t), tube(t, 1, 1, 0, 0.5, 2), Cut)
		r := run(bored.Solid, tube(t, 1, 1, 0, 0.5, 2), Union)
		if top := r.Solid.Topology(); top.Faces != 6 || math.Abs(r.Volume-8) > 1e-6 {
			t.Fatalf("%d faces, volume %g", top.Faces, r.Volume)
		}
	})
}
