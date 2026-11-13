package trimmed

import (
	"context"
	"fmt"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// FaceLoopSplit records the original face (which acquires a hole), the new
// inner face, and the added vertices and edges in loop order.
type FaceLoopSplit struct {
	Outer, Inner    int
	Vertices, Edges []int
	// UVTopologyCertified reports an exact proof for both output faces.
	UVTopologyCertified bool
}

// SplitFaceLoop divides a face using a closed polygonal loop in its parameter
// domain. Loop points must be counterclockwise and strictly inside the face.
// Disjoint existing holes are kept on the appropriate output face.
// The shared model edges are evaluated on the original surface. Both new
// faces retain the original surface and the operation preserves the solid.
func (s Solid) SplitFaceLoop(faceIndex int, loop []geom.Vec2) (Solid, FaceLoopSplit, error) {
	return s.splitFaceLoop(context.Background(), faceIndex, loop, nil, true)
}

func (s Solid) splitFaceLoop(ctx context.Context, faceIndex int, loop []geom.Vec2, curves []PCurve, prove bool) (Solid, FaceLoopSplit, error) {
	var h FaceLoopSplit
	if ctx == nil || faceIndex < 0 || faceIndex >= len(s.faces) || len(loop) < 3 || len(loop) > 64 ||
		curves != nil && len(curves) != len(loop) {
		return Solid{}, h, fmt.Errorf("trimmed: invalid loop split")
	}
	f := s.faces[faceIndex]
	vertices, edges, faces := s.Vertices(), s.Edges(), s.Faces()
	h = FaceLoopSplit{Outer: faceIndex, Inner: len(faces), UVTopologyCertified: prove}
	for i, u := range loop {
		if e := ctx.Err(); e != nil {
			return Solid{}, FaceLoopSplit{}, e
		}
		if !uvFinite(u) || u == loop[(i+1)%len(loop)] {
			return Solid{}, FaceLoopSplit{}, fmt.Errorf("trimmed: invalid loop vertex")
		}
		p, e := f.Surface.Eval(u.X, u.Y)
		if e != nil || !p.IsFinite() {
			return Solid{}, FaceLoopSplit{}, fmt.Errorf("trimmed: invalid loop surface point")
		}
		h.Vertices = append(h.Vertices, len(vertices))
		vertices = append(vertices, p)
	}
	inner := Face{Surface: f.Surface, Reversed: f.Reversed}
	outer := cloneFace(f)
	outer.Holes = nil
	hole := make([]Use, len(loop))
	for i, a := range loop {
		j := (i + 1) % len(loop)
		b := loop[j]
		id := len(edges)
		var c PCurve
		if curves != nil {
			c = curves[i]
		}
		edges = append(edges, parametric.Edge{Start: h.Vertices[i], End: h.Vertices[j], Curve: surfaceTrimCurve{f.Surface, a, b, c}, Range: parametric.Interval{Max: 1}})
		h.Edges = append(h.Edges, id)
		inner.Outer = append(inner.Outer, Use{Edge: id, UVStart: a, UVEnd: b, Curve: c})
		hole[len(loop)-1-i] = Use{Edge: id, Reversed: true, UVStart: b, UVEnd: a, Curve: reversePCurve(c)}
	}
	outer.Holes = append(outer.Holes, hole)
	if prove {
		for _, existing := range f.Holes {
			a, b := outer, inner
			a.Holes = [][]Use{hole, existing}
			b.Holes = [][]Use{existing}
			owner, e := certifiedHoleOwner(ctx, [2]Face{a, b})
			if e != nil {
				return Solid{}, FaceLoopSplit{}, e
			}
			if owner == 0 {
				outer.Holes = append(outer.Holes, existing)
			} else {
				inner.Holes = append(inner.Holes, existing)
			}
		}
		for _, fragment := range [2]Face{outer, inner} {
			p, e := certifyFaceLoops(ctx, fragment, trimvalidate.Options{})
			if e != nil {
				return Solid{}, FaceLoopSplit{}, e
			}
			if p.Status != trimvalidate.Certified {
				return Solid{}, FaceLoopSplit{}, fmt.Errorf("trimmed: unproved loop split: %s", p.Reason)
			}
		}
	}
	faces[faceIndex] = outer
	faces = append(faces, inner)
	r, e := NewSolidContext(ctx, vertices, edges, faces, s.tolerance)
	if e != nil {
		return Solid{}, FaceLoopSplit{}, e
	}
	return r, h, nil
}

// Both trial faces are complete and include the hole under consideration.
func certifiedHoleOwner(ctx context.Context, trials [2]Face) (int, error) {
	owner := -1
	for i, f := range trials {
		proof, e := certifyFaceLoops(ctx, f, trimvalidate.Options{})
		if e != nil {
			return -1, e
		}
		if proof.Status == trimvalidate.Certified {
			if owner >= 0 {
				return -1, fmt.Errorf("trimmed: hole belongs to both fragments")
			}
			owner = i
		}
	}
	if owner < 0 {
		return -1, fmt.Errorf("trimmed: cannot prove hole placement after split")
	}
	return owner, nil
}

// SplitFaceCurvedLoop divides a face using a closed loop of
// differentiable parameter curves. Their endpoints form a strictly contained
// simple CCW polygon. Supported rational spline loops require an exact UV
// topology proof and support existing certified holes and curved boundaries.
// Other curves use sampled validation on a straight hole-free face only.
// Shared model curves are evaluated on the original surface.
func (s Solid) SplitFaceCurvedLoop(faceIndex int, curves []PCurve) (Solid, FaceLoopSplit, error) {
	return s.SplitFaceCurvedLoopContext(context.Background(), faceIndex, curves)
}

// SplitFaceCurvedLoopContext supports cancellation during curve and
// reconstructed boundary validation.
func (s Solid) SplitFaceCurvedLoopContext(ctx context.Context, faceIndex int, curves []PCurve) (Solid, FaceLoopSplit, error) {
	h := FaceLoopSplit{}
	if ctx == nil || faceIndex < 0 || faceIndex >= len(s.faces) || len(curves) < 3 || len(curves) > 64 {
		return Solid{}, h, fmt.Errorf("trimmed: invalid curved loop size")
	}
	points := make([]geom.Vec2, len(curves))
	for i, c := range curves {
		if e := ctx.Err(); e != nil {
			return Solid{}, h, e
		}
		if _, ok := c.(differentiableTrim); !ok {
			return Solid{}, h, fmt.Errorf("trimmed: loop requires differentiable trims")
		}
		p, e := c.EvalUV(0)
		if e != nil || !uvFinite(p) {
			return Solid{}, h, fmt.Errorf("trimmed: invalid curved loop endpoint")
		}
		points[i] = p
	}
	supported := true
	for i, c := range curves {
		p, e := c.EvalUV(1)
		if e != nil || p != points[(i+1)%len(points)] {
			return Solid{}, h, fmt.Errorf("trimmed: open curved loop")
		}
		supported = supportsTrimProof(Use{UVStart: points[i], UVEnd: p, Curve: c}) && supported
	}
	if !supported {
		f := s.faces[faceIndex]
		if len(f.Holes) != 0 {
			return Solid{}, h, fmt.Errorf("trimmed: uncertified loop on a face with holes")
		}
		for _, u := range f.Outer {
			if u.Curve != nil {
				return Solid{}, h, fmt.Errorf("trimmed: uncertified loop on a face with curved boundaries")
			}
		}
		sampled := make([]geom.Vec2, 0, len(curves)*16)
		for _, c := range curves {
			if e := ctx.Err(); e != nil {
				return Solid{}, FaceLoopSplit{}, e
			}
			var previous geom.Vec2
			for k := 0; k < 16; k++ {
				p, e := c.EvalUV(float64(k) / 16)
				if e != nil || !uvFinite(p) || k > 0 && p == previous {
					return Solid{}, FaceLoopSplit{}, fmt.Errorf("trimmed: invalid curved loop sample")
				}
				sampled = append(sampled, p)
				previous = p
			}
		}
		if e := validateLoopSamples(f.Outer, sampled); e != nil {
			return Solid{}, FaceLoopSplit{}, e
		}
	}
	return s.splitFaceLoop(ctx, faceIndex, points, curves, supported)
}

func validateLoopSamples(outer []Use, s []geom.Vec2) error {
	bad := fmt.Errorf("trimmed: sampled loop crosses its boundary or itself")
	turn := func(a, b, c geom.Vec2) int { v, _ := geom.Orient2D(a, b, c); return v }
	on := func(p, a, b geom.Vec2) bool {
		return turn(a, b, p) == 0 && p.X >= min(a.X, b.X) && p.X <= max(a.X, b.X) && p.Y >= min(a.Y, b.Y) && p.Y <= max(a.Y, b.Y)
	}
	cross := func(a, b, c, d geom.Vec2) bool {
		if max(a.X, b.X) < min(c.X, d.X) || max(c.X, d.X) < min(a.X, b.X) || max(a.Y, b.Y) < min(c.Y, d.Y) || max(c.Y, d.Y) < min(a.Y, b.Y) {
			return false
		}
		x, y, z, w := turn(a, b, c), turn(a, b, d), turn(c, d, a), turn(c, d, b)
		return x*y < 0 && z*w < 0 || x == 0 && on(c, a, b) || y == 0 && on(d, a, b) || z == 0 && on(a, c, d) || w == 0 && on(b, c, d)
	}
	for i, a := range s {
		b := s[(i+1)%len(s)]
		if a == b {
			return bad
		}
		w := 0
		for _, x := range outer {
			c, d := x.UVStart, x.UVEnd
			if cross(a, b, c, d) {
				return bad
			}
			if c.Y <= a.Y && d.Y > a.Y && turn(c, d, a) > 0 {
				w++
			}
			if c.Y > a.Y && d.Y <= a.Y && turn(c, d, a) < 0 {
				w--
			}
		}
		if w == 0 {
			return bad
		}
		for j := 0; j < i-1; j++ {
			if i == len(s)-1 && j == 0 {
				continue
			}
			if cross(a, b, s[j], s[j+1]) {
				return bad
			}
		}
	}
	return nil
}
