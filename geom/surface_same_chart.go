package geom

import (
	"context"
	"math/big"
	"slices"
)

// SurfaceChartComparisonOptions bounds exact affine-free spline support
// comparison. Matching degrees and represented knot vectors are required.
type SurfaceChartComparisonOptions struct{ MaxOperations, MaxCoefficientBits int }

// SurfaceChartComparison compares exact homogeneous representations with a
// constant positive weight scale. Identical proves the same point at every
// original UV value. A false value does not prove distinct surfaces.
type SurfaceChartComparison struct {
	Identical       bool
	ExactOperations int
}

// CompareSurfaceCharts proves exact parametric identity of homogeneous
// patches with identical tensor basis and proportional nets. A common
// global weight multiplier is ignored; sampled equality is never used.
func CompareSurfaceCharts(ctx context.Context, a, b *SurfacePatch, o SurfaceChartComparisonOptions) (r SurfaceChartComparison, err error) {
	if ctx == nil || a == nil || b == nil || len(a.net) == 0 || len(b.net) == 0 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	limits := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = limits.operations }()
	r.Identical, err = identicalSurfacePatches(ctx, a, b, limits)
	return r, err
}

func identicalSurfacePatches(ctx context.Context, a, b *SurfacePatch, limits *polynomialBudget) (bool, error) {
	if a.degreeU != b.degreeU || a.degreeV != b.degreeV || a.countU != b.countU || a.countV != b.countV ||
		!slices.Equal(a.knotsU, b.knotsU) || !slices.Equal(a.knotsV, b.knotsV) {
		return false, nil
	}
	x, y := a.net[0][3], b.net[0][3]
	for i, v := range a.net {
		if e := ctx.Err(); e != nil {
			return false, e
		}
		for j, p := range v {
			l, rr := new(big.Rat).Mul(p, y), new(big.Rat).Mul(b.net[i][j], x)
			for _, q := range []*big.Rat{l, rr} {
				if e := limits.check(q); e != nil {
					return false, e
				}
			}
			if l.Cmp(rr) != 0 {
				return false, nil
			}
		}
	}
	return true, nil
}
