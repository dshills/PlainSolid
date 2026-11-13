package geom

import (
	"context"
	"math/big"
)

// SplitWrappedFrustumAtZ exactly partitions a canonical positive-radius
// frustum. The two new supports retain original U and use V=[0,1].
// Their common homogeneous rim is identical even at nonbinary original
// V or with nonrepresentable model controls.
func SplitWrappedFrustumAtZ(ctx context.Context, w *WrappedSurfacePatch, z float64, o SurfacePatchOptions) (*WrappedSurfacePatch, *WrappedSurfacePatch, error) {
	if ctx == nil || !finite(z) {
		return nil, nil, ErrInvalid
	}
	p, e := CertifyWrappedFrustumEmbedding(ctx, w, o)
	if e != nil {
		return nil, nil, e
	}
	o, _ = patchOptions(o)
	level := new(big.Rat).SetFloat64(z)
	if level.Cmp(p.LowerZ) <= 0 || level.Cmp(p.UpperZ) >= 0 {
		return nil, nil, ErrDomain
	}
	t := new(big.Rat).Quo(new(big.Rat).Sub(level, p.LowerZ), new(big.Rat).Sub(p.UpperZ, p.LowerZ))
	s := new(big.Rat).Sub(big.NewRat(1, 1), t)
	b := &polynomialBudget{operations: p.ExactOperations, maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	if e = b.check(t); e != nil {
		return nil, nil, e
	}
	patches := [2]SurfacePatch{*w.patch, *w.patch}
	for i := range patches {
		patches[i].knotsV = []float64{0, 0, 1, 1}
		patches[i].options = o
		patches[i].net = append([]exactHomogeneous(nil), w.patch.net...)
	}
	for i := 0; i < len(w.patch.net); i += 2 {
		if e = ctx.Err(); e != nil {
			return nil, nil, e
		}
		var h exactHomogeneous
		for j := range h {
			h[j] = new(big.Rat).Add(new(big.Rat).Mul(w.patch.net[i][j], s), new(big.Rat).Mul(w.patch.net[i+1][j], t))
			if e = b.check(h[j]); e != nil {
				return nil, nil, e
			}
		}
		patches[0].net[i+1], patches[1].net[i] = h, h
	}
	var out [2]*WrappedSurfacePatch
	for i := range out {
		if e = patches[i].computeBounds(b); e != nil {
			return nil, nil, e
		}
		remaining := o.MaxOperations - b.operations
		if remaining < 1 {
			return nil, nil, ErrConvergence
		}
		patches[i].operations = b.operations
		var proof SurfaceSeamCertificate
		out[i], proof, e = NewWrappedSurfacePatch(ctx, &patches[i], WrappedSurfaceOptions{
			WrapU: true, ContinuityOrder: w.options.ContinuityOrder,
			MaxPoles: o.MaxPoles, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits,
		})
		if e != nil {
			return nil, nil, e
		}
		b.operations += proof.ExactOperations
	}
	return out[0], out[1], nil
}
