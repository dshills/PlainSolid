package geom

import (
	"context"
	"math/big"
)

func bindCuspidalCubicOwners(ctx context.Context, sources [2]*SurfacePatch, out *CuspidalCubicSection,
	residualAxis, side int, near, far, exit *big.Rat, b *polynomialBudget) error {
	zero := new(big.Rat)
	height := new(big.Rat).Mul(far, exit)
	canonical := [3][2]*big.Rat{
		{zero, zero}, {far, new(big.Rat).Neg(height)}, {far, height},
	}
	var jets [3][2]RationalSurfacePatchJet
	for i, xy := range canonical {
		v := &out.Vertices[i]
		var err error
		v.UV, err = applyGraphCubicChartMaps(out.ChartMaps, xy, false, b)
		if err != nil {
			return err
		}
		jets[i], err = graphCubicVertexJets(ctx, sources, v.UV, b)
		if err != nil {
			return err
		}
		for axis, value := range jets[i][0].Point {
			if value.Cmp(jets[i][1].Point[axis]) != 0 {
				return ErrConvergence
			}
		}
		v.Point = jets[i][0].Point
		for owner, d := range out.Domains {
			mask := graphCubicBoundary(d, v.UV[owner])
			if (i == 0 && mask != 0) || (i > 0 && (mask == 0 || mask&(mask-1) != 0)) {
				return ErrConvergence
			}
			v.Boundary[owner] = mask
		}
	}
	if out.Vertices[1].Boundary != out.Vertices[2].Boundary {
		return ErrConvergence
	}

	// The canonical negative region lies between the two ordinates on the open
	// longitudinal interval. The two exterior sides join through x<0.
	for i, x := range [2]*big.Rat{new(big.Rat).Mul(near, big.NewRat(-1, 2)), new(big.Rat).Mul(far, big.NewRat(1, 2))} {
		r := &out.Regions[i]
		var err error
		r.UV, err = applyGraphCubicChartMaps(out.ChartMaps, [2]*big.Rat{x, zero}, false, b)
		if err != nil {
			return err
		}
		j, err := graphCubicOriginalJets(ctx, sources, r.UV, b)
		if err != nil {
			return err
		}
		r.HeightSide = j[0].Point[residualAxis].Cmp(j[1].Point[residualAxis])
		if r.HeightSide != ([2]int{side, -side})[i] {
			return ErrConvergence
		}
	}

	out.Germs[0].Range = [2]*big.Rat{new(big.Rat).Neg(exit), new(big.Rat)}
	out.Germs[1].Range = [2]*big.Rat{new(big.Rat), new(big.Rat).Set(exit)}
	for i, sign := range [2]int64{-1, 1} {
		if err := bindCuspidalCubicGerm(out, &out.Germs[i], jets[0], jets[i+1], out.Vertices[i+1].Boundary, exit, sign, b); err != nil {
			return err
		}
	}
	x, y := out.ProjectionAxes[0], out.ProjectionAxes[1]
	second, third := out.Germs[1].ModelSecond, out.Germs[1].ModelThird
	det := new(big.Rat).Sub(new(big.Rat).Mul(second[x], third[y]), new(big.Rat).Mul(second[y], third[x]))
	if err := b.check(det); err != nil {
		return err
	}
	if det.Sign() == 0 {
		return ErrDegenerate
	}
	out.ProjectionOrientation = det.Sign()
	return nil
}

func bindCuspidalCubicGerm(section *CuspidalCubicSection, germ *CuspidalCubicGerm, cusp, terminal [2]RationalSurfacePatchJet,
	boundary [2]uint8, exit *big.Rat, side int64, b *polynomialBudget) error {
	// UV'(0)=0, so the second and third model derivatives are exactly
	// DS·UV'' and DS·UV''' on each original differentiable support.
	zero := new(big.Rat)
	second := [2]*big.Rat{new(big.Rat).Quo(big.NewRat(2, 1), section.Scale), zero}
	third := [2]*big.Rat{zero, new(big.Rat).Quo(big.NewRat(6*side, 1), section.Scale)}
	var err error
	germ.UVSecond, err = applyGraphCubicChartMaps(section.ChartMaps, second, true, b)
	if err != nil {
		return err
	}
	germ.UVThird, err = applyGraphCubicChartMaps(section.ChartMaps, third, true, b)
	if err != nil {
		return err
	}
	germ.ModelSecond, err = bindGraphCubicModelVector(germ.UVSecond, cusp, b)
	if err != nil {
		return err
	}
	germ.ModelThird, err = bindGraphCubicModelVector(germ.UVThird, cusp, b)
	if err != nil {
		return err
	}
	first := [2]*big.Rat{
		new(big.Rat).Mul(second[0], exit),
		new(big.Rat).Mul(new(big.Rat).Mul(exit, exit), new(big.Rat).Quo(big.NewRat(3*side, 1), section.Scale)),
	}
	germ.TerminalUV, err = applyGraphCubicChartMaps(section.ChartMaps, first, true, b)
	if err != nil {
		return err
	}
	for owner, mask := range boundary {
		for i, direction := range [4]int{-1, 1, -1, 1} {
			if mask == 1<<uint(i) && germ.TerminalUV[owner][i/2].Sign() != direction {
				return ErrConvergence
			}
		}
	}
	germ.TerminalModel, err = bindGraphCubicModelVector(germ.TerminalUV, terminal, b)
	return err
}
