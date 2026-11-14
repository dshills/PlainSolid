package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// AlgebraicGraphFootprintMoments measures the stock's exact projected
// material in the original XY chart. First is (∫X,∫Y); Second is
// (∫X²,∫XY,∫Y²). Integrals are over projected material, so holes
// subtract through their certified opposite wire orientation.
type AlgebraicGraphFootprintMoments struct {
	Area                          geom.ExactParameterInterval
	First                         [2]geom.ExactParameterInterval
	Second                        [3]geom.ExactParameterInterval
	ExactOperations, Subdivisions int
}

// BoundFootprintMoments encloses the six projected moments, each within
// width, using one arithmetic and subdivision ledger. No material measure
// along the vertical axis is included.
func (m AlgebraicGraphVerticalMaterial) BoundFootprintMoments(ctx context.Context, width *big.Rat,
	opts geom.ExactCurvePlaneEventOptions) (AlgebraicGraphFootprintMoments, error) {
	var zero AlgebraicGraphFootprintMoments
	if width == nil || width.Sign() <= 0 {
		return zero, geom.ErrInvalid
	}
	b, err := m.extremaBudget(ctx, opts)
	if err != nil {
		return zero, err
	}
	if max(width.Num().BitLen(), width.Denom().BitLen()) > b.bits {
		return zero, geom.ErrConvergence
	}
	work, splits := b.work, b.subdiv
	cap := m.caps[m.lower]
	axes := orientedRationalGraphProjection(cap.regions.plane)
	if axes != [2]int{0, 1} && axes != [2]int{1, 0} {
		return zero, geom.ErrUnrepresentable
	}
	count := 0
	for _, w := range cap.wires {
		count += len(w)
	}
	if count == 0 {
		return zero, geom.ErrConvergence
	}
	part := new(big.Rat).Quo(width, big.NewRat(int64(count), 1))
	var out AlgebraicGraphFootprintMoments
	dst := []*geom.ExactParameterInterval{&out.Area, &out.First[0], &out.First[1], &out.Second[0], &out.Second[1], &out.Second[2]}
	powers := [6][2]int{{0, 0}, {1, 0}, {0, 1}, {2, 0}, {1, 1}, {0, 2}}
	for i, result := range dst {
		result.Lower, result.Upper = new(big.Rat), new(big.Rat)
		power := powers[i]
		if axes[0] == 1 {
			power[0], power[1] = power[1], power[0]
		}
		for _, row := range cap.wires {
			for _, use := range row {
				v, e := use.Cell.BoundProjectedMonomialGreen(ctx, axes, power, part, b.limits())
				if e != nil {
					return zero, e
				}
				if e = b.charge(v.Operations+2, v.Subdivisions); e != nil {
					return zero, e
				}
				if use.Reversed {
					result.Lower.Sub(result.Lower, v.Interval.Upper)
					result.Upper.Sub(result.Upper, v.Interval.Lower)
				} else {
					result.Lower.Add(result.Lower, v.Interval.Lower)
					result.Upper.Add(result.Upper, v.Interval.Upper)
				}
				if !graphStockMomentFits(*result, b.bits) {
					return zero, geom.ErrConvergence
				}
			}
		}
		if result.Lower.Cmp(result.Upper) > 0 || new(big.Rat).Sub(result.Upper, result.Lower).Cmp(width) > 0 {
			return zero, geom.ErrConvergence
		}
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	out.ExactOperations, out.Subdivisions = work-b.work, splits-b.subdiv
	return out, nil
}
