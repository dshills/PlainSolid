package trimmed

import (
	"context"
	"math"
	"strings"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// planarShell builds faces on planes through the listed polygons, which are
// vertex indices wound counterclockwise seen from outside. Edges are lines
// shared by index pair. A polygon that spans no area takes its outward
// direction from normals.
func planarShell(t *testing.T, vertices []geom.Vec3, polygons [][]int, normals map[int]geom.Vec3) ([]parametric.Edge, []Face) {
	t.Helper()
	var edges []parametric.Edge
	lookup := map[[2]int]int{}
	faces := make([]Face, len(polygons))
	for fi, polygon := range polygons {
		normal, ok := normals[fi]
		for i, a := range polygon {
			if !ok {
				normal = normal.Add(vertices[a].Cross(vertices[polygon[(i+1)%len(polygon)]]))
			}
		}
		frame, err := geom.FrameFromNormal(vertices[polygon[0]], normal)
		if err != nil {
			t.Fatal(err)
		}
		surface, err := geom.NewPlaneSurface(frame)
		if err != nil {
			t.Fatal(err)
		}
		faces[fi].Surface = surface
		locate := func(vertex int) geom.Vec2 {
			p := frame.ToLocal(vertices[vertex])
			return geom.Vec2{X: p.X, Y: p.Y}
		}
		for i, a := range polygon {
			b := polygon[(i+1)%len(polygon)]
			key := [2]int{min(a, b), max(a, b)}
			id, ok := lookup[key]
			if !ok {
				d := vertices[key[1]].Sub(vertices[key[0]])
				line, err := geom.NewLine(vertices[key[0]], d)
				if err != nil {
					t.Fatal(err)
				}
				id = len(edges)
				lookup[key] = id
				edges = append(edges, parametric.Edge{Start: key[0], End: key[1], Curve: line, Range: parametric.Interval{Max: d.Length()}})
			}
			faces[fi].Outer = append(faces[fi].Outer, Use{Edge: id, Reversed: a > b, UVStart: locate(a), UVEnd: locate(b)})
		}
	}
	return edges, faces
}

// moveCorner gives corner i of the outer loop new chart coordinates.
func moveCorner(f *Face, i int, uv geom.Vec2) {
	n := len(f.Outer)
	f.Outer[i].UVStart, f.Outer[(i+n-1)%n].UVEnd = uv, uv
}

// pushCorner moves corner i of the outer loop in the chart by the given
// distance to the right of the chord from the corner before it to the
// corner after it. For a positively wound loop that is out of the face, and
// it leaves a convex corner.
func pushCorner(f *Face, i int, by float64) {
	n := len(f.Outer)
	a, x, b := f.Outer[(i+n-1)%n].UVStart, f.Outer[i].UVStart, f.Outer[(i+1)%n].UVStart
	length := math.Hypot(b.X-a.X, b.Y-a.Y)
	moveCorner(f, i, geom.Vec2{X: x.X + by*(b.Y-a.Y)/length, Y: x.Y - by*(b.X-a.X)/length})
}

// tiltedBox is a 6 by 6 by 3 box with integer corners and no face in a
// coordinate plane. Its edge from corner 2 to corner 1 can carry further
// vertices, which lie exactly on it and are corners 3 and up of face 0 and
// corners of face 1.
func tiltedBox(on ...float64) ([]geom.Vec3, [][]int) {
	vertices := []geom.Vec3{{X: 2, Z: -1}, {X: 6, Y: 4, Z: 1}, {X: 2, Y: 6, Z: 5}, {X: -2, Y: 2, Z: 3},
		{X: 3, Y: -2, Z: 1}, {X: 7, Y: 2, Z: 3}, {X: 3, Y: 4, Z: 7}, {X: -1, Z: 5}}
	bottom, side := []int{0, 3, 2}, []int{1}
	for i, s := range on {
		vertices = append(vertices, vertices[2].Add(vertices[1].Sub(vertices[2]).Scale(s)))
		bottom = append(bottom, 8+i)
		side = append(side, 8+len(on)-1-i)
	}
	bottom, side = append(bottom, 1), append(side, 2, 6, 5)
	return vertices, [][]int{bottom, side, {4, 5, 6, 7}, {0, 1, 5, 4}, {2, 3, 7, 6}, {3, 0, 4, 7}}
}

// A vertex where a third edge meets a straight edge is a corner of the
// faces along that edge, and its neighbors on the edge lie on one line with
// it. Chart coordinates are rounded, so such a corner need not be straight
// in the chart, whichever way the rounding went.
func TestNewSolidCollinearCorners(t *testing.T) {
	// The chart coordinates a Boolean of this box produced for face 0.
	rounded := []geom.Vec2{
		{},
		{X: -4.2426406871192848, Y: -4.2426406871192848},
		{X: 5.5511151231257827e-17, Y: -8.4852813742385695},
		{X: 1.0606601717798212, Y: -7.4246212024587486},
		{X: 4.2426406871192848, Y: -4.2426406871192848},
	}
	for _, c := range []struct {
		name string
		on   []float64
		// Chart coordinates of face 0, or the distance each added corner
		// is moved out of face 0, or neither.
		chart []geom.Vec2
		push  []float64
	}{
		{name: "no corner on the edge"},
		{name: "chart coordinates of the frame", on: []float64{.25}},
		{name: "rounded chart coordinates", on: []float64{.25}, chart: rounded},
		{name: "corner outside its chord", on: []float64{.25}, push: []float64{1e-12}},
		{name: "corner inside its chord", on: []float64{.25}, push: []float64{-1e-12}},
		{name: "three corners outside", on: []float64{.25, .5, .75}, push: []float64{1e-12, 1e-12, 1e-12}},
		{name: "three corners on an arc", on: []float64{.25, .5, .75}, push: []float64{3e-12, 4e-12, 3e-12}},
		{name: "corners on both sides", on: []float64{.25, .5, .75}, push: []float64{1e-12, -1e-12, 1e-12}},
	} {
		t.Run(c.name, func(t *testing.T) {
			vertices, polygons := tiltedBox(c.on...)
			for i := 8; i < len(vertices); i++ {
				a, b := vertices[2].Sub(vertices[i]), vertices[1].Sub(vertices[i])
				if a.Cross(b) != (geom.Vec3{}) || !(a.Dot(b) < 0) {
					t.Fatalf("vertex %d is not exactly on the edge", i)
				}
			}
			edges, faces := planarShell(t, vertices, polygons, nil)
			for i, uv := range c.chart {
				moveCorner(&faces[0], i, uv)
			}
			// The chord of each added corner runs along the edge, so the
			// corners can be moved in any order.
			for i, by := range c.push {
				pushCorner(&faces[0], 3+i, by)
			}
			solid, err := NewSolid(vertices, edges, faces, 1e-9)
			if err != nil {
				t.Fatal(err)
			}
			if got := solid.Topology(); got.Vertices != len(vertices) || got.Edges != len(edges) || got.Faces != 6 {
				t.Fatalf("topology %+v", got)
			}
			m, err := solid.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: 1e-6})
			if err != nil {
				t.Fatal(err)
			}
			if err := m.ValidateSolid(); err != nil {
				t.Fatal(err)
			}
			if got := m.SignedVolume(); math.Abs(got-108) > 1e-9 {
				t.Fatalf("volume %.17g, want 108", got)
			}
			if got := m.Area(); math.Abs(got-144) > 1e-9 {
				t.Fatalf("area %.17g, want 144", got)
			}
		})
	}
}

// Faces that are degenerate, and not merely subdivided, stay rejected.
func TestNewSolidRejectsDegenerateFaces(t *testing.T) {
	down := geom.Vec3{X: -1, Y: 2, Z: -2} // out of face 0 of the tilted box
	for _, c := range []struct {
		name  string
		build func(t *testing.T) ([]geom.Vec3, []parametric.Edge, []Face)
		want  string
	}{
		{"no area between an edge and its subdivision", func(t *testing.T) ([]geom.Vec3, []parametric.Edge, []Face) {
			// Face 0 keeps the whole edge, and a seventh face closes the
			// gap to the two parts that face 1 uses. Its chart triangle
			// has a positive area and its corners are on one line.
			vertices, polygons := tiltedBox(.25)
			polygons[0] = []int{0, 3, 2, 1}
			edges, faces := planarShell(t, vertices, append(polygons, []int{2, 8, 1}), map[int]geom.Vec3{6: down})
			pushCorner(&faces[6], 1, 1e-12)
			return vertices, edges, faces
		}, "degenerate"},
		{"no area in two chart triangles", func(t *testing.T) ([]geom.Vec3, []parametric.Edge, []Face) {
			// As above with two vertices on the edge. The seventh face
			// is a convex chart quadrilateral, and neither of its
			// diagonals gives a triangle with area.
			vertices, polygons := tiltedBox(.25, .5)
			polygons[0] = []int{0, 3, 2, 1}
			edges, faces := planarShell(t, vertices, append(polygons, []int{2, 8, 9, 1}), map[int]geom.Vec3{6: down})
			pushCorner(&faces[6], 1, 1e-12)
			pushCorner(&faces[6], 2, 1e-12)
			return vertices, edges, faces
		}, "degenerate"},
		{"loop through one vertex twice", func(t *testing.T) ([]geom.Vec3, []parametric.Edge, []Face) {
			// Two tetrahedra that meet at vertex 0, with their coplanar
			// bases given as a single face.
			vertices := []geom.Vec3{{}, {X: 1, Y: 1}, {X: 1, Y: -1}, {X: -1, Y: -1}, {X: -1, Y: 1}, {X: 1, Z: 1}, {X: -1, Z: 1}}
			edges, faces := planarShell(t, vertices, [][]int{{0, 1, 2, 0, 3, 4}, {0, 2, 5}, {2, 1, 5}, {1, 0, 5}, {0, 4, 6}, {4, 3, 6}, {3, 0, 6}}, nil)
			return vertices, edges, faces
		}, "repeated UV vertex in face 0"},
		{"loop that crosses itself", func(t *testing.T) ([]geom.Vec3, []parametric.Edge, []Face) {
			// The turn at the lowest chart corner is positive, so the
			// loop passes as wound and its domain is examined.
			vertices, polygons := tiltedBox()
			polygons[0] = []int{0, 1, 3, 2}
			edges, faces := planarShell(t, vertices, polygons, map[int]geom.Vec3{0: down})
			return vertices, edges, faces
		}, "face 0 domain"},
	} {
		t.Run(c.name, func(t *testing.T) {
			vertices, edges, faces := c.build(t)
			_, err := NewSolid(vertices, edges, faces, 1e-9)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("got %v, want an error with %q", err, c.want)
			}
		})
	}
}
