package trimmed

import (
	"context"
	"fmt"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// FaceSplit records the retained and appended face IDs and the new shared edge.
type FaceSplit struct{ First, Second, Edge int }

// SplitFace splits a face along a supplied parameter-space curve joining two
// existing outer-loop vertices. The new model edge is evaluated on the
// original surface, so both new faces share exact geometry. This operation
// supports a hole-free face with straight outer p-curves. The cut must form
// two simple, positively wound domains under the normal trim validation.
// Curved cuts additionally require a differentiable p-curve.
func (s Solid) SplitFace(faceIndex, from, to int, cut PCurve) (Solid, FaceSplit, error) {
	var report FaceSplit
	if faceIndex < 0 || faceIndex >= len(s.faces) {
		return Solid{}, report, fmt.Errorf("trimmed: face index out of range")
	}
	f := s.faces[faceIndex]
	n := len(f.Outer)
	if len(f.Holes) > 0 || from < 0 || to < 0 || from >= n || to >= n || from == to || (from+1)%n == to || (to+1)%n == from {
		return Solid{}, report, fmt.Errorf("trimmed: split requires nonadjacent corners of a hole-free face")
	}
	for _, u := range f.Outer {
		if u.Curve != nil {
			return Solid{}, report, fmt.Errorf("trimmed: split requires a straight outer wire")
		}
	}
	a, b := f.Outer[from].UVStart, f.Outer[to].UVStart
	if a == b {
		return Solid{}, report, fmt.Errorf("trimmed: coincident split corners")
	}
	if cut != nil {
		if _, ok := cut.(differentiableTrim); !ok {
			return Solid{}, report, fmt.Errorf("trimmed: cut requires derivative evaluation")
		}
		p, e := cut.EvalUV(0)
		if e != nil || p != a {
			return Solid{}, report, fmt.Errorf("trimmed: split start mismatch")
		}
		p, e = cut.EvalUV(1)
		if e != nil || p != b {
			return Solid{}, report, fmt.Errorf("trimmed: split end mismatch")
		}
	}
	if e := validateCut(f.Outer, from, to, cut); e != nil {
		return Solid{}, report, e
	}
	modelIndex := func(u Use) int {
		e := s.edges[u.Edge]
		if u.Reversed {
			return e.End
		}
		return e.Start
	}
	edges := s.Edges()
	edgeID := len(edges)
	model := surfaceTrimCurve{f.Surface, a, b, cut}
	edges = append(edges, parametric.Edge{Start: modelIndex(f.Outer[from]), End: modelIndex(f.Outer[to]), Curve: model, Range: parametric.Interval{Max: 1}})
	one, two := f, f
	walk := func(i, j int) []Use {
		var r []Use
		for ; i != j; i = (i + 1) % n {
			r = append(r, f.Outer[i])
		}
		return r
	}
	forward := Use{Edge: edgeID, UVStart: a, UVEnd: b, Curve: cut}
	backward := Use{Edge: edgeID, Reversed: true, UVStart: b, UVEnd: a}
	switch c := cut.(type) {
	case nil:
	case QuadraticUV:
		backward.Curve = QuadraticUV{c.End, c.Control, c.Start}
	case EllipseUV:
		c.From, c.To = c.To, c.From
		backward.Curve = c
	case SplineUV:
		c.Reversed = !c.Reversed
		backward.Curve = c
	default:
		backward.Curve = reverseTrim{cut.(differentiableTrim)}
	}
	one.Outer = append(walk(from, to), backward)
	two.Outer = append(walk(to, from), forward)
	if _, ok := cut.(QuadraticUV); ok || cut == nil {
		for _, f := range [2]Face{one, two} {
			p, e := certifyFaceLoops(context.Background(), f, trimvalidate.Options{})
			if e != nil {
				return Solid{}, FaceSplit{}, e
			}
			if p.Status != trimvalidate.Certified {
				return Solid{}, FaceSplit{}, fmt.Errorf("trimmed: unproved split domain: %s", p.Reason)
			}
		}
	}
	// A chord outside a concave face can create a positive and a negative
	// domain; NewSolid checks the winding and each resulting polygon.
	faces := s.Faces()
	faces[faceIndex] = one
	report = FaceSplit{faceIndex, len(faces), edgeID}
	faces = append(faces, two)
	out, e := NewSolid(s.vertices, edges, faces, s.tolerance)
	if e != nil {
		return Solid{}, FaceSplit{}, e
	}
	return out, report, nil
}

func validateCut(outer []Use, from, to int, cut PCurve) error {
	bad := fmt.Errorf("trimmed: split curve leaves or touches the outer wire")
	turn := func(a, b, c geom.Vec2) int { s, _ := geom.Orient2D(a, b, c); return s }
	on := func(p, a, b geom.Vec2) bool {
		return turn(a, b, p) == 0 && p.X >= min(a.X, b.X) && p.X <= max(a.X, b.X) && p.Y >= min(a.Y, b.Y) && p.Y <= max(a.Y, b.Y)
	}
	crosses := func(a, b, c, d geom.Vec2) bool {
		x, y, z, w := turn(a, b, c), turn(a, b, d), turn(c, d, a), turn(c, d, b)
		return x*y < 0 && z*w < 0 || x == 0 && on(c, a, b) || y == 0 && on(d, a, b) || z == 0 && on(a, c, d) || w == 0 && on(b, c, d)
	}
	inside := func(p geom.Vec2) bool {
		w := 0
		for _, x := range outer {
			a, b := x.UVStart, x.UVEnd
			if on(p, a, b) {
				return false
			}
			if a.Y <= p.Y && b.Y > p.Y && turn(a, b, p) > 0 {
				w++
			}
			if a.Y > p.Y && b.Y <= p.Y && turn(a, b, p) < 0 {
				w--
			}
		}
		return w != 0
	}
	n := 2
	if cut != nil {
		n = 64
	}
	r := make([]geom.Vec2, n+1)
	u := Use{UVStart: outer[from].UVStart, UVEnd: outer[to].UVStart, Curve: cut}
	for i := range r {
		p, e := u.uv(float64(i) / float64(n))
		if e != nil {
			return e
		}
		if i > 0 && i < n && !inside(p) {
			return bad
		}
		r[i] = p
	}
	for i := 0; i < n; i++ {
		a, b := r[i], r[i+1]
		if a == b {
			return bad
		}
		for _, x := range outer {
			c, d := x.UVStart, x.UVEnd
			if !crosses(a, b, c, d) {
				continue
			}
			// Only one shared endpoint, not a collinear overlap, is permitted.
			if i == 0 && (a == c || a == d) && turn(c, d, b) != 0 || i == n-1 && (b == c || b == d) && turn(c, d, a) != 0 {
				continue
			}
			return bad
		}
		for j := 0; j < i-1; j++ {
			if crosses(a, b, r[j], r[j+1]) {
				return bad
			}
		}
	}
	return nil
}

type reverseTrim struct{ differentiableTrim }

func (r reverseTrim) EvalUV(t float64) (geom.Vec2, error) { return r.differentiableTrim.EvalUV(1 - t) }
func (r reverseTrim) DerivativeUV(t float64) (geom.Vec2, error) {
	d, e := r.differentiableTrim.DerivativeUV(1 - t)
	return scale2(d, -1), e
}

type surfaceTrimCurve struct {
	surface geom.Surface
	a, b    geom.Vec2
	curve   PCurve
}

func (c surfaceTrimCurve) transformed(m geom.Affine) surfaceTrimCurve {
	c.surface = parametric.MapSurface(c.surface, m)
	return c
}

func (c surfaceTrimCurve) Eval(t float64) (geom.Vec3, error) {
	p, e := (Use{UVStart: c.a, UVEnd: c.b, Curve: c.curve}).uv(t)
	if e != nil {
		return geom.Vec3{}, e
	}
	return c.surface.Eval(p.X, p.Y)
}
func (c surfaceTrimCurve) Derivative(t float64) (geom.Vec3, error) {
	p, e := (Use{UVStart: c.a, UVEnd: c.b, Curve: c.curve}).uv(t)
	if e != nil {
		return geom.Vec3{}, e
	}
	d := sub2(c.b, c.a)
	if q, ok := c.curve.(differentiableTrim); ok {
		d, e = q.DerivativeUV(t)
		if e != nil {
			return geom.Vec3{}, e
		}
	}
	u, v, e := c.surface.Partials(p.X, p.Y)
	if e != nil {
		return geom.Vec3{}, e
	}
	return u.Scale(d.X).Add(v.Scale(d.Y)), nil
}
