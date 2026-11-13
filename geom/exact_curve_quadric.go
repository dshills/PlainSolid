package geom

import (
	"context"
	"math/big"
)

// ExactQuadric stores coefficients of xx,yy,zz,xy,xz,yz,x,y,z,1, in that order.
// Cross terms appear once (not twice). Nil coefficients mean zero. An optional
// support plane is supplied separately for implicit planar conic intersection.
type ExactQuadric [10]*big.Rat

// ExactCurveQuadricOptions adds an optional exact plane Nx*x+Ny*y+Nz*z+D=0.
// A zero plane normal or identically zero quadric is invalid. No conic-arc
// trimming is inferred: this operation intersects the entire implicit locus
// against the bounded input curve domain.
type ExactCurveQuadricOptions struct {
	ExactCurveSegmentOptions
	Plane *[4]*big.Rat
}

// ExactCurveQuadricHit encloses a proved implicit-locus contact. Parameter and
// Point are populated only for rationally recovered roots. Multiplicity is the
// common residual vanishing order (GCD with Plane, when supplied).
// RationalDiagnostics marks exact first-derivative diagnostics. Transverse and
// Tangent refer to the quadric itself; they do not assert a bounded conic-arc
// crossing. Tangent requires nonzero curve velocity and quadric gradient.
type ExactCurveQuadricHit struct {
	Lower, Upper, Parameter                                                  *big.Rat
	Point                                                                    [3]*big.Rat
	Span                                                                     ParameterSpan
	Multiplicity                                                             int
	RationalDiagnostics, CurveSingular, QuadricSingular, Transverse, Tangent bool
}

// ExactCurveQuadricIntersection is exhaustive when Complete, including proved
// coincident spans and algebraic isolated points. At knots each incident span
// reports its own multiplicity and one-sided differential diagnostics.
type ExactCurveQuadricIntersection struct {
	Hits                     []ExactCurveQuadricHit
	Coincident               []ParameterSpan
	Complete                 bool
	Operations, Subdivisions int
}

// IntersectExactCurveQuadric substitutes each exact homogeneous Bézier span
// into Q(X/W), clears its positive W² denominator, and isolates all roots.
// When Plane is supplied, an exact polynomial GCD enforces both equations.
// Algebraic brackets are actual proved contacts, not numerical candidates.
func IntersectExactCurveQuadric(ctx context.Context, c *ExactSplineCurve, q ExactQuadric, o ExactCurveQuadricOptions) (r ExactCurveQuadricIntersection, err error) {
	if ctx == nil || c == nil || c.patch == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.ParameterTolerance == 0 {
		o.ParameterTolerance = 1e-10
	}
	if o.MaxDegree == 0 {
		o.MaxDegree = 12
	}
	if o.MaxControlPoints == 0 {
		o.MaxControlPoints = 4096
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 2000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 8192
	}
	if !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 || o.MaxDegree < 1 || o.MaxDegree > 64 || o.MaxControlPoints < 2 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxSubdivisions < 1 {
		return r, ErrInvalid
	}
	if c.patch.degreeU > o.MaxDegree || c.patch.countU > o.MaxControlPoints {
		return r, ErrConvergence
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.Operations = b.operations }()
	var coefficient [10]*big.Rat
	nonzero := false
	for i, x := range q {
		coefficient[i] = new(big.Rat)
		if x != nil {
			coefficient[i].Set(x)
		}
		if e := b.check(coefficient[i]); e != nil {
			return r, e
		}
		nonzero = nonzero || coefficient[i].Sign() != 0
	}
	if !nonzero {
		return r, ErrInvalid
	}
	var plane [4]*big.Rat
	if o.Plane != nil {
		normal := false
		for i, x := range *o.Plane {
			plane[i] = new(big.Rat)
			if x != nil {
				plane[i].Set(x)
			}
			if e := b.check(plane[i]); e != nil {
				return r, e
			}
			if i < 3 {
				normal = normal || plane[i].Sign() != 0
			}
		}
		if !normal {
			return r, ErrInvalid
		}
	}
	pairs := [10][2]int{{0, 0}, {1, 1}, {2, 2}, {0, 1}, {0, 2}, {1, 2}, {0, 3}, {1, 3}, {2, 3}, {3, 3}}
	stack := []*SurfacePatch{c.patch}
	index := 0
	for len(stack) > 0 {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if knot, ok := internalPatchKnot(p.knotsU, p.degreeU, p.countU); ok {
			l, h, e := p.split(knot, true, b)
			if e != nil {
				return r, e
			}
			stack = append(stack, h, l)
			continue
		}
		d := p.Domain()
		span := ParameterSpan{Start: d.UMin, End: d.UMax, Index: index}
		index++
		fields, e := sharedProjectionPowers(p, b)
		if e != nil {
			return r, e
		}
		var residual rationalPolynomial
		for term, pair := range pairs {
			if coefficient[term].Sign() == 0 {
				continue
			}
			product, e := polynomialProduct(fields[pair[0]], fields[pair[1]], b)
			if e != nil {
				return r, e
			}
			residual, e = addScaledQuadricPolynomial(residual, product, coefficient[term], b)
			if e != nil {
				return r, e
			}
		}
		common := trimPolynomial(residual)
		if o.Plane != nil {
			var planeResidual rationalPolynomial
			for axis := 0; axis < 4; axis++ {
				planeResidual, e = addScaledQuadricPolynomial(planeResidual, fields[axis], plane[axis], b)
				if e != nil {
					return r, e
				}
			}
			common, e = polynomialGCD(common, trimPolynomial(planeResidual), b)
			if e != nil {
				return r, e
			}
		}
		if len(common) == 0 {
			r.Coincident = append(r.Coincident, span)
			continue
		}
		if len(common) == 1 {
			continue
		}
		remaining := o.MaxSubdivisions - r.Subdivisions
		if remaining < 1 {
			return r, ErrConvergence
		}
		n, e := isolatePolynomialRoots(common, new(big.Rat), big.NewRat(1, 1), b, remaining, func(lo, hi *big.Rat, m int) (bool, error) {
			if e := ctx.Err(); e != nil {
				return false, e
			}
			candidate, e := recoverRationalPolynomialRoot(common, lo, hi, b)
			if e != nil {
				return false, e
			}
			lower, upper := parameterMap(span, lo), parameterMap(span, hi)
			if candidate == nil && new(big.Rat).Sub(upper, lower).Cmp(new(big.Rat).SetFloat64(o.ParameterTolerance)) > 0 {
				return false, nil
			}
			hit := ExactCurveQuadricHit{Lower: lower, Upper: upper, Span: span, Multiplicity: m}
			if candidate != nil {
				parameter := parameterMap(span, candidate)
				point, jet, e := exactSurfaceJet(p, parameter, new(big.Rat), b)
				if e != nil {
					return false, e
				}
				hit.Parameter = parameter
				hit.Point = point
				hit.Lower = new(big.Rat).Set(parameter)
				hit.Upper = new(big.Rat).Set(parameter)
				gradient := [3]*big.Rat{}
				for axis := 0; axis < 3; axis++ {
					gradient[axis] = new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(coefficient[axis], point[axis])), coefficient[axis+6])
				}
				for term, pair := range pairs[3:6] {
					i, j := pair[0], pair[1]
					gradient[i].Add(gradient[i], new(big.Rat).Mul(coefficient[term+3], point[j]))
					gradient[j].Add(gradient[j], new(big.Rat).Mul(coefficient[term+3], point[i]))
				}
				derivative := new(big.Rat)
				hit.CurveSingular = true
				hit.QuadricSingular = true
				for axis := 0; axis < 3; axis++ {
					if e := b.check(gradient[axis]); e != nil {
						return false, e
					}
					hit.CurveSingular = hit.CurveSingular && jet[axis][0].Sign() == 0
					hit.QuadricSingular = hit.QuadricSingular && gradient[axis].Sign() == 0
					derivative.Add(derivative, new(big.Rat).Mul(gradient[axis], jet[axis][0]))
				}
				if e := b.check(derivative); e != nil {
					return false, e
				}
				hit.RationalDiagnostics = true
				hit.Transverse = derivative.Sign() != 0
				hit.Tangent = !hit.CurveSingular && !hit.QuadricSingular && derivative.Sign() == 0
			}
			r.Hits = append(r.Hits, hit)
			return true, nil
		})
		r.Subdivisions += n
		if e != nil {
			return r, e
		}
	}
	r.Complete = true
	return r, nil
}
func addScaledQuadricPolynomial(a, c rationalPolynomial, scale *big.Rat, b *polynomialBudget) (rationalPolynomial, error) {
	out := make(rationalPolynomial, max(len(a), len(c)))
	for i := range out {
		out[i] = new(big.Rat)
		if i < len(a) {
			out[i].Set(a[i])
		}
		if i < len(c) {
			out[i].Add(out[i], new(big.Rat).Mul(scale, c[i]))
		}
		if e := b.check(out[i]); e != nil {
			return nil, e
		}
	}
	return trimPolynomial(out), nil
}
