package trimmed

import (
	"errors"
	"math/big"

	"plainsolid/geom"
)

func (w *shellImprintWork) closedFacePathParents(face, op int, s AlgebraicShellImprint, paths []shellImprintPath) ([]int, error) {
	if len(paths) > 128 {
		return nil, geom.ErrConvergence
	}
	parents := make([]int, len(paths))
	boxes := make([]geom.UVBox, 0, len(paths))
	needContainment := false
	for i, p := range paths {
		if e := w.ctx.Err(); e != nil {
			return nil, e
		}
		if !p.closed || len(p.uses) == 0 {
			return nil, geom.ErrConvergence
		}
		parents[i] = -1
		if len(paths) == 1 {
			continue
		}
		var box geom.UVBox
		for j, use := range p.uses {
			x := s.Section[use.SectionPiece]
			b := w.source.branches[x.Branch]
			if b.Faces[op] != face {
				return nil, geom.ErrConvergence
			}
			seg := b.Segments[x.Index]
			trim := seg.TrimA()
			if op == 1 {
				trim = seg.TrimB()
			}
			q, e := trim.BoundsUV()
			if e != nil {
				return nil, e
			}
			if j == 0 {
				box = q
				continue
			}
			box.Min.X, box.Min.Y = min(box.Min.X, q.Min.X), min(box.Min.Y, q.Min.Y)
			box.Max.X, box.Max.Y = max(box.Max.X, q.Max.X), max(box.Max.Y, q.Max.Y)
		}
		for _, other := range boxes {
			needContainment = needContainment || !imprintBoxesDisjoint(box, other)
		}
		boxes = append(boxes, box)
	}
	// A strict gap between conservative boxes excludes nesting. Where boxes
	// overlap, classify the exact closed loops instead.
	if needContainment {
		return w.closedPathParents(face, op, s, paths)
	}
	return parents, nil
}

// Existing shell coverage already proves disjoint regular wires. For nested
// imprint, keep one common original affine graph projection and classify exact
// points on each wire against all the others in their original UV chart.
// The deepest containing loop is the immediate parent; the uncut face
// exterior is the parent of each root loop.
func (w *shellImprintWork) closedPathParents(face, op int, s AlgebraicShellImprint, paths []shellImprintPath) ([]int, error) {
	if len(paths) < 2 || len(paths) > 16 {
		return nil, geom.ErrConvergence
	}
	pair := [2]int{-1, -1}
	cuts := make([][]AlgebraicTubeFaceCutPiece, len(paths))
	root := func(x *big.Rat) (*geom.ExactAlgebraicRoot, error) {
		if w.exact.MaxOperations < 1 {
			return nil, geom.ErrConvergence
		}
		r, used, e := geom.NewExactRationalRoot(w.ctx, x, w.exact)
		w.exact.MaxOperations -= used
		return r, e
	}
	for i, p := range paths {
		branchID := -1
		for _, u := range p.uses {
			x := s.Section[u.SectionPiece]
			b := w.source.branches[x.Branch]
			if !b.Closed || b.Faces[op] != face || branchID >= 0 && x.Branch != branchID {
				return nil, geom.ErrConvergence
			}
			branchID = x.Branch
			if pair[0] < 0 {
				pair = b.Faces
			}
			if b.Faces != pair {
				return nil, geom.ErrConvergence
			}
			seg := b.Segments[x.Index]
			_, d, rev, e := seg.ExactFreeDomain()
			if e != nil {
				return nil, e
			}
			lo, e := root(d.Lower)
			if e != nil {
				return nil, e
			}
			hi, e := root(d.Upper)
			if e != nil {
				return nil, e
			}
			if rev {
				lo, hi = hi, lo
			}
			cuts[i] = append(cuts[i], AlgebraicTubeFaceCutPiece{Segment: seg, FreeStart: lo, FreeEnd: hi})
		}
	}
	var support [2]*geom.SurfacePatch
	for i, solid := range [2]Solid{w.source.a, w.source.b} {
		f := solid.faces[pair[i]]
		if !convexStraightTubeLensFace(f) || w.exact.MaxOperations < 1 {
			return nil, geom.ErrConvergence
		}
		p := geom.SurfacePatchOptions{MaxOperations: w.exact.MaxOperations, MaxCoefficientBits: w.exact.MaxCoefficientBits}
		patch, used, e := exactLensQueryPatch(f, p)
		w.exact.MaxOperations -= used
		if e != nil {
			return nil, e
		}
		support[i] = patch
	}
	projection := [2]int{-1, -1}
	for _, axes := range [3][2]int{{0, 1}, {1, 2}, {0, 2}} {
		ok := true
		for _, patch := range support {
			if w.exact.MaxOperations < 1 {
				return nil, geom.ErrConvergence
			}
			p := geom.SurfacePatchOptions{MaxOperations: w.exact.MaxOperations, MaxCoefficientBits: w.exact.MaxCoefficientBits}
			_, used, e := geom.NewAffineProjectedGraphWithWork(w.ctx, patch, axes, p)
			w.exact.MaxOperations -= used
			if errors.Is(e, geom.ErrDegenerate) || errors.Is(e, geom.ErrConvergence) {
				ok = false
				break
			}
			if e != nil {
				return nil, e
			}
		}
		if ok {
			projection = axes
			break
		}
	}
	if projection[0] < 0 {
		return nil, geom.ErrConvergence
	}

	contains := make([][]bool, len(paths))
	for i, outer := range cuts {
		contains[i] = make([]bool, len(paths))
		for j, inner := range cuts {
			if i == j {
				continue
			}
			in, used, e := tubeLoopContainmentOnChart(w.ctx, op, support[op], outer, inner, projection, w.exact)
			w.exact.MaxOperations -= used
			if e != nil {
				return nil, e
			}
			contains[i][j] = in
		}
	}
	parents := make([]int, len(paths))
	for i := range parents {
		parents[i] = -1
		p := sortedTubeParents(contains, i)
		if len(p) == 0 {
			continue
		}
		parent := p[0].index
		if contains[i][parent] {
			return nil, geom.ErrConvergence
		}
		for _, ancestor := range p[1:] {
			if !contains[ancestor.index][parent] || contains[parent][ancestor.index] || contains[i][ancestor.index] {
				return nil, geom.ErrConvergence
			}
		}
		// Every child's ancestors must also contain all of its descendants.
		for j, in := range contains[i] {
			if !in {
				continue
			}
			for _, ancestor := range p {
				if !contains[ancestor.index][j] {
					return nil, geom.ErrConvergence
				}
			}
		}
		parents[i] = parent
	}
	return parents, nil
}
