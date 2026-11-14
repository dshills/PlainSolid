package trimmed

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func tripleCommonQuadrature(r float64) float64 {
	area := func(z float64) float64 {
		q := math.Sqrt(max(0, (r-z)*(r+z)))
		if q <= r/math.Sqrt2 {
			return 4 * q * q
		}
		return math.Pi*r*r - 4*(r*r*math.Acos(q/r)-q*z)
	}
	integrate := func(a, b float64) float64 {
		n := 4096
		step := (b - a) / float64(n)
		sum := 0.
		for i := 0; i <= n; i++ {
			w := 2.
			if i == 0 || i == n {
				w = 1
			} else if i%2 == 1 {
				w = 4
			}
			sum += w * area(a+float64(i)*step)
		}
		return sum * step / 3
	}
	return 2 * (integrate(0, r/math.Sqrt2) + integrate(r/math.Sqrt2, r))
}
func TestBoxTripleCrossedBores(t *testing.T) {
	for _, n := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		f, _ := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, n)
		d, c := geom.Vec3{X: 20, Y: 16, Z: 12}, geom.Vec3{X: 8, Y: 7, Z: 5}
		radius := 2.
		r, e := BoxTripleCrossedBores(context.Background(), f, d, c, radius, 1e-9)
		if e != nil {
			t.Fatal(n, e)
		}
		m, e := r.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		common := tripleCommonQuadrature(radius)
		if math.Abs(common-8*(2-math.Sqrt2)*radius*radius*radius) > 1e-8 {
			t.Fatal(common)
		}
		correction := 16*radius*radius*radius - common
		removed := math.Pi*radius*radius*(d.X+d.Y+d.Z) - correction
		delta := 2*math.Pi*radius*(d.X+d.Y+d.Z) - 24*math.Sqrt2*radius*radius - 6*math.Pi*radius*radius
		removedMoment := geom.Vec3{X: d.X / 2, Y: c.Y, Z: c.Z}.Scale(math.Pi * radius * radius * d.X).Add(geom.Vec3{X: c.X, Y: d.Y / 2, Z: c.Z}.Scale(math.Pi * radius * radius * d.Y)).Add(geom.Vec3{X: c.X, Y: c.Y, Z: d.Z / 2}.Scale(math.Pi * radius * radius * d.Z)).Sub(c.Scale(correction))
		centroid := d.Scale(3840 / 2.).Sub(removedMoment).Scale(1 / (3840 - removed))
		if math.Abs(m.Volume-(3840-removed)) > 1e-7 || math.Abs(m.Area-(1504+delta)) > 1e-7 || m.Centroid.Distance(f.ToWorld(centroid)) > 1e-8 || math.Abs(r.RemovedVolume-removed) > 1e-8 || math.Abs(r.SurfaceAreaDelta-delta) > 1e-8 {
			t.Fatal(n, m, removed, delta, centroid)
		}
		if len(r.Solid.Faces()) != 54 {
			t.Fatal(len(r.Solid.Faces()))
		}
		for i := 0; i < 6; i++ {
			if len(r.Solid.Faces()[i].Holes) != 1 || len(r.Solid.Faces()[i].Holes[0]) != 8 {
				t.Fatal("cap", i)
			}
		}
		assertTripleVertexLinks(t, r.Solid)
		mesh, e := r.Solid.TessellateTripleCrossedBores(context.Background(), parametric.TessellationOptions{Deflection: .5, Angle: .05, MaxTriangles: 100000, MaxSubdivisions: 128})
		if e != nil {
			t.Fatal(n, e)
		}
		if e = mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		if math.Abs(mesh.SignedVolume()-(3840-removed)) > removed*.005 {
			t.Fatal("mesh volume", mesh.SignedVolume())
		}
		if n == (geom.Vec3{Z: 1}) {
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
func TestBoxTripleCrossedBoresLimits(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	d, c := geom.Vec3{X: 20, Y: 16, Z: 12}, geom.Vec3{X: 8, Y: 7, Z: 5}
	for _, r := range []float64{0, -1, 5, math.NaN(), math.Inf(1)} {
		if _, e := BoxTripleCrossedBores(context.Background(), f, d, c, r, 1e-9); e == nil {
			t.Fatal(r)
		}
	}
	r, e := BoxTripleCrossedBores(context.Background(), f, d, c, 2, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = r.Solid.TessellateTripleCrossedBores(context.Background(), parametric.TessellationOptions{Deflection: .1, Angle: .1, MaxTriangles: 2}); e == nil {
		t.Fatal("budget")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = BoxTripleCrossedBores(ctx, f, d, c, 2, 1e-9); e != context.Canceled {
		t.Fatal(e)
	}
}

// Each closed-manifold vertex must have one connected cyclic edge link.
func assertTripleVertexLinks(t *testing.T, s Solid) {
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
