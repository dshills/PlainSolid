package geom

import (
	"context"
	"errors"
)

// VerifiedNormalOffsetSeparationOptions bounds exact pair subdivision.
// Zero selects depth 28, 16,384 visited pairs and four million exact
// operations. The exact patch poles and coefficient limits still apply.
type VerifiedNormalOffsetSeparationOptions struct {
	Patch              SurfacePatchOptions
	MaxDepth, MaxPairs int
}

// VerifiedNormalOffsetPairCell covers a still possible contact between the
// two original parameter rectangles. It is not a contact or intersection.
type VerifiedNormalOffsetPairCell struct {
	A, B  ParameterRectangle
	Depth int
}

// VerifiedNormalOffsetSeparation proves that the two constructed offset
// images are disjoint if Complete is true. Every possible contact remains
// in Unresolved when the proof cannot exclude the full original pair.
type VerifiedNormalOffsetSeparation struct {
	Complete                    bool
	Unresolved                  []VerifiedNormalOffsetPairCell
	VisitedPairs, ExcludedPairs int
	ExactOperations             int
}

// CertifyVerifiedNormalOffsetSeparation independently bounds both final
// offset images and excludes pairs of cells by strict rational box gaps.
// Neither a shared boundary nor a mere overlap of enclosure boxes is
// accepted as disjointness. It retains an unresolved cover on
// ErrConvergence or cancellation, including all unprocessed pairs.
func CertifyVerifiedNormalOffsetSeparation(ctx context.Context, a, b *VerifiedNormalOffsetPatch,
	o VerifiedNormalOffsetSeparationOptions) (r VerifiedNormalOffsetSeparation, err error) {
	if ctx == nil || a == nil || b == nil || !a.Domain().Valid() || !b.Domain().Valid() ||
		o.MaxDepth < 0 || o.MaxDepth > 64 || o.MaxPairs < 0 || o.MaxPairs > 1<<20 {
		return r, ErrInvalid
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 28
	}
	if o.MaxPairs == 0 {
		o.MaxPairs = 16384
	}
	if o.Patch.MaxOperations == 0 {
		o.Patch.MaxOperations = 4_000_000
	}
	o.Patch, err = patchOptions(o.Patch)
	if err != nil {
		return r, err
	}
	type box struct {
		domain ParameterRectangle
		bounds RationalSurfaceOffsetPoint
		tested bool
	}
	type pair struct {
		sides [2]box
		depth int
	}
	parts := [2]*VerifiedNormalOffsetPatch{a, b}
	global := [2]ParameterRectangle{a.Domain(), b.Domain()}
	stack := []pair{{sides: [2]box{{domain: global[0]}, {domain: global[1]}}}}
	keep := func(n pair) {
		r.Unresolved = append(r.Unresolved, VerifiedNormalOffsetPairCell{n.sides[0].domain, n.sides[1].domain, n.depth})
	}
	defer func() {
		for i := len(stack) - 1; i >= 0; i-- {
			keep(stack[i])
		}
	}()
	for len(stack) != 0 {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		if r.VisitedPairs >= o.MaxPairs {
			return r, ErrConvergence
		}
		i := len(stack) - 1
		n := &stack[i]
		r.VisitedPairs++
		for k, part := range parts {
			cell := &n.sides[k]
			if cell.tested {
				continue
			}
			limit := o.Patch
			limit.MaxOperations -= r.ExactOperations
			if limit.MaxOperations < 1 {
				return r, ErrConvergence
			}
			bounds, err := part.BoundsRational(ctx, cell.domain, limit)
			r.ExactOperations += min(bounds.ExactOperations, limit.MaxOperations)
			if err != nil && (!errors.Is(err, ErrConvergence) || bounds.ExactOperations >= limit.MaxOperations) {
				return r, err
			}
			cell.bounds, cell.tested = bounds, true
		}
		x, y := n.sides[0].bounds, n.sides[1].bounds
		if x.Lower[0] != nil && y.Lower[0] != nil && exactOffsetBoxesDisjoint(x, y) {
			r.ExcludedPairs++
			stack = stack[:i]
			continue
		}
		if n.depth == o.MaxDepth {
			keep(*n)
			stack = stack[:i]
			continue
		}
		domains := [2]ParameterRectangle{n.sides[0].domain, n.sides[1].domain}
		prefer := -1
		if x.Lower[0] == nil && y.Lower[0] != nil {
			prefer = 0
		} else if y.Lower[0] == nil && x.Lower[0] != nil {
			prefer = 1
		}
		which, left, right := splitOffsetPairDomain(domains, global, prefer)
		if which < 0 {
			keep(*n)
			stack = stack[:i]
			continue
		}
		l, h := *n, *n
		l.depth, h.depth = n.depth+1, n.depth+1
		l.sides[which], h.sides[which] = box{domain: left}, box{domain: right}
		stack = append(stack[:i], h, l)
	}
	if len(r.Unresolved) > 0 {
		return r, ErrConvergence
	}
	r.Complete = true
	return r, nil
}

func exactOffsetBoxesDisjoint(a, b RationalSurfaceOffsetPoint) bool {
	for axis := range 3 {
		if a.Upper[axis].Cmp(b.Lower[axis]) < 0 || b.Upper[axis].Cmp(a.Lower[axis]) < 0 {
			return true
		}
	}
	return false
}

func splitOffsetPairDomain(d, source [2]ParameterRectangle, preferred int) (which int, left, right ParameterRectangle) {
	which, score := -1, -1.0
	width := func(low, high, rootLow, rootHigh float64) float64 {
		if span := rootHigh - rootLow; finite(span) {
			return (high - low) / span
		}
		return (high/2 - low/2) / (rootHigh/2 - rootLow/2)
	}
	for i, cell := range d {
		root := source[i]
		for axis := range 2 {
			lo, hi, rootLo, rootHi := cell.UMin, cell.UMax, root.UMin, root.UMax
			if axis == 1 {
				lo, hi, rootLo, rootHi = cell.VMin, cell.VMax, root.VMin, root.VMax
			}
			t := parameterMidpoint(lo, hi)
			if !(lo < t && t < hi) {
				continue
			}
			w := width(lo, hi, rootLo, rootHi)
			if i == preferred {
				w += 1
			}
			if w <= score {
				continue
			}
			score, which, left, right = w, i, cell, cell
			if axis == 0 {
				left.UMax, right.UMin = t, t
			} else {
				left.VMax, right.VMin = t, t
			}
		}
	}
	return which, left, right
}
