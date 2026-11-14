package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// AlgebraicGraphCapNearest is the exact squared distance to the filled cap.
// FaceInterior indicates its orthogonal projection lies strictly in material.
// Otherwise Wire/Use identify one nearest boundary owner if known; multiple
// equally near owners can exist.
type AlgebraicGraphCapNearest struct {
	SquaredDistance *geom.ExactAlgebraicRoot
	CurveParameter  *geom.ExactAlgebraicRoot
	Wire, Use       int
	FaceInterior    bool
}

// NearestPolynomialRational considers the orthogonal projection and all
// exact domain endpoints and stationary points on polynomial curves of model
// degree at most two. A query nearest to the filled interior directly proves
// the rational plane distance. Unsupported curve degrees or weights fail
// atomically when a boundary search is required.
func (c AlgebraicGraphCapClip) NearestPolynomialRational(ctx context.Context, point [3]*big.Rat, options geom.ExactCurvePlaneEventOptions) (AlgebraicGraphCapNearest, error) {
	var zero AlgebraicGraphCapNearest
	if ctx == nil || c.regions == nil || options.MaxOperations < 0 || options.MaxSubdivisions < 0 || options.MaxCoefficientBits < 0 {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if options.MaxOperations == 0 {
		options.MaxOperations = 2000000
	}
	if options.MaxSubdivisions == 0 {
		options.MaxSubdivisions = 8192
	}
	if options.MaxCoefficientBits == 0 {
		options.MaxCoefficientBits = 16384
	}
	if options.MaxCoefficientBits < 64 {
		return zero, geom.ErrInvalid
	}
	b := capClipBudget{ctx, options.MaxOperations, options.MaxSubdivisions, options.MaxCoefficientBits}
	for _, p := range point {
		if p == nil {
			return zero, geom.ErrInvalid
		}
		if p.Num().BitLen() > b.bits || p.Denom().BitLen() > b.bits {
			return zero, geom.ErrConvergence
		}
	}
	plane := c.regions.plane
	norm := new(big.Rat)
	for _, n := range plane.normal {
		norm.Add(norm, new(big.Rat).Mul(n, n))
	}
	delta := new(big.Rat).Sub(plane.dot(point), plane.offset)
	displacement := new(big.Rat).Quo(delta, norm)
	var projection [3]*big.Rat
	for i := range projection {
		projection[i] = new(big.Rat).Sub(point[i], new(big.Rat).Mul(displacement, plane.normal[i]))
	}
	where, e := c.LocateRational(ctx, projection)
	if e != nil {
		return zero, e
	}
	if where == trimvalidate.Inside {
		exact := new(big.Rat).Mul(delta, displacement)
		d, _, e := geom.NewExactRationalRoot(ctx, exact, b.limits())
		if e != nil {
			return zero, e
		}
		return AlgebraicGraphCapNearest{SquaredDistance: d, Wire: -1, Use: -1, FaceInterior: true}, nil
	}
	out := AlgebraicGraphCapNearest{Wire: -1, Use: -1}
	for i, wire := range c.wires {
		for j, u := range wire {
			set, e := u.Cell.PolynomialDistanceCandidates(ctx, point, b.limits())
			if e != nil {
				return zero, e
			}
			if e := b.charge(set.Operations, set.Subdivisions); e != nil {
				return zero, e
			}
			for _, p := range set.Candidates {
				if out.SquaredDistance != nil {
					order, e := p.SquaredDistance.Compare(ctx, out.SquaredDistance, b.limits())
					if e != nil {
						return zero, e
					}
					if e := b.charge(order.Operations, order.Subdivisions); e != nil {
						return zero, e
					}
					if !order.Proven {
						return zero, geom.ErrConvergence
					}
					if order.Order >= 0 {
						continue
					}
				}
				out = AlgebraicGraphCapNearest{SquaredDistance: p.SquaredDistance, CurveParameter: p.Parameter, Wire: i, Use: j}
			}
		}
	}
	if out.SquaredDistance == nil {
		return zero, geom.ErrDomain
	}
	return out, nil
}
