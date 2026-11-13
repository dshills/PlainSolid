package trimmed

import (
	"context"
	"math"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

type ExtrusionCircle struct {
	Center geom.Vec2
	Radius float64
}

type anchoredCircularUV struct {
	EllipseUV
	Start, End geom.Vec2
}

func newAnchoredCircularUV(x EllipseUV, start, end geom.Vec2) (anchoredCircularUV, error) {
	r := x.Cos.X
	if !finite(r) || r <= 0 || x.Cos.Y != 0 || x.Sin != (geom.Vec2{Y: -r}) || x.Linear != (geom.Vec2{}) || !uvFinite(x.Center) {
		return anchoredCircularUV{}, geom.ErrInvalid
	}
	p := [4]geom.Vec2{{X: x.Center.X + r, Y: x.Center.Y}, {X: x.Center.X, Y: x.Center.Y - r},
		{X: x.Center.X - r, Y: x.Center.Y}, {X: x.Center.X, Y: x.Center.Y + r}}
	for i, a := range p {
		if x.From == float64(i)*math.Pi/2 && x.To == float64(i+1)*math.Pi/2 && start == a && end == p[(i+1)%4] && uvFinite(start) && uvFinite(end) {
			return anchoredCircularUV{x, start, end}, nil
		}
	}
	return anchoredCircularUV{}, geom.ErrDomain
}

func (c anchoredCircularUV) EvalUV(t float64) (geom.Vec2, error) {
	if t == 0 {
		return c.Start, nil
	}
	if t == 1 {
		return c.End, nil
	}
	return c.EllipseUV.EvalUV(t)
}

func (c anchoredCircularUV) BoundsUV() (geom.UVBox, error) {
	a, b, e := trimBounds(Use{UVStart: c.Start, UVEnd: c.End, Curve: c.EllipseUV})
	return geom.UVBox{Min: a, Max: b}, e
}

// ExtrudePolygonWithCircularHoles creates a straight signed extrusion of a
// convex sketch polygon with circular interior wires. Holes are strictly
// disjoint and strictly inside the polygon, as proved in exact represented
// sketch coordinates. Cylindrical walls and circular cap arcs stay analytic.
// The same authoritative circular arcs are shared between each cap and wall.
func ExtrudePolygonWithCircularHoles(ctx context.Context, frame geom.Frame, outer []geom.Vec2, holes []ExtrusionCircle, height float64) (Solid, error) {
	if ctx == nil || frame.ZAxis().Length() == 0 || !finite(height) || height == 0 || len(outer) < 3 || len(outer) > 64 || len(holes) > 16 {
		return Solid{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, e
	}
	p := append([]geom.Vec2(nil), outer...)
	for _, q := range p {
		if !uvFinite(q) {
			return Solid{}, geom.ErrInvalid
		}
	}
	sign, e := geom.Orient2D(p[0], p[1], p[2])
	if e != nil || sign == 0 {
		return Solid{}, geom.ErrDegenerate
	}
	if sign < 0 {
		for i, j := 0, len(p)-1; i < j; i, j = i+1, j-1 {
			p[i], p[j] = p[j], p[i]
		}
	}
	// Every nonincident vertex must be strictly left of each oriented
	// side; this establishes convexity and excludes self-intersection.
	for i, a := range p {
		b := p[(i+1)%len(p)]
		for j, c := range p {
			if j == i || j == (i+1)%len(p) {
				continue
			}
			q, e := geom.Orient2D(a, b, c)
			if e != nil || q <= 0 {
				return Solid{}, geom.ErrDegenerate
			}
		}
	}
	for i, h := range holes {
		if !uvFinite(h.Center) || !finite(h.Radius) || h.Radius <= 0 {
			return Solid{}, geom.ErrInvalid
		}
		r2 := new(big.Rat).Mul(ratInset(h.Radius), ratInset(h.Radius))
		for j, a := range p {
			b := p[(j+1)%len(p)]
			x, y := new(big.Rat).Sub(ratInset(b.X), ratInset(a.X)), new(big.Rat).Sub(ratInset(b.Y), ratInset(a.Y))
			u, v := new(big.Rat).Sub(ratInset(h.Center.X), ratInset(a.X)), new(big.Rat).Sub(ratInset(h.Center.Y), ratInset(a.Y))
			c := new(big.Rat).Sub(new(big.Rat).Mul(x, v), new(big.Rat).Mul(y, u))
			len2 := new(big.Rat).Add(new(big.Rat).Mul(x, x), new(big.Rat).Mul(y, y))
			if c.Sign() <= 0 || new(big.Rat).Mul(c, c).Cmp(new(big.Rat).Mul(r2, len2)) <= 0 {
				return Solid{}, geom.ErrDomain
			}
		}
		for _, g := range holes[:i] {
			x, y := new(big.Rat).Sub(ratInset(h.Center.X), ratInset(g.Center.X)), new(big.Rat).Sub(ratInset(h.Center.Y), ratInset(g.Center.Y))
			r := new(big.Rat).Add(ratInset(h.Radius), ratInset(g.Radius))
			if new(big.Rat).Add(new(big.Rat).Mul(x, x), new(big.Rat).Mul(y, y)).Cmp(new(big.Rat).Mul(r, r)) <= 0 {
				return Solid{}, geom.ErrDomain
			}
		}
	}
	return extrudePolygonCircularTopology(ctx, frame, p, holes, min(0, height), max(0, height))
}

func extrudePolygonCircularTopology(ctx context.Context, frame geom.Frame, outer []geom.Vec2, holes []ExtrusionCircle, lo, hi float64) (Solid, error) {
	var vertices []geom.Vec3
	var edges []parametric.Edge
	var caps [2]Face
	for i, z := range [2]float64{lo, hi} {
		f, e := geom.NewFrame(frame.ToWorld(geom.Vec3{Z: z}), frame.XAxis(), frame.ZAxis())
		if e != nil {
			return Solid{}, e
		}
		x, e := geom.NewPlaneSurface(f)
		if e != nil {
			return Solid{}, e
		}
		caps[i] = Face{Surface: x, Reversed: i == 0}
	}
	add := func(a, b int, surface geom.Surface, start, end geom.Vec2) int {
		id := len(edges)
		edges = append(edges, parametric.Edge{Start: a, End: b, Curve: surfaceTrimCurve{surface, start, end, nil}, Range: parametric.Interval{Max: 1}})
		return id
	}
	var walls []Face
	loop := func(p []geom.Vec2, chart func(int) (geom.Surface, [4]geom.Vec2, error), hole *ExtrusionCircle) error {
		n := len(p)
		base := len(vertices)
		for _, f := range caps {
			for _, a := range p {
				x, e := f.Surface.Eval(a.X, a.Y)
				if e != nil {
					return e
				}
				vertices = append(vertices, x)
			}
		}
		var rings [2][]Use
		vertical := make([]int, n)
		for i := range vertical {
			vertical[i] = -1
		}
		for i, a := range p {
			if e := ctx.Err(); e != nil {
				return e
			}
			j := (i + 1) % n
			s, u, e := chart(i)
			if e != nil {
				return e
			}
			for k, v := range [2]int{i, j} {
				if vertical[v] < 0 {
					vertical[v] = add(base+v, base+n+v, s, u[k], u[3-k])
				}
			}
			low := add(base+i, base+j, s, u[0], u[1])
			high := add(base+n+i, base+n+j, s, u[3], u[2])
			f := Face{Surface: s}
			ids := [4]int{low, vertical[j], high, vertical[i]}
			if hole == nil {
				for k, id := range ids {
					f.Outer = append(f.Outer, Use{Edge: id, Reversed: k >= 2, UVStart: u[k], UVEnd: u[(k+1)%4]})
				}
			} else {
				// In the hole cylinder chart axial V decreases toward the
				// high cap. Reverse UV traversal, then reverse the face's
				// orientation to retain its inward physical normal.
				f.Reversed = true
				for _, k := range [4]int{3, 2, 1, 0} {
					f.Outer = append(f.Outer, Use{Edge: ids[k], Reversed: k < 2, UVStart: u[(k+1)%4], UVEnd: u[k]})
				}
			}
			walls = append(walls, f)
			for k, id := range [2]int{low, high} {
				q := Use{Edge: id, UVStart: a, UVEnd: p[j]}
				if hole != nil {
					q.Curve, e = newAnchoredCircularUV(EllipseUV{Center: hole.Center, Cos: geom.Vec2{X: hole.Radius}, Sin: geom.Vec2{Y: -hole.Radius},
						From: float64(i) * math.Pi / 2, To: float64(i+1) * math.Pi / 2}, a, p[j])
					if e != nil {
						return e
					}
				}
				rings[k] = append(rings[k], q)
			}
		}
		for i, r := range rings {
			if hole == nil {
				caps[i].Outer = r
			} else {
				caps[i].Holes = append(caps[i].Holes, r)
			}
		}
		return nil
	}
	if e := loop(outer, func(i int) (geom.Surface, [4]geom.Vec2, error) {
		a, b := outer[i], outer[(i+1)%len(outer)]
		axial := frame.ZAxis()
		u := frame.XAxis().Scale(b.X - a.X).Add(frame.YAxis().Scale(b.Y - a.Y))
		f, e := geom.NewFrame(frame.ToWorld(geom.Vec3{X: a.X, Y: a.Y}), u, u.Cross(axial))
		if e != nil {
			return nil, [4]geom.Vec2{}, e
		}
		s, e := geom.NewPlaneSurface(f)
		length := u.Length()
		return s, [4]geom.Vec2{{Y: lo}, {X: length, Y: lo}, {X: length, Y: hi}, {Y: hi}}, e
	}, nil); e != nil {
		return Solid{}, e
	}
	for _, h := range holes {
		f, e := geom.NewFrame(frame.ToWorld(geom.Vec3{X: h.Center.X, Y: h.Center.Y}), frame.XAxis(), frame.ZAxis().Scale(-1))
		if e != nil {
			return Solid{}, e
		}
		s, e := geom.NewCylinderSurface(f, h.Radius)
		if e != nil {
			return Solid{}, e
		}
		p := []geom.Vec2{{X: h.Center.X + h.Radius, Y: h.Center.Y}, {X: h.Center.X, Y: h.Center.Y - h.Radius},
			{X: h.Center.X - h.Radius, Y: h.Center.Y}, {X: h.Center.X, Y: h.Center.Y + h.Radius}}
		if e := loop(p, func(i int) (geom.Surface, [4]geom.Vec2, error) {
			a, b := float64(i)*math.Pi/2, float64(i+1)*math.Pi/2
			return s, [4]geom.Vec2{{X: a, Y: -lo}, {X: b, Y: -lo}, {X: b, Y: -hi}, {X: a, Y: -hi}}, nil
		}, &h); e != nil {
			return Solid{}, e
		}
	}
	scale := hi - lo
	for _, q := range outer {
		scale = max(scale, math.Abs(q.X-outer[0].X), math.Abs(q.Y-outer[0].Y))
	}
	return NewSolidContext(ctx, vertices, edges, append(caps[:], walls...), scale*1e-11)
}
