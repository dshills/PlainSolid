package geom

import (
	"context"
	"math/big"
)

// AffineProjectedSplineGraphPolynomialVolumes records the exact positive
// layers measured over a proved affine XY footprint. AreaScale is the
// absolute determinant from reference UV to physical XY.
type AffineProjectedSplineGraphPolynomialVolumes struct {
	ProjectedSplineGraphPolynomialVolumes
	AreaScale *big.Rat
}

// IntegrateAffineProjectedSplineGraphPolynomialVolumes measures the exact
// positive A-minus-B layers of two original roofs. Each roof must cover the
// independent reference footprint and have an exact tensor chart alignment.
// The reference's whole affine XY projection is certified before its
// constant physical area Jacobian is applied. Its height is independent.
// The original difference denominator must be positive and constant on
// each union-knot cell, with one common quadratic height scale.
func IntegrateAffineProjectedSplineGraphPolynomialVolumes(ctx context.Context, reference, a, b *SurfacePatch, axis int,
	o GraphPointContactOptions) (r AffineProjectedSplineGraphPolynomialVolumes, err error) {
	if ctx == nil || reference == nil || a == nil || b == nil || axis < 0 || axis > 1 {
		return r, ErrInvalid
	}
	o, e := alignedPolynomialProofOptions(o)
	if e != nil {
		return r, e
	}
	limits := SurfacePatchOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}
	graph, work, e := newAffineXYGraphWithWork(ctx, reference, limits)
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits, operations: work}
	defer func() {
		if err != nil {
			r = AffineProjectedSplineGraphPolynomialVolumes{}
		}
		r.ExactOperations = budget.operations
	}()
	if e != nil {
		return r, e
	}
	m := graph.inverse
	det := new(big.Rat).Sub(new(big.Rat).Mul(m[0][0], m[1][1]), new(big.Rat).Mul(m[0][1], m[1][0]))
	if e := budget.check(det); e != nil {
		return r, e
	}
	r.AreaScale = new(big.Rat).Inv(det.Abs(det))
	if e := budget.check(r.AreaScale); e != nil {
		return r, e
	}
	o.MaxOperations -= budget.operations
	if o.MaxOperations < 1 {
		return r, ErrConvergence
	}
	var volumes polynomialPositiveVolumes
	p, e := discoverProjectedSplineGraphPolynomialArrangement(ctx, reference, a, b, axis, o,
		polynomialVolumeDiscovery(false, r.AreaScale, &volumes))
	budget.operations += p.ExactOperations
	if e != nil {
		return r, e
	}
	r.ProjectedSplineGraphPolynomialVolumes = ProjectedSplineGraphPolynomialVolumes{
		Projection: p, Regions: volumes.regions, Volumes: volumes.volumes, Total: volumes.total,
	}
	return r, nil
}
