package geom

import (
	"context"
	"math/big"
)

// SurfacePairBoundaryPointOptions supplies exact common parameters and two
// fixed boundary coordinates (one per chart). Limits bound one proof.
type SurfacePairBoundaryPointOptions struct {
	Anchor                            [4]*big.Rat
	Axes                              [2]int
	MaxOperations, MaxCoefficientBits int
}

// CertifySurfacePairOnlyBoundaryPoint proves that the given exact contact is
// the sole common parameter tuple of two single-span rectangles. The anchor
// must lie on one chosen side of each patch. The possible null direction of
// every averaged Jacobian must point into one rectangle and out of the other.
// No separating tolerance or perturbation is used. Inconclusive tests return
// ErrConvergence and do not rule out other contacts.
func CertifySurfacePairOnlyBoundaryPoint(ctx context.Context, a, c *SurfacePatch, o SurfacePairBoundaryPointOptions) error {
	if ctx == nil || a == nil || c == nil || o.Axes[0] < 0 || o.Axes[0] > 1 || o.Axes[1] < 2 || o.Axes[1] > 3 {
		return ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	for _, p := range []*SurfacePatch{a, c} {
		if p.countU != p.degreeU+1 || p.countV != p.degreeV+1 || len(p.net) == 0 {
			return ErrDomain
		}
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	d := pairParameterRanges(SurfacePairParameterBox{a.Domain(), c.Domain()})
	var side [4]int
	for j, v := range o.Anchor {
		if v == nil {
			return ErrInvalid
		}
		if e := b.check(v); e != nil {
			return e
		}
		p, q := v.Cmp(new(big.Rat).SetFloat64(d[j][0])), v.Cmp(new(big.Rat).SetFloat64(d[j][1]))
		if p < 0 || q > 0 {
			return ErrDomain
		}
		if p == 0 {
			side[j] = 1
		} else if q == 0 {
			side[j] = -1
		}
	}
	x, y := o.Axes[0], o.Axes[1]
	if side[x] == 0 || side[y] == 0 {
		return ErrDomain
	}
	u, _, e := exactSurfaceJet(a, o.Anchor[0], o.Anchor[1], b)
	if e != nil {
		return e
	}
	v, _, e := exactSurfaceJet(c, o.Anchor[2], o.Anchor[3], b)
	if e != nil {
		return e
	}
	for i := range u {
		if u[i].Cmp(v[i]) != 0 {
			return ErrConvergence
		}
	}
	k, e := surfaceTubeCofactorIntervals(ctx, a, c, b)
	if e != nil {
		return e
	}
	p, q := k[x].sign()*side[x], k[y].sign()*side[y]
	if p*q != -1 {
		return ErrConvergence
	}
	// Along the segment from the anchor to any other contact, the
	// average Jacobian remains within the same entrywise intervals.
	// Its one-dimensional kernel has opposite inward signs in x,y.
	// No nonzero displacement in that kernel fits both patch domains.
	return ctx.Err()
}
