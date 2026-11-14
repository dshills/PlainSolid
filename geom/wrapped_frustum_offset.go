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

// ErodeWrappedCircularFrustum constructs the lateral support of the exact
// ball erosion of a closed circular frustum with flat end caps. Distance is
// a nonnegative radius. The cap planes move inward by distance; the side's
// meridian supporting line moves inward by the same Euclidean distance.
// Only rational normal lengths and strictly positive retained end radii are
// supported. A pole or vanishing closed solid returns a domain error.
func ErodeWrappedCircularFrustum(ctx context.Context, w *WrappedSurfacePatch, distance float64,
	o SurfacePatchOptions) (*WrappedSurfacePatch, WrappedFrustumEmbedding, error) {
	var empty WrappedFrustumEmbedding
	if ctx == nil || !finite(distance) || distance < 0 {
		return nil, empty, ErrInvalid
	}
	proof, e := CertifyWrappedFrustumEmbedding(ctx, w, o)
	if e != nil {
		return nil, empty, e
	}
	if !proof.Circular {
		return nil, empty, ErrConvergence
	}
	if distance == 0 {
		return w, proof, nil
	}
	o, _ = patchOptions(o)
	b := &polynomialBudget{operations: proof.ExactOperations, maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	d := new(big.Rat).SetFloat64(distance)
	lo := new(big.Rat).Add(proof.LowerZ, d)
	hi := new(big.Rat).Sub(proof.UpperZ, d)
	if lo.Cmp(hi) >= 0 {
		return nil, empty, ErrDomain
	}
	slope := new(big.Rat).Quo(new(big.Rat).Sub(proof.UpperRadius, proof.LowerRadius), new(big.Rat).Sub(proof.UpperZ, proof.LowerZ))
	length, e := rationalFrustumNormalLength(slope)
	if e != nil {
		return nil, empty, e
	}
	move := new(big.Rat).Mul(d, length)
	// r(z) <= r0 + s(z−z0) becomes r(z) <= r0 + s(z−z0) − d√(1+s²).
	// Intersecting these offset supporting halfspaces over all azimuths
	// and the two translated cap halfspaces is exactly ball erosion.
	radii := [2]*big.Rat{
		new(big.Rat).Sub(new(big.Rat).Add(proof.LowerRadius, new(big.Rat).Mul(slope, d)), move),
		new(big.Rat).Sub(new(big.Rat).Sub(proof.UpperRadius, new(big.Rat).Mul(slope, d)), move),
	}
	if radii[0].Sign() <= 0 || radii[1].Sign() <= 0 {
		return nil, empty, ErrDomain
	}
	for _, x := range []*big.Rat{lo, hi, radii[0], radii[1]} {
		if e := b.check(x); e != nil {
			return nil, empty, e
		}
	}
	p := *w.patch
	p.options = o
	p.net = append([]exactHomogeneous(nil), p.net...)
	q := [9][2]int64{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}}
	for i, xy := range q {
		if e := ctx.Err(); e != nil {
			return nil, empty, e
		}
		for j, z := range [2]*big.Rat{lo, hi} {
			h := &p.net[2*i+j]
			for axis, center := range [2]*big.Rat{proof.CenterX, proof.CenterY} {
				x := new(big.Rat).Add(center, new(big.Rat).Mul(radii[j], big.NewRat(xy[axis], 1)))
				h[axis] = x.Mul(x, h[3])
				if e := b.check(h[axis]); e != nil {
					return nil, empty, e
				}
			}
			h[2] = new(big.Rat).Mul(z, h[3])
			if e := b.check(h[2]); e != nil {
				return nil, empty, e
			}
		}
	}
	if e := p.computeBounds(b); e != nil {
		return nil, empty, e
	}
	remaining := o.MaxOperations - b.operations
	if remaining < 1 {
		return nil, empty, ErrConvergence
	}
	p.operations = b.operations
	result, seam, e := NewWrappedSurfacePatch(ctx, &p, WrappedSurfaceOptions{WrapU: true, ContinuityOrder: w.options.ContinuityOrder,
		MaxPoles: o.MaxPoles, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return nil, empty, e
	}
	proof.LowerZ, proof.UpperZ, proof.LowerRadius, proof.UpperRadius = lo, hi, radii[0], radii[1]
	proof.ExactOperations = b.operations + seam.ExactOperations
	return result, proof, nil
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
	length, e := rationalFrustumNormalLength(s)
	if e != nil {
		return r, e
	}
	dr := new(big.Rat).Quo(d, length)
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

func rationalFrustumNormalLength(s *big.Rat) (*big.Rat, error) {
	z := new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Mul(s, s))
	n, d := new(big.Int).Sqrt(z.Num()), new(big.Int).Sqrt(z.Denom())
	if new(big.Int).Mul(n, n).Cmp(z.Num()) != 0 || new(big.Int).Mul(d, d).Cmp(z.Denom()) != 0 {
		return nil, ErrUnrepresentable
	}
	return new(big.Rat).SetFrac(n, d), nil
}
