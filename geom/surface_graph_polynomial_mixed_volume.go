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
			volumes, e := integratePolynomialPositiveLayers(ctx, arrangement.Power, arrangement.Regions[:], arrangement.ContactOrder[:], roots, domain, pair.heightScale, b)
			result.Regions, result.Volumes, result.Total = volumes.regions, volumes.volumes, volumes.total
			return e
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

type polynomialPositiveVolumes struct {
	regions []int
	volumes []*big.Rat
	total   *big.Rat
}

func integratePolynomialPositiveLayers(ctx context.Context, powers [2][]*big.Rat, regions []SplineGraphPolynomialRegion, orders []int, roots []*big.Rat,
	domain ParameterRectangle, scale *big.Rat, b *polynomialBudget) (polynomialPositiveVolumes, error) {
	zero := polynomialPositiveVolumes{}
	if scale == nil || scale.Sign() == 0 || len(regions) != len(roots)+3 || len(orders) != len(roots) {
		return zero, ErrConvergence
	}
	stations := make([]*big.Rat, 0, len(roots)+2)
	stations = append(stations, new(big.Rat).SetFloat64(domain.UMin))
	stations = append(stations, roots...)
	stations = append(stations, new(big.Rat).SetFloat64(domain.UMax))
	limits := [2]*big.Rat{new(big.Rat).SetFloat64(domain.VMin), new(big.Rat).SetFloat64(domain.VMax)}
	factor := new(big.Rat).Quo(new(big.Rat).Abs(scale), big.NewRat(6, 1))
	if e := b.check(factor); e != nil {
		return zero, e
	}
	out := polynomialPositiveVolumes{total: new(big.Rat)}
	exterior := scale.Sign() > 0
	if exterior {
		out.regions = []int{0, len(regions) - 1}
		out.volumes = []*big.Rat{new(big.Rat), new(big.Rat)}
	}
	lower := 0
	for slab := 0; slab < len(stations)-1; slab++ {
		if e := ctx.Err(); e != nil {
			return zero, e
		}
		lo, hi := stations[slab], stations[slab+1]
		f, g := powers[lower], powers[1-lower]
		if exterior {
			for side, region := range out.regions {
				if regions[region].HeightSide != 1 {
					return zero, ErrConvergence
				}
				l, h := rationalPolynomial(f), rationalPolynomial(g)
				if side == 1 {
					l, h = h, l
				}
				poly, e := graphPolynomialExteriorFiber(l, h, limits[side], side == 1, b)
				if e != nil {
					return zero, e
				}
				volume, e := integrateGraphPolynomialFiber(poly, lo, hi, factor, b)
				if e != nil {
					return zero, e
				}
				if volume.Sign() <= 0 {
					return zero, ErrConvergence
				}
				out.volumes[side].Add(out.volumes[side], volume)
				if e = b.check(out.volumes[side]); e != nil {
					return zero, e
				}
			}
		} else {
			if regions[slab+1].HeightSide != 1 {
				return zero, ErrConvergence
			}
			gap, e := graphPolynomialSum(g, f, -1, b)
			if e != nil {
				return zero, e
			}
			square, e := polynomialProduct(gap, gap, b)
			if e != nil {
				return zero, e
			}
			cube, e := polynomialProduct(square, gap, b)
			if e != nil {
				return zero, e
			}
			volume, e := integrateGraphPolynomialFiber(cube, lo, hi, factor, b)
			if e != nil {
				return zero, e
			}
			if volume.Sign() <= 0 {
				return zero, ErrConvergence
			}
			out.regions = append(out.regions, slab+1)
			out.volumes = append(out.volumes, volume)
		}
		if slab < len(orders) && orders[slab] == 1 {
			lower = 1 - lower
		}
	}
	for _, volume := range out.volumes {
		out.total.Add(out.total, volume)
		if e := b.check(out.total); e != nil {
			return zero, e
		}
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
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
