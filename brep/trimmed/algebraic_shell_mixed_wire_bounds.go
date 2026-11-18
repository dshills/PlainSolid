//go:build research

package trimmed

import (
	"context"
	"errors"
	"math/big"

	"plainsolid/geom"
)

// MixedSectionWireBounds encloses the named original section wire, preserving
// each oriented use in wire order. The boxes cover entire curve images, not
// merely endpoints. An enclosure can be wider than the actual curve extrema.
// ExactOperations counts original interval enclosure arithmetic.
type MixedSectionWireBounds struct {
	Box                     geom.Box
	Uses                    []MixedSectionUseBounds
	Pieces, ExactOperations int
}

// MixedSectionUseBounds retains the wire's original branch and face owners.
type MixedSectionUseBounds struct {
	Use   AlgebraicShellSectionWireUse
	Faces [2]int
	Box   geom.Box
}

// MixedSectionWireBoundsOptions specifies the whole-wire arithmetic and piece
// budgets. Zero means 32M exact operations and 4096 original curve pieces.
// Each piece uses up to Interval.MaxIterations contractions (default 4).
// If an interval refinement goal is not reached but a rigorous enclosure
// exists, that enclosure may still be used. No partial wire is published.
type MixedSectionWireBoundsOptions struct {
	Interval                 geom.SurfaceTubeIntervalOptions
	MaxOperations, MaxPieces int
}

// EncloseWire retains original rational restrictions for mixed pieces and
// obtains whole-interval bounds from immutable native curves. No display
// tessellation or sampled extrema enter the result.
func (s *MixedAlgebraicShellSection) EncloseWire(ctx context.Context, index int, o MixedSectionWireBoundsOptions) (MixedSectionWireBounds, error) {
	var z MixedSectionWireBounds
	if ctx == nil || s == nil || s.source == nil || index < 0 || index >= len(s.source.wires) || o.MaxOperations < 0 || o.MaxOperations > 256000000 || o.MaxPieces < 0 || o.MaxPieces > 32768 {
		return z, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return z, err
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 32000000
	}
	if o.MaxPieces == 0 {
		o.MaxPieces = 4096
	}
	if o.Interval.MaxIterations == 0 {
		o.Interval.MaxIterations = 4
	}
	var out MixedSectionWireBounds
	for _, use := range s.source.wires[index] {
		b := s.source.branches[use.Branch]
		entry := MixedSectionUseBounds{Use: use, Faces: b.Faces}
		n := shellSectionPieceCount(b)
		if n < 1 || n > o.MaxPieces-out.Pieces {
			return z, geom.ErrConvergence
		}
		for i := 0; i < n; i++ {
			if err := ctx.Err(); err != nil {
				return z, err
			}
			left := o.MaxOperations - out.ExactOperations
			if left <= 0 {
				return z, geom.ErrConvergence
			}
			limits := o.Interval
			if limits.MaxOperations == 0 || limits.MaxOperations > left {
				limits.MaxOperations = left
			}
			var q geom.SurfaceTubeIntervalEnclosure
			var err error
			if b.rational != nil {
				p, e := b.rational.Piece(i)
				if e != nil {
					return z, e
				}
				q, err = b.rational.EnclosePiece(ctx, i, p.Free.Lower, p.Free.Upper, limits)
			} else {
				x := b.Segments[i]
				_, d, _, e := x.ExactFreeDomain()
				if e != nil {
					return z, e
				}
				var r [2]float64
				for j, v := range [2]*big.Rat{d.Lower, d.Upper} {
					r[j], e = geom.RepresentedExactScalar(v)
					if e != nil {
						return z, e
					}
				}
				q, err = x.EncloseFreeInterval(ctx, r[0], r[1], limits)
			}
			if !q.HasBounds || err != nil && !errors.Is(err, geom.ErrConvergence) {
				if err == nil {
					err = geom.ErrConvergence
				}
				return z, err
			}
			if i == 0 {
				entry.Box = q.Bounds
			} else if entry.Box, err = entry.Box.Union(q.Bounds); err != nil {
				return z, err
			}
			out.Pieces++
			out.ExactOperations += q.ExactOperations
		}
		if len(out.Uses) == 0 {
			out.Box = entry.Box
		} else {
			var err error
			if out.Box, err = out.Box.Union(entry.Box); err != nil {
				return z, err
			}
		}
		out.Uses = append(out.Uses, entry)
	}
	if err := ctx.Err(); err != nil {
		return z, err
	}
	if len(out.Uses) == 0 {
		return z, geom.ErrConvergence
	}
	return out, nil
}
