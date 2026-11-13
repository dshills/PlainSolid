package scene

import (
	"context"
	"fmt"
	"math"
	"sort"

	"plainsolid/geom"
	"plainsolid/mesh"
)

// Perspective depth is compared along each viewing ray using reciprocal
// depth, an affine function of screen coordinates over a projected triangle.
// A fixed world-space depth tolerance produces a quadratic inequality.
func perspectiveHidden(ctx context.Context, c Camera, m mesh.Mesh, edges []DrawingEdge, o HiddenLineOptions) ([]VisibleSpan, error) {
	local := func(p geom.Vec3) projected {
		q := c.frame.ToLocal(p)
		return projected{2 * (q.X / c.width), 2 * (q.Y / c.height), -q.Z}
	}
	camera := make([]projected, len(m.Vertices))
	for i, p := range m.Vertices {
		camera[i] = local(p)
		if !camera[i].isFinite() {
			return nil, fmt.Errorf("scene: projection overflow")
		}
	}
	type projectedTriangle [3]projected
	triangles := make([]projectedTriangle, 0, len(m.Triangles))
	clip := func(in []projected, plane float64, far bool) []projected {
		var out []projected
		for i, a := range in {
			b := in[(i+1)%len(in)]
			va, vb := a.z-plane, b.z-plane
			if far {
				va, vb = -va, -vb
			}
			if va >= 0 {
				out = append(out, a)
			}
			if va < 0 && vb > 0 || va > 0 && vb < 0 {
				scale := max(math.Abs(va), math.Abs(vb))
				k := (va / scale) / ((va / scale) - (vb / scale))
				p := a.at(b, k)
				p.z = plane
				out = append(out, p)
			}
		}
		return out
	}
	project := func(p projected) projected { return projected{p.x / p.z, p.y / p.z, 1 / p.z} }
	for i, t := range m.Triangles {
		if i%1024 == 0 {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
		}
		p := []projected{camera[t[0]], camera[t[1]], camera[t[2]]}
		p = clip(clip(p, c.near, false), c.far, true)
		for j := range p {
			p[j] = project(p[j])
			if !p[j].isFinite() {
				return nil, fmt.Errorf("scene: projection overflow")
			}
		}
		for j := 1; j+1 < len(p); j++ {
			triangles = append(triangles, projectedTriangle{p[0], p[j], p[j+1]})
		}
	}
	if len(triangles) > 0 && len(edges) > o.MaxTests/len(triangles) {
		return nil, fmt.Errorf("scene: hidden-line work limit exceeded after clipping")
	}
	var result []VisibleSpan
	for _, edge := range edges {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		p, q := local(edge.Start), local(edge.End)
		if !p.isFinite() || !q.isFinite() {
			return nil, fmt.Errorf("scene: invalid drawing edge")
		}
		valid := span{0, 1}
		for _, g := range [][2]float64{{p.x + p.z, q.x + q.z}, {p.z - p.x, q.z - q.x}, {p.y + p.z, q.y + q.z}, {p.z - p.y, q.z - q.y}, {p.z - c.near, q.z - c.near}, {c.far - p.z, c.far - q.z}} {
			ok, e := keepPositive(&valid, g[0], g[1])
			if e != nil {
				return nil, e
			}
			if !ok {
				valid.hi = valid.lo
				break
			}
		}
		if valid.lo >= valid.hi {
			continue
		}
		a, b := project(p.at(q, valid.lo)), project(p.at(q, valid.hi))
		if !a.isFinite() || !b.isFinite() || a.z <= 0 || b.z <= 0 {
			return nil, fmt.Errorf("scene: unrepresentable clipped perspective edge")
		}
		if a.x == b.x && a.y == b.y {
			continue
		}
		var hidden []span
		for i, t := range triangles {
			if i%1024 == 0 {
				if e := ctx.Err(); e != nil {
					return nil, e
				}
			}
			h, e := hiddenPerspective(a, b, t, o.DepthTolerance)
			if e != nil {
				return nil, e
			}
			hidden = append(hidden, h...)
		}
		sort.Slice(hidden, func(i, j int) bool { return hidden[i].lo < hidden[j].lo })
		emit := func(l, r float64) {
			if r-l <= 8*0x1p-52 {
				return
			}
			x, y := a.at(b, l), a.at(b, r)
			// Convert projected fraction back to original 3D edge parameter.
			t0 := valid.lo + (valid.hi-valid.lo)*max(0, min(1, l*b.z/x.z))
			t1 := valid.lo + (valid.hi-valid.lo)*max(0, min(1, r*b.z/y.z))
			if t1 > t0 {
				result = append(result, VisibleSpan{edge.ID, geom.Vec2{X: x.x, Y: x.y}, geom.Vec2{X: y.x, Y: y.y}, t0, t1})
			}
		}
		at := 0.
		for _, h := range hidden {
			if h.lo > at {
				emit(at, h.lo)
			}
			at = max(at, h.hi)
		}
		emit(at, 1)
	}
	return result, nil
}

func hiddenPerspective(a, b projected, t [3]projected, tol float64) ([]span, error) {
	h := span{0, 1}
	d := turn(t[0], t[1], t[2])
	if !finite(d) {
		return nil, geom.ErrInvalid
	}
	if d == 0 {
		return nil, nil
	}
	sign := math.Copysign(1, d)
	p, q := 0., 0.
	for i := range 3 {
		j, k := (i+1)%3, (i+2)%3
		x, y := turn(t[j], t[k], a), turn(t[j], t[k], b)
		ok, e := keepPositive(&h, sign*x, sign*y)
		if e != nil || !ok {
			return nil, e
		}
		p += x / d * t[i].z
		q += y / d * t[i].z
	}
	// 1/edgeInv - 1/triInv > tol
	// iff triInv-edgeInv-tol*edgeInv*triInv > 0.
	da, dp := b.z-a.z, q-p
	c0, c1, c2 := p-a.z-tol*a.z*p, dp-da-tol*(a.z*dp+da*p), -tol*da*dp
	if !finite(c0) || !finite(c1) || !finite(c2) {
		return nil, geom.ErrInvalid
	}
	f := func(x float64) float64 { return (c2*x+c1)*x + c0 }
	keys := []float64{h.lo, h.hi}
	if c2 != 0 {
		x := -c1 / (2 * c2)
		if x > h.lo && x < h.hi {
			keys = []float64{h.lo, x, h.hi}
		}
	}
	var out []span
	for i := 0; i+1 < len(keys); i++ {
		l, r := keys[i], keys[i+1]
		x, y := f(l), f(r)
		if x <= 0 && y <= 0 {
			continue
		}
		if (x > 0) != (y > 0) {
			a, b := l, r
			for range 53 {
				m := a + (b-a)/2
				if m == a || m == b {
					break
				}
				if (f(m) > 0) == (x > 0) {
					a = m
				} else {
					b = m
				}
			}
			if x > 0 {
				r = a + (b-a)/2
			} else {
				l = a + (b-a)/2
			}
		}
		if r > l {
			out = append(out, span{l, r})
		}
	}
	return out, nil
}
