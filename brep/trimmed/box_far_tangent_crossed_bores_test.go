package trimmed

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestBoxFarTangentCrossedBores(t *testing.T) {
	for _, n := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		f, _ := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, n)
		r, e := BoxFarTangentCrossedBores(context.Background(), f, geom.Vec3{X: 20, Y: 16, Z: 12}, geom.Vec3{X: 10, Y: 8, Z: 6}, 2, 1e-9)
		if e != nil {
			t.Fatal(n, e)
		}
		mass, e := r.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		removed := 112*math.Pi - 128.0/3
		delta := 104*math.Pi - 64
		if math.Abs(mass.Volume-(3840-removed)) > 1e-7 || math.Abs(mass.Area-(1504+delta)) > 1e-7 || math.Abs(r.RemovedVolume-removed) > 1e-8 || math.Abs(r.SurfaceAreaDelta-delta) > 1e-8 {
			t.Fatal(mass, r, removed, delta)
		}
		a, e := r.Solid.PlanarFaceArea(6)
		if e != nil || math.Abs(a-4*math.Pi) > 1e-8 {
			t.Fatal("floor", a, e)
		}
		assertCrossedVertexLinks(t, r.Solid)
		m, e := r.Solid.TessellateBlindCrossedBores(context.Background(), parametric.TessellationOptions{Deflection: .5, Angle: .05, MaxTriangles: 100000, MaxSubdivisions: 128})
		if e != nil {
			t.Fatal(n, e)
		}
		if e = m.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		if n == (geom.Vec3{Z: 1}) {
			for _, binary := range []bool{false, true} {
				candidate := m
				candidate.Vertices = append([]geom.Vec3(nil), m.Vertices...)
				if binary {
					for i, p := range candidate.Vertices {
						candidate.Vertices[i] = geom.Vec3{X: float64(float32(p.X)), Y: float64(float32(p.Y)), Z: float64(float32(p.Z))}
					}
				}
				if e = candidate.ValidateSolid(); e != nil {
					t.Fatal(e)
				}
				rep, e := collision.SelfIntersectionsBVHContext(context.Background(), candidate, collision.Options{})
				if e != nil || !rep.Complete || len(rep.Contacts) > 0 {
					t.Fatal(binary, e, rep)
				}
			}
		}
	}
}

// Each closed-manifold vertex must have one connected cyclic edge link.
func assertCrossedVertexLinks(t *testing.T, s Solid) {
	t.Helper()
	edges := s.Edges()
	for vertex := range s.Vertices() {
		adj := map[int][]int{}
		for _, f := range s.Faces() {
			for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
				for i, u := range loop {
					start := edges[u.Edge].Start
					if u.Reversed {
						start = edges[u.Edge].End
					}
					if start != vertex {
						continue
					}
					previous := loop[(i+len(loop)-1)%len(loop)].Edge
					adj[previous] = append(adj[previous], u.Edge)
					adj[u.Edge] = append(adj[u.Edge], previous)
				}
			}
		}
		if len(adj) == 0 {
			t.Fatal("unused vertex", vertex)
		}
		first := -1
		for id, n := range adj {
			first = id
			if len(n) != 2 {
				t.Fatal("non-cycle link", vertex, id, n)
			}
		}
		seen := map[int]bool{}
		todo := []int{first}
		for len(todo) > 0 {
			id := todo[len(todo)-1]
			todo = todo[:len(todo)-1]
			if seen[id] {
				continue
			}
			seen[id] = true
			todo = append(todo, adj[id]...)
		}
		if len(seen) != len(adj) {
			t.Fatal("disconnected link", vertex)
		}
	}
}
