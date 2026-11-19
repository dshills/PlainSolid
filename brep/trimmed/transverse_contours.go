package trimmed

import (
	"context"
	"fmt"
	"runtime"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

type transverseRing struct {
	vertices []geom.Vec3
	parts    [2][]transverseRingUse
	boxes    [2]geom.UVBox
	base     int
	sections []int // optional explicit IDs when the ring spans opposing faces
}

type transverseRingUse struct {
	segment  int
	curve    PCurve
	from, to geom.Vec2
	reversed bool
}

func transverseRings(ctx context.Context, curves [][]intersect.TubeSegment, da, db geom.ParameterRectangle) ([]transverseRing, error) {
	var rings []transverseRing
	seen := map[geom.Vec3]bool{}
	count := 0
	for _, c := range curves {
		if len(c) < 3 || len(c) > 64 || count+len(c) > 256 {
			return nil, parametric.ErrBudget
		}
		count += len(c)
		r := transverseRing{base: count - len(c)}
		for i, s := range c {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
			x, e := s.Evaluate(0)
			if e != nil {
				return nil, e
			}
			if seen[x.Point] {
				return nil, geom.ErrUnrepresentable
			}
			seen[x.Point] = true
			r.vertices = append(r.vertices, x.Point)
			for j, trim := range [2]intersect.TubeTrim{s.TrimA(), s.TrimB()} {
				from, e := trim.EvalUV(0)
				if e != nil {
					return nil, e
				}
				to, e := trim.EvalUV(1)
				if e != nil {
					return nil, e
				}
				box, e := trim.BoundsUV()
				if e != nil {
					return nil, e
				}
				d := [2]geom.ParameterRectangle{da, db}[j]
				if box.Min.X <= d.UMin || box.Max.X >= d.UMax || box.Min.Y <= d.VMin || box.Max.Y >= d.VMax {
					return nil, geom.ErrConvergence
				}
				if i == 0 {
					r.boxes[j] = box
				} else {
					q := &r.boxes[j]
					q.Min.X, q.Min.Y = min(q.Min.X, box.Min.X), min(q.Min.Y, box.Min.Y)
					q.Max.X, q.Max.Y = max(q.Max.X, box.Max.X), max(q.Max.Y, box.Max.Y)
				}
				r.parts[j] = append(r.parts[j], transverseRingUse{i, trim, from, to, false})
			}
		}
		for j, part := range r.parts {
			var e error
			r.parts[j], e = orientTransverseRing(ctx, part)
			if e != nil {
				return nil, e
			}
		}
		rings = append(rings, r)
	}
	return rings, nil
}

func orientTransverseRing(ctx context.Context, part []transverseRingUse) ([]transverseRingUse, error) {
	points := make([]geom.Vec2, len(part))
	polygon := make([]geom.Vec3, len(part))
	for i, s := range part {
		points[i] = s.from
		polygon[i] = geom.Vec3{X: s.from.X, Y: s.from.Y}
		if s.to != part[(i+1)%len(part)].from || s.from == s.to {
			return nil, geom.ErrConvergence
		}
	}
	sign := loopOrientation(points)
	if sign == 0 {
		return nil, geom.ErrConvergence
	}
	if sign < 0 {
		part = reverseTransverseRing(part)
		for i, k := 0, len(polygon)-1; i < k; i, k = i+1, k-1 {
			polygon[i], polygon[k] = polygon[k], polygon[i]
		}
	}
	triangles, e := triangulateUV([][]geom.Vec3{polygon})
	if e != nil {
		return nil, fmt.Errorf("trimmed: transverse contour chord loop: %w", e)
	}
	if e := certifyTransverseRingWinding(ctx, part, triangles); e != nil {
		return nil, e
	}
	return part, nil
}

// transverseRingNesting classifies each connected ring by winding of every
// other exact branch around a certified point box on that ring. Injective
// support charts and the independent branch disjointness proof guarantee
// that membership cannot change while traversing a ring.
func transverseRingNesting(ctx context.Context, rings []transverseRing, chart int) ([]int, []int, error) {
	n := len(rings)
	inside := make([][]bool, n)
	depth, parent := make([]int, n), make([]int, n)
	for i, ring := range rings {
		inside[i] = make([]bool, n)
		for j, other := range rings {
			if i == j || !uvBoxesOverlap(ring.boxes[chart], other.boxes[chart]) {
				continue
			}
			if sharedExactRingEndpoint(ring.parts[chart], other.parts[chart]) {
				return nil, nil, fmt.Errorf("trimmed: contours %d and %d share a source chart endpoint: %w", i, j, geom.ErrConvergence)
			}
			face := Face{}
			for _, u := range other.parts[chart] {
				face.Outer = append(face.Outer, Use{UVStart: u.from, UVEnd: u.to, Curve: u.curve})
			}
			runtime.KeepAlive(&other) // golang/go#82026: keeps other in the stack map. Remove once the Go fix is in use.
			known := false
			for _, sample := range ring.parts[chart][:min(8, len(ring.parts[chart]))] {
				trim, reversed, ok := trimTube(sample.curve)
				if !ok {
					return nil, nil, geom.ErrInvalid
				}
				t := 0.
				if reversed {
					t = 1
				}
				q, e := trim.EvaluateBounds(t)
				if e != nil {
					return nil, nil, e
				}
				loc, resolved, e := classifyTrimBox(ctx, face, q, 128)
				if e != nil {
					return nil, nil, e
				}
				if resolved {
					known, inside[i][j] = true, loc == parametric.Inside
					break
				}
			}
			if !known {
				return nil, nil, fmt.Errorf("trimmed: cannot prove contour %d relative to contour %d: %w", i, j, geom.ErrConvergence)
			}
			if inside[i][j] {
				depth[i]++
			}
		}
	}
	for i := range rings {
		parent[i] = -1
		for j := range rings {
			if !inside[i][j] {
				continue
			}
			if depth[j] >= depth[i] {
				return nil, nil, geom.ErrConvergence
			}
			if depth[j]+1 == depth[i] {
				if parent[i] >= 0 {
					return nil, nil, geom.ErrConvergence
				}
				parent[i] = j
			}
		}
		if depth[i] != 0 && parent[i] < 0 {
			return nil, nil, geom.ErrConvergence
		}
	}
	return parent, depth, nil
}

// The proof requires exact singleton UV enclosures on both curves; an equal
// displayed endpoint alone does not prove that distinct algebraic roots meet.
func sharedExactRingEndpoint(a, b []transverseRingUse) bool {
	endpoints := make(map[geom.Vec2]transverseRingUse, len(a))
	for _, p := range a {
		endpoints[p.from] = p
	}
	isExact := func(p transverseRingUse) bool {
		c, reversed, ok := trimTube(p.curve)
		if !ok {
			return false
		}
		t := 0.
		if reversed {
			t = 1
		}
		box, err := c.EvaluateBounds(t)
		return err == nil && box.Min == p.from && box.Max == p.from
	}
	for _, p := range b {
		if q, ok := endpoints[p.from]; ok && isExact(q) && isExact(p) {
			return true
		}
	}
	return false
}

func uvBoxesOverlap(a, b geom.UVBox) bool {
	return a.Min.X <= b.Max.X && a.Max.X >= b.Min.X && a.Min.Y <= b.Max.Y && a.Max.Y >= b.Min.Y
}

func certifyTransverseRingWinding(ctx context.Context, ring []transverseRingUse, triangles [][3]geom.Vec2) error {
	// An exact simple closed branch has a single orientation. A point with
	// certified nonzero winding fixes it independently of the chord polygon.
	face := Face{}
	for _, u := range ring {
		face.Outer = append(face.Outer, Use{UVStart: u.from, UVEnd: u.to, Curve: u.curve})
	}
	for _, tri := range triangles[:min(8, len(triangles))] {
		p := geom.Vec2{X: tri[0].X/3 + tri[1].X/3 + tri[2].X/3, Y: tri[0].Y/3 + tri[1].Y/3 + tri[2].Y/3}
		loc, known, e := classifyTrimBox(ctx, face, geom.UVBox{Min: p, Max: p}, 128)
		if e != nil {
			return e
		}
		if known && loc == parametric.Inside {
			return nil
		}
	}
	return fmt.Errorf("trimmed: transverse contour orientation not certified: %w", geom.ErrConvergence)
}

func reverseTransverseRing(in []transverseRingUse) []transverseRingUse {
	out := make([]transverseRingUse, len(in))
	for i, x := range in {
		x.from, x.to, x.curve, x.reversed = x.to, x.from, reversePCurve(x.curve), !x.reversed
		out[len(in)-1-i] = x
	}
	return out
}

func assembleTransverseContours(ctx context.Context, a, b Solid, ap, bp []splineEmbeddingFace,
	faceA, faceB int, op parametric.BooleanOp, h SplineShellTransverseHistory, o SplineShellTransverseOptions) (Solid, SplineShellTransverseHistory, error) {
	rings, e := transverseRings(ctx, h.Trace.Curves, ap[faceA].patch.Domain(), bp[faceB].patch.Domain())
	if e != nil {
		return Solid{}, h, e
	}
	for _, r := range rings {
		for _, p := range r.vertices {
			for _, s := range []Solid{a, b} {
				for _, v := range s.vertices {
					if p == v {
						return Solid{}, h, geom.ErrUnrepresentable
					}
				}
			}
		}
	}
	var parts []SewFace
	section := map[[2]int]int{}
	h.FacesA, h.FacesB = make([][]int, len(a.faces)), make([][]int, len(b.faces))
	ab, bb := splineShellPatchBounds(ap), splineShellPatchBounds(bp)
	count := 0
	for _, r := range rings {
		count += len(r.vertices)
	}
	for chart, x := range []struct {
		s, opposing Solid
		bound       geom.Box
		proof       splineEmbeddingFace
		face        int
		history     [][]int
	}{{a, b, bb, ap[faceA], faceA, h.FacesA}, {b, a, ab, bp[faceB], faceB, h.FacesB}} {
		parent, depth, e := transverseRingNesting(ctx, rings, chart)
		if e != nil {
			return Solid{}, h, e
		}
		outerInside, e := classifySeparatedSplineWitness(ctx, x.opposing, x.bound, x.s.vertices[x.proof.vertex[0]], o.Shells)
		if e != nil {
			return Solid{}, h, e
		}
		keep := func(inside bool) bool {
			if op == parametric.Intersection || op == parametric.Difference && chart == 1 {
				return inside
			}
			return !inside
		}
		reverse := op == parametric.Difference && chart == 1
		if keep(outerInside) {
			for i, f := range x.s.faces {
				if e := ctx.Err(); e != nil {
					return Solid{}, h, e
				}
				f.Reversed = f.Reversed != reverse
				part := x.s.splineSewFace(f)
				if i == x.face {
					for j, r := range rings {
						if parent[j] < 0 {
							addTransverseRing(&part, r, chart, true, len(parts), section)
						}
					}
				}
				x.history[i] = append(x.history[i], len(parts))
				parts = append(parts, part)
			}
		}
		for i, r := range rings {
			if !keep(outerInside != (depth[i]%2 == 0)) {
				continue
			}
			f := x.s.faces[x.face]
			part := SewFace{Face: Face{Surface: f.Surface, Reversed: f.Reversed != reverse}}
			addTransverseRing(&part, r, chart, false, len(parts), section)
			for j, other := range rings {
				if parent[j] == i {
					addTransverseRing(&part, other, chart, true, len(parts), section)
				}
			}
			x.history[x.face] = append(x.history[x.face], len(parts))
			parts = append(parts, part)
		}
	}
	return sewTransverseFragments(ctx, parts, section, count, max(a.tolerance, b.tolerance), h, o.Sew)
}

func addTransverseRing(face *SewFace, r transverseRing, chart int, hole bool, part int, ancestry map[[2]int]int) {
	vertices := len(face.Vertices)
	face.Vertices = append(face.Vertices, r.vertices...)
	loop := r.parts[chart]
	if hole {
		loop = reverseTransverseRing(loop)
	}
	uses := make([]Use, 0, len(loop))
	for _, x := range loop {
		id := len(face.Edges)
		i, j := x.segment, (x.segment+1)%len(loop)
		if x.reversed {
			i, j = j, i
		}
		face.Edges = append(face.Edges, parametric.Edge{Start: vertices + i, End: vertices + j,
			Curve: surfaceTrimCurve{face.Face.Surface, x.from, x.to, x.curve}, Range: parametric.Interval{Max: 1}})
		uses = append(uses, Use{Edge: id, UVStart: x.from, UVEnd: x.to, Curve: x.curve})
		ancestry[[2]int{part, id}] = r.base + x.segment
	}
	if hole {
		face.Face.Holes = append(face.Face.Holes, uses)
	} else {
		face.Face.Outer = uses
	}
}
