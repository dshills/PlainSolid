package query_test

import (
	"math"
	"math/rand"
	"testing"

	"plainsolid/geom"
	"plainsolid/mesh"
	"plainsolid/query"
)

func randomPoint(r *rand.Rand) geom.Vec3 {
	return geom.Vec3{X: r.Float64()*10 - 5, Y: r.Float64()*10 - 5, Z: r.Float64()*10 - 5}
}

// Independently solve barycentrics with a Gram matrix, then enumerate all edge
// projections. This differs from the production Voronoi-region implementation.
func projected(p, a, b, c geom.Vec3) (geom.Vec3, bool) {
	u, v, w := b.Sub(a), c.Sub(a), p.Sub(a)
	uu, uv, vv := u.Dot(u), u.Dot(v), v.Dot(v)
	wu, wv := w.Dot(u), w.Dot(v)
	den := uu*vv - uv*uv
	s, t := (wu*vv-wv*uv)/den, (wv*uu-wu*uv)/den
	return a.Add(u.Scale(s)).Add(v.Scale(t)), s >= 0 && t >= 0 && s+t <= 1
}
func bruteClosest(p geom.Vec3, m mesh.Mesh) float64 {
	best := math.Inf(1)
	for _, tr := range m.Triangles {
		a, b, c := m.Vertices[tr[0]], m.Vertices[tr[1]], m.Vertices[tr[2]]
		v, inside := projected(p, a, b, c)
		if inside {
			best = math.Min(best, p.Distance(v))
		}
		for _, edge := range [][2]geom.Vec3{{a, b}, {b, c}, {c, a}} {
			u := edge[1].Sub(edge[0])
			s := math.Max(0, math.Min(1, p.Sub(edge[0]).Dot(u)/u.Dot(u)))
			best = math.Min(best, p.Distance(edge[0].Add(u.Scale(s))))
		}
	}
	return best
}
func bruteRay(o, d geom.Vec3, max float64, m mesh.Mesh) (float64, bool) {
	d, _ = d.Unit()
	best := max
	found := false
	for _, tr := range m.Triangles {
		a, b, c := m.Vertices[tr[0]], m.Vertices[tr[1]], m.Vertices[tr[2]]
		n := b.Sub(a).Cross(c.Sub(a))
		den := n.Dot(d)
		if den == 0 {
			continue
		}
		dist := n.Dot(a.Sub(o)) / den
		if dist < 0 || dist > best {
			continue
		}
		p := o.Add(d.Scale(dist))
		_, inside := projected(p, a, b, c)
		if inside {
			best = dist
			found = true
		}
	}
	return best, found
}
func verifyHit(t *testing.T, h query.Hit, m mesh.Mesh) {
	t.Helper()
	if h.Triangle < 0 || h.Triangle >= len(m.Triangles) {
		t.Fatal(h)
	}
	b := h.Barycentric
	if math.Abs(b.X+b.Y+b.Z-1) > 1e-10 || b.X < -1e-10 || b.Y < -1e-10 || b.Z < -1e-10 {
		t.Fatal(h)
	}
	tr := m.Triangles[h.Triangle]
	p := m.Vertices[tr[0]].Scale(b.X).Add(m.Vertices[tr[1]].Scale(b.Y)).Add(m.Vertices[tr[2]].Scale(b.Z))
	if p.Distance(h.Point) > 1e-9 {
		t.Fatal("barycentric reconstruction", h, p)
	}
}
func TestRandomBVHAgainstBruteForce(t *testing.T) {
	r := rand.New(rand.NewSource(223))
	m := mesh.Mesh{}
	for i := 0; i < 90; i++ {
		n := len(m.Vertices)
		m.Vertices = append(m.Vertices, randomPoint(r), randomPoint(r), randomPoint(r))
		m.Triangles = append(m.Triangles, [3]int{n, n + 1, n + 2})
	}
	index, e := query.New(m)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 400; i++ {
		p := randomPoint(r).Scale(2)
		h, ok, e := index.Closest(p)
		if e != nil || !ok {
			t.Fatal(h, ok, e)
		}
		want := bruteClosest(p, m)
		if math.Abs(h.Distance-want) > 1e-9 {
			t.Fatalf("closest %g want %g", h.Distance, want)
		}
		verifyHit(t, h, m)
		d := randomPoint(r)
		limit := r.Float64()*20 + .01
		h, ok, e = index.Ray(p, d, limit)
		dist, found := bruteRay(p, d, limit, m)
		if e != nil || ok != found {
			t.Fatalf("ray found %v want %v error %v", ok, found, e)
		}
		if ok {
			if math.Abs(h.Distance-dist) > 1e-9 {
				t.Fatalf("ray distance %g want %g", h.Distance, dist)
			}
			verifyHit(t, h, m)
		}
	}
}
func TestRandomBoxClassificationOracle(t *testing.T) {
	m, e := mesh.Box(geom.Vec3{X: -1, Y: -2, Z: -3}, geom.Vec3{X: 2, Y: 3, Z: 4})
	if e != nil {
		t.Fatal(e)
	}
	index, e := query.New(m)
	if e != nil {
		t.Fatal(e)
	}
	r := rand.New(rand.NewSource(812))
	for i := 0; i < 300; i++ {
		p := randomPoint(r)
		want := query.Outside
		if p.X > -1 && p.X < 2 && p.Y > -2 && p.Y < 3 && p.Z > -3 && p.Z < 4 {
			want = query.Inside
		}
		got, e := index.Classify(p, 0)
		if e != nil || got != want {
			t.Fatal(p, got, want, e)
		}
	}
}

// Finite valid triangles must not yield a silent miss or a fabricated zero
// distance when scalar triple products leave the representable range. An
// explicit arithmetic error is acceptable outside the supported numeric domain.
func TestRayExtremeScaleOracle(t *testing.T) {
	for _, s := range []float64{1e150, 1e-150} {
		m := mesh.Mesh{Vertices: []geom.Vec3{{}, {X: s}, {Y: s}}, Triangles: [][3]int{{0, 1, 2}}}
		index, e := query.New(m)
		if e != nil {
			t.Fatal(e)
		}
		h, ok, e := index.Ray(geom.Vec3{X: s / 4, Y: s / 4, Z: s}, geom.Vec3{Z: -1}, math.Inf(1))
		if e != nil {
			continue
		}
		if !ok || math.Abs(h.Distance/s-1) > 1e-12 || math.Abs(h.Point.Z/s) > 1e-12 {
			t.Fatalf("scale %g: hit %+v found %v; expected distance %g", s, h, ok, s)
		}
	}
}

func TestRayFarOriginScaleOracle(t *testing.T) {
	const size = 1e100
	m := mesh.Mesh{Vertices: []geom.Vec3{{}, {X: size}, {Y: size}}, Triangles: [][3]int{{0, 1, 2}}}
	index, e := query.New(m)
	if e != nil {
		t.Fatal(e)
	}
	const height = 1e200
	h, ok, e := index.Ray(geom.Vec3{X: size / 4, Y: size / 4, Z: height}, geom.Vec3{Z: -1}, math.Inf(1))
	if e != nil {
		return
	}
	if !ok || math.Abs(h.Distance/height-1) > 1e-12 || math.Abs(h.Point.X/size-.25) > 1e-12 || math.Abs(h.Point.Y/size-.25) > 1e-12 || h.Point.Z != 0 {
		t.Fatalf("far origin: hit %+v found %v", h, ok)
	}
}

func TestClosestExtremeScaleOracle(t *testing.T) {
	for _, s := range []float64{1e-150, 1e-100, 1e100, 1e150} {
		m := mesh.Mesh{Vertices: []geom.Vec3{{}, {X: s}, {Y: s}}, Triangles: [][3]int{{0, 1, 2}}}
		index, e := query.New(m)
		if e != nil {
			t.Fatal(e)
		}
		for _, c := range []struct {
			p, want  geom.Vec3
			distance float64
		}{
			{geom.Vec3{X: s / 4, Y: s / 4, Z: s}, geom.Vec3{X: s / 4, Y: s / 4}, s},
			{geom.Vec3{X: s / 4, Y: s / 4}, geom.Vec3{X: s / 4, Y: s / 4}, 0},
			{geom.Vec3{X: s / 4, Y: -s / 4, Z: s}, geom.Vec3{X: s / 4}, math.Hypot(s, s/4)},
			{geom.Vec3{X: -s, Y: -s, Z: s}, geom.Vec3{}, math.Sqrt(3) * s},
		} {
			h, ok, e := index.Closest(c.p)
			if e != nil {
				continue
			}
			if !ok || math.Abs((h.Distance-c.distance)/s) > 1e-12 || h.Point.Sub(c.want).Length()/s > 1e-12 {
				t.Errorf("scale %g point %v: hit %+v found %v expected %v distance %g", s, c.p, h, ok, c.want, c.distance)
			}
		}
	}
}
