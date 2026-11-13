package geom

import (
	"context"
	"math/big"
)

// ComposedProjectionOptions bounds dyadic depth/piece count and each exact
// restriction/projection proof. A full binary tree has fewer than 2*MaxPieces
// visited nodes; depth bounds unresolved weight-sign refinement.
type ComposedProjectionOptions struct {
	EventOptions        ExactCurvePlaneEventOptions
	MaxPieces, MaxDepth int
}
type ComposedBezierProjectionPieces struct {
	Complete      bool
	OriginalRange [2]*big.Rat
	Axes          [2]int
	Pieces        []ComposedBezierProjection
}

// ProjectBezierPieces2D covers the exact rational domain by ordered positive
// Bernstein projections. Splits are exact rational midpoints in original units.
// Only degree<=4 projections are supported. Failures return no partial pieces.
// Mixed weight signs trigger subdivision; mathematical denominator positivity
// alone is not reported as a successful coefficient certificate.
func (c *ExactComposedCurveCell) ProjectBezierPieces2D(ctx context.Context, axes [2]int, ends [2]*big.Rat, o ComposedProjectionOptions) (ComposedBezierProjectionPieces, error) {
	var zero ComposedBezierProjectionPieces
	if ctx == nil || c == nil || axes[0] < 0 || axes[0] > 2 || axes[1] < 0 || axes[1] > 2 || axes[0] == axes[1] || ends[0] == nil || ends[1] == nil || ends[0].Cmp(ends[1]) >= 0 {
		return zero, ErrInvalid
	}
	if o.MaxPieces == 0 {
		o.MaxPieces = 64
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 12
	}
	if o.MaxPieces < 1 || o.MaxPieces > 4096 || o.MaxDepth < 1 || o.MaxDepth > 64 {
		return zero, ErrInvalid
	}
	opts, e := graphRootOptions(o.EventOptions)
	if e != nil {
		return zero, e
	}
	for _, axis := range []int{axes[0], axes[1], 3} {
		if len(c.power[axis]) > 5 {
			return zero, ErrConvergence
		}
	}
	// Establish exact endpoint identity before any subdivision can change scope.
	for i, q := range ends {
		r, _, e := NewExactRationalRoot(ctx, q, opts)
		if e != nil {
			return zero, e
		}
		actual, e := c.EndpointRoot(i)
		if e != nil {
			return zero, e
		}
		cmp, e := actual.Compare(ctx, r, opts)
		if e != nil {
			return zero, e
		}
		if !cmp.Proven || cmp.Order != 0 {
			return zero, ErrDomain
		}
	}
	type node struct {
		cell  *ExactComposedCurveCell
		ends  [2]*big.Rat
		depth int
	}
	stack := []node{{c, [2]*big.Rat{new(big.Rat).Set(ends[0]), new(big.Rat).Set(ends[1])}, 0}}
	out := ComposedBezierProjectionPieces{Axes: axes, OriginalRange: [2]*big.Rat{new(big.Rat).Set(ends[0]), new(big.Rat).Set(ends[1])}}
	for len(stack) > 0 {
		if e := ctx.Err(); e != nil {
			return zero, e
		}
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if len(out.Pieces)+len(stack)+1 > o.MaxPieces {
			return zero, ErrConvergence
		}
		piece, e := n.cell.ProjectBezier2D(ctx, axes, n.ends, opts)
		if e == nil {
			out.Pieces = append(out.Pieces, piece)
			continue
		}
		if e != ErrConvergence {
			return zero, e
		}
		if n.depth >= o.MaxDepth || len(out.Pieces)+len(stack)+2 > o.MaxPieces {
			return zero, ErrConvergence
		}
		mid := new(big.Rat).Quo(new(big.Rat).Add(n.ends[0], n.ends[1]), big.NewRat(2, 1))
		r, _, e := NewExactRationalRoot(ctx, mid, opts)
		if e != nil {
			return zero, e
		}
		a, e := n.cell.EndpointRoot(0)
		if e != nil {
			return zero, e
		}
		z, e := n.cell.EndpointRoot(1)
		if e != nil {
			return zero, e
		}
		left, e := n.cell.RestrictAlgebraic(ctx, a, r, opts)
		if e != nil {
			return zero, e
		}
		right, e := n.cell.RestrictAlgebraic(ctx, r, z, opts)
		if e != nil {
			return zero, e
		}
		stack = append(stack, node{right.Cell, [2]*big.Rat{new(big.Rat).Set(mid), n.ends[1]}, n.depth + 1}, node{left.Cell, [2]*big.Rat{n.ends[0], new(big.Rat).Set(mid)}, n.depth + 1})
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	out.Complete = true
	return out, nil
}
