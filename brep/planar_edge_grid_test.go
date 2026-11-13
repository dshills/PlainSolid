package brep

import (
	"math/rand"
	"plainsolid/geom"
	"testing"
)

func TestPlanarEdgeGridConservative(t *testing.T) {
	rng := rand.New(rand.NewSource(9182))
	points := []geom.Vec2{{X: -1, Y: -1}, {X: 1, Y: 1}, {}, {X: 1}, {Y: 1}, {X: -1}, {Y: -1}}
	for range 60 {
		points = append(points, geom.Vec2{X: 2*rng.Float64() - 1, Y: 2*rng.Float64() - 1})
	}
	g := newPlanarEdgeGrid(points)
	edges := []segment2{}
	for i := 0; i < len(points)-1; i++ {
		edges = append(edges, segment2{i, i + 1})
		g.add(points[i], points[i+1], i)
	}
	for _, a := range points {
		for _, b := range points {
			ids, _ := g.query(a, b)
			present := map[int]bool{}
			for _, id := range ids {
				present[id] = true
			}
			for id, e := range edges {
				p, q := points[e.a], points[e.b]
				if intersects2(a, b, p, q) && !present[id] {
					t.Fatal("missed intersection", a, b, p, q)
				}
				if (onSegment2(p, a, b) || onSegment2(q, a, b)) && !present[id] {
					t.Fatal("missed boundary vertex")
				}
			}
		}
	}
}
