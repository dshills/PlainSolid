package geom

import (
	"context"
	"math/big"
)

// unitParabolaCircleFactors binds the complete original numerator to a
// parabola and one circle on the shared identity unit charts.
func unitParabolaCircleFactors(ctx context.Context, a, c *SurfacePatch, freeAxis int, b *polynomialBudget) (
	rationalPolynomial, [2]*big.Rat, *big.Rat, int, error) {
	if err := validateUnitGraphCircleCharts(ctx, a, c, 6, b); err != nil {
		return nil, [2]*big.Rat{}, nil, 0, err
	}
	unit := ParameterRectangle{UMax: 1, VMax: 1}
	diff, err := orderingDifference(ctx, a, c, 1, b)
	if err != nil {
		return nil, [2]*big.Rat{}, nil, 0, err
	}
	if diff.Domain() != unit || diff.countU != diff.degreeU+1 || diff.countV != diff.degreeV+1 ||
		diff.degreeU > 6 || diff.degreeV > 6 {
		return nil, [2]*big.Rat{}, nil, 0, ErrConvergence
	}
	columns, err := parabolaCirclePowerColumns(diff, freeAxis, b)
	if err != nil {
		return nil, [2]*big.Rat{}, nil, 0, err
	}
	return factorParabolaCircle(columns, b)
}

// validateUnitGraphCircleCharts binds both original single cells to the
// same unmodified physical unit XY chart before a contact is lifted.
func validateUnitGraphCircleCharts(ctx context.Context, a, c *SurfacePatch, maxDegree int, b *polynomialBudget) error {
	unit := ParameterRectangle{UMax: 1, VMax: 1}
	for _, patch := range [2]*SurfacePatch{a, c} {
		if patch.Domain() != unit || patch.countU != patch.degreeU+1 || patch.countV != patch.degreeV+1 ||
			patch.degreeU > maxDegree || patch.degreeV > maxDegree {
			return ErrConvergence
		}
		remaining := b.maxOperations - b.operations
		if remaining < 1 {
			return ErrConvergence
		}
		graph, work, err := newAffineXYGraphWithWork(ctx, patch, SurfacePatchOptions{MaxOperations: remaining, MaxCoefficientBits: b.maxBits})
		b.operations += work
		if err != nil {
			return err
		}
		for row, diagonal := range [2][3]int64{{1, 0, 0}, {0, 1, 0}} {
			for col, want := range diagonal {
				if graph.inverse[row][col].Cmp(big.NewRat(want, 1)) != 0 {
					return ErrConvergence
				}
			}
		}
	}
	return nil
}
