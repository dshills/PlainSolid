package geom

import (
	"context"
	"math/big"
	"sort"
)

// IntersectWrappedAnisotropicBandLineRational isolates all original lateral
// contacts when both finite endpoint heights lie strictly inside this band.
// Positive affine scales permit exact denominator clearing to quartics. Every
// root is filtered by exact quadratic sector signs, never rounded coordinates.
// A single operation ledger covers source, construction, isolation and signs.
// At most four quartics and sixteen roots are processed. Subdivision quotas
// apply independently to each quartic and each of its three root-sign proofs.
// Identically zero quartics and unresolved distinct-root ordering fail closed.
func IntersectWrappedAnisotropicBandLineRational(ctx context.Context, w *WrappedSurfacePatch, origin, direction [3]*big.Rat, minT, maxT *big.Rat, o ToroidLineOptions) (CylinderLineIntersections, error) {
	return intersectWrappedAnisotropicBandLine(ctx, w, origin, direction, minT, maxT, o, false, nil)
}

// WrappedAnisotropicBandLineReport supplies charged work for collection queries.
// Only original lateral contacts are reported; endpoint-plane disks are not caps.
type WrappedAnisotropicBandLineReport struct {
	Intersections   CylinderLineIntersections
	ExactOperations int
}

// IntersectWrappedAnisotropicClosedBandLineRational admits endpoint heights on
// either station plane, retaining exact lateral contacts there. It does not
// classify the station disk or assert that it is exposed in a surrounding loft.
func IntersectWrappedAnisotropicClosedBandLineRational(ctx context.Context, w *WrappedSurfacePatch, origin, direction [3]*big.Rat, minT, maxT *big.Rat, o ToroidLineOptions) (WrappedAnisotropicBandLineReport, error) {
	var work int
	q, e := intersectWrappedAnisotropicBandLine(ctx, w, origin, direction, minT, maxT, o, true, &work)
	if e != nil {
		return WrappedAnisotropicBandLineReport{}, e
	}
	return WrappedAnisotropicBandLineReport{q, work}, nil
}

func intersectWrappedAnisotropicBandLine(ctx context.Context, w *WrappedSurfacePatch, origin, direction [3]*big.Rat, minT, maxT *big.Rat, o ToroidLineOptions, closed bool, charged *int) (CylinderLineIntersections, error) {
	empty := CylinderLineIntersections{}
	options, e := validateRationalCircularLine(ctx, origin[:], direction[:], minT, maxT, o)
	if e != nil {
		return empty, e
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 1024
	}
	if o.MaxSubdivisions < 1 || o.MaxSubdivisions > 65536 {
		return empty, ErrInvalid
	}
	p, e := CertifyWrappedAnisotropicEmbedding(ctx, w, options)
	if e != nil {
		return empty, e
	}
	b := &wrappedAnisotropicBudget{ctx: ctx, work: polynomialBudget{operations: p.ExactOperations, maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}}
	if charged != nil {
		defer func() { *charged = b.work.operations }()
	}
	for _, t := range []*big.Rat{minT, maxT} {
		z := b.add(origin[2], b.mul(direction[2], t))
		if b.err != nil {
			return empty, b.err
		}
		if z.Cmp(p.LowerZ) < 0 || z.Cmp(p.UpperZ) > 0 || (!closed && (z.Cmp(p.LowerZ) == 0 || z.Cmp(p.UpperZ) == 0)) {
			return empty, ErrInvalid
		}
	}
	var scales [2]rationalPolynomial
	for a := 0; a < 2; a++ {
		slope := b.div(b.sub(p.UpperScale[a], p.LowerScale[a]), b.sub(p.UpperZ, p.LowerZ))
		scales[a] = rationalPolynomial{b.add(p.LowerScale[a], b.mul(slope, b.sub(origin[2], p.LowerZ))), b.mul(slope, direction[2])}
	}
	if b.err != nil {
		return empty, b.err
	}
	// Cardinal contacts have an original affine coordinate equation even
	// when their rational parameter is not dyadic. Retain exact candidates
	// for subsequent polynomial identity checks, never interval-only welding.
	var cardinal []*big.Rat
	for axis := 0; axis < 2; axis++ {
		for _, sign := range []int64{-1, 1} {
			a := b.sub(b.sub(origin[axis], p.Center[axis]), b.mul(big.NewRat(sign, 1), scales[axis][0]))
			c := b.sub(direction[axis], b.mul(big.NewRat(sign, 1), scales[axis][1]))
			if c.Sign() != 0 {
				cardinal = append(cardinal, b.div(b.sub(new(big.Rat), a), c))
			}
		}
	}
	if b.err != nil {
		return empty, b.err
	}
	product := func(a, c rationalPolynomial) rationalPolynomial {
		if b.err != nil {
			return nil
		}
		v, e := polynomialProduct(a, c, &b.work)
		if e != nil {
			b.err = e
		}
		return v
	}
	add := func(a, c rationalPolynomial, sign int64) rationalPolynomial {
		v := make(rationalPolynomial, max(len(a), len(c)))
		for i := range v {
			v[i] = new(big.Rat)
			if i < len(a) {
				v[i] = b.add(v[i], a[i])
			}
			if i < len(c) {
				v[i] = b.add(v[i], b.mul(big.NewRat(sign, 1), c[i]))
			}
		}
		return v
	}
	neg := func(a rationalPolynomial) rationalPolynomial {
		v := make(rationalPolynomial, len(a))
		for i := range a {
			v[i] = b.sub(new(big.Rat), a[i])
		}
		return v
	}
	x := product(rationalPolynomial{b.sub(origin[0], p.Center[0]), direction[0]}, scales[1])
	y := product(rationalPolynomial{b.sub(origin[1], p.Center[1]), direction[1]}, scales[0])
	r := product(scales[0], scales[1])
	quarters := [4][2]rationalPolynomial{{x, y}, {y, neg(x)}, {neg(x), neg(y)}, {neg(y), x}}
	if b.err != nil {
		return empty, b.err
	}
	result := CylinderLineIntersections{}
	tol := new(big.Rat).SetFloat64(o.ParameterTolerance)
	for j, uv := range quarters {
		guards := []rationalPolynomial{add(r, uv[0], -1), add(r, uv[1], -1), add(add(uv[0], uv[1], 1), r, -1)}
		left, right := product(guards[2], guards[2]), product(guards[0], guards[1])
		ls := b.mul(w.patch.net[4*j][3], w.patch.net[4*j+4][3])
		rs := b.mul(big.NewRat(4, 1), b.mul(w.patch.net[4*j+2][3], w.patch.net[4*j+2][3]))
		poly := make(rationalPolynomial, max(len(left), len(right)))
		for k := range poly {
			poly[k] = new(big.Rat)
			if k < len(left) {
				poly[k] = b.mul(ls, left[k])
			}
			if k < len(right) {
				poly[k] = b.sub(poly[k], b.mul(rs, right[k]))
			}
		}
		if b.err != nil {
			return empty, b.err
		}
		poly = trimPolynomial(poly)
		if len(poly) == 0 {
			return empty, ErrDomain
		}
		_, e = isolatePolynomialRoots(poly, minT, maxT, &b.work, o.MaxSubdivisions, func(lo, hi *big.Rat, m int) (bool, error) {
			if e := ctx.Err(); e != nil {
				return false, e
			}
			if new(big.Rat).Sub(hi, lo).Cmp(tol) > 0 {
				return false, nil
			}
			if lo.Cmp(hi) != 0 {
				for _, t := range []*big.Rat{lo, hi} {
					v, e := polynomialValue(poly, t, &b.work)
					if e != nil {
						return false, e
					}
					if v.Sign() == 0 {
						return false, nil
					}
				}
			}
			root := &ExactGraphLineRoot{polynomial: poly, bounds: [2]*big.Rat{lo, hi}}
			for _, g := range guards {
				sign, _, e := composedPolynomialRootSign(ctx, root, trimPolynomial(g), &b.work, o.MaxSubdivisions)
				if e != nil {
					return false, e
				}
				if sign < 0 {
					return true, nil
				}
			}
			for _, t := range cardinal {
				if t.Cmp(lo) < 0 || t.Cmp(hi) > 0 {
					continue
				}
				v, e := polynomialValue(poly, t, &b.work)
				if e != nil {
					return false, e
				}
				if v.Sign() == 0 {
					lo, hi = t, t
					break
				}
			}
			if len(result.Hits) >= 16 {
				return false, ErrConvergence
			}
			result.Hits = append(result.Hits, CylinderLineHit{Parameter: ToroidLineRoot{Min: new(big.Rat).Set(lo), Max: new(big.Rat).Set(hi), Multiplicity: m}, Parts: CylinderLateral})
			return true, nil
		})
		if e != nil {
			return empty, e
		}
	}
	sort.Slice(result.Hits, func(i, j int) bool { return result.Hits[i].Parameter.Min.Cmp(result.Hits[j].Parameter.Min) < 0 })
	out := result.Hits[:0]
	for _, h := range result.Hits {
		if len(out) > 0 {
			last := &out[len(out)-1]
			a, c := last.Parameter, h.Parameter
			if a.Max.Cmp(c.Min) >= 0 {
				if a.Min.Cmp(a.Max) != 0 || c.Min.Cmp(c.Max) != 0 || a.Min.Cmp(c.Min) != 0 {
					return empty, ErrConvergence
				}
				last.Parameter.Multiplicity = max(a.Multiplicity, c.Multiplicity)
				continue
			}
		}
		out = append(out, h)
	}
	result.Hits = out
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	return result, nil
}
