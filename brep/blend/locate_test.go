package blend

import (
	"errors"
	"math"
	"testing"

	"plainsolid/brep/boolean"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// TestLocate resolves references that name one edge (R1 to R7): the
// opening angle, the convexity, and the two directions into the faces.
func TestLocate(t *testing.T) {
	for _, c := range []struct {
		name       string
		solid      trimmed.Solid
		start, end geom.Vec3
		edges      int
		angle      float64
		convex     bool
		// inside is a point of the material beside the edge for a
		// convex edge, and of the empty space for a concave one: the
		// two directions into the faces must enclose it.
		inside geom.Vec3
	}{
		{"box", boxSolid(t, vec(0, 0, 0), vec(20, 10, 10)), vec(0, 0, 0), vec(20, 0, 0), 1, math.Pi / 2, true, vec(10, 1, 1)},
		{"box, ends exchanged", boxSolid(t, vec(0, 0, 0), vec(20, 10, 10)), vec(20, 0, 0), vec(0, 0, 0), 1, math.Pi / 2, true, vec(10, 1, 1)},
		{"result of a cut", slottedBox(t), vec(0, 0, 0), vec(20, 0, 0), 1, math.Pi / 2, true, vec(10, 1, 1)},
		{"up to a slot", slottedBox(t), vec(0, 0, 10), vec(12, 0, 10), 1, math.Pi / 2, true, vec(6, 1, 9)},
		{"ends within the point tolerance", boxSolid(t, vec(0, 0, 0), vec(20, 10, 10)), vec(5e-8, 0, 0), vec(20, 5e-8, 0), 1, math.Pi / 2, true, vec(10, 1, 1)},
		{"L prism, convex", lPrism(t), vec(20, 10, 0), vec(20, 10, 10), 1, math.Pi / 2, true, vec(19, 9, 5)},
		{"L prism, concave", lPrism(t), vec(10, 10, 0), vec(10, 10, 10), 1, math.Pi / 2, false, vec(11, 11, 5)},
		{"hexagonal prism", hexPrism(t), vec(5, 0, 0), vec(5, 0, 8), 1, 2 * math.Pi / 3, true, vec(4.5, 0, 4)},
		{"triangular prism", trianglePrism(t), vec(4, 0, 0), vec(4, 0, 5), 1, math.Atan2(3, 4), true, vec(3, 0.3, 2)},
		{"a chain of two edges", touchedBox(t), vec(0, 0, 10), vec(20, 0, 10), 2, math.Pi / 2, true, vec(10, 1, 9)},
	} {
		t.Run(c.name, func(t *testing.T) {
			e, err := Locate(c.solid, c.start, c.end, testTolerance)
			if err != nil {
				t.Fatal(err)
			}
			if len(e.chain) != c.edges || len(e.vertices) != c.edges+1 {
				t.Fatalf("chain of %d edges and %d vertices, want %d edges", len(e.chain), len(e.vertices), c.edges)
			}
			if e.start.Sub(c.start).Length() > 1e-7 || e.end.Sub(c.end).Length() > 1e-7 {
				t.Fatalf("chain from %v to %v", e.start, e.end)
			}
			if math.Abs(e.angle-c.angle) > 1e-12 || e.convex != c.convex {
				t.Fatalf("angle %.15g, convex %v; want %.15g, %v", e.angle, e.convex, c.angle, c.convex)
			}
			axis, _ := c.end.Sub(c.start).Unit()
			d := c.inside.Sub(c.start)
			d = d.Sub(axis.Scale(d.Dot(axis)))
			for k := range 2 {
				if math.Abs(e.into[k].Dot(axis)) > 1e-12 || math.Abs(e.into[k].Dot(e.normal[k])) > 1e-12 || math.Abs(e.into[k].Length()-1) > 1e-12 {
					t.Fatalf("direction %d into its face is %v, with normal %v", k, e.into[k], e.normal[k])
				}
				// The point lies between the two directions: on the
				// positive side of each, measured across the other.
				other := e.into[1-k]
				across := other.Sub(e.into[k].Scale(other.Dot(e.into[k])))
				if d.Dot(across) <= 0 {
					t.Fatalf("the directions %v and %v do not enclose %v", e.into[0], e.into[1], c.inside)
				}
			}
		})
	}
}

// straightEdges lists the straight edges of a solid with the faces that
// use them.
func straightEdges(s trimmed.Solid) map[int][]int {
	out := map[int][]int{}
	for i, e := range s.Edges() {
		if e.Curve != nil {
			if base, _ := parametric.CurveMapping(e.Curve); isLine(base) {
				out[i] = nil
			}
		}
	}
	for f, face := range s.Faces() {
		for _, loop := range append([][]trimmed.Use{face.Outer}, face.Holes...) {
			for _, u := range loop {
				if _, ok := out[u.Edge]; ok {
					out[u.Edge] = append(out[u.Edge], f)
				}
			}
		}
	}
	return out
}

// TestLocateRefuses gives references that do not resolve, each with the
// class of R8.
func TestLocateRefuses(t *testing.T) {
	box := boxSolid(t, vec(0, 0, 0), vec(20, 10, 10))
	touched := touchedBox(t)
	vertices, edges, faces := touched.Vertices(), touched.Edges(), touched.Faces()
	// The edge by which the Boolean divided the top face of the touched
	// box, and the seam of its cylinder, are found by inspection.
	var divider, seam [2]geom.Vec3
	found := [2]int{}
	// top reports whether a face lies in the plane z = 10.
	top := func(f int) bool {
		for _, u := range faces[f].Outer {
			if vertices[edges[u.Edge].Start].Z != 10 || vertices[edges[u.Edge].End].Z != 10 {
				return false
			}
		}
		return true
	}
	for e, users := range straightEdges(touched) {
		if len(users) != 2 {
			continue
		}
		ends := [2]geom.Vec3{vertices[edges[e].Start], vertices[edges[e].End]}
		switch {
		case users[0] == users[1]:
			seam, found[1] = ends, found[1]+1
		case top(users[0]) && top(users[1]):
			divider, found[0] = ends, found[0]+1
		}
	}
	if found != [2]int{1, 1} {
		t.Fatalf("found %d dividers and %d seams in the touched box", found[0], found[1])
	}
	// A bore that leaves through the front face: its wall meets the face
	// along two straight edges.
	bored := combined(t, box, cylinderSolid(t, vec(5, 0, -1), vec(0, 0, 1), 2, 12), boolean.Cut).Solid
	// A fillet tool: a prism less a cylinder that is tangent to two of
	// its faces. Where they meet, the edge is smooth.
	tool := combined(t, boxSolid(t, vec(0, 0, 0), vec(20, 2, 2)), cylinderSolid(t, vec(0, 2, 2), vec(1, 0, 0), 2, 20), boolean.Cut).Solid
	for _, c := range []struct {
		name       string
		solid      trimmed.Solid
		start, end geom.Vec3
		want       error
	}{
		{"half an edge", box, vec(0, 0, 0), vec(10, 0, 0), ErrLost},
		{"no edge between the ends", box, vec(0, 0, 0), vec(20, 10, 0), ErrLost},
		{"an end beyond the point tolerance", box, vec(2e-7, 0, 0), vec(20, 0, 0), ErrLost},
		{"the two ends are one point", box, vec(0, 0, 0), vec(0, 0, 0), ErrLost},
		{"an edge interrupted by a slot", slottedBox(t), vec(0, 0, 10), vec(20, 0, 10), ErrLost},
		{"an edge consumed by a cut", slottedBox(t), vec(12, 0, 10), vec(16, 0, 10), ErrLost},
		{"the seam of a cylinder", touched, seam[0], seam[1], ErrLost},
		{"a plane against a cylinder", bored, vec(3, 0, 0), vec(3, 0, 10), ErrCurved},
		{"a smooth edge of a fillet tool", tool, vec(0, 2, 0), vec(20, 2, 0), ErrCurved},
		{"between coplanar faces", touched, divider[0], divider[1], ErrParallel},
	} {
		t.Run(c.name, func(t *testing.T) {
			if _, err := Locate(c.solid, c.start, c.end, testTolerance); !errors.Is(err, c.want) {
				t.Fatalf("got %v, want %v", err, c.want)
			}
		})
	}
}
