package trimmed

import (
	"errors"

	"plainsolid/geom"
	"plainsolid/intersect"
)

// The full shell section has already proved that the simple closed loops
// are disjoint from all open cuts and the source rectangle. Locate a single
// point of each outermost closed loop in the open-cut partition. The whole
// closed disk then lies in that fragment by Jordan separation; child loops
// retain the separately proved closed-loop nesting order.
func (w *shellImprintWork) arrangeMixedFacePaths(face, op int, original AlgebraicShellImprintFace, s AlgebraicShellImprint,
	parts []AlgebraicShellImprintFace, closed []shellImprintPath) ([]AlgebraicShellImprintFace, error) {
	parents, e := w.closedFacePathParents(face, op, s, closed)
	if e != nil {
		return nil, e
	}
	n := len(parts)
	parts = append(parts, make([]AlgebraicShellImprintFace, len(closed))...)
	for i, p := range closed {
		parent := n + parents[i]
		if parents[i] < 0 {
			parent, e = w.openFragmentContainingLoop(face, op, s, parts[:n], p)
			if e != nil {
				return nil, e
			}
		}
		winding, e := w.closedPathWinding(face, op, p, s)
		if e != nil {
			return nil, e
		}
		outer, inner := p.uses, reverseImprintUses(p.uses)
		if winding < 0 {
			outer, inner = inner, outer
		}
		parts[parent].Holes = append(parts[parent].Holes, inner)
		part := &parts[n+i]
		part.Original, part.Fragment, part.Reversed, part.Outer = face, n+i, original.Reversed, outer
	}
	return parts, nil
}

func (w *shellImprintWork) openFragmentContainingLoop(face, op int, s AlgebraicShellImprint,
	parts []AlgebraicShellImprintFace, path shellImprintPath) (int, error) {
	seg, e := w.imprintUseSegment(face, op, s, path.uses[0])
	if e != nil {
		return -1, e
	}
	lo, hi, _, e := imprintFreeRange(seg)
	if e != nil {
		return -1, e
	}
	mid := lo/2 + hi/2
	if mid <= lo || mid >= hi {
		return -1, geom.ErrConvergence
	}
	if w.exact.MaxOperations < 1 {
		return -1, geom.ErrConvergence
	}
	q, e := seg.EncloseAtFree(mid, geom.SurfaceTubeEvaluationOptions{
		MaxOperations: w.exact.MaxOperations, MaxCoefficientBits: w.exact.MaxCoefficientBits,
	})
	w.exact.MaxOperations -= q.ExactOperations
	if e != nil {
		return -1, e
	}
	if !q.Enclosed {
		return -1, geom.ErrConvergence
	}
	witness := [2]geom.UVBox{q.ParametersA, q.ParametersB}[op]
	owner := -1
	for i, f := range parts {
		n, e := w.fragmentWindingAwayFromBox(face, op, s, f.Outer, witness)
		if e != nil {
			return -1, e
		}
		if n != 0 {
			if n != 1 || owner >= 0 {
				return -1, geom.ErrConvergence
			}
			owner = i
		}
	}
	if owner < 0 {
		return -1, geom.ErrConvergence
	}
	return owner, nil
}

// Replace each exact path segment by a chord in a convex outward enclosure
// disjoint from the witness. Adjacent enclosures contain their shared exact
// endpoint, so their intersection contains a represented surrogate used by
// both chords. The resulting loop is homotopic to the exact boundary in the
// complement of the witness box and has the same integer winding there.
func (w *shellImprintWork) fragmentWindingAwayFromBox(face, op int, s AlgebraicShellImprint,
	boundary []AlgebraicShellImprintUse, witness geom.UVBox) (int, error) {
	var boxes []geom.UVBox
	for _, use := range boundary {
		if e := w.ctx.Err(); e != nil {
			return 0, e
		}
		if use.OriginalEdge >= 0 {
			// Original coedges are straight axis-aligned sides of a proven
			// rectangle. The full source side encloses any algebraic subedge.
			original := s.Subdivision.edges[use.OriginalEdge].Original
			f := [2]Solid{w.source.a, w.source.b}[op].faces[face]
			found := false
			for _, u := range f.Outer {
				if u.Edge != original || u.Curve != nil {
					continue
				}
				if found || u.UVStart.X != u.UVEnd.X && u.UVStart.Y != u.UVEnd.Y {
					return 0, geom.ErrConvergence
				}
				found = true
				box := geom.UVBox{
					Min: geom.Vec2{X: min(u.UVStart.X, u.UVEnd.X), Y: min(u.UVStart.Y, u.UVEnd.Y)},
					Max: geom.Vec2{X: max(u.UVStart.X, u.UVEnd.X), Y: max(u.UVStart.Y, u.UVEnd.Y)},
				}
				if !imprintBoxesDisjoint(box, witness) {
					return 0, geom.ErrConvergence
				}
				if len(boxes) >= 16384 {
					return 0, geom.ErrConvergence
				}
				boxes = append(boxes, box)
			}
			if !found {
				return 0, geom.ErrConvergence
			}
			continue
		}
		seg, e := w.imprintUseSegment(face, op, s, use)
		if e != nil {
			return 0, e
		}
		trim := seg.TrimA()
		if op == 1 {
			trim = seg.TrimB()
		}
		box, e := trim.BoundsUV()
		if e != nil {
			return 0, e
		}
		if imprintBoxesDisjoint(box, witness) {
			if len(boxes) >= 16384 {
				return 0, geom.ErrConvergence
			}
			boxes = append(boxes, box)
			continue
		}
		lo, hi, reverse, e := imprintFreeRange(seg)
		if e != nil {
			return 0, e
		}
		if reverse != use.Reversed {
			lo, hi = hi, lo
		}
		if e = w.fragmentTubeAwayFromBox(seg, op, lo, hi, 0, witness, &boxes); e != nil {
			return 0, e
		}
	}
	if len(boxes) < 2 || len(boxes) > 16384 {
		return 0, geom.ErrConvergence
	}
	points := make([]geom.Vec2, len(boxes))
	for i, b := range boxes {
		a := boxes[(i+len(boxes)-1)%len(boxes)]
		lo := geom.Vec2{X: max(a.Min.X, b.Min.X), Y: max(a.Min.Y, b.Min.Y)}
		if lo.X > min(a.Max.X, b.Max.X) || lo.Y > min(a.Max.Y, b.Max.Y) {
			return 0, geom.ErrConvergence
		}
		points[i] = lo
	}
	var winding int
	for i, x := range points {
		y := points[(i+1)%len(points)]
		up := x.Y <= witness.Min.Y && witness.Min.Y < y.Y
		down := y.Y <= witness.Min.Y && witness.Min.Y < x.Y
		if up || down {
			if e := w.ctx.Err(); e != nil {
				return 0, e
			}
			sign, e := geom.Orient2D(x, y, witness.Min)
			if e != nil {
				return 0, e
			}
			if up && sign > 0 || down && sign < 0 {
				winding += sign
			}
		}
	}
	return winding, nil
}

func (w *shellImprintWork) fragmentTubeAwayFromBox(seg intersect.TubeSegment, op int, from, to float64, depth int,
	witness geom.UVBox, boxes *[]geom.UVBox) error {
	if w.exact.MaxOperations < 1 || len(*boxes) >= 16384 {
		return geom.ErrConvergence
	}
	q, e := seg.EncloseFreeInterval(w.ctx, min(from, to), max(from, to), geom.SurfaceTubeIntervalOptions{
		MaxIterations: 4, MaxOperations: w.exact.MaxOperations, MaxCoefficientBits: w.exact.MaxCoefficientBits,
	})
	w.exact.MaxOperations -= q.ExactOperations
	if e != nil && !errors.Is(e, geom.ErrConvergence) {
		return e
	}
	if q.HasBounds {
		box := [2]geom.UVBox{q.ParametersA, q.ParametersB}[op]
		if imprintBoxesDisjoint(box, witness) {
			*boxes = append(*boxes, box)
			return nil
		}
	}
	mid := from/2 + to/2
	if depth >= 14 || mid <= min(from, to) || mid >= max(from, to) {
		return geom.ErrConvergence
	}
	if e = w.fragmentTubeAwayFromBox(seg, op, from, mid, depth+1, witness, boxes); e != nil {
		return e
	}
	return w.fragmentTubeAwayFromBox(seg, op, mid, to, depth+1, witness, boxes)
}

func (w *shellImprintWork) imprintUseSegment(face, op int, s AlgebraicShellImprint, u AlgebraicShellImprintUse) (intersect.TubeSegment, error) {
	x := s.Section[u.SectionPiece]
	b := w.source.branches[x.Branch]
	if b.Faces[op] != face {
		return intersect.TubeSegment{}, geom.ErrConvergence
	}
	return b.Segments[x.Index], nil
}

func imprintFreeRange(seg intersect.TubeSegment) (lo, hi float64, reverse bool, err error) {
	_, d, reverse, err := seg.ExactFreeDomain()
	if err != nil {
		return 0, 0, false, err
	}
	lo, err = geom.RepresentedExactScalar(d.Lower)
	if err != nil {
		return 0, 0, false, err
	}
	hi, err = geom.RepresentedExactScalar(d.Upper)
	return lo, hi, reverse, err
}

func imprintBoxesDisjoint(a, b geom.UVBox) bool {
	return max(a.Min.X, b.Min.X) > min(a.Max.X, b.Max.X) ||
		max(a.Min.Y, b.Min.Y) > min(a.Max.Y, b.Max.Y)
}
