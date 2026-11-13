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

// ExtrusionBlindHole starts on the sketch cap at axial zero and advances
// toward the signed extrusion endpoint by the positive depth.
type ExtrusionBlindHole struct {
	ExtrusionCircle
	Depth float64
}

type anchoredCircularUV struct {
	EllipseUV
	Start, End geom.Vec2
}

func newAnchoredCircularUV(x EllipseUV, start, end geom.Vec2) (anchoredCircularUV, error) {
	r := x.Cos.X
	if !finite(r) || r <= 0 || x.Cos.Y != 0 || math.Abs(x.Sin.Y) != r || x.Sin.X != 0 || x.Linear != (geom.Vec2{}) || !uvFinite(x.Center) {
		return anchoredCircularUV{}, geom.ErrInvalid
	}
	p := [4]geom.Vec2{{X: x.Center.X + r, Y: x.Center.Y}, {X: x.Center.X, Y: x.Center.Y + x.Sin.Y},
		{X: x.Center.X - r, Y: x.Center.Y}, {X: x.Center.X, Y: x.Center.Y - x.Sin.Y}}
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
	return ExtrudePolygonWithHoles(ctx, frame, outer, holes, nil, height)
}

// ExtrudePolygonWithHoles retains circular through holes and finite blind
// holes with analytic cylindrical walls and planar floors. Every footprint is
// disjoint, and each blind hole starts at axial zero of the original sketch.
func ExtrudePolygonWithHoles(ctx context.Context, frame geom.Frame, outer []geom.Vec2, through []ExtrusionCircle, blind []ExtrusionBlindHole, height float64) (Solid, error) {
	holes, e := validateExtrusionHoles(ctx, frame, through, blind, height)
	if e != nil {
		return Solid{}, e
	}
	if len(outer) < 3 || len(outer) > 64 {
		return Solid{}, geom.ErrInvalid
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
	for _, h := range holes {
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
	}
	return extrudeCircularTopology(ctx, frame, p, nil, through, blind, min(0, height), max(0, height))
}

func extrudeCircularTopology(ctx context.Context, frame geom.Frame, outer []geom.Vec2, disk *ExtrusionCircle, holes []ExtrusionCircle,
	blind []ExtrusionBlindHole, lo, hi float64) (Solid, error) {
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
	var floors []Face
	loop := func(p []geom.Vec2, chart func(int) (geom.Surface, [4]geom.Vec2, error), circle *ExtrusionCircle, inner bool, floor *Face, entry int) error {
		n := len(p)
		base := len(vertices)
		var endFaces = [2]*Face{&caps[0], &caps[1]}
		if floor != nil {
			endFaces[entry^1] = floor
		}
		for _, f := range endFaces {
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
			if !inner {
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
				if circle != nil && (floor == nil || k == entry) {
					q.Curve, e = extrusionArc(*circle, i, a, p[j], inner)
					if e != nil {
						return e
					}
				}
				rings[k] = append(rings[k], q)
			}
		}
		for i, r := range rings {
			if !inner {
				caps[i].Outer = r
			} else if floor == nil || i == entry {
				caps[i].Holes = append(caps[i].Holes, r)
			} else {
				// A floor's boundary is the same model edge but its own
				// positive UV loop runs counterclockwise.
				for k := range r {
					q := r[len(r)-1-k]
					x := EllipseUV{Center: circle.Center, Cos: geom.Vec2{X: circle.Radius}, Sin: geom.Vec2{Y: circle.Radius},
						From: float64(k) * math.Pi / 2, To: float64(k+1) * math.Pi / 2}
					c, e := newAnchoredCircularUV(x, q.UVEnd, q.UVStart)
					if e != nil {
						return e
					}
					floor.Outer = append(floor.Outer, Use{Edge: q.Edge, Reversed: true, UVStart: q.UVEnd, UVEnd: q.UVStart, Curve: c})
				}
			}
		}
		return nil
	}
	if disk == nil {
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
		}, nil, false, nil, 0); e != nil {
			return Solid{}, e
		}
	}
	addCircle := func(h ExtrusionCircle, z0, z1 float64, inner bool, floor *Face, entry int) error {
		sign := 1.
		if inner {
			sign = -1
		}
		f, e := geom.NewFrame(frame.ToWorld(geom.Vec3{X: h.Center.X, Y: h.Center.Y}), frame.XAxis(), frame.ZAxis().Scale(sign))
		if e != nil {
			return e
		}
		s, e := geom.NewCylinderSurface(f, h.Radius)
		if e != nil {
			return e
		}
		p := []geom.Vec2{{X: h.Center.X + h.Radius, Y: h.Center.Y}, {X: h.Center.X, Y: h.Center.Y + sign*h.Radius},
			{X: h.Center.X - h.Radius, Y: h.Center.Y}, {X: h.Center.X, Y: h.Center.Y - sign*h.Radius}}
		return loop(p, func(i int) (geom.Surface, [4]geom.Vec2, error) {
			a, b := float64(i)*math.Pi/2, float64(i+1)*math.Pi/2
			return s, [4]geom.Vec2{{X: a, Y: sign * z0}, {X: b, Y: sign * z0}, {X: b, Y: sign * z1}, {X: a, Y: sign * z1}}, nil
		}, &h, inner, floor, entry)
	}
	if disk != nil {
		if e := addCircle(*disk, lo, hi, false, nil, 0); e != nil {
			return Solid{}, e
		}
	}

	for _, h := range holes {
		if e := addCircle(h, lo, hi, true, nil, 0); e != nil {
			return Solid{}, e
		}
	}
	for _, h := range blind {
		depth, entry := h.Depth, 0
		if lo < 0 {
			depth, entry = -depth, 1
		}
		f, e := geom.NewFrame(frame.ToWorld(geom.Vec3{Z: depth}), frame.XAxis(), frame.ZAxis())
		if e != nil {
			return Solid{}, e
		}
		plane, e := geom.NewPlaneSurface(f)
		if e != nil {
			return Solid{}, e
		}
		floor := Face{Surface: plane, Reversed: entry == 0}
		if e := addCircle(h.ExtrusionCircle, min(0, depth), max(0, depth), true, &floor, entry); e != nil {
			return Solid{}, e
		}
		floors = append(floors, floor)
	}
	scale := hi - lo
	if disk != nil {
		scale = max(scale, disk.Radius)
	}
	for _, q := range outer {
		scale = max(scale, math.Abs(q.X-outer[0].X), math.Abs(q.Y-outer[0].Y))
	}
	return NewSolidContext(ctx, vertices, edges, append(append(caps[:], walls...), floors...), scale*1e-11)
}
