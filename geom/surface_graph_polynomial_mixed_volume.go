package geom

import (
	"context"
	"math/big"
)

// SplineGraphPolynomialMixedVolumes gives the exact positive volumes of the
// original A-minus-B graph layers. Regions and Volumes are in arrangement
// order. ExactOperations includes arrangement reproof and rational integration.
type SplineGraphPolynomialMixedVolumes struct {
	Arrangement     SplineGraphPolynomialMixedContact
	Regions         []int
	Volumes         []*big.Rat
	Total           *big.Rat
	ExactOperations int
}

// IntegrateSplineGraphPolynomialMixedVolumes re-proves the original mixed
// crossing and tangency on identity XY charts. Every original union-knot cell
// must have a positive constant height-difference denominator and the same
// physical coefficient of the dependent quadratic. The exact region integrals
// cover the two exterior layers or the three between-branch layers. A failure
// returns an empty report.
func IntegrateSplineGraphPolynomialMixedVolumes(ctx context.Context, a, b *SurfacePatch, axis int, o GraphPointContactOptions) (SplineGraphPolynomialMixedVolumes, error) {
	var arrangement SplineGraphPolynomialMixedContact
	var result SplineGraphPolynomialMixedVolumes
	mode := polynomialGraphContactMode{
		crossings: 1, tangencies: 1, polynomialHeight: true,
		certify: func(ctx context.Context, a *SurfacePatch, pair SplineGraphTwoPolynomials, roots []*big.Rat, domain ParameterRectangle, b *polynomialBudget) error {
			if e := arrangement.certifyArrangement(ctx, a, pair, roots, domain, b); e != nil {
				return e
			}
			return integratePolynomialMixedPositiveLayers(ctx, &result, arrangement, roots, domain, pair.heightScale, b)
		},
	}
	pair, e := discoverSplineGraphTwoPolynomials(ctx, a, b, axis, o, mode)
	if e != nil {
		return SplineGraphPolynomialMixedVolumes{}, e
	}
	arrangement.Proven = true
	arrangement.ExactOperations = pair.ExactOperations
	result.Arrangement, result.ExactOperations = arrangement, pair.ExactOperations
	return result, nil
}

func graphPolynomialCellHeightScale(cell *SurfacePatch, numerator, dependentWidth *big.Rat, b *polynomialBudget) (*big.Rat, error) {
	weight := cell.net[0][3]
	for _, p := range cell.net {
		if e := b.check(p[3]); e != nil {
			return nil, e
		}
		if p[3].Cmp(weight) != 0 {
			return nil, ErrConvergence
		}
	}
	if weight.Sign() <= 0 {
		return nil, ErrConvergence
	}
	if e := proveIdentityGraphCell(cell, b); e != nil {
		return nil, e
	}
	denominator := new(big.Rat).Mul(weight, dependentWidth)
	denominator.Mul(denominator, dependentWidth)
	if e := b.check(denominator); e != nil {
		return nil, e
	}
	scale := new(big.Rat).Quo(numerator, denominator)
	return scale, b.check(scale)
}

func integratePolynomialMixedPositiveLayers(ctx context.Context, out *SplineGraphPolynomialMixedVolumes, arrangement SplineGraphPolynomialMixedContact, roots []*big.Rat,
	domain ParameterRectangle, scale *big.Rat, b *polynomialBudget) error {
	if scale == nil || scale.Sign() == 0 {
		return ErrConvergence
	}
	stations := [4]*big.Rat{new(big.Rat).SetFloat64(domain.UMin), roots[0], roots[1], new(big.Rat).SetFloat64(domain.UMax)}
	limits := [2]*big.Rat{new(big.Rat).SetFloat64(domain.VMin), new(big.Rat).SetFloat64(domain.VMax)}
	factor := new(big.Rat).Quo(new(big.Rat).Abs(scale), big.NewRat(6, 1))
	if e := b.check(factor); e != nil {
		return e
	}
	out.Total = new(big.Rat)
	lower := 0
	for slab := 0; slab < len(stations)-1; slab++ {
		if e := ctx.Err(); e != nil {
			return e
		}
		lo, hi := stations[slab], stations[slab+1]
		f, g := arrangement.Power[lower], arrangement.Power[1-lower]
		if scale.Sign() > 0 {
			if slab == 0 {
				out.Regions = []int{0, 4}
				out.Volumes = []*big.Rat{new(big.Rat), new(big.Rat)}
			}
			for side, region := range out.Regions {
				if arrangement.Regions[region].HeightSide != 1 {
					return ErrConvergence
				}
				l, h := rationalPolynomial(f), rationalPolynomial(g)
				if side == 1 {
					l, h = h, l
				}
				poly, e := graphPolynomialExteriorFiber(l, h, limits[side], side == 1, b)
				if e != nil {
					return e
				}
				volume, e := integrateGraphPolynomialFiber(poly, lo, hi, factor, b)
				if e != nil {
					return e
				}
				if volume.Sign() <= 0 {
					return ErrConvergence
				}
				out.Volumes[side].Add(out.Volumes[side], volume)
				if e = b.check(out.Volumes[side]); e != nil {
					return e
				}
			}
		} else {
			if arrangement.Regions[slab+1].HeightSide != 1 {
				return ErrConvergence
			}
			gap, e := graphPolynomialSum(g, f, -1, b)
			if e != nil {
				return e
			}
			square, e := polynomialProduct(gap, gap, b)
			if e != nil {
				return e
			}
			cube, e := polynomialProduct(square, gap, b)
			if e != nil {
				return e
			}
			volume, e := integrateGraphPolynomialFiber(cube, lo, hi, factor, b)
			if e != nil {
				return e
			}
			if volume.Sign() <= 0 {
				return ErrConvergence
			}
			out.Regions = append(out.Regions, slab+1)
			out.Volumes = append(out.Volumes, volume)
		}
		if slab < len(arrangement.ContactOrder) && arrangement.ContactOrder[slab] == 1 {
			lower = 1 - lower
		}
	}
	for _, volume := range out.Volumes {
		out.Total.Add(out.Total, volume)
		if e := b.check(out.Total); e != nil {
			return e
		}
	}
	return ctx.Err()
}

// graphPolynomialExteriorFiber computes l²(3h-l), with l and h the distances
// to the near and far branch. The caller multiplies the integral by |C|/6.
func graphPolynomialExteriorFiber(near, far rationalPolynomial, boundary *big.Rat, upper bool, b *polynomialBudget) (rationalPolynomial, error) {
	base := rationalPolynomial{boundary}
	distance := func(branch rationalPolynomial) (rationalPolynomial, error) {
		if upper {
			return graphPolynomialSum(base, branch, -1, b)
		}
		return graphPolynomialSum(branch, base, -1, b)
	}
	l, e := distance(near)
	if e != nil {
		return nil, e
	}
	h, e := distance(far)
	if e != nil {
		return nil, e
	}
	for _, x := range h {
		x.Mul(x, big.NewRat(3, 1))
		if e = b.check(x); e != nil {
			return nil, e
		}
	}
	h, e = graphPolynomialSum(h, l, -1, b)
	if e != nil {
		return nil, e
	}
	square, e := polynomialProduct(l, l, b)
	if e != nil {
		return nil, e
	}
	return polynomialProduct(square, h, b)
}

func integrateGraphPolynomialFiber(poly rationalPolynomial, lo, hi, factor *big.Rat, b *polynomialBudget) (*big.Rat, error) {
	primitive := make(rationalPolynomial, len(poly)+1)
	primitive[0] = new(big.Rat)
	for i, c := range poly {
		primitive[i+1] = new(big.Rat).Quo(c, big.NewRat(int64(i+1), 1))
		if e := b.check(primitive[i+1]); e != nil {
			return nil, e
		}
	}
	a, e := polynomialValue(primitive, lo, b)
	if e != nil {
		return nil, e
	}
	z, e := polynomialValue(primitive, hi, b)
	if e != nil {
		return nil, e
	}
	z.Sub(z, a)
	z.Mul(z, factor)
	return z, b.check(z)
}
