package geom

import (
	"context"
	"math/big"
)

// DraftWrappedFrustum adds an exact radial change at either end of a
// canonical wrapped extrusion or frustum. Radius remains affine in height,
// keeping the original support basis, knot ranges and weights. The inputs
// are radial changes in model units, not angles. Neither end may collapse.
func DraftWrappedFrustum(ctx context.Context, w *WrappedSurfacePatch, lowerChange, upperChange float64, o SurfacePatchOptions) (*WrappedSurfacePatch, WrappedFrustumEmbedding, error) {
	if !finite(lowerChange) || !finite(upperChange) {
		return nil, WrappedFrustumEmbedding{}, ErrInvalid
	}
	return DraftWrappedFrustumExact(ctx, w, new(big.Rat).SetFloat64(lowerChange), new(big.Rat).SetFloat64(upperChange), o)
}

// DraftWrappedFrustumExact accepts exact rational endpoint changes. Its
// homogeneous support remains exact even if the resulting model vertices
// cannot yet be represented by a trimmed solid.
func DraftWrappedFrustumExact(ctx context.Context, w *WrappedSurfacePatch, lowerChange, upperChange *big.Rat, o SurfacePatchOptions) (*WrappedSurfacePatch, WrappedFrustumEmbedding, error) {
	var zero WrappedFrustumEmbedding
	if ctx == nil || lowerChange == nil || upperChange == nil {
		return nil, zero, ErrInvalid
	}
	proof, e := CertifyWrappedFrustumEmbedding(ctx, w, o)
	if e != nil {
		return nil, zero, e
	}
	o, _ = patchOptions(o)
	b := &polynomialBudget{operations: proof.ExactOperations, maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	delta := [2]*big.Rat{lowerChange, upperChange}
	for _, r := range delta {
		if e = b.check(r); e != nil {
			return nil, zero, e
		}
	}
	radii := [2]*big.Rat{new(big.Rat).Add(proof.LowerRadius, delta[0]), new(big.Rat).Add(proof.UpperRadius, delta[1])}
	for _, r := range radii {
		if e = b.check(r); e != nil {
			return nil, zero, e
		}
		if r.Sign() <= 0 {
			return nil, zero, ErrDegenerate
		}
	}
	if lowerChange.Sign() == 0 && upperChange.Sign() == 0 {
		return w, proof, nil
	}
	p := *w.patch
	p.options = o
	p.net = append([]exactHomogeneous(nil), p.net...)
	q := [9][2]int64{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}}
	for i, xy := range q {
		if e = ctx.Err(); e != nil {
			return nil, zero, e
		}
		for j := range 2 {
			h := &p.net[2*i+j]
			for axis, x := range xy {
				d := new(big.Rat).Mul(delta[j], big.NewRat(x, 1))
				h[axis] = new(big.Rat).Add(h[axis], d.Mul(d, h[3]))
				if e = b.check(h[axis]); e != nil {
					return nil, zero, e
				}
			}
		}
	}
	if e = p.computeBounds(b); e != nil {
		return nil, zero, e
	}
	remaining := o.MaxOperations - b.operations
	if remaining < 1 {
		return nil, zero, ErrConvergence
	}
	p.operations = b.operations
	out, seam, e := NewWrappedSurfacePatch(ctx, &p, WrappedSurfaceOptions{
		WrapU: true, ContinuityOrder: w.options.ContinuityOrder, MaxOperations: remaining,
		MaxPoles: o.MaxPoles, MaxCoefficientBits: o.MaxCoefficientBits,
	})
	if e != nil {
		return nil, zero, e
	}
	proof.LowerRadius, proof.UpperRadius = radii[0], radii[1]
	proof.ExactOperations = b.operations + seam.ExactOperations
	return out, proof, nil
}
