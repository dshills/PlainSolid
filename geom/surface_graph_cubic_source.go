package geom

import (
	"context"
	"math/big"
)

type graphCubicProjection struct {
	powers       [3]rationalPolynomial
	alignment    [2][3]*big.Rat
	residualAxis int
	budget       *polynomialBudget
}

// Both cubic section families start from the same bounded homogeneous
// difference of the exactly aligned original charts.
func projectGraphCubicSource(ctx context.Context, a, c *SurfacePatch, axes [2]int, freeAxis int,
	o GraphPointContactOptions) (r graphCubicProjection, err error) {
	if ctx == nil || a == nil || c == nil || freeAxis < 0 || freeAxis > 1 {
		return r, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	o, err = alignedPolynomialProofOptions(o)
	if err != nil {
		return r, err
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	order, err := modelProjectionPermutation(axes)
	if err != nil {
		return r, err
	}
	pa, err := permuteGraphPatch(a, order, b)
	if err != nil {
		return r, err
	}
	pc, err := permuteGraphPatch(c, order, b)
	if err != nil {
		return r, err
	}
	remaining := b.maxOperations - b.operations
	if remaining < 1 {
		return r, ErrConvergence
	}
	alignment, work, err := AlignAffineXYChartsWithWork(ctx, pa, pc, SurfacePatchOptions{MaxOperations: remaining, MaxCoefficientBits: b.maxBits})
	b.operations += work
	if err != nil {
		return r, err
	}
	residual, err := orderingDifference(ctx, pa, alignment.TargetOnSource(), o.MaxCells, b)
	if err != nil {
		return r, err
	}
	if residual.countU != residual.degreeU+1 || residual.countV != residual.degreeV+1 ||
		residual.degreeU > 6 || residual.degreeV > 6 {
		return r, ErrConvergence
	}
	r.powers, err = graphBivariateCoefficients(residual, freeAxis, b)
	if err != nil {
		return r, err
	}
	r.alignment, r.residualAxis, r.budget = alignment.mapping, order[2], b
	return r, nil
}

type graphCubicCoefficients struct {
	monic           rationalPolynomial
	dependentCenter *big.Rat
	signedScale     *big.Rat
	side            int
}

func recoverGraphCubicCoefficients(p [3]rationalPolynomial, b *polynomialBudget) (r graphCubicCoefficients, err error) {
	if len(p[0]) != 4 || len(p[1]) > 1 || len(p[2]) != 1 {
		return r, ErrConvergence
	}
	// The dependent coefficients prove a constant quadratic of the form
	// lambda*(v-s)². Remove its constant term before solving the free cubic.
	lambda, lead := p[2][0], p[0][3]
	s := new(big.Rat)
	if len(p[1]) == 1 {
		s.Quo(p[1][0], new(big.Rat).Mul(big.NewRat(-2, 1), lambda))
	}
	constant := new(big.Rat).Sub(p[0][0], new(big.Rat).Mul(lambda, new(big.Rat).Mul(s, s)))
	constant.Quo(constant, lead)
	linear := new(big.Rat).Quo(p[0][1], lead)
	quadratic := new(big.Rat).Quo(p[0][2], lead)
	monic := rationalPolynomial{constant, linear, quadratic, big.NewRat(1, 1)}
	for _, q := range append(monic, s) {
		if err = b.check(q); err != nil {
			return r, err
		}
	}
	signedScale := new(big.Rat).Quo(new(big.Rat).Neg(lead), lambda)
	return graphCubicCoefficients{monic: monic, dependentCenter: s, signedScale: signedScale, side: lambda.Sign()}, nil
}

func graphCubicChartMaps(d ParameterRectangle, free, direction int, center [2]*big.Rat, alignment [2][3]*big.Rat,
	b *polynomialBudget) (out [2][2][3]*big.Rat, err error) {
	limits := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	for i, axis := range [2]int{free, 1 - free} {
		lo, hi := new(big.Rat).SetFloat64(limits[axis][0]), new(big.Rat).SetFloat64(limits[axis][1])
		width := new(big.Rat).Sub(hi, lo)
		row := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat).Add(lo, new(big.Rat).Mul(width, center[i]))}
		row[i].Set(width)
		if i == 0 && direction < 0 {
			row[i].Neg(row[i])
		}
		for _, q := range []*big.Rat{row[i], row[2]} {
			if err := b.check(q); err != nil {
				return out, err
			}
		}
		out[0][axis] = row
	}
	for axis := range alignment {
		for i := range out[1][axis] {
			x := new(big.Rat).Add(new(big.Rat).Mul(alignment[axis][0], out[0][0][i]), new(big.Rat).Mul(alignment[axis][1], out[0][1][i]))
			if i == 2 {
				x.Add(x, alignment[axis][2])
			}
			if err = b.check(x); err != nil {
				return out, err
			}
			out[1][axis][i] = x
		}
	}
	return out, nil
}

func applyGraphCubicChartMaps(m [2][2][3]*big.Rat, xy [2]*big.Rat, vector bool, b *polynomialBudget) (out [2][2]*big.Rat, err error) {
	for owner := range m {
		for axis, row := range m[owner] {
			x := new(big.Rat).Add(new(big.Rat).Mul(row[0], xy[0]), new(big.Rat).Mul(row[1], xy[1]))
			if !vector {
				x.Add(x, row[2])
			}
			if err = b.check(x); err != nil {
				return out, err
			}
			out[owner][axis] = x
		}
	}
	return out, nil
}

func graphCubicBoundary(d ParameterRectangle, uv [2]*big.Rat) (mask uint8) {
	for i, edge := range [4]float64{d.UMin, d.UMax, d.VMin, d.VMax} {
		if uv[i/2].Cmp(new(big.Rat).SetFloat64(edge)) == 0 {
			mask |= 1 << uint(i)
		}
	}
	return mask
}

func graphCubicOriginalJets(ctx context.Context, patches [2]*SurfacePatch, uv [2][2]*big.Rat,
	b *polynomialBudget) (jets [2]RationalSurfacePatchJet, err error) {
	for owner, source := range patches {
		remaining := b.maxOperations - b.operations
		if remaining < 1 {
			return jets, ErrConvergence
		}
		jets[owner], err = source.EvaluateRational(ctx, uv[owner][0], uv[owner][1],
			SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: b.maxBits})
		b.operations += jets[owner].ExactOperations
		if err != nil {
			return jets, err
		}
	}
	return jets, nil
}

func bindGraphCubicModelVector(uv [2][2]*big.Rat, jets [2]RationalSurfacePatchJet,
	b *polynomialBudget) (out [3]*big.Rat, err error) {
	for axis := range out {
		var model [2]*big.Rat
		for owner, jet := range jets {
			model[owner] = new(big.Rat).Add(new(big.Rat).Mul(uv[owner][0], jet.Du[axis]), new(big.Rat).Mul(uv[owner][1], jet.Dv[axis]))
			if err = b.check(model[owner]); err != nil {
				return out, err
			}
		}
		if model[0].Cmp(model[1]) != 0 {
			return out, ErrConvergence
		}
		out[axis] = model[0]
	}
	return out, nil
}
