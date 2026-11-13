package geom

import (
	"context"
	"math/big"
)

// GraphCornerContact is immutable evidence for exactly one common model point
// over both complete original chart domains. It is not a shell assembly or a
// transverse-branch certificate. Accessors return owned rational values.
type GraphCornerContact struct {
	source, target   *SurfacePatch
	uv               [2]*big.Rat
	point            [3]*big.Rat
	sign, operations int
}

func (c *GraphCornerContact) Domains() [2]ParameterRectangle {
	return [2]ParameterRectangle{c.source.Domain(), c.target.Domain()}
}
func (c *GraphCornerContact) Parameters() [2][2]*big.Rat {
	var r [2][2]*big.Rat
	for i := range r {
		for j := range r[i] {
			r[i][j] = new(big.Rat).Set(c.uv[j])
		}
	}
	return r
}
func (c *GraphCornerContact) Point() [3]*big.Rat {
	var r [3]*big.Rat
	for i := range r {
		r[i] = new(big.Rat).Set(c.point[i])
	}
	return r
}

// HeightSign is the strict sign of source Z minus target Z at every other
// projected point of these domains.
func (c *GraphCornerContact) HeightSign() int      { return c.sign }
func (c *GraphCornerContact) ExactOperations() int { return c.operations }
func (c *GraphCornerContact) Binds(a, b *SurfacePatch) bool {
	return c != nil && c.source == a && c.target == b
}

// CertifyGraphCornerContact supports single-span positive-weight patches of
// degree at most six with identical domains and identical affine XY maps.
// All homogeneous height-difference Bernstein coefficients except one corner
// must have the same strict sign. This sufficient condition excludes every
// other contact, including contacts on the other edges. Differing parameter
// maps, additional zero coefficients and inconclusive signs fail closed.
func CertifyGraphCornerContact(ctx context.Context, a, b *SurfacePatch, o SurfacePatchOptions) (*GraphCornerContact, error) {
	if ctx == nil || a == nil || b == nil {
		return nil, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	opts, e := patchOptions(o)
	if e != nil {
		return nil, e
	}
	if a.Domain() != b.Domain() {
		return nil, ErrDomain
	}
	budget := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	for _, p := range []*SurfacePatch{a, b} {
		if len(p.net) > opts.MaxPoles || p.degreeU < 1 || p.degreeV < 1 || p.degreeU > 6 || p.degreeV > 6 || p.countU != p.degreeU+1 || p.countV != p.degreeV+1 {
			return nil, ErrConvergence
		}
		for _, h := range p.net {
			if h[3].Sign() <= 0 {
				return nil, ErrInvalid
			}
		}
		if _, _, e = normalizeAffineXYGraph(ctx, p, budget); e != nil {
			return nil, e
		}
	}
	var z []*big.Rat
	for axis := 0; axis < 3; axis++ {
		left, e := tensorHomogeneousBernsteinProduct(a, axis, b, 3, budget)
		if e != nil {
			return nil, e
		}
		right, e := tensorHomogeneousBernsteinProduct(a, 3, b, axis, budget)
		if e != nil {
			return nil, e
		}
		for i := range left {
			left[i].Sub(left[i], right[i])
			if e = budget.check(left[i]); e != nil {
				return nil, e
			}
			if axis < 2 && left[i].Sign() != 0 {
				return nil, ErrConvergence
			}
		}
		if axis == 2 {
			z = left
		}
	}
	nu, nv := a.degreeU+b.degreeU+1, a.degreeV+b.degreeV+1
	zero, sign := -1, 0
	for i, v := range z {
		if e = ctx.Err(); e != nil {
			return nil, e
		}
		if v.Sign() == 0 {
			if zero >= 0 {
				return nil, ErrConvergence
			}
			zero = i
		} else if sign == 0 {
			sign = v.Sign()
		} else if sign != v.Sign() {
			return nil, ErrConvergence
		}
	}
	if zero < 0 || sign == 0 {
		return nil, ErrConvergence
	}
	u, v := zero/nv, zero%nv
	if (u != 0 && u != nu-1) || (v != 0 && v != nv-1) {
		return nil, ErrConvergence
	}
	d := a.Domain()
	uv := [2]*big.Rat{new(big.Rat).SetFloat64(d.UMin), new(big.Rat).SetFloat64(d.VMin)}
	if u != 0 {
		uv[0].SetFloat64(d.UMax)
	}
	if v != 0 {
		uv[1].SetFloat64(d.VMax)
	}
	p, _, e := exactSurfaceJet(a, uv[0], uv[1], budget)
	if e != nil {
		return nil, e
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	return &GraphCornerContact{source: a, target: b, uv: uv, point: p, sign: sign, operations: budget.operations}, nil
}
