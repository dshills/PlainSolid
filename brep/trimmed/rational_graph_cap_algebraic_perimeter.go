package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// AlgebraicGraphFootprintLength records the original trim ownership of
// one projected length. Seam is -1 for inherited stock wall trims.
type AlgebraicGraphFootprintLength struct {
	Wire, Use, StockFace, Seam int
	Length                     geom.ExactParameterInterval
}

// AlgebraicGraphFootprintPerimeter encloses all outer and inner perimeter
// lengths of the stock footprint along its original vertical direction.
// Original seams of a cap pair are counted once, not once per cap.
type AlgebraicGraphFootprintPerimeter struct {
	Interval, Stock, Seams        geom.ExactParameterInterval
	Uses                          []AlgebraicGraphFootprintLength
	ExactOperations, Subdivisions int
}

// BoundFootprintPerimeter encloses the exact XY perimeter with a single
// work budget. Original rational trim weights and algebraic cuts are
// retained. Through-hole boundaries contribute positively to perimeter.
func (m AlgebraicGraphVerticalMaterial) BoundFootprintPerimeter(ctx context.Context, width *big.Rat,
	opts geom.ExactCurvePlaneEventOptions) (AlgebraicGraphFootprintPerimeter, error) {
	var zero AlgebraicGraphFootprintPerimeter
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
	count := 0
	for _, w := range cap.wires {
		count += len(w)
	}
	if count == 0 {
		return zero, geom.ErrConvergence
	}
	part := new(big.Rat).Quo(width, big.NewRat(int64(count), 1))
	blank := func() geom.ExactParameterInterval {
		return geom.ExactParameterInterval{Lower: new(big.Rat), Upper: new(big.Rat)}
	}
	out := AlgebraicGraphFootprintPerimeter{Stock: blank(), Seams: blank()}
	scale := [3]*big.Rat{big.NewRat(1, 1), new(big.Rat), new(big.Rat)}
	for i, row := range cap.wires {
		for j, use := range row {
			r, err := use.Cell.BoundVerticalSweepArea(ctx, [2]int{0, 1}, scale, part, b.limits())
			if err != nil {
				return zero, err
			}
			if err = b.charge(r.Operations+5, r.Subdivisions); err != nil {
				return zero, err
			}
			owner, dst := use.Source.Face, &out.Stock
			if use.Seam >= 0 {
				owner, dst = -1, &out.Seams
			} else if owner < 0 {
				return zero, geom.ErrConvergence
			}
			dst.Lower.Add(dst.Lower, r.Interval.Lower)
			dst.Upper.Add(dst.Upper, r.Interval.Upper)
			if !graphStockMomentFits(*dst, b.bits) {
				return zero, geom.ErrConvergence
			}
			out.Uses = append(out.Uses, AlgebraicGraphFootprintLength{i, j, owner, use.Seam, r.Interval})
		}
	}
	out.Interval = geom.ExactParameterInterval{Lower: new(big.Rat).Add(out.Stock.Lower, out.Seams.Lower),
		Upper: new(big.Rat).Add(out.Stock.Upper, out.Seams.Upper)}
	if !graphStockMomentFits(out.Interval, b.bits) || new(big.Rat).Sub(out.Interval.Upper, out.Interval.Lower).Cmp(width) > 0 {
		return zero, geom.ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	out.ExactOperations, out.Subdivisions = work-b.work, splits-b.subdiv
	return out, nil
}

// BoundFootprintPerimeter uses the placed stock's original vertical axis.
// Proper rigid placement preserves that projection's physical lengths.
func (o AlgebraicGraphStockOccurrence) BoundFootprintPerimeter(ctx context.Context, width *big.Rat,
	opts geom.ExactCurvePlaneEventOptions) (AlgebraicGraphFootprintPerimeter, error) {
	return o.model.BoundFootprintPerimeter(ctx, width, opts)
}
