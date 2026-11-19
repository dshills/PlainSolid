//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// AlgebraicGraphCommonLinePiece retains a common closed line interval and
// the source cap/wall owners from each operand. Each side's owners describe
// its relative interior, or its point when the piece is a singleton.
type AlgebraicGraphCommonLinePiece struct {
	Extent         [2]*geom.ExactAlgebraicRoot
	First          AlgebraicGraphStockLinePiece
	Second         AlgebraicGraphStockLinePiece
	InteriorOfBoth bool
}

// AlgebraicGraphCommonLine proves intersection with the caller's exact
// line, not intersection or separation away from that line.
type AlgebraicGraphCommonLine struct {
	Spans, Points                 []AlgebraicGraphCommonLinePiece
	ExactOperations, Subdivisions int
}

// CoverCommonLineRational intersects the complete finite line material
// of two placed originals. Exact root comparisons and both source queries
// share one budget. Point contacts and coincident runs retain independent
// owners even when their numeric face IDs happen to match.
func (o AlgebraicGraphStockOccurrence) CoverCommonLineRational(ctx context.Context, other AlgebraicGraphStockOccurrence,
	origin, direction [3]*big.Rat, extent [2]*big.Rat, opts geom.ExactCurvePlaneEventOptions) (AlgebraicGraphCommonLine, error) {
	var zero AlgebraicGraphCommonLine
	b, err := o.model.extremaBudget(ctx, opts)
	if err != nil {
		return zero, err
	}
	work, splits := b.work, b.subdiv
	var sides [2]AlgebraicGraphStockLineCoverage
	for i, owner := range [2]AlgebraicGraphStockOccurrence{o, other} {
		if err = b.charge(60, 0); err != nil {
			return zero, err
		}
		s, e := owner.CoverLineRational(ctx, origin, direction, extent, b.limits())
		if e != nil {
			return zero, e
		}
		if err = b.charge(s.ExactOperations, s.Subdivisions); err != nil {
			return zero, err
		}
		sides[i] = s
	}
	cmp := func(a, z *geom.ExactAlgebraicRoot) (int, error) {
		r, e := a.Compare(ctx, z, b.limits())
		if e != nil {
			return 0, e
		}
		if e = b.charge(r.Operations, r.Subdivisions); e != nil {
			return 0, e
		}
		if !r.Proven {
			return 0, geom.ErrConvergence
		}
		return r.Order, nil
	}
	var knots []*geom.ExactAlgebraicRoot
	insert := func(root *geom.ExactAlgebraicRoot) error {
		lo, hi := 0, len(knots)
		for lo < hi {
			k := lo + (hi-lo)/2
			c, e := cmp(knots[k], root)
			if e != nil {
				return e
			}
			if c == 0 {
				return nil
			}
			if c < 0 {
				lo = k + 1
			} else {
				hi = k
			}
		}
		knots = append(knots, nil)
		copy(knots[lo+1:], knots[lo:])
		knots[lo] = root
		return nil
	}
	for _, side := range sides {
		for _, p := range side.Spans {
			for _, root := range p.Extent {
				if e := insert(root); e != nil {
					return zero, e
				}
			}
		}
		for _, p := range side.Points {
			if e := insert(p.Extent[0]); e != nil {
				return zero, e
			}
		}
	}
	// These forward cursors visit every span and explicit point at
	// most once while sweeping the union of the original endpoints.
	type cursor struct{ span, point int }
	var cursors [2]cursor
	at := func(side int, a, z *geom.ExactAlgebraicRoot, singleton bool) (AlgebraicGraphStockLinePiece, bool, error) {
		var none AlgebraicGraphStockLinePiece
		src, c := sides[side], &cursors[side]
		if singleton {
			for c.point < len(src.Points) {
				p := src.Points[c.point]
				order, err := cmp(p.Extent[0], a)
				if err != nil {
					return none, false, err
				}
				if order < 0 {
					c.point++
					continue
				}
				if order == 0 {
					p.Extent = [2]*geom.ExactAlgebraicRoot{a, z}
					return p, true, nil
				}
				break
			}
		}
		for c.span < len(src.Spans) {
			p := src.Spans[c.span]
			end, err := cmp(p.Extent[1], z)
			if err != nil {
				return none, false, err
			}
			if end < 0 {
				// A span that ended before a can never contain a
				// future point. Otherwise keep it until the next knot.
				earlier, err := cmp(p.Extent[1], a)
				if err != nil {
					return none, false, err
				}
				if earlier <= 0 {
					c.span++
					continue
				}
				return none, false, geom.ErrConvergence
			}
			start, err := cmp(p.Extent[0], a)
			if err != nil {
				return none, false, err
			}
			if start <= 0 {
				p.Extent = [2]*geom.ExactAlgebraicRoot{a, z}
				return p, true, nil
			}
			break
		}
		return none, false, nil
	}
	var out AlgebraicGraphCommonLine
	for i, root := range knots {
		for kind := range 2 {
			a, z := root, root
			if kind == 1 {
				if i+1 == len(knots) {
					continue
				}
				z = knots[i+1]
			}
			x, xok, e := at(0, a, z, kind == 0)
			if e != nil {
				return zero, e
			}
			y, yok, e := at(1, a, z, kind == 0)
			if e != nil {
				return zero, e
			}
			if !xok || !yok {
				continue
			}
			p := AlgebraicGraphCommonLinePiece{Extent: [2]*geom.ExactAlgebraicRoot{a, z}, First: x,
				Second: y, InteriorOfBoth: !x.BoundaryOnly && !y.BoundaryOnly}
			if kind == 0 {
				if !p.InteriorOfBoth {
					out.Points = append(out.Points, p)
				}
			} else {
				out.Spans = append(out.Spans, p)
			}
		}
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	out.ExactOperations, out.Subdivisions = work-b.work, splits-b.subdiv
	return out, nil
}
