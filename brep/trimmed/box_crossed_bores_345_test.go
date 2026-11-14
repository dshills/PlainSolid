package trimmed

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestBoxCrossedBores345(t *testing.T) {
	for _, n := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		for _, radius := range []float64{2, 1.5} {
			f, _ := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, n)
			d, c := geom.Vec3{X: 20, Y: 16, Z: 40}, geom.Vec3{X: 8, Y: 7, Z: 18}
			r, e := BoxCrossedBores345(context.Background(), f, d, c, radius, 1e-9)
			if e != nil {
				t.Fatal(n, radius, e)
			}
			m, e := r.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-10})
			if e != nil {
				t.Fatal(e)
			}
			// At transverse coordinate y, each cylinder contributes an interval of
			// width2sqrt(r²−y²); their independent-axis Jacobian is1/sin(theta)=5/4.
			common := 0.
			steps := 4096
			dy := 2 * radius / float64(steps)
			for i := 0; i <= steps; i++ {
				y := -radius + float64(i)*dy
				w := 2.
				if i == 0 || i == steps {
					w = 1
				} else if i%2 == 1 {
					w = 4
				}
				common += w * 5 * (radius*radius - y*y)
			}
			common *= dy / 3
			vZ, vB := math.Pi*radius*radius*d.Z, math.Pi*radius*radius*1.25*d.X
			removed := vZ + vB - common
			delta := 2*math.Pi*radius*(d.Z+1.25*d.X) - 20*radius*radius - 4.5*math.Pi*radius*radius
			moment := geom.Vec3{X: c.X, Y: c.Y, Z: d.Z / 2}.Scale(vZ).Add(geom.Vec3{X: d.X / 2, Y: c.Y, Z: c.Z + .75*(d.X/2-c.X)}.Scale(vB)).Sub(c.Scale(common))
			centroid := d.Scale(12800 / 2.).Sub(moment).Scale(1 / (12800 - removed))
			if math.Abs(m.Volume-(12800-removed)) > 3e-7 || math.Abs(m.Area-(3520+delta)) > 3e-7 || m.Centroid.Distance(f.ToWorld(centroid)) > 1e-8 || math.Abs(r.RemovedVolume-removed) > 1e-8 || math.Abs(r.SurfaceAreaDelta-delta) > 1e-8 {
				t.Fatal(n, radius, m, removed, delta, centroid)
			}
			for _, id := range []int{0, 1} {
				a, e := r.Solid.PlanarFaceArea(id)
				if e != nil || math.Abs(a-(320-math.Pi*radius*radius)) > 1e-8 {
					t.Fatal("circle cap", id, a, e)
				}
			}
			for _, id := range []int{3, 5} {
				a, e := r.Solid.PlanarFaceArea(id)
				if e != nil || math.Abs(a-(640-1.25*math.Pi*radius*radius)) > 1e-8 {
					t.Fatal("ellipse cap", id, a, e)
				}
			}
			assert345VertexLinks(t, r.Solid)
			mesh, e := r.Solid.TessellateCrossedBores345(context.Background(), parametric.TessellationOptions{Deflection: .5, Angle: .05, MaxTriangles: 100000, MaxSubdivisions: 128})
			if e != nil {
				t.Fatal(n, radius, e)
			}
			if e = mesh.ValidateSolid(); e != nil {
				t.Fatal(e)
			}
			if math.Abs(mesh.SignedVolume()-(12800-removed)) > removed*.005 {
				t.Fatal("mesh volume")
			}
			if n == (geom.Vec3{Z: 1}) && radius == 2 {
				for _, binary := range []bool{false, true} {
					candidate := mesh
					candidate.Vertices = append([]geom.Vec3(nil), mesh.Vertices...)
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
}
func TestBoxCrossedBores345Rejects(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	d := geom.Vec3{X: 20, Y: 16, Z: 40}
	for _, c := range []geom.Vec3{{X: 2, Y: 8, Z: 20}, {X: 10, Y: 2, Z: 20}, {X: 10, Y: 8, Z: 4}, {X: 10, Y: 8, Z: 10}, {X: 10, Y: 8, Z: 30}} {
		if _, e := BoxCrossedBores345(context.Background(), f, d, c, 2, 1e-9); e == nil {
			t.Fatal("accepted tangent", c)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := BoxCrossedBores345(ctx, f, d, geom.Vec3{X: 10, Y: 8, Z: 20}, 2, 1e-9); e != context.Canceled {
		t.Fatal(e)
	}
}

// Each closed-manifold vertex must have one connected cyclic edge link.
func assert345VertexLinks(t *testing.T, s Solid) {
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
