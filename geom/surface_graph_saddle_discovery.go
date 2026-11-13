package geom

import (
	"context"
	"errors"
)

// QuadraticGraphSaddleDiscovery retains one independently certified saddle.
// Exhausting finite quadratic candidates is not a general absence certificate.
type QuadraticGraphSaddleDiscovery struct {
	Certificate                       QuadraticGraphSaddleBranches
	CandidatesTested, ExactOperations int
}

// DiscoverQuadraticGraphSaddle derives exact stationary candidates only from
// genuinely quadratic numerator knot cells, then independently verifies the
// original supports and four-branch saddle certificate. Nonzero critical levels,
// degenerate or higher-order contacts and witnesses on knots remain unresolved.
func DiscoverQuadraticGraphSaddle(ctx context.Context, a, c *SurfacePatch, o GraphPointContactDiscoveryOptions) (r QuadraticGraphSaddleDiscovery, err error) {
	if ctx == nil || a == nil || c == nil || len(a.net) == 0 || len(c.net) == 0 {
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
	if o.MaxCandidates == 0 {
		o.MaxCandidates = 128
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxCells < 1 || o.MaxCandidates < 1 || o.MaxDepth < 0 {
		return r, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	difference, e := orderingDifference(ctx, a, c, o.MaxCells, b)
	if e != nil {
		return r, e
	}
	cells, e := graphContactBezierCells(ctx, difference, o.MaxCells, b)
	if e != nil {
		return r, e
	}
	seen := map[string]bool{}
	for _, cell := range cells {
		candidates, e := quadraticGraphWitnesses(cell, b)
		if e != nil {
			return r, e
		}
		for _, uv := range candidates {
			if e := ctx.Err(); e != nil {
				return r, e
			}
			key := uv[0].RatString() + "," + uv[1].RatString()
			if seen[key] {
				continue
			}
			seen[key] = true
			if r.CandidatesTested >= o.MaxCandidates {
				return r, ErrConvergence
			}
			r.CandidatesTested++
			remaining := o.MaxOperations - b.operations
			if remaining < 1 {
				return r, ErrConvergence
			}
			options := o.GraphPointContactOptions
			options.MaxOperations = remaining
			proof, e := CertifyQuadraticGraphSaddleBranches(ctx, a, c, uv, options)
			b.operations += proof.ExactOperations
			if e == nil && proof.Proven {
				r.Certificate = proof
				return r, nil
			}
			if e != nil && !errors.Is(e, ErrConvergence) {
				return r, e
			}
		}
	}
	return r, ErrConvergence
}
