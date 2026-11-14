package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// BoundPolynomialArea measures the clipped planar material without a mesh.
// The boundary must be polynomial (constant homogeneous weights). Each exact
// algebraic parameter is refined against a Green antiderivative. The returned
// projected area interval has width at most projectedWidth. World area is
// the projected area times the positive square root of ModelScaleSquared.
func (c AlgebraicGraphCapClip) BoundPolynomialArea(ctx context.Context, projectedWidth *big.Rat, o RationalGraphShellRegionOptions) (RationalGraphSectionArea, error) {
	return c.boundArea(ctx, projectedWidth, o, false)
}

// BoundArea measures the clipped cap with polynomial or rational trim curves.
// Rational curve integrals use interval quadrature certified with exact
// arithmetic; polynomial curves retain the exact antiderivative path.
func (c AlgebraicGraphCapClip) BoundArea(ctx context.Context, projectedWidth *big.Rat, o RationalGraphShellRegionOptions) (RationalGraphSectionArea, error) {
	return c.boundArea(ctx, projectedWidth, o, true)
}

func (c AlgebraicGraphCapClip) boundArea(ctx context.Context, projectedWidth *big.Rat, o RationalGraphShellRegionOptions, rational bool) (RationalGraphSectionArea, error) {
	var zero RationalGraphSectionArea
	if ctx == nil || c.regions == nil || projectedWidth == nil || projectedWidth.Sign() <= 0 {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	opts, e := o.Topology.WithDefaults()
	if e != nil {
		return zero, e
	}
	if projectedWidth.Num().BitLen() > opts.MaxBits || projectedWidth.Denom().BitLen() > opts.MaxBits {
		return zero, geom.ErrConvergence
	}
	pieces := 0
	for _, wire := range c.wires {
		pieces += len(wire)
	}
	if pieces > opts.MaxEdges {
		return zero, geom.ErrConvergence
	}
	width := new(big.Rat).Quo(projectedWidth, big.NewRat(int64(max(pieces, 1)), 1))
	b := capClipBudget{ctx, opts.MaxWork, opts.MaxSubdivisions, opts.MaxBits}
	axes := orientedRationalGraphProjection(c.regions.plane)
	lo, hi := new(big.Rat), new(big.Rat)
	check := func(q *big.Rat) error {
		if q.Num().BitLen() > b.bits || q.Denom().BitLen() > b.bits {
			return geom.ErrConvergence
		}
		return b.charge(1, 0)
	}
	for _, wire := range c.wires {
		for _, use := range wire {
			bound := use.Cell.BoundPolynomialProjectedGreen
			if rational {
				bound = use.Cell.BoundProjectedGreen
			}
			p, e := bound(ctx, axes, width, b.limits())
			if e != nil {
				return zero, e
			}
			if e = b.charge(p.Operations, p.Subdivisions); e != nil {
				return zero, e
			}
			if use.Reversed {
				lo.Sub(lo, p.Interval.Upper)
				hi.Sub(hi, p.Interval.Lower)
			} else {
				lo.Add(lo, p.Interval.Lower)
				hi.Add(hi, p.Interval.Upper)
			}
			for _, v := range []*big.Rat{lo, hi} {
				if e := check(v); e != nil {
					return zero, e
				}
			}
		}
	}
	scale := new(big.Rat)
	for _, v := range c.regions.plane.normal {
		squared := new(big.Rat).Mul(v, v)
		if e := check(squared); e != nil {
			return zero, e
		}
		scale.Add(scale, squared)
		if e := check(scale); e != nil {
			return zero, e
		}
	}
	if lo.Sign() < 0 {
		lo.SetInt64(0) // nonnegative material area by the original winding proof
	}
	if hi.Cmp(lo) < 0 || new(big.Rat).Sub(hi, lo).Cmp(projectedWidth) > 0 {
		return zero, geom.ErrConvergence
	}
	return RationalGraphSectionArea{Projection: trimvalidate.SignedAreaInterval{
		Lower: lo, Upper: hi, Work: opts.MaxWork - b.work, Subdivisions: opts.MaxSubdivisions - b.subdiv,
	}, ModelScaleSquared: scale}, nil
}
