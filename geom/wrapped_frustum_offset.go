package geom

import (
	"context"
	"math/big"
)

// WrappedFrustumNormalOffset is the exact support of a constant Euclidean
// normal offset on a circular lateral frustum. Cap planes are outside this
// construction. Axial and radial shifts have squared length DistanceSquared.
type WrappedFrustumNormalOffset struct {
	Support                  *WrappedSurfacePatch
	RadiusShift, HeightShift *big.Rat
	DistanceSquared          *big.Rat
}

// OffsetWrappedCircularFrustum shifts the lateral frustum along its outward
// unit normals by the represented signed distance. The slope stays fixed,
// dr=distance/sqrt(1+slope²) and dz=-slope*dr. The current exact rational
// representation requires sqrt(1+slope²) to be rational. No rounded
// approximations or collapsed radii are accepted.
func OffsetWrappedCircularFrustum(ctx context.Context, w *WrappedSurfacePatch, distance float64,
	o SurfacePatchOptions) (WrappedFrustumNormalOffset, error) {
	var r WrappedFrustumNormalOffset
	if ctx == nil || !finite(distance) {
		return r, ErrInvalid
	}
	p, e := CertifyWrappedFrustumEmbedding(ctx, w, o)
	if e != nil {
		return r, e
	}
	if !p.Circular {
		return r, ErrConvergence
	}
	d := new(big.Rat).SetFloat64(distance)
	if d.Sign() == 0 {
		return WrappedFrustumNormalOffset{w, new(big.Rat), new(big.Rat), new(big.Rat)}, nil
	}
	s := new(big.Rat).Quo(new(big.Rat).Sub(p.UpperRadius, p.LowerRadius), new(big.Rat).Sub(p.UpperZ, p.LowerZ))
	normal2 := new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Mul(s, s))
	root := func(n *big.Int) (*big.Int, bool) {
		x := new(big.Int).Sqrt(n)
		return x, new(big.Int).Mul(x, x).Cmp(n) == 0
	}
	n, ok := root(normal2.Num())
	q, ok2 := root(normal2.Denom())
	if !ok || !ok2 {
		return r, ErrUnrepresentable
	}
	dr := new(big.Rat).Quo(d, new(big.Rat).SetFrac(n, q))
	dz := new(big.Rat).Neg(new(big.Rat).Mul(s, dr))
	w, _, e = DraftWrappedFrustumExact(ctx, w, dr, dr, o)
	if e != nil {
		return r, e
	}
	if dz.Sign() != 0 {
		// Shift exact height controls without changing the rational chart.
		o, _ = patchOptions(o)
		b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
		p := *w.patch
		p.options = o
		p.net = append([]exactHomogeneous(nil), p.net...)
		for i := range p.net {
			if e := ctx.Err(); e != nil {
				return r, e
			}
			h := &p.net[i]
			h[2] = new(big.Rat).Add(h[2], new(big.Rat).Mul(h[3], dz))
			if e := b.check(h[2]); e != nil {
				return r, e
			}
		}
		if e := p.computeBounds(b); e != nil {
			return r, e
		}
		left := o.MaxOperations - b.operations
		if left < 1 {
			return r, ErrConvergence
		}
		w, _, e = NewWrappedSurfacePatch(ctx, &p, WrappedSurfaceOptions{WrapU: true, ContinuityOrder: w.options.ContinuityOrder,
			MaxPoles: o.MaxPoles, MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits})
		if e != nil {
			return r, e
		}
	}
	return WrappedFrustumNormalOffset{w, dr, dz, new(big.Rat).Mul(d, d)}, nil
}
