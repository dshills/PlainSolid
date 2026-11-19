//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// AlgebraicGraphProjectedRelation classifies the intervals between the
// extrema of two stocks in one rational direction. Touching or overlapping
// intervals alone give no conclusion about intersection of the actual solids.
type AlgebraicGraphProjectedRelation uint8

const (
	AlgebraicGraphProjectionUnknown AlgebraicGraphProjectedRelation = iota
	AlgebraicGraphProjectionSeparated
	AlgebraicGraphProjectionTouching
	AlgebraicGraphProjectionOverlapping
)

// AlgebraicGraphProjectedSeparation contains the exact extremal intervals.
// First and Second provide extremal trim witnesses. Before is 0 if First
// precedes Second, 1 if Second precedes First, and -1 if their interiors
// overlap. Gap is a rational enclosure in units of the original, unnormalized
// functional. It is present only for separated or touching projections.
// PlaneOffset is present only for strict separation and defines the plane
// direction·point=PlaneOffset, with operand Before on its negative side.
type AlgebraicGraphProjectedSeparation struct {
	First, Second                 AlgebraicGraphDirectionalBounds
	Relation                      AlgebraicGraphProjectedRelation
	Before                        int
	Gap                           geom.ExactParameterInterval
	PlaneOffset                   *big.Rat
	ExactOperations, Subdivisions int
}

// ProjectedSeparationRational compares exact directional projections of
// two placed originals using one work ledger. A strictly positive gap
// certifies that a plane with this normal strictly separates the solids.
// A gap bounded on both sides is computed only after equality or strict
// root order is proved; overlapping brackets alone cannot report contact.
func (o AlgebraicGraphStockOccurrence) ProjectedSeparationRational(ctx context.Context,
	other AlgebraicGraphStockOccurrence, direction [3]*big.Rat, width *big.Rat,
	opts geom.ExactCurvePlaneEventOptions) (AlgebraicGraphProjectedSeparation, error) {
	var zero AlgebraicGraphProjectedSeparation
	if width == nil || width.Sign() <= 0 {
		return zero, geom.ErrInvalid
	}
	b, err := o.model.extremaBudget(ctx, opts)
	if err != nil {
		return zero, err
	}
	if other.model.caps[0].recipe == nil || !other.model.lineReady {
		return zero, geom.ErrUnrepresentable
	}
	if max(width.Num().BitLen(), width.Denom().BitLen()) > b.bits {
		return zero, geom.ErrConvergence
	}
	work, splits := b.work, b.subdiv
	out := AlgebraicGraphProjectedSeparation{Before: -1}
	for i, owner := range [2]AlgebraicGraphStockOccurrence{o, other} {
		form, err := owner.directionForm(ctx, direction)
		if err != nil {
			return zero, err
		}
		if err = b.charge(30, 0); err != nil {
			return zero, err
		}
		r, err := owner.model.boundDirectionBudget([3]*big.Rat{form[0], form[1], form[2]}, form[3], &b)
		if err != nil {
			return zero, err
		}
		if i == 0 {
			out.First = r
		} else {
			out.Second = r
		}
	}
	front := [2]AlgebraicGraphDirectionalBounds{out.First, out.Second}
	for i := range front {
		a, z := front[i].Max.Value, front[i^1].Min.Value
		r, err := a.Compare(ctx, z, b.limits())
		if err != nil {
			return zero, err
		}
		if err = b.charge(r.Operations, r.Subdivisions); err != nil {
			return zero, err
		}
		if !r.Proven {
			return zero, geom.ErrConvergence
		}
		if r.Order <= 0 {
			out.Before = i
			if r.Order == 0 {
				out.Relation = AlgebraicGraphProjectionTouching
				out.Gap = geom.ExactParameterInterval{Lower: new(big.Rat), Upper: new(big.Rat)}
				break
			}
			out.Relation = AlgebraicGraphProjectionSeparated
			lo := new(big.Rat).Sub(r.Intervals[1][0], r.Intervals[0][1])
			hi := new(big.Rat).Sub(r.Intervals[1][1], r.Intervals[0][0])
			if lo.Sign() <= 0 {
				return zero, geom.ErrConvergence
			}
			// Any rational value in this strictly open gap defines a
			// separating plane with the exact caller-supplied normal.
			out.PlaneOffset = new(big.Rat).Quo(new(big.Rat).Add(r.Intervals[0][1], r.Intervals[1][0]), big.NewRat(2, 1))
			if max(out.PlaneOffset.Num().BitLen(), out.PlaneOffset.Denom().BitLen()) > b.bits {
				return zero, geom.ErrConvergence
			}
			if new(big.Rat).Sub(hi, lo).Cmp(width) > 0 {
				tol := new(big.Rat).Quo(width, big.NewRat(2, 1))
				x, err := a.Enclose(ctx, tol, b.limits())
				if err != nil {
					return zero, err
				}
				if err = b.charge(x.Operations, x.Subdivisions); err != nil {
					return zero, err
				}
				y, err := z.Enclose(ctx, tol, b.limits())
				if err != nil {
					return zero, err
				}
				if err = b.charge(y.Operations, y.Subdivisions); err != nil {
					return zero, err
				}
				if !x.HasBounds || !x.Converged || !y.HasBounds || !y.Converged {
					return zero, geom.ErrConvergence
				}
				if candidate := new(big.Rat).Sub(y.Interval.Lower, x.Interval.Upper); candidate.Cmp(lo) > 0 {
					lo = candidate
				}
				if candidate := new(big.Rat).Sub(y.Interval.Upper, x.Interval.Lower); candidate.Cmp(hi) < 0 {
					hi = candidate
				}
			}
			gap := geom.ExactParameterInterval{Lower: lo, Upper: hi}
			if !graphStockMomentFits(gap, b.bits) || lo.Cmp(hi) > 0 || new(big.Rat).Sub(hi, lo).Cmp(width) > 0 {
				return zero, geom.ErrConvergence
			}
			out.Gap = gap
			break
		}
	}
	if out.Relation == AlgebraicGraphProjectionUnknown {
		out.Relation = AlgebraicGraphProjectionOverlapping
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	out.ExactOperations, out.Subdivisions = work-b.work, splits-b.subdiv
	return out, nil
}
