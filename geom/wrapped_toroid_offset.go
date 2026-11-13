package geom

import (
	"context"
	"math/big"
)

// OffsetWrappedCircularToroid applies a constant Euclidean normal offset to
// the canonical wrapped toroid when both profile curves are exact circles
// and the axial and radial minor radii agree. A positive distance expands
// the tube, a negative distance erodes it. Distances that collapse the tube
// or close the central hole are rejected. The exact original chart speeds,
// knots, weights and seam order are preserved.
func OffsetWrappedCircularToroid(ctx context.Context, w *WrappedSurfacePatch, distance float64, o SurfacePatchOptions) (*WrappedSurfacePatch, WrappedToroidEmbedding, error) {
	var zero WrappedToroidEmbedding
	if ctx == nil || !finite(distance) {
		return nil, zero, ErrInvalid
	}
	proof, e := CertifyWrappedToroidEmbedding(ctx, w, o)
	if e != nil {
		return nil, zero, e
	}
	if proof.RadialMinorRadius.Cmp(proof.AxialMinorRadius) != 0 {
		return nil, zero, ErrConvergence
	}
	o, _ = patchOptions(o)
	p := *w.patch
	b := &polynomialBudget{operations: proof.ExactOperations, maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	for _, stride := range []int{1, 9} {
		for j := 0; j < 8; j += 2 {
			a, c, d := p.net[j*stride][3], p.net[(j+1)*stride][3], p.net[(j+2)*stride][3]
			left := new(big.Rat).Mul(a, d)
			right := new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(c, c))
			for _, x := range []*big.Rat{left, right} {
				if e = b.check(x); e != nil {
					return nil, zero, e
				}
			}
			if left.Cmp(right) != 0 {
				return nil, zero, ErrConvergence
			}
		}
	}
	if distance == 0 {
		return w, proof, nil
	}
	delta := new(big.Rat).SetFloat64(distance)
	radius := new(big.Rat).Add(proof.RadialMinorRadius, delta)
	if e = b.check(radius); e != nil {
		return nil, zero, e
	}
	if radius.Sign() <= 0 || radius.Cmp(proof.MajorRadius) >= 0 {
		return nil, zero, ErrDegenerate
	}
	q := [9][2]int64{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}}
	p.net = make([]exactHomogeneous, len(w.patch.net))
	p.options = o
	for i, a := range q {
		if e = ctx.Err(); e != nil {
			return nil, zero, e
		}
		for j, c := range q {
			k := 9*i + j
			h := w.patch.net[k]
			p.net[k] = h
			for k2, v := range [3]int64{a[0] * c[0], a[1] * c[0], c[1]} {
				d := new(big.Rat).Mul(delta, big.NewRat(v, 1))
				p.net[k][k2] = new(big.Rat).Add(h[k2], d.Mul(d, h[3]))
				if e = b.check(p.net[k][k2]); e != nil {
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
	result, seam, e := NewWrappedSurfacePatch(ctx, &p, WrappedSurfaceOptions{
		WrapU: true, WrapV: true, ContinuityOrder: w.options.ContinuityOrder,
		MaxPoles: o.MaxPoles, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits,
	})
	if e != nil {
		return nil, zero, e
	}
	proof.ExactOperations = b.operations + seam.ExactOperations
	proof.RadialMinorRadius, proof.AxialMinorRadius = radius, new(big.Rat).Set(radius)
	return result, proof, nil
}
