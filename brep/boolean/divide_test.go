package boolean

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/geom"
)

// flat is an arrangement that holds one face of the plane z = 0, given by
// its loops as chart points. Equal points are one point, and a segment
// between two points is one edge whichever way a loop walks it. A loop
// may hold, in place of a point, a circle that starts and ends at the
// point before it: its centre with a negative radius in Z.
func flat(t *testing.T, loops [][]geom.Vec3) (*arrangement, []element, sheet) {
	t.Helper()
	frame, err := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	a := &arrangement{ctx: context.Background(), t: tolerances{eps: 1e-7, angular: 1e-9, margin: 4e-7, size: 20}}
	sh := sheet{s: plane{frame}, sources: [][2]int{{0, 0}}}
	point := func(p geom.Vec3) int {
		for i, q := range a.points {
			if q.pos == p {
				return i
			}
		}
		a.points = append(a.points, point{p, a.t.eps})
		return len(a.points) - 1
	}
	var edges []element
	for _, loop := range loops {
		var hops []hop
		var corners []geom.Vec3
		for _, p := range loop {
			if p.Z < 0 {
				// A circle walked clockwise from the corner before it.
				at := corners[len(corners)-1]
				centre := geom.Vec3{X: p.X, Y: p.Y}
				x, _ := at.Sub(centre).Unit()
				k := conic{centre, x, geom.Vec3{X: x.Y, Y: -x.X}, -p.Z, -p.Z}
				edges = append(edges, element{k, 0, 2 * math.Pi, point(at), point(at)})
				hops = append(hops, hop{len(edges) - 1, false})
				continue
			}
			corners = append(corners, p)
		}
		var straight []hop
		for i, p := range corners {
			q := cyclic(corners, i+1)
			from, to := point(p), point(q)
			found := -1
			for e, el := range edges {
				if el.c.period() == 0 && (el.a == from && el.b == to || el.a == to && el.b == from) {
					found = e
				}
			}
			if found < 0 {
				dir, _ := q.Sub(p).Unit()
				edges = append(edges, element{line{p, dir}, 0, q.Distance(p), from, to})
				found = len(edges) - 1
			}
			straight = append(straight, hop{found, edges[found].a != from})
		}
		// Put each circle after the straight hop that arrives at its point.
		for _, h := range hops {
			for i, s := range straight {
				end := edges[s.elem].b
				if s.reversed {
					end = edges[s.elem].a
				}
				if end == edges[h.elem].a {
					straight = append(straight[:i+1], append([]hop{h}, straight[i+1:]...)...)
					break
				}
			}
		}
		sh.loops = append(sh.loops, straight)
	}
	return a, edges, sh
}

// TestDivide covers M6 on faces built directly.
func TestDivide(t *testing.T) {
	v := func(x, y float64) geom.Vec3 { return geom.Vec3{X: x, Y: y} }
	for _, c := range []struct {
		name     string
		loops    [][]geom.Vec3
		dividers [][2]geom.Vec3
		faces    int
		holes    int // holes over all faces afterwards
		area     float64
		err      error
	}{
		{"a rectangle less a triangle that touches its edge",
			[][]geom.Vec3{{v(0, 0), v(1, 0), v(0.5, 1), v(1.5, 1), v(1, 0), v(4, 0), v(4, 4), v(0, 4)}},
			[][2]geom.Vec3{{v(0, 0), v(0.5, 1)}}, 2, 0, 15.5, nil},
		{"the shortest chord continues a side and is passed over",
			[][]geom.Vec3{{v(0, 2), v(8, 2), v(8, 3), v(2, 6), v(5, 9), v(8, 3), v(8, 12), v(0, 12)}},
			[][2]geom.Vec3{{v(0, 2), v(2, 6)}}, 2, 0, 80 - 13.5, nil},
		{"a loop that passes three times through one point",
			[][]geom.Vec3{{v(0, 0), v(2, 0), v(0.5, 1), v(1.5, 1.5), v(2, 0), v(2.5, 1.5), v(3.5, 1), v(2, 0), v(4, 0), v(4, 4), v(0, 4)}},
			nil, 3, 0, 16 - 0.875 - 0.875, nil},
		{"a hole that touches itself, and two chords of one length",
			[][]geom.Vec3{{v(0, 0), v(6, 0), v(6, 6), v(0, 6)}, {v(3, 3), v(1, 2), v(1, 4), v(3, 3), v(5, 4), v(5, 2)}},
			[][2]geom.Vec3{{v(1, 2), v(5, 2)}}, 2, 1, 36 - 4, nil},
		{"a round hole that touches an edge has no chord",
			[][]geom.Vec3{{v(0, 0), v(2, 0), geom.Vec3{X: 2, Y: 1, Z: -1}, v(4, 0), v(4, 4), v(0, 4)}},
			nil, 0, 0, 0, ErrUnsupported},
		{"two loops at one point",
			[][]geom.Vec3{{v(0, 0), v(2, 0), v(4, 0), v(4, 4), v(0, 4)}, {v(2, 0), v(3, 2), v(1, 2)}},
			nil, 0, 0, 0, ErrInconsistent},
		{"a face that does not touch itself is left alone",
			[][]geom.Vec3{{v(0, 0), v(4, 0), v(4, 4), v(0, 4)}, {v(1, 1), v(1, 2), v(2, 1)}},
			nil, 1, 1, 15.5, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			a, edges, sh := flat(t, c.loops)
			before := len(edges)
			edges, faces, err := a.divide(edges, []sheet{sh})
			if !errors.Is(err, c.err) || c.err != nil {
				if !errors.Is(err, c.err) {
					t.Fatalf("got %v, want %v", err, c.err)
				}
				return
			}
			holes, area := 0, 0.
			for _, f := range faces {
				holes += len(f.loops) - 1
				area += chartArea(f.s.chart(), legsOf(edges, f.loops))
				if _, repeated, err := a.cornersOf(edges, f); err != nil || repeated || len(f.sources) != 1 {
					t.Fatalf("a face still touches itself, or lost its source: %v", err)
				}
			}
			if len(faces) != c.faces || holes != c.holes || math.Abs(area-c.area) > 1e-9 || len(edges)-before != c.faces-1 {
				t.Fatalf("%d faces, %d holes, area %g, %d dividers; want %d, %d, %g, %d", len(faces), holes, area, len(edges)-before, c.faces, c.holes, c.area, c.faces-1)
			}
			for i, want := range c.dividers {
				d := edges[before+i]
				p, q := a.points[d.a].pos, a.points[d.b].pos
				if forward, back := p == want[0] && q == want[1], p == want[1] && q == want[0]; !forward && !back {
					t.Fatalf("divider %d runs from %v to %v, want %v to %v", i, p, q, want[0], want[1])
				}
			}
		})
	}
}
