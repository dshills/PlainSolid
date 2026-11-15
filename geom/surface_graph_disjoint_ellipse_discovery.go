package geom

import (
	"context"
	"errors"
	"sort"
)

// DisjointEllipseDiscoveryOptions bounds a complete two-loop search on original
// U or V knot separators. MaxSeparators defaults to 8, at most 64. Work and certified
// cell visits include failed candidates, discovery and final independent reproof.
type DisjointEllipseDiscoveryOptions struct {
	GraphPointContactOptions
	MaxSeparators int
	PartitionAxis int // 0 (default) uses original selected-axis knots; 1 uses original V knots.
}

type DiscoveredSplineGraphDisjointEllipses struct {
	PartitionAxis int
	Separator     float64

	Complete                          bool
	Section                           SplineGraphDisjointEllipses
	SplitU                            float64 // Legacy U-only separator; zero for V partitions.
	TestedSeparators, ExactOperations int
}

// DiscoverSplineGraphDisjointEllipses discovers exactly two separated closed
// contours without caller hints. Candidates are exclusively original selected-axis knots;
// every accepted candidate covers both complete closed original slabs. Failure
// is unresolved, never absence. No finite sampling infers additional contours.
func DiscoverSplineGraphDisjointEllipses(ctx context.Context, a, b *SurfacePatch, o DisjointEllipseDiscoveryOptions) (r DiscoveredSplineGraphDisjointEllipses, err error) {
	used, tested := 0, 0
	defer func() {
		if err != nil {
			r = DiscoveredSplineGraphDisjointEllipses{}
		}
		r.ExactOperations = used
		r.TestedSeparators = tested
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
	if o.MaxOperations < 1 || o.MaxCells < 1 || o.MaxCoefficientBits < 64 || o.MaxDepth < 0 || o.PartitionAxis < 0 || o.PartitionAxis > 1 || o.MaxSeparators < 1 || o.MaxSeparators > 64 {
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
	if len(unique) == 0 || len(unique) > o.MaxSeparators {
		return r, ErrConvergence
	}
	separators := make([]float64, 0, len(unique))
	for k := range unique {
		separators = append(separators, k)
	}
	sort.Float64s(separators)
	cells := 0
	for _, split := range separators {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		tested++
		var hints [2]GraphEllipseHint
		valid := true
		for side := 0; side < 2; side++ {
			slab := d
			if o.PartitionAxis == 0 {
				if side == 0 {
					slab.UMax = split
				} else {
					slab.UMin = split
				}
			} else {
				if side == 0 {
					slab.VMax = split
				} else {
					slab.VMin = split
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
		section, e := CertifySplineGraphDisjointEllipsesOnAxis(ctx, sources[0], sources[1], o.PartitionAxis, split, hints, proof)
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
		r.Separator = split
		if o.PartitionAxis == 0 {
			r.SplitU = split
		}
		return r, nil
	}
	return r, ErrConvergence
}
