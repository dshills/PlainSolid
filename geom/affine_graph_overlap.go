package geom

import (
	"context"
	"math/big"
)

// AffineGraphOverlap gives exact original parameter rectangles whose projected
// XY images coincide. Disjoint proves strictly separated projections; touching
// projections are unresolved, since a lower-dimensional contact may remain.
type AffineGraphOverlap struct {
	A, B            ParameterRectangle
	Disjoint        bool
	ExactOperations int
}

// ClipAffineXYGraphOverlap supports signed axis-aligned affine XY charts.
// Both maps are proved against all homogeneous coefficients. Returned clipping
// values must be exactly representable; no rounded domain is accepted.
func ClipAffineXYGraphOverlap(ctx context.Context, a, c *SurfacePatch, o SurfacePatchOptions) (out AffineGraphOverlap, err error) {
	opts, e := patchOptions(o)
	if e != nil {
		return out, e
	}
	if ctx == nil || a == nil || c == nil {
		return out, ErrInvalid
	}
	if len(a.net) > opts.MaxPoles || len(c.net) > opts.MaxPoles {
		return out, ErrConvergence
	}
	budget := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	defer func() { out.ExactOperations = budget.operations }()
	var inverse [2][2][3]*big.Rat
	var limits [2][2][2]*big.Rat
	var axes [2][2]int
	for i, p := range []*SurfacePatch{a, c} {
		_, inverse[i], e = normalizeAffineXYGraph(ctx, p, budget)
		if e != nil {
			return out, e
		}
		d := p.Domain()
		lo := [2]float64{d.UMin, d.VMin}
		hi := [2]float64{d.UMax, d.VMax}
		for j, row := range inverse[i] {
			axis := -1
			for k := 0; k < 2; k++ {
				if row[k].Sign() != 0 {
					if axis != -1 {
						return out, ErrConvergence
					}
					axis = k
				}
			}
			if axis < 0 {
				return out, ErrDegenerate
			}
			axes[i][j] = axis
			l := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(lo[j]), row[2]), row[axis])
			h := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(hi[j]), row[2]), row[axis])
			if l.Cmp(h) > 0 {
				l, h = h, l
			}
			limits[i][axis] = [2]*big.Rat{l, h}
			if e = budget.check(l); e != nil {
				return out, e
			}
			if e = budget.check(h); e != nil {
				return out, e
			}
		}
	}
	var overlap [2][2]*big.Rat
	touch := false
	for k := 0; k < 2; k++ {
		l, h := limits[0][k][0], limits[0][k][1]
		if limits[1][k][0].Cmp(l) > 0 {
			l = limits[1][k][0]
		}
		if limits[1][k][1].Cmp(h) < 0 {
			h = limits[1][k][1]
		}
		if l.Cmp(h) > 0 {
			out.Disjoint = true
			return out, nil
		}
		touch = touch || l.Cmp(h) == 0
		overlap[k] = [2]*big.Rat{l, h}
	}
	if touch {
		return out, ErrConvergence
	}
	var domains [2]ParameterRectangle
	for i := range inverse {
		var lo, hi [2]float64
		for j, row := range inverse[i] {
			axis := axes[i][j]
			l := new(big.Rat).Add(new(big.Rat).Mul(row[axis], overlap[axis][0]), row[2])
			h := new(big.Rat).Add(new(big.Rat).Mul(row[axis], overlap[axis][1]), row[2])
			if l.Cmp(h) > 0 {
				l, h = h, l
			}
			if e = budget.check(l); e != nil {
				return out, e
			}
			if e = budget.check(h); e != nil {
				return out, e
			}
			lo[j], e = RepresentedExactScalar(l)
			if e != nil {
				return out, e
			}
			hi[j], e = RepresentedExactScalar(h)
			if e != nil {
				return out, e
			}
		}
		domains[i] = ParameterRectangle{UMin: lo[0], UMax: hi[0], VMin: lo[1], VMax: hi[1]}
	}
	out.A, out.B = domains[0], domains[1]
	return out, nil
}
