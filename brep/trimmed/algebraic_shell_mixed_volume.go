package trimmed

import (
	"context"
	"errors"
	"math/big"

	"plainsolid/geom"
)

// BoundVolume encloses the original material's oriented flux. Every restricted
// section piece contributes a conservative UV cover in BOTH original charts;
// exact rational cuts partition its retained free interval before enclosure.
// An unresolved face cell is integrated as a partial contribution including
// zero. No legacy unrestricted segment substitutes for a rational piece.
func (s *MixedAlgebraicShellBoolean) BoundVolume(ctx context.Context, o AlgebraicShellVolumeOptions) (AlgebraicShellVolumeBounds, error) {
	if s == nil || s.section == nil {
		return AlgebraicShellVolumeBounds{}, geom.ErrInvalid
	}
	return s.boundary.BoundVolume(ctx, o)
}
func rationalShellBranchCover(ctx context.Context, b AlgebraicShellSectionBranch, n int, o geom.SurfacePatchOptions) (out [2][]geom.UVBox, err error) {
	if ctx == nil || b.rational == nil || b.rational.PieceCount() == 0 || b.rationalSource < 0 || b.rationalSource > 1 || n < 1 || n > 128 || o.MaxOperations < 0 {
		return out, geom.ErrInvalid
	}
	defer func() {
		if err != nil {
			out = [2][]geom.UVBox{}
		}
	}()
	bits := o.MaxCoefficientBits
	if bits == 0 {
		bits = 16384
	}
	if bits < 64 {
		return out, geom.ErrInvalid
	}
	within := func(q *big.Rat) bool { return q.Num().BitLen() <= bits && q.Denom().BitLen() <= bits }
	budget := o.MaxOperations
	if budget == 0 {
		budget = 16000000
	}
	for piece := 0; piece < b.rational.PieceCount(); piece++ {
		domain, e := b.rational.Piece(piece)
		if e != nil {
			return out, e
		}
		lo, hi := domain.Free.Lower, domain.Free.Upper
		if !within(lo) || !within(hi) {
			return out, geom.ErrConvergence
		}
		width := new(big.Rat).Sub(hi, lo)
		if !within(width) {
			return out, geom.ErrConvergence
		}
		at := func(i int) *big.Rat {
			return new(big.Rat).Add(lo, new(big.Rat).Mul(width, big.NewRat(int64(i), int64(n))))
		}
		for i := 0; i < n; i++ {
			if e := ctx.Err(); e != nil {
				return out, e
			}
			budget-- // charge the bounded rational interval partition
			if budget < 1 {
				return out, geom.ErrConvergence
			}
			q, e := b.rational.EnclosePiece(ctx, piece, at(i), at(i+1), geom.SurfaceTubeIntervalOptions{MaxIterations: 4, MaxOperations: budget, MaxCoefficientBits: o.MaxCoefficientBits})
			budget -= q.ExactOperations
			if budget < 0 {
				return out, geom.ErrConvergence
			}
			if !q.HasBounds || e != nil && !errors.Is(e, geom.ErrConvergence) {
				if e == nil {
					e = geom.ErrConvergence
				}
				return out, e
			}
			boxes := [2]geom.UVBox{q.ParametersA, q.ParametersB}
			if b.rationalSource == 1 {
				boxes[0], boxes[1] = boxes[1], boxes[0]
			}
			for op, box := range boxes {
				out[op] = append(out[op], box)
			}
		}
	}
	return out, nil
}
