package intersect

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// UniformGraphDiscoveryOptions adds a bounded exact dyadic partition to the
// height-difference contour proposal. GridDivisions defaults to 16, is a power
// of two, and cannot exceed 32. It does not require sampled zero points.
type UniformGraphDiscoveryOptions struct {
	SplinePlanePairDiscoveryOptions
	GridDivisions int
	// AxisDivisions optionally replaces the common resolution on each axis.
	// Each nonzero value is a power of two in [1,32]; 1 adds no cuts.
	// Caller cuts are merged and sorted, with at most 64 total cuts.
	AxisDivisions [2]int
}

// DiscoverUniformGraphPairTubes proposes all traced regular components on two
// common-XY graphs. Grid lines are exact original-domain coordinates, not
// sampled geometry. Success still requires independent CertifySplinePairBranches
// coverage of the original surfaces; a finite grid never proves completeness.
// An unsuccessful discovery publishes no partial seed list.
func DiscoverUniformGraphPairTubes(ctx context.Context, a, b Patch, p geom.SurfacePatchOptions, o UniformGraphDiscoveryOptions) (SplineSeedReport, error) {
	if ctx == nil || !a.valid() || !b.valid() || len(o.UCuts)+len(o.VCuts) > 64 {
		return SplineSeedReport{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return SplineSeedReport{}, e
	}
	n := o.GridDivisions
	if n == 0 {
		n = 16
	}
	if n < 2 || n > 32 || n&(n-1) != 0 {
		return SplineSeedReport{}, geom.ErrInvalid
	}
	child := o.SplinePlanePairDiscoveryOptions
	for axis, ends := range [2][2]float64{{a.Min.X, a.Max.X}, {a.Min.Y, a.Max.Y}} {
		resolution := n
		if o.AxisDivisions[axis] != 0 {
			resolution = o.AxisDivisions[axis]
		}
		if resolution < 1 || resolution > 32 || resolution&(resolution-1) != 0 {
			return SplineSeedReport{}, geom.ErrInvalid
		}
		values := map[float64]bool{}
		old := child.UCuts
		if axis == 1 {
			old = child.VCuts
		}
		for _, x := range old {
			if !(x > ends[0] && x < ends[1]) {
				return SplineSeedReport{}, geom.ErrInvalid
			}
			values[x] = true
		}
		lo := new(big.Rat).SetFloat64(ends[0])
		width := new(big.Rat).Sub(new(big.Rat).SetFloat64(ends[1]), lo)
		for i := 1; i < resolution; i++ {
			q := new(big.Rat).Add(lo, new(big.Rat).Mul(width, big.NewRat(int64(i), int64(resolution))))
			x, e := geom.RepresentedExactScalar(q)
			if e != nil {
				return SplineSeedReport{}, e
			}
			values[x] = true
		}
		cuts := make([]float64, 0, len(values))
		for x := range values {
			cuts = append(cuts, x)
		}
		sort.Float64s(cuts)
		if axis == 0 {
			child.UCuts = cuts
		} else {
			child.VCuts = cuts
		}
	}
	if len(child.UCuts)+len(child.VCuts) > 64 {
		return SplineSeedReport{}, geom.ErrInvalid
	}
	result, e := DiscoverCommonXYGraphPairTubes(ctx, a, b, p, child)
	if e != nil {
		return SplineSeedReport{}, e
	}
	return result, nil
}
