package geom

import (
	"context"
	"math/big"
)

// EvaluateRationalWitness proves this immutable event parameter equals the
// supplied rational exactly before evaluating the original model continuation.
// A nearby parameter, even inside the current enclosure, returns ErrDomain.
// An irrational event cannot be replaced by a rounded rational witness.
// Operations and subdivisions are cumulatively bounded; failures are atomic.
func (e *ExactComposedPlaneEvent) EvaluateRationalWitness(ctx context.Context, parameter *big.Rat, o ExactCurvePlaneEventOptions) (RationalComposedCurvePoint, error) {
	var zero RationalComposedCurvePoint
	if ctx == nil || e == nil || e.root == nil || e.cell == nil || parameter == nil {
		return zero, ErrInvalid
	}
	opts, err := graphRootOptions(o)
	if err != nil {
		return zero, err
	}
	root, ops, err := NewExactRationalRoot(ctx, parameter, opts)
	if err != nil {
		return zero, err
	}
	opts.MaxOperations -= ops
	if opts.MaxOperations < 1 {
		return zero, ErrConvergence
	}
	cmp, err := e.Root().Compare(ctx, root, opts)
	ops += cmp.Operations
	if err != nil {
		return zero, err
	}
	if !cmp.Proven {
		return zero, ErrConvergence
	}
	if cmp.Order != 0 {
		return zero, ErrDomain
	}
	opts.MaxOperations -= cmp.Operations
	opts.MaxSubdivisions -= cmp.Subdivisions
	if opts.MaxOperations < 1 || opts.MaxSubdivisions < 1 {
		return zero, ErrConvergence
	}
	result, err := e.cell.EvaluateRational(ctx, parameter, opts)
	if err != nil {
		return zero, err
	}
	result.ExactOperations += ops
	result.Subdivisions += cmp.Subdivisions
	return result, nil
}
