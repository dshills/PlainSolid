package boolean

import (
	"context"
	"math"
	"testing"

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
