package geom

import (
	"context"
	"errors"
	"sort"
)

// ThreeEllipseDiscoveryOptions bounds a complete three-loop search on original
// U or V knot separators. MaxSeparators defaults to 8, at most 16. Work and certified
// cell visits include failed candidates, discovery and final independent reproof.
type ThreeEllipseDiscoveryOptions struct {
	GraphPointContactOptions
	MaxSeparators int
	PartitionAxis int // 0 (default) uses original selected-axis knots; 1 uses original V knots.
}

// DiscoveredSplineGraphThreeEllipses reports an atomic complete original-domain
// certificate and deterministic search work; no partial contour list is published.
type DiscoveredSplineGraphThreeEllipses struct {
	PartitionAxis int
	Separators    [2]float64

	Complete                          bool
	Section                           SplineGraphThreeEllipses
	SplitU                            [2]float64 // Legacy U-only separator; zero for V partitions.
	TestedPartitions, ExactOperations int
}

// DiscoverSplineGraphThreeEllipses discovers exactly three separated closed
// contours without caller hints. Candidates are exclusively original selected-axis knots;
// every accepted candidate covers all three complete closed original slabs. Failure
// is unresolved, never absence. No finite sampling infers additional contours.
func DiscoverSplineGraphThreeEllipses(ctx context.Context, a, b *SurfacePatch, o ThreeEllipseDiscoveryOptions) (r DiscoveredSplineGraphThreeEllipses, err error) {
	used, tested := 0, 0
	defer func() {
		if err != nil {
			r = DiscoveredSplineGraphThreeEllipses{}
		}
		r.ExactOperations = used
		r.TestedPartitions = tested
	}()
	if ctx == nil || a == nil || b == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.MaxSeparators == 0 {
		o.MaxSeparators = 8
	}
	if o.MaxOperations < 1 || o.MaxCells < 1 || o.MaxCoefficientBits < 64 || o.MaxDepth < 0 || o.PartitionAxis < 0 || o.PartitionAxis > 1 || o.MaxSeparators < 1 || o.MaxSeparators > 16 {
		return r, ErrInvalid
	}
	d := a.Domain()
	if !d.Valid() || d != b.Domain() {
		return r, ErrInvalid
	}
	var sources [2]*SurfacePatch
	for i, p := range []*SurfacePatch{a, b} {
		if used >= o.MaxOperations {
			return r, ErrConvergence
		}
		q, e := p.WithLimits(SurfacePatchOptions{MaxOperations: o.MaxOperations - used, MaxCoefficientBits: o.MaxCoefficientBits})
		if e != nil {
			return r, e
		}
		used += q.ExactOperations()
		sources[i] = q
	}
	unique := map[float64]bool{}
	for _, p := range sources {
		knots, vKnots := p.KnotBreaks()
		if o.PartitionAxis == 1 {
			knots = vKnots
		}
		for _, k := range knots {
			if k > [2]float64{d.UMin, d.VMin}[o.PartitionAxis] && k < [2]float64{d.UMax, d.VMax}[o.PartitionAxis] {
				unique[k] = true
			}
		}
	}
	if len(unique) < 2 || len(unique) > o.MaxSeparators {
		return r, ErrConvergence
	}
	separators := make([]float64, 0, len(unique))
	for k := range unique {
		separators = append(separators, k)
	}
	sort.Float64s(separators)
	cells := 0
	for left := 0; left < len(separators)-1; left++ {
		for right := left + 1; right < len(separators); right++ {
			split := [2]float64{separators[left], separators[right]}
			if e := ctx.Err(); e != nil {
				return r, e
			}
			tested++
			var hints [3]GraphEllipseHint
			valid := true
			for side := 0; side < 3; side++ {
				slab := d
				if o.PartitionAxis == 0 {
					if side > 0 {
						slab.UMin = split[side-1]
					}
					if side < 2 {
						slab.UMax = split[side]
					}
				} else {
					if side > 0 {
						slab.VMin = split[side-1]
					}
					if side < 2 {
						slab.VMax = split[side]
					}
				}
				var patches [2]*SurfacePatch
				for owner, p := range sources {
					if used >= o.MaxOperations {
						return r, ErrConvergence
					}
					q, work, e := p.TrimWithWorkLimit(slab, o.MaxOperations-used)
					used += work
					if e != nil {
						return r, e
					}
					patches[owner] = q
				}
				if used >= o.MaxOperations || cells >= o.MaxCells {
					return r, ErrConvergence
				}
				proof := o.GraphPointContactOptions
				proof.MaxOperations -= used
				proof.MaxCells -= cells
				q, e := DiscoverSplineGraphEllipse(ctx, patches[0], patches[1], proof)
				used += q.ExactOperations
				cells += len(q.CertifiedCells)
				if e != nil {
					if errors.Is(e, ErrConvergence) {
						valid = false
						break
					}
					return r, e
				}
				if !q.Proven {
					valid = false
					break
				}
				hints[side] = q.Ellipse
			}
			if !valid {
				continue
			}
			if used >= o.MaxOperations || cells >= o.MaxCells {
				return r, ErrConvergence
			}
			proof := o.GraphPointContactOptions
			proof.MaxOperations -= used
			proof.MaxCells -= cells
			section, e := CertifySplineGraphThreeEllipsesOnAxis(ctx, sources[0], sources[1], o.PartitionAxis, split, hints, proof)
			used += section.ExactOperations
			for _, q := range section.Contours {
				cells += len(q.CertifiedCells)
			}
			if e != nil {
				if errors.Is(e, ErrConvergence) {
					continue
				}
				return r, e
			}
			if !section.Complete {
				return r, ErrConvergence
			}
			r.Complete = true
			r.Section = section
			r.PartitionAxis = o.PartitionAxis
			r.Separators = split
			if o.PartitionAxis == 0 {
				r.SplitU = split
			}
			return r, nil
		}
	}
	return r, ErrConvergence
}
