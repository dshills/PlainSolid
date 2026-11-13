package geom

import (
	"context"
	"math/big"
)

type ComposedCollectionAffineParameterOptions struct {
	EventOptions      ExactCurvePlaneEventOptions
	MaxCells, MaxCuts int
}

// ComposedCollectionAffineParameterComparison covers exactly the supplied A
// domain. Pieces counts the common exact partition, not source knot counts.
// Equivalent=false is a proved failure of the supplied affine correspondence,
// not a proof of disjoint images. No periodic endpoint identification is made.
type ComposedCollectionAffineParameterComparison struct {
	Proven, Equivalent, Reversed bool
	Domain                       [2]*ExactAlgebraicRoot
	Pieces                       int
}

// CompareComposedCollectionsAffineParameter proves B(offset+scale*t)=A(t)
// across differing knot partitions. All endpoint ordering, restrictions and
// mapped domains use exact algebraic identities. It validates each collection's
// seams independently. Each proof uses EventOptions limits; MaxCells/MaxCuts
// bound the number of validations, cut comparisons and subinterval proofs.
// Unsupported algebraic seam validation or any limit returns atomic failure.
func CompareComposedCollectionsAffineParameter(ctx context.Context, a, b SurfaceCurveCompositionCells, offset, scale *big.Rat, o ComposedCollectionAffineParameterOptions) (ComposedCollectionAffineParameterComparison, error) {
	var zero ComposedCollectionAffineParameterComparison
	if ctx == nil || !a.Complete || !b.Complete || len(a.Cells) == 0 || len(b.Cells) == 0 || offset == nil || scale == nil || scale.Sign() == 0 {
		return zero, ErrInvalid
	}
	if o.MaxCells == 0 {
		o.MaxCells = 64
	}
	if o.MaxCuts == 0 {
		o.MaxCuts = 256
	}
	if o.MaxCells < 1 || o.MaxCells > 4096 || o.MaxCuts < 2 || o.MaxCuts > 8192 {
		return zero, ErrInvalid
	}
	if len(a.Cells) > o.MaxCells || len(b.Cells) > o.MaxCells {
		return zero, ErrConvergence
	}
	opts, e := graphRootOptions(o.EventOptions)
	if e != nil {
		return zero, e
	}
	budget := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	for _, q := range []*big.Rat{offset, scale} {
		if e := budget.check(q); e != nil {
			return zero, e
		}
	}
	if e = a.validateContactSeams(ctx, opts); e != nil {
		return zero, e
	}
	if e = b.validateContactSeams(ctx, opts); e != nil {
		return zero, e
	}
	domain := [2]*ExactAlgebraicRoot{}
	domain[0], _ = a.Cells[0].EndpointRoot(0)
	domain[1], _ = a.Cells[len(a.Cells)-1].EndpointRoot(1)
	bDomain := [2]*ExactAlgebraicRoot{}
	bDomain[0], _ = b.Cells[0].EndpointRoot(0)
	bDomain[1], _ = b.Cells[len(b.Cells)-1].EndpointRoot(1)
	for i, r := range domain {
		mapped, _, e := r.AffineMap(ctx, offset, scale, opts)
		if e != nil {
			return zero, e
		}
		side := i
		if scale.Sign() < 0 {
			side = 1 - i
		}
		q, e := mapped.Compare(ctx, bDomain[side], opts)
		if e != nil {
			return zero, e
		}
		if !q.Proven || q.Order != 0 {
			return zero, ErrDomain
		}
	}
	inverseScale := new(big.Rat).Inv(scale)
	inverseOffset := new(big.Rat).Mul(new(big.Rat).Neg(offset), inverseScale)
	var cuts []*ExactAlgebraicRoot
	insert := func(root *ExactAlgebraicRoot) error {
		at := len(cuts)
		for i, old := range cuts {
			q, e := root.Compare(ctx, old, opts)
			if e != nil {
				return e
			}
			if !q.Proven {
				return ErrConvergence
			}
			if q.Order == 0 {
				return nil
			}
			if q.Order < 0 {
				at = i
				break
			}
		}
		if len(cuts) >= o.MaxCuts {
			return ErrConvergence
		}
		cuts = append(cuts, nil)
		copy(cuts[at+1:], cuts[at:])
		cuts[at] = root
		return nil
	}
	for owner, collection := range []SurfaceCurveCompositionCells{a, b} {
		for i, cell := range collection.Cells {
			for _, side := range []int{0, 1} {
				if side == 0 && i > 0 {
					continue
				}
				r, _ := cell.EndpointRoot(side)
				if owner == 1 {
					r, _, e = r.AffineMap(ctx, inverseOffset, inverseScale, opts)
					if e != nil {
						return zero, e
					}
				}
				if e := insert(r); e != nil {
					return zero, e
				}
			}
		}
	}
	// Find a source cell containing an entire closed subinterval; exact endpoint
	// equality can belong to either adjacent cell, but positive width fixes owner.
	restrict := func(collection SurfaceCurveCompositionCells, lo, hi *ExactAlgebraicRoot) (*ExactComposedCurveCell, error) {
		for _, cell := range collection.Cells {
			start, _ := cell.EndpointRoot(0)
			end, _ := cell.EndpointRoot(1)
			l, e := lo.Compare(ctx, start, opts)
			if e != nil {
				return nil, e
			}
			h, e := hi.Compare(ctx, end, opts)
			if e != nil {
				return nil, e
			}
			if !l.Proven || !h.Proven {
				return nil, ErrConvergence
			}
			if l.Order >= 0 && h.Order <= 0 {
				r, e := cell.RestrictAlgebraic(ctx, lo, hi, opts)
				if e != nil {
					return nil, e
				}
				if !r.Proven {
					return nil, ErrConvergence
				}
				return r.Cell, nil
			}
		}
		return nil, ErrDomain
	}
	equivalent := true
	for i := 0; i+1 < len(cuts); i++ {
		ac, e := restrict(a, cuts[i], cuts[i+1])
		if e != nil {
			return zero, e
		}
		lo, _, e := cuts[i].AffineMap(ctx, offset, scale, opts)
		if e != nil {
			return zero, e
		}
		hi, _, e := cuts[i+1].AffineMap(ctx, offset, scale, opts)
		if e != nil {
			return zero, e
		}
		if scale.Sign() < 0 {
			lo, hi = hi, lo
		}
		bc, e := restrict(b, lo, hi)
		if e != nil {
			return zero, e
		}
		r, e := CompareComposedCurvesAffineParameter(ctx, ac, bc, offset, scale, opts)
		if e != nil {
			return zero, e
		}
		if !r.Proven {
			return zero, ErrConvergence
		}
		equivalent = equivalent && r.Equivalent
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	return ComposedCollectionAffineParameterComparison{Proven: true, Equivalent: equivalent, Reversed: scale.Sign() < 0, Domain: domain, Pieces: len(cuts) - 1}, nil
}
