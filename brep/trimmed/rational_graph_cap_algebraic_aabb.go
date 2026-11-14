package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// AlgebraicGraphStockBounds is an axis-aligned enclosure of the original
// retained stock. Exact stores six true extrema and their source owners.
// Bounds stores outward rational endpoints; each exact endpoint is
// localized to at most the requested width.
type AlgebraicGraphStockBounds struct {
	Exact                         [3]AlgebraicGraphDirectionalBounds
	Bounds                        [3]geom.ExactParameterInterval
	ExactOperations, Subdivisions int
}

// BoundBoxRational proves all six local axis extrema and encloses each
// with one exact arithmetic budget. Width must be strictly positive.
func (m AlgebraicGraphVerticalMaterial) BoundBoxRational(ctx context.Context, width *big.Rat,
	opts geom.ExactCurvePlaneEventOptions) (AlgebraicGraphStockBounds, error) {
	var forms [3][4]*big.Rat
	for i := range forms {
		for j := range 3 {
			forms[i][j] = new(big.Rat)
		}
		forms[i][i].SetInt64(1)
	}
	return m.boundBoxRational(ctx, forms, width, opts)
}

// BoundBoxRational bounds the world occurrence tightly along the world
// axes. It pulls all three functionals to the original geometry rather
// than transforming the corners of a previously computed box.
func (o AlgebraicGraphStockOccurrence) BoundBoxRational(ctx context.Context, width *big.Rat,
	opts geom.ExactCurvePlaneEventOptions) (AlgebraicGraphStockBounds, error) {
	m, err := o.pose.Matrix()
	if err != nil {
		return AlgebraicGraphStockBounds{}, err
	}
	return o.model.boundBoxRational(ctx, m, width, opts)
}

func (m AlgebraicGraphVerticalMaterial) boundBoxRational(ctx context.Context, forms [3][4]*big.Rat,
	width *big.Rat, opts geom.ExactCurvePlaneEventOptions) (AlgebraicGraphStockBounds, error) {
	var zero AlgebraicGraphStockBounds
	if width == nil || width.Sign() <= 0 {
		return zero, geom.ErrInvalid
	}
	b, err := m.extremaBudget(ctx, opts)
	if err != nil {
		return zero, err
	}
	if max(width.Num().BitLen(), width.Denom().BitLen()) > b.bits {
		return zero, geom.ErrConvergence
	}
	maxWork, maxSubdiv := b.work, b.subdiv
	var out AlgebraicGraphStockBounds
	for i, form := range forms {
		r, err := m.boundDirectionBudget([3]*big.Rat{form[0], form[1], form[2]}, form[3], &b)
		if err != nil {
			return zero, err
		}
		out.Exact[i] = r
		var ends [2]geom.ExactParameterInterval
		for j, v := range []*geom.ExactAlgebraicRoot{r.Min.Value, r.Max.Value} {
			s, err := v.Enclose(ctx, width, b.limits())
			if err != nil {
				return zero, err
			}
			if err = b.charge(s.Operations, s.Subdivisions); err != nil {
				return zero, err
			}
			if !s.Converged || !s.HasBounds {
				return zero, geom.ErrConvergence
			}
			ends[j] = s.Interval
		}
		out.Bounds[i] = geom.ExactParameterInterval{Lower: ends[0].Lower, Upper: ends[1].Upper}
	}
	out.ExactOperations, out.Subdivisions = maxWork-b.work, maxSubdiv-b.subdiv
	return out, nil
}
