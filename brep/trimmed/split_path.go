package trimmed

import (
	"context"
	"fmt"
	"math"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// CutSegment supplies one directed segment of an open face cut. Curve is
// nil for a line, otherwise its normalized endpoints must match Start/End.
type CutSegment struct {
	Start, End geom.Vec2
	Curve      PCurve
}

// FacePathSplit records retained/appended faces and generated entities
// in cut direction. The first/last cut vertices are existing face vertices.
type FacePathSplit struct {
	First, Second           int
	Edges, InteriorVertices []int
	UVTopologyCertified     bool
}

// SplitFacePath splits a face along a connected open cut between two original
// outer vertices. New model edges lie on the original surface and are shared
// by the resulting faces. A face with curved boundaries or holes requires an
// exact UV topology proof, including strict containment of every original hole
// in one output fragment. For a straight hole-free input, unsupported
// differentiable cuts can still use sampled validation.
func (s Solid) SplitFacePath(ctx context.Context, faceIndex, from, to int, path []CutSegment) (Solid, FacePathSplit, error) {
	bad := func(reason string) (Solid, FacePathSplit, error) {
		return Solid{}, FacePathSplit{}, fmt.Errorf("trimmed: invalid split path: %s", reason)
	}
	if ctx == nil || faceIndex < 0 || faceIndex >= len(s.faces) || len(path) == 0 || len(path) > 64 {
		return bad("invalid face, context or segment count")
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, FacePathSplit{}, e
	}
	f := s.faces[faceIndex]
	n := len(f.Outer)
	if from < 0 || from >= n || to < 0 || to >= n || from == to ||
		len(path) == 1 && ((from+1)%n == to || (to+1)%n == from) {
		return bad("invalid loop or endpoint indices")
	}
	if path[0].Start != f.Outer[from].UVStart || path[len(path)-1].End != f.Outer[to].UVStart {
		return bad("endpoint does not match face vertex")
	}
	provable := true
	for i, x := range path {
		if !uvFinite(x.Start) || !uvFinite(x.End) || x.Start == x.End || i > 0 && x.Start != path[i-1].End {
			return bad("disconnected or degenerate segment")
		}
		if x.Curve == nil {
			continue
		}
		if !supportsTrimProof(Use{UVStart: x.Start, UVEnd: x.End, Curve: x.Curve}) {
			provable = false
		}
		if _, ok := x.Curve.(differentiableTrim); !ok {
			return bad("curve lacks derivatives")
		}
		a, e := x.Curve.EvalUV(0)
		if e != nil || a != x.Start {
			return bad("curve start mismatch")
		}
		b, e := x.Curve.EvalUV(1)
		if e != nil || b != x.End {
			return bad("curve end mismatch")
		}
	}
	if !provable {
		if len(f.Holes) != 0 {
			return bad("uncertified path on a face with holes")
		}
		for _, x := range f.Outer {
			if x.Curve != nil {
				return bad("uncertified path on a face with curved boundaries")
			}
		}
		if e := validateCut(f.Outer, from, to, cutPath{path}); e != nil {
			return Solid{}, FacePathSplit{}, e
		}
	}
	vertices, edges, faces := s.Vertices(), s.Edges(), s.Faces()
	vtx := func(u Use) int {
		e := edges[u.Edge]
		if u.Reversed {
			return e.End
		}
		return e.Start
	}
	h := FacePathSplit{First: faceIndex, Second: len(faces), UVTopologyCertified: provable}
	a := vtx(f.Outer[from])
	b := vtx(f.Outer[to])
	fwd, rev := make([]Use, len(path)), make([]Use, len(path))
	for i, p := range path {
		if e := ctx.Err(); e != nil {
			return Solid{}, FacePathSplit{}, e
		}
		end := b
		if i+1 < len(path) {
			x, e := f.Surface.Eval(p.End.X, p.End.Y)
			if e != nil {
				return Solid{}, FacePathSplit{}, e
			}
			end = len(vertices)
			vertices = append(vertices, x)
			h.InteriorVertices = append(h.InteriorVertices, end)
		}
		id := len(edges)
		edges = append(edges, parametric.Edge{Start: a, End: end, Curve: surfaceTrimCurve{f.Surface, p.Start, p.End, p.Curve}, Range: parametric.Interval{Max: 1}})
		h.Edges = append(h.Edges, id)
		fwd[i] = Use{Edge: id, UVStart: p.Start, UVEnd: p.End, Curve: p.Curve}
		rev[len(path)-1-i] = Use{Edge: id, Reversed: true, UVStart: p.End, UVEnd: p.Start, Curve: reversePCurve(p.Curve)}
		a = end
	}
	walk := func(a, b int) []Use {
		var l []Use
		for ; a != b; a = (a + 1) % n {
			l = append(l, f.Outer[a])
		}
		return l
	}
	x, y := f, f
	x.Outer, y.Outer = append(walk(from, to), rev...), append(walk(to, from), fwd...)
	x.Holes, y.Holes = nil, nil
	if provable {
		for _, hole := range f.Holes {
			a, b := x, y
			a.Holes, b.Holes = [][]Use{hole}, [][]Use{hole}
			owner, e := certifiedHoleOwner(ctx, [2]Face{a, b})
			if e != nil {
				return Solid{}, FacePathSplit{}, e
			}
			if owner == 0 {
				x.Holes = append(x.Holes, hole)
			} else {
				y.Holes = append(y.Holes, hole)
			}
		}
		for _, face := range [2]Face{x, y} {
			p, e := certifyFaceLoops(ctx, face, trimvalidate.Options{})
			if e != nil {
				return Solid{}, FacePathSplit{}, e
			}
			if p.Status != trimvalidate.Certified {
				return bad(p.Reason)
			}
		}
	}
	faces[faceIndex] = x
	faces = append(faces, y)
	out, e := NewSolidContext(ctx, vertices, edges, faces, s.tolerance)
	if e != nil {
		return Solid{}, FacePathSplit{}, e
	}
	return out, h, nil
}

func reversePCurve(c PCurve) PCurve {
	switch p := c.(type) {
	case nil:
		return nil
	case QuadraticUV:
		return QuadraticUV{p.End, p.Control, p.Start}
	case ExactQuadraticUV:
		return p.reversed()
	case EllipseUV:
		p.From, p.To = p.To, p.From
		return p
	case SplineUV:
		p.Reversed = !p.Reversed
		return p
	case *QuadraticUV:
		return QuadraticUV{p.End, p.Control, p.Start}
	case *SplineUV:
		q := *p
		q.Reversed = !q.Reversed
		return q
	case reverseTrim:
		return p.differentiableTrim
	default:
		return reverseTrim{c.(differentiableTrim)}
	}
}

type cutPath struct{ segments []CutSegment }

func (c cutPath) EvalUV(t float64) (geom.Vec2, error) {
	if !finite(t) || t < 0 || t > 1 {
		return geom.Vec2{}, geom.ErrDomain
	}
	x := float64(len(c.segments)) * t
	i := min(int(math.Floor(x)), len(c.segments)-1)
	s := c.segments[i]
	return (Use{UVStart: s.Start, UVEnd: s.End, Curve: s.Curve}).uv(x - float64(i))
}
