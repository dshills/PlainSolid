package geom

import (
	"context"
	"math/big"
)

// HelicalLineLocation is relative to a closed one-dimensional interval union.
// Interior here means along the selected line, not a 3D manifold assertion.
type HelicalLineLocation uint8

const (
	HelicalLineOutside HelicalLineLocation = iota
	HelicalLineInterior
	HelicalLineBoundary
)

// HelicalBandPoint separates the finite band's location from swept material.
// RetainedInterior proves band interior minus closed swept material. Removed
// means a point in the closed band belongs to closed swept material (including
// its boundary). A band boundary is never labeled RetainedInterior. These are
// exact set-membership semantics, not regularized Boolean topology semantics.
type HelicalBandPoint struct {
	Complete                       bool
	BandLocation, MaterialLocation HelicalLineLocation
	RetainedInterior, Removed      bool
	Parameter                      *big.Rat
	Material                       HelicalPolygonLineResult
	Refinements, ExactOperations   int
}

// ClassifyBandPoint classifies a rational parameter on the caller's original
// axis line against a finite closed band and the swept polygon material. Ray,
// line units and turn-domain semantics match IntersectAxisLine. Algebraic/
// transcendental boundary enclosures are refined with cumulative budgets until
// the rational query separates; unsupported equality remains ErrConvergence.
// Exact cardinal boundaries are reported as boundaries, never tolerance hits.
func (p HelicalPolygon) ClassifyBandPoint(ctx context.Context, cs, sn *big.Rat, origin [3]*big.Rat, axisScale *big.Rat, band [2]*big.Rat, parameter *big.Rat, o HelicalPolygonLineOptions) (out HelicalBandPoint, err error) {
	if ctx == nil || parameter == nil || band[0] == nil || band[1] == nil || band[0].Cmp(band[1]) >= 0 {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	maxOperations := o.MaxOperations
	if maxOperations == 0 {
		maxOperations = 1000000
	}
	maxBits := o.MaxCoefficientBits
	if maxBits == 0 {
		maxBits = 16384
	}
	if maxOperations < 1 || maxBits < 64 {
		return out, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: maxOperations, maxBits: maxBits}
	defer func() {
		out.ExactOperations = b.operations
		if err != nil {
			out.Complete = false
			out.RetainedInterior = false
			out.Removed = false
		}
	}()
	for _, q := range []*big.Rat{band[0], band[1], parameter} {
		if e := b.check(q); e != nil {
			return out, e
		}
	}
	out.Parameter = new(big.Rat).Set(parameter)
	if parameter.Cmp(band[0]) < 0 || parameter.Cmp(band[1]) > 0 {
		out.BandLocation = HelicalLineOutside
	} else if parameter.Cmp(band[0]) == 0 || parameter.Cmp(band[1]) == 0 {
		out.BandLocation = HelicalLineBoundary
	} else {
		out.BandLocation = HelicalLineInterior
	}
	bits := o.PhaseBits
	if bits == 0 {
		bits = 64
	}
	if bits < 16 || bits > 512 {
		return out, ErrInvalid
	}
	for {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		remaining := maxOperations - b.operations
		if remaining < 1 {
			return out, ErrConvergence
		}
		query := o
		query.MaxOperations = remaining
		query.MaxCoefficientBits = maxBits
		query.PhaseBits = bits
		report, e := p.IntersectAxisLine(ctx, cs, sn, origin, axisScale, query)
		b.operations += report.ExactOperations
		if e != nil {
			return out, e
		}
		out.Material = report
		unresolved := false
		location := HelicalLineOutside
		for _, interval := range report.Intervals {
			lo, hi := interval.Bounds[0], interval.Bounds[1]
			if parameter.Cmp(lo[0]) < 0 || parameter.Cmp(hi[1]) > 0 {
				continue
			}
			if (lo[0].Cmp(lo[1]) == 0 && parameter.Cmp(lo[0]) == 0) || (hi[0].Cmp(hi[1]) == 0 && parameter.Cmp(hi[0]) == 0) {
				location = HelicalLineBoundary
				unresolved = false
				break
			}
			if parameter.Cmp(lo[1]) > 0 && parameter.Cmp(hi[0]) < 0 {
				location = HelicalLineInterior
				unresolved = false
				break
			}
			unresolved = true
		}
		if !unresolved {
			out.MaterialLocation = location
			out.RetainedInterior = out.BandLocation == HelicalLineInterior && location == HelicalLineOutside
			out.Removed = out.BandLocation != HelicalLineOutside && location != HelicalLineOutside
			out.Complete = true
			return out, nil
		}
		if bits == 512 {
			return out, ErrConvergence
		}
		bits *= 2
		if bits > 512 {
			bits = 512
		}
		out.Refinements++
	}
}
