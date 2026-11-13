// Package section computes plane sections of closed polygonal boundaries.
package section

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"

	"plainsolid/brep"
	"plainsolid/geom"
)

var ErrContact = errors.New("section: plane passes through a source vertex")
var ErrLimit = errors.New("section: triangle work limit exceeded")

// Loop is a section boundary without a repeated last point. Outer boundaries
// are counterclockwise viewed from the plane's positive side; inner boundaries
// are clockwise. SignedArea is positive on outer boundaries.
type Loop struct {
	Points     []geom.Vec3
	SignedArea float64
}

// Planar computes a transverse section through a planar B-rep. It accepts
// nonconvex and multiple oriented shells. Exact signed distances and edge
// interpolation are used for represented coordinates. Points are rounded to
// float64 after stitching. Sections that hit a source vertex are returned as
// ErrContact so coplanar, edge and vertex degeneracies are explicit. Callers
// must provide an embedded oriented solid.
func Planar(ctx context.Context, s brep.Solid, p geom.Plane, maxTriangles int) ([]Loop, error) {
	if ctx == nil || p.Normal().Length() == 0 || maxTriangles < 0 {
		return nil, fmt.Errorf("section: invalid plane or options")
	}
	if maxTriangles == 0 {
		maxTriangles = 100000
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	m := s.Tessellate()
	if len(m.Triangles) > maxTriangles {
		return nil, ErrLimit
	}
	if len(m.Triangles) == 0 {
		return nil, nil
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	n, o := p.Normal(), p.Origin()
	nr, or := [3]*big.Rat{q(n.X), q(n.Y), q(n.Z)}, [3]*big.Rat{q(o.X), q(o.Y), q(o.Z)}
	type xyz [3]*big.Rat
	exact := make([]xyz, len(m.Vertices))
	dist := make([]*big.Rat, len(m.Vertices))
	for i, v := range m.Vertices {
		if i%256 == 0 {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
		}
		x := xyz{q(v.X), q(v.Y), q(v.Z)}
		d := new(big.Rat)
		for k, c := range x {
			w := new(big.Rat).Sub(c, or[k])
			d.Add(d, w.Mul(w, nr[k]))
		}
		if d.Sign() == 0 {
			return nil, ErrContact
		}
		exact[i], dist[i] = x, d
	}
	type edge [2]int
	pts := map[edge]xyz{}
	key := func(a, b int) edge { return edge{min(a, b), max(a, b)} }
	crossing := func(k edge) {
		if _, ok := pts[k]; ok {
			return
		}
		a, b := k[0], k[1]
		t := new(big.Rat).Quo(dist[a], new(big.Rat).Sub(dist[a], dist[b]))
		var x xyz
		for i := range x {
			d := new(big.Rat).Sub(exact[b][i], exact[a][i])
			x[i] = d.Add(d.Mul(d, t), exact[a][i])
		}
		pts[k] = x
	}
	var order []edge
	next := map[edge]edge{}
	prev := map[edge]bool{}
	for _, t := range m.Triangles {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		from, to := edge{-1, -1}, edge{-1, -1}
		for i, a := range t {
			b := t[(i+1)%3]
			x, y := dist[a].Sign(), dist[b].Sign()
			if x == y {
				continue
			}
			k := key(a, b)
			crossing(k)
			if x > 0 {
				from = k
			} else {
				to = k
			}
		}
		if from[0] < 0 {
			continue
		}
		if _, ok := next[from]; ok || prev[to] {
			return nil, fmt.Errorf("section: ambiguous crossing incidence")
		}
		next[from], prev[to] = to, true
		order = append(order, from)
	}
	axis := 0
	if math.Abs(n.Y) > math.Abs(n.X) {
		axis = 1
	}
	if math.Abs(n.Z) > math.Abs([3]float64{n.X, n.Y, n.Z}[axis]) {
		axis = 2
	}
	u, v := (axis+1)%3, (axis+2)%3
	var result []Loop
	for _, start := range order {
		if _, ok := next[start]; !ok {
			continue
		}
		var ring []xyz
		for k := start; ; {
			e, ok := next[k]
			if !ok {
				return nil, fmt.Errorf("section: open intersection loop")
			}
			delete(next, k)
			ring = append(ring, pts[k])
			k = e
			if k == start {
				break
			}
		}
		// A tessellation diagonal introduces a point on an original face's
		// section segment. Remove these exact collinear subdivisions.
		var r []xyz
		for i, x := range ring {
			a, b := ring[(i+len(ring)-1)%len(ring)], ring[(i+1)%len(ring)]
			ax, ay := new(big.Rat).Sub(x[u], a[u]), new(big.Rat).Sub(x[v], a[v])
			bx, by := new(big.Rat).Sub(b[u], x[u]), new(big.Rat).Sub(b[v], x[v])
			if new(big.Rat).Sub(ax.Mul(ax, by), ay.Mul(ay, bx)).Sign() != 0 {
				r = append(r, x)
			}
		}
		if len(r) < 3 {
			return nil, fmt.Errorf("section: degenerate intersection loop")
		}
		sum := new(big.Rat)
		out := Loop{Points: make([]geom.Vec3, len(r))}
		for i, x := range r {
			y := r[(i+1)%len(r)]
			ab, ba := new(big.Rat).Mul(x[u], y[v]), new(big.Rat).Mul(x[v], y[u])
			sum.Add(sum, ab.Sub(ab, ba))
			a, _ := x[0].Float64()
			b, _ := x[1].Float64()
			c, _ := x[2].Float64()
			out.Points[i] = geom.Vec3{X: a, Y: b, Z: c}
		}
		out.SignedArea, _ = sum.Quo(sum, new(big.Rat).Mul(q(2), nr[axis])).Float64()
		if out.SignedArea == 0 || math.IsInf(out.SignedArea, 0) {
			return nil, fmt.Errorf("section: unrepresentable section area")
		}
		for i, p := range out.Points {
			if !p.IsFinite() || p == out.Points[(i+1)%len(r)] {
				return nil, fmt.Errorf("section: point conversion collapsed boundary")
			}
		}
		result = append(result, out)
	}
	return result, nil
}
