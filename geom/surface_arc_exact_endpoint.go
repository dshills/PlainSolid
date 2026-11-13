package geom

import "math/big"

// ArcExactEndpointOptions limits rational endpoint recovery and verification.
// Zero limits select 1M operations and 16384 coefficient bits.
type ArcExactEndpointOptions struct{ MaxOperations, MaxCoefficientBits int }

// SurfaceArcExactEndpoint exposes caller-owned exact parameter/model values and
// outward display bounds. FreeParameter follows the arc's chosen dependent axis.
// The model point need not have a lossless float64 representation.
type SurfaceArcExactEndpoint struct {
	Parameters      [2]*big.Rat
	Point           [3]*big.Rat
	FreeParameter   *big.Rat
	ParameterBounds UVBox
	PointBounds     Box
	ExactOperations int
}

// ExactEndpoint recovers singleton, linear and rational quadratic roots, and
// tries bounded continued-fraction reconstruction for higher-degree roots. All
// proposals are checked exactly. Unrecovered roots remain ErrConvergence; their
// rounded midpoint is never promoted to an exact endpoint. A private boundary
// root identity and exact source evaluation are used, rather than public boxes.
func (a SurfacePlaneArc) ExactEndpoint(index int, o ArcExactEndpointOptions) (r SurfaceArcExactEndpoint, err error) {
	if a.source == nil || index < 0 || index > 1 {
		return r, ErrInvalid
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = budget.operations }()
	identity := a.Endpoints[index].identity
	if identity == nil || identity.axis < 0 || identity.axis > 1 || identity.fixed == nil || identity.lo == nil || identity.hi == nil {
		return r, ErrConvergence
	}
	var varying *big.Rat
	if identity.lo.Cmp(identity.hi) == 0 {
		varying = new(big.Rat).Set(identity.lo)
	} else {
		start := new(big.Rat).SetFloat64(identity.start)
		width := new(big.Rat).Sub(new(big.Rat).SetFloat64(identity.end), start)
		if width.Sign() <= 0 {
			return r, ErrConvergence
		}
		lo := new(big.Rat).Quo(new(big.Rat).Sub(identity.lo, start), width)
		hi := new(big.Rat).Quo(new(big.Rat).Sub(identity.hi, start), width)
		normalized, e := recoverRationalPolynomialRoot(identity.polynomial, lo, hi, budget)
		if e != nil {
			return r, e
		}
		if normalized == nil {
			return r, ErrConvergence
		}
		varying = parameterMap(ParameterSpan{Start: identity.start, End: identity.end}, normalized)
	}
	if varying.Cmp(identity.lo) < 0 || varying.Cmp(identity.hi) > 0 {
		return r, ErrConvergence
	}
	uv := [2]*big.Rat{}
	uv[identity.axis] = new(big.Rat).Set(identity.fixed)
	uv[1-identity.axis] = varying
	d := a.source.Domain()
	ranges := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	for axis, x := range uv {
		if e := budget.check(x); e != nil {
			return r, e
		}
		lo, hi := new(big.Rat).SetFloat64(ranges[axis][0]), new(big.Rat).SetFloat64(ranges[axis][1])
		if x.Cmp(lo) < 0 || x.Cmp(hi) > 0 {
			return r, ErrDomain
		}
		if axis == identity.axis && x.Cmp(lo) != 0 && x.Cmp(hi) != 0 {
			return r, ErrDomain
		}
	}
	exact, _, e := exactSurfaceJet(a.source, uv[0], uv[1], budget)
	if e != nil {
		return r, e
	}
	residual := exactPlaneHomogeneousValue(exactPlaneCoefficients(a.plane), exactHomogeneous{exact[0], exact[1], exact[2], big.NewRat(1, 1)})
	if e := budget.check(residual); e != nil {
		return r, e
	}
	if residual.Sign() != 0 {
		return r, ErrConvergence
	}
	for axis, x := range uv {
		r.Parameters[axis] = new(big.Rat).Set(x)
	}
	r.ParameterBounds = UVBox{Min: Vec2{rationalFloatBound(uv[0], true), rationalFloatBound(uv[1], true)}, Max: Vec2{rationalFloatBound(uv[0], false), rationalFloatBound(uv[1], false)}}
	fixed := a.Certificate.FixedParameterAxis
	if fixed < 0 || fixed > 1 {
		return r, ErrInvalid
	}
	r.FreeParameter = new(big.Rat).Set(uv[1-fixed])
	for axis, x := range exact {
		r.Point[axis] = new(big.Rat).Set(x)
		lo, hi := rationalFloatBound(x, true), rationalFloatBound(x, false)
		switch axis {
		case 0:
			r.PointBounds.Min.X, r.PointBounds.Max.X = lo, hi
		case 1:
			r.PointBounds.Min.Y, r.PointBounds.Max.Y = lo, hi
		case 2:
			r.PointBounds.Min.Z, r.PointBounds.Max.Z = lo, hi
		}
	}
	return r, nil
}
