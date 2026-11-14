package intersect

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// DyadicGraphDiscoveryOptions bounds exact lattice guidance. GridDivisions
// defaults to16 (power of two,<=64); MaxSamples defaults to4096. Guidance only
// proposes knot cuts. A complete contour proof and original tube proofs must
// succeed afterward; absence of dyadic zero points never proves disjointness.
type DyadicGraphDiscoveryOptions struct {
	SplinePlanePairDiscoveryOptions
	GridDivisions, MaxSamples int
}

// DiscoverDyadicGraphPairTubes discovers trace cuts from exact zero-height
// points on a finite rational lattice. It assumes no conic or radial factor.
// Result seeds still require independent pair coverage and branch topology.
func DiscoverDyadicGraphPairTubes(ctx context.Context, a, b Patch, p geom.SurfacePatchOptions, o DyadicGraphDiscoveryOptions) (out SplineSeedReport, err error) {
	if ctx == nil || !a.valid() || !b.valid() {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if o.GridDivisions == 0 {
		o.GridDivisions = 16
	}
	if o.MaxSamples == 0 {
		o.MaxSamples = 4096
	}
	n := o.GridDivisions
	if n < 2 || n > 64 || n&(n-1) != 0 || o.MaxSamples < 1 {
		return out, geom.ErrInvalid
	}
	if (n-1)*(n-1) > o.MaxSamples {
		return out, geom.ErrConvergence
	}
	maximum := o.Tubes.MaxOperations
	if maximum == 0 {
		maximum = 8000000
	}
	bits := o.Tubes.MaxCoefficientBits
	if bits == 0 {
		bits = 16384
	}
	if maximum < 1 || bits < 64 {
		return out, geom.ErrInvalid
	}
	pa, e := exactSplinePatch(a, p)
	if e != nil {
		return out, e
	}
	pb, e := exactSplinePatch(b, p)
	if e != nil {
		return out, e
	}
	difference, e := geom.NewGeneralSurfaceHeightDifferencePatch(ctx, pa, pb, geom.GeneralSurfaceHeightOptions{MaxOperations: maximum, MaxCoefficientBits: bits})
	if e != nil {
		return out, e
	}
	used := difference.ExactOperations()
	domain := difference.Domain()
	axes := [2][2]float64{{domain.UMin, domain.UMax}, {domain.VMin, domain.VMax}}
	values := [2][]*big.Rat{}
	for axis := 0; axis < 2; axis++ {
		values[axis] = make([]*big.Rat, n+1)
		lo := new(big.Rat).SetFloat64(axes[axis][0])
		width := new(big.Rat).Sub(new(big.Rat).SetFloat64(axes[axis][1]), lo)
		for i := 0; i <= n; i++ {
			values[axis][i] = new(big.Rat).Add(lo, new(big.Rat).Mul(width, big.NewRat(int64(i), int64(n))))
		}
	}
	cuts := [2]map[float64]bool{{}, {}}
	for i := 1; i < n; i++ {
		for j := 1; j < n; j++ {
			if e := ctx.Err(); e != nil {
				return out, e
			}
			remaining := maximum - used
			if remaining < 1 {
				return out, geom.ErrConvergence
			}
			jet, e := difference.EvaluateRational(ctx, values[0][i], values[1][j], geom.SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: bits})
			used += jet.ExactOperations
			if e != nil {
				return out, e
			}
			if jet.Point[2].Sign() != 0 {
				continue
			}
			for axis, index := range []int{i, j} {
				value, e := geom.RepresentedExactScalar(values[axis][index])
				if e != nil {
					return out, e
				}
				cuts[axis][value] = true
			}
		}
	}
	if len(cuts[0]) == 0 || len(cuts[1]) == 0 {
		return out, ErrUnresolved
	}
	if len(cuts[0])+len(cuts[1]) > 64 {
		return out, ErrUnresolved
	}
	child := o.SplinePlanePairDiscoveryOptions
	child.UCuts = append([]float64(nil), child.UCuts...)
	child.VCuts = append([]float64(nil), child.VCuts...)
	for axis := 0; axis < 2; axis++ {
		var list []float64
		for x := range cuts[axis] {
			list = append(list, x)
		}
		sort.Float64s(list)
		if axis == 0 {
			child.UCuts = append(child.UCuts, list...)
		} else {
			child.VCuts = append(child.VCuts, list...)
		}
	}
	child.Tubes.MaxOperations = maximum - used
	if child.Tubes.MaxOperations < 1 {
		return out, geom.ErrConvergence
	}
	return DiscoverCommonXYGraphPairTubes(ctx, a, b, p, child)
}
