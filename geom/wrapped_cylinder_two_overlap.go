package geom

import (
	"context"
	"math/big"
)

// WrappedCylinderTwoOverlapOptions bounds the canonical quotient embedding,
// exact arithmetic and occurrence enumeration together. Defaults: 4096 poles,
// two occurrences, 1M operations and 16384 coefficient bits.
type WrappedCylinderTwoOverlapOptions struct {
	WrappedRectangleOverlapOptions
	MaxPoles int
}

// WrappedCylinderIntersectionComponent retains an original occurrence and
// directed corner loops. ALoop is CCW on the canonical source chart (outward
// on the cylinder); BLoop follows the same model traversal in B's original
// chart, including any signed axis map and integer period shift.
type WrappedCylinderIntersectionComponent struct {
	Overlap      WrappedRectangleOverlap
	ALoop, BLoop [4][2]*big.Rat
}

// WrappedCylinderTwoOverlap certifies a specific two-component periodic
// overlap, not a generalized shell Boolean. The two closed intersection
// rectangles have disjoint model images. Their union of source faces covers
// every circumferential direction with a positive common height interval,
// hence is one annular band with two boundary components. No projected hull
// injectivity is asserted: a full cylindrical quotient cannot have that proof.
// All exact values are caller-owned. No result survives an unresolved check.
type WrappedCylinderTwoOverlap struct {
	Complete      bool
	Embedding     WrappedCylinderEmbedding
	Components    []WrappedCylinderIntersectionComponent
	CommonSourceV [2]*big.Rat
	// Gaps separate the two intersection components in A's lifted angular
	// order, then across the period. Both must be strictly positive.
	AngularGaps        [2]*big.Rat
	UnionAngularLength *big.Rat
	// Loop winding signs in each original UV chart, not normalized normals.
	OwnerOrientations                               [2]int
	IntersectionComponents, UnionBoundaryComponents int
	UnionConnected, UnionAnnular                    bool
	ExactOperations                                 int
}

// CertifyCylinderTwoRectangleOverlap is a narrow alternative to the general
// single-overlap arrangement. The source must satisfy the independent global
// canonical cylinder embedding certificate. Both input angular widths must be
// strictly below one period; exactly two positive-area occurrences are required.
// A retained exact chart relation proves the target support, which may have
// independent knots, normalized/reversed domains, or an axis permutation.
// This never combines local embedding proofs or merges repeated physical
// branches. Other overlap/contact regimes remain explicitly unresolved.
func (r *WrappedSurfaceChartRelation) CertifyCylinderTwoRectangleOverlap(ctx context.Context, a, b [2][2]*big.Rat, o WrappedCylinderTwoOverlapOptions) (WrappedCylinderTwoOverlap, error) {
	var zero WrappedCylinderTwoOverlap
	if ctx == nil || r == nil || r.source == nil || r.target == nil {
		return zero, ErrInvalid
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxComponents == 0 {
		o.MaxComponents = 2
	}
	if o.MaxPoles == 0 {
		o.MaxPoles = 4096
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxComponents < 1 || o.MaxComponents > 65536 || o.MaxPoles < 4 {
		return zero, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	for _, rect := range [2][2][2]*big.Rat{a, b} {
		for _, span := range rect {
			for _, q := range span {
				if q == nil {
					return zero, ErrInvalid
				}
				if e := budget.check(q); e != nil {
					return zero, e
				}
			}
			if span[0].Cmp(span[1]) >= 0 {
				return zero, ErrDomain
			}
		}
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if budget.operations >= o.MaxOperations {
		return zero, ErrConvergence
	}
	embedding, e := CertifyWrappedCylinderEmbedding(ctx, r.source, SurfacePatchOptions{MaxPoles: o.MaxPoles, MaxOperations: o.MaxOperations - budget.operations, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return zero, e
	}
	budget.operations += embedding.ExactOperations
	if !embedding.Complete {
		return zero, ErrConvergence
	}
	period := r.source.period[0]
	targetAxis := r.axis[0]
	targetPeriod := r.target.period[targetAxis]
	if period == nil || targetPeriod == nil {
		return zero, ErrConvergence
	}
	widthA := new(big.Rat).Sub(a[0][1], a[0][0])
	widthB := new(big.Rat).Sub(b[targetAxis][1], b[targetAxis][0])
	for _, q := range []*big.Rat{widthA, widthB} {
		if e := budget.check(q); e != nil {
			return zero, e
		}
	}
	if widthA.Cmp(period) >= 0 || widthB.Cmp(targetPeriod) >= 0 {
		return zero, ErrConvergence
	}
	if budget.operations >= o.MaxOperations {
		return zero, ErrConvergence
	}
	overlapOptions := o.WrappedRectangleOverlapOptions
	overlapOptions.MaxOperations = o.MaxOperations - budget.operations
	overlap, e := r.OverlapRectanglesRational(ctx, a, b, overlapOptions)
	if e != nil {
		return zero, e
	}
	budget.operations += overlap.ExactOperations
	if !overlap.Complete || len(overlap.Components) != 2 || overlap.Components[0].Dimension != 2 || overlap.Components[1].Dimension != 2 {
		return zero, ErrConvergence
	}
	parts := overlap.Components
	if parts[0].A[0][0].Cmp(parts[1].A[0][0]) > 0 {
		parts[0], parts[1] = parts[1], parts[0]
	}
	for end := 0; end < 2; end++ {
		if parts[0].A[1][end].Cmp(parts[1].A[1][end]) != 0 {
			return zero, ErrConvergence
		}
	}
	gaps := [2]*big.Rat{new(big.Rat).Sub(parts[1].A[0][0], parts[0].A[0][1]), new(big.Rat).Sub(new(big.Rat).Add(parts[0].A[0][0], period), parts[1].A[0][1])}
	for _, q := range gaps {
		if e := budget.check(q); e != nil {
			return zero, e
		}
		if q.Sign() <= 0 {
			return zero, ErrConvergence
		}
	}
	// Both source intervals are injective in the quotient. The complete exact
	// overlap enumeration therefore gives angular union length by inclusion-
	// exclusion. A closed arc union missing any angular point has an open gap
	// of positive length; equality to the period proves full angular coverage.
	scale := new(big.Rat).Abs(r.scale[0])
	sourceWidthB := new(big.Rat).Quo(widthB, scale)
	union := new(big.Rat).Add(widthA, sourceWidthB)
	for _, p := range parts {
		union.Sub(union, new(big.Rat).Sub(p.A[0][1], p.A[0][0]))
	}
	for _, q := range []*big.Rat{sourceWidthB, union} {
		if e := budget.check(q); e != nil {
			return zero, e
		}
	}
	if union.Cmp(period) != 0 {
		return zero, ErrConvergence
	}
	result := WrappedCylinderTwoOverlap{Complete: true, Embedding: embedding, CommonSourceV: [2]*big.Rat{new(big.Rat).Set(parts[0].A[1][0]), new(big.Rat).Set(parts[0].A[1][1])}, AngularGaps: gaps, UnionAngularLength: union, OwnerOrientations: [2]int{1, r.orientation}, IntersectionComponents: 2, UnionBoundaryComponents: 2, UnionConnected: true, UnionAnnular: true}
	for _, p := range parts {
		component := WrappedCylinderIntersectionComponent{Overlap: p}
		corners := [4][2]*big.Rat{{p.A[0][0], p.A[1][0]}, {p.A[0][1], p.A[1][0]}, {p.A[0][1], p.A[1][1]}, {p.A[0][0], p.A[1][1]}}
		for i, uv := range corners {
			for axis, target := range r.axis {
				component.ALoop[i][axis] = new(big.Rat).Set(uv[axis])
				q := new(big.Rat).Add(r.offset[axis], new(big.Rat).Mul(r.scale[axis], uv[axis]))
				if period := r.target.period[target]; period != nil {
					q.Sub(q, new(big.Rat).Mul(new(big.Rat).SetInt(p.TargetPeriodShift[target]), period))
				}
				if e := budget.check(q); e != nil {
					return zero, e
				}
				component.BLoop[i][target] = q
			}
		}
		result.Components = append(result.Components, component)
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	result.ExactOperations = budget.operations
	return result, nil
}
