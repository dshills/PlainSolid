package geom

import (
	"context"
	"math/big"
)

// graphCubicResidualCellLaw compares every homogeneous numerator after
// moving its local polynomial to the original domain's normalized chart.
// A positive constant ratio preserves the zeros and signed regions. The
// original SurfacePatch invariant supplies continuity across the knots.
func graphCubicResidualCellLaw(ctx context.Context, residual *SurfacePatch, axis, maxCells int, b *polynomialBudget) ([3]rationalPolynomial, error) {
	var zero [3]rationalPolynomial
	cells, err := graphContactBezierCells(ctx, residual, maxCells, b)
	if err != nil {
		return zero, err
	}
	if len(cells) == 0 {
		return zero, ErrConvergence
	}
	domain := residual.Domain()
	var law [3]rationalPolynomial
	for i, cell := range cells {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		local, err := graphBivariateCoefficients(cell, axis, b)
		if err != nil {
			return zero, err
		}
		whole, err := graphCubicCellToDomain(local, cell.Domain(), domain, axis, b)
		if err != nil {
			return zero, err
		}
		if len(whole[2]) != 1 || whole[2][0].Sign() == 0 {
			return zero, ErrConvergence
		}
		if i == 0 {
			law = whole
			continue
		}
		ratio := new(big.Rat).Quo(whole[2][0], law[2][0])
		if err := b.check(ratio); err != nil {
			return zero, err
		}
		if ratio.Sign() <= 0 {
			return zero, ErrConvergence
		}
		for column, want := range law {
			if len(whole[column]) != len(want) {
				return zero, ErrConvergence
			}
			for k, v := range want {
				scaled := new(big.Rat).Mul(v, ratio)
				if err := b.check(scaled); err != nil {
					return zero, err
				}
				if scaled.Cmp(whole[column][k]) != 0 {
					return zero, ErrConvergence
				}
			}
		}
	}
	return law, nil
}

func graphCubicCellToDomain(local [3]rationalPolynomial, cell, domain ParameterRectangle, axis int, b *polynomialBudget) (out [3]rationalPolynomial, err error) {
	affine := func(orientation int) (rationalPolynomial, error) {
		clo, chi, dlo, dhi := cell.UMin, cell.UMax, domain.UMin, domain.UMax
		if orientation == 1 {
			clo, chi, dlo, dhi = cell.VMin, cell.VMax, domain.VMin, domain.VMax
		}
		exact := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
		width := new(big.Rat).Sub(exact(chi), exact(clo))
		if width.Sign() <= 0 {
			return nil, ErrConvergence
		}
		station := new(big.Rat).Quo(new(big.Rat).Sub(exact(dlo), exact(clo)), width)
		scale := new(big.Rat).Quo(new(big.Rat).Sub(exact(dhi), exact(dlo)), width)
		for _, q := range []*big.Rat{station, scale} {
			if err := b.check(q); err != nil {
				return nil, err
			}
		}
		return rationalPolynomial{station, scale}, nil
	}
	u, err := affine(axis)
	if err != nil {
		return out, err
	}
	v, err := affine(1 - axis)
	if err != nil {
		return out, err
	}
	v2, err := polynomialProduct(v, v, b)
	if err != nil {
		return out, err
	}
	vPowers := [3]rationalPolynomial{{big.NewRat(1, 1)}, v, v2}
	for j := range out {
		out[j] = make(rationalPolynomial, 7)
		for k := range out[j] {
			out[j][k] = new(big.Rat)
		}
	}
	for j, column := range local {
		if len(column) == 0 {
			continue
		}
		wholeU, err := composeProjectionPolynomial(column, u, b)
		if err != nil {
			return out, err
		}
		for degreeV, vcoef := range vPowers[j] {
			for degreeU, ucoef := range wholeU {
				term := new(big.Rat).Mul(ucoef, vcoef)
				if err := b.check(term); err != nil {
					return out, err
				}
				sum := out[degreeV][degreeU].Add(out[degreeV][degreeU], term)
				if err := b.check(sum); err != nil {
					return out, err
				}
			}
		}
	}
	for j := range out {
		out[j] = trimPolynomial(out[j])
	}
	return out, nil
}
