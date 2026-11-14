package geom

import (
	"context"
	"math/big"
)

// NormalizeVerticalProjectiveChart changes only the independent parameter of
// an exact single-span vertical surface. If s=t/(k+(1-k)*t), its returned
// patch is the same model surface and image in parameter t. Original controls
// for transverse index i are multiplied by (1/k)^i. The requested k must
// be positive, and the resulting surface is proved to have an affine model
// projection by the caller. Use this only as a local display chart; native
// edges and model ownership retain their original parameters.
func (p *SurfacePatch) NormalizeVerticalProjectiveChart(ctx context.Context, axis int, k *big.Rat,
	o SurfacePatchOptions) (*SurfacePatch, int, error) {
	if ctx == nil || p == nil || axis < 0 || axis > 1 || k == nil || k.Sign() <= 0 || len(p.net) == 0 {
		return nil, 0, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, 0, e
	}
	if p.countU != p.degreeU+1 || p.countV != p.degreeV+1 {
		return nil, 0, ErrUnrepresentable
	}
	o, e := patchOptions(o)
	if e != nil {
		return nil, 0, e
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	if e := b.check(k); e != nil {
		return nil, b.operations, e
	}
	factor := new(big.Rat).Inv(k)
	count := p.countU
	if axis == 1 {
		count = p.countV
	}
	powers := make([]*big.Rat, count)
	powers[0] = big.NewRat(1, 1)
	for i := 1; i < count; i++ {
		powers[i] = new(big.Rat).Mul(powers[i-1], factor)
		if e := b.check(powers[i]); e != nil {
			return nil, b.operations, e
		}
	}
	countPoles := len(p.net)
	if countPoles > o.MaxPoles {
		return nil, b.operations, ErrConvergence
	}
	q := *p
	q.net = make([]exactHomogeneous, countPoles)
	q.options = o
	for i, h := range p.net {
		if e := ctx.Err(); e != nil {
			return nil, b.operations, e
		}
		j := i / p.countV
		if axis == 1 {
			j = i % p.countV
		}
		for a, x := range h {
			q.net[i][a] = new(big.Rat).Mul(x, powers[j])
			if e := b.check(q.net[i][a]); e != nil {
				return nil, b.operations, e
			}
		}
	}
	if e := q.computeBounds(b); e != nil {
		return nil, b.operations, e
	}
	q.operations = b.operations
	return &q, b.operations, nil
}

// VerticalProjectiveScale discovers a simple positive projective weight
// across one chart axis. Specifically the first and second Bernstein weights
// must be proportional by k, and every original tensor weight must be the
// same positive scalar times k^i. The exact cross-multiplication fixes
// both original parameter endpoints, reports no geometry, and admits no fit.
func (p *SurfacePatch) VerticalProjectiveScale(ctx context.Context, axis int,
	o SurfacePatchEvaluationOptions) (*big.Rat, int, error) {
	if ctx == nil || p == nil || axis < 0 || axis > 1 || len(p.net) == 0 {
		return nil, 0, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, 0, e
	}
	if p.countU != p.degreeU+1 || p.countV != p.degreeV+1 || p.countU < 2 || p.countV < 2 {
		return nil, 0, ErrUnrepresentable
	}
	l, e := patchOptions(SurfacePatchOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return nil, 0, e
	}
	b := &polynomialBudget{maxOperations: l.MaxOperations, maxBits: l.MaxCoefficientBits}
	stride := p.countV
	if axis == 1 {
		stride = 1
	}
	first := p.net[0][3]
	if first.Sign() <= 0 {
		return nil, b.operations, ErrInvalid
	}
	k := new(big.Rat).Quo(p.net[stride][3], first)
	if e := b.check(k); e != nil {
		return nil, b.operations, e
	}
	if k.Sign() <= 0 || k.Cmp(big.NewRat(1, 1)) == 0 {
		return nil, b.operations, ErrUnrepresentable
	}
	seen := map[int]*big.Rat{0: new(big.Rat).Set(first)}
	for i, h := range p.net {
		if e := ctx.Err(); e != nil {
			return nil, b.operations, e
		}
		x := i / p.countV
		if axis == 1 {
			x = i % p.countV
		}
		q := seen[x]
		if q == nil {
			q = new(big.Rat).Mul(seen[x-1], k)
			seen[x] = q
		}
		if e := b.check(q); e != nil {
			return nil, b.operations, e
		}
		if e := b.check(h[3]); e != nil {
			return nil, b.operations, e
		}
		if q.Cmp(h[3]) != 0 {
			return nil, b.operations, ErrUnrepresentable
		}
	}
	return k, b.operations, nil
}
